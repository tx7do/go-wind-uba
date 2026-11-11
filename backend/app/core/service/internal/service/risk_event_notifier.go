package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"go-wind-uba/pkg/topic"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// RiskEventNotifier 订阅 Kafka 的风险事件流，把高风险事件推送给匹配的网络钩子。
// 以 kratos transport.Server 形态接入应用生命周期；broker 不可用时静默降级为空转。
type RiskEventNotifier struct {
	log *log.Helper

	broker        broker.Broker
	webhookSender *WebhookSender

	mu         sync.Mutex
	subscriber broker.Subscriber
}

func NewRiskEventNotifier(
	ctx *bootstrap.Context,
	kafkaBroker broker.Broker,
	webhookSender *WebhookSender,
) *RiskEventNotifier {
	return &RiskEventNotifier{
		log:           ctx.NewLoggerHelper("risk-event-notifier/service/core-service"),
		broker:        kafkaBroker,
		webhookSender: webhookSender,
	}
}

// Start 实现 transport.Server：订阅风险事件主题。
func (s *RiskEventNotifier) Start(_ context.Context) error {
	if s.broker == nil {
		return nil
	}

	sub, err := broker.Subscribe(
		s.broker,
		topic.UbaEventRisk,
		s.onRiskEvent,
		broker.WithSubscribeQueueName(topic.UbaGroupAlertSender),
	)
	if err != nil {
		// 订阅失败不阻断启动：风险事件的采集与入湖不受影响。
		s.log.Errorf("subscribe risk events failed: %s — webhook delivery for risk events is disabled", err.Error())
		return nil
	}

	s.mu.Lock()
	s.subscriber = sub
	s.mu.Unlock()

	return nil
}

// Stop 实现 transport.Server：退订并断开。
func (s *RiskEventNotifier) Stop(_ context.Context) error {
	s.mu.Lock()
	sub := s.subscriber
	s.subscriber = nil
	s.mu.Unlock()

	if sub == nil {
		return nil
	}
	if err := sub.Unsubscribe(false); err != nil {
		s.log.Errorf("unsubscribe risk events failed: %s", err.Error())
	}
	return nil
}

// onRiskEvent 风险事件 → 匹配的网络钩子（事件类型为 risk.<level>，如 risk.high）。
func (s *RiskEventNotifier) onRiskEvent(ctx context.Context, _ string, _ broker.Headers, event *ubaV1.RiskEvent) error {
	if event == nil {
		return nil
	}

	level := strings.ToLower(event.GetRiskLevel())
	if level == "" || level == "unknown" || level == "info" {
		// 低价值级别不推送，避免 webhook 被刷屏。
		return nil
	}

	// 应用维度不传：webhook 的应用过滤由其自身配置决定。
	s.webhookSender.Send(ctx, SendPayload{
		TenantID:  event.GetTenantId(),
		AppID:     "",
		EventType: "risk." + level,
		Data:      protoMessageToMap(event),
	})

	return nil
}

// protoMessageToMap 把 proto 消息转为可 JSON 序列化的 map（投递 payload 的 data 字段）。
func protoMessageToMap(msg proto.Message) map[string]any {
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return map[string]any{"id": prototext.Format(msg)}
	}
	ret := map[string]any{}
	if err := json.Unmarshal(raw, &ret); err != nil {
		return map[string]any{"id": prototext.Format(msg)}
	}
	return ret
}

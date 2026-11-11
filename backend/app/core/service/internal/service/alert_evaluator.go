package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	"go-wind-uba/app/core/service/internal/data/ent"

	internalMessageV1 "go-wind-uba/api/gen/go/internal_message/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	appViewer "go-wind-uba/pkg/entgo/viewer"
)

const (
	alertEvaluateInterval = time.Minute
	alertEvaluateLockKey  = "uba:lock:alert-evaluator"
	alertEvaluateLockTTL  = 55 * time.Second

	// alertEventType 告警触发的 webhook 事件类型，与 Webhook.event_types 配对。
	alertEventType = "alert.triggered"
)

// alertInternalMessageCategory 站内信分类 ID；0 表示未分类。
// 用变量而非常量：消息创建需要可取地址的分类指针。
var alertInternalMessageCategory int64 = 0

// trendAnalytics 告警评估所需的 OLAP 查询能力（Doris/ClickHouse 双实现共用签名）。
type trendAnalytics interface {
	EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error)
}

// AlertEvaluator 指标阈值告警与分群定时刷新的后台评估器。
// 以 kratos transport.Server 形态接入应用生命周期：Start 启动滴答循环，Stop 优雅退出。
type AlertEvaluator struct {
	log *log.Helper

	alertRepo       *data.AlertRuleRepo
	internalMsgRepo *data.InternalMessageRepo
	recipientRepo   *data.InternalMessageRecipientRepo
	segmentService  *SegmentService
	webhookSender   *WebhookSender
	redis           *redis.Client

	trend   trendAnalytics
	chTrend trendAnalytics

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewAlertEvaluator(
	ctx *bootstrap.Context,
	alertRepo *data.AlertRuleRepo,
	internalMsgRepo *data.InternalMessageRepo,
	recipientRepo *data.InternalMessageRecipientRepo,
	segmentService *SegmentService,
	webhookSender *WebhookSender,
	redisClient *redis.Client,
	analytics *dorisData.AnalyticsRepo,
	chAnalytics *clickhouse.AnalyticsRepo,
) *AlertEvaluator {
	return &AlertEvaluator{
		log:             ctx.NewLoggerHelper("alert-evaluator/service/core-service"),
		alertRepo:       alertRepo,
		internalMsgRepo: internalMsgRepo,
		recipientRepo:   recipientRepo,
		segmentService:  segmentService,
		webhookSender:   webhookSender,
		redis:           redisClient,
		trend:           analytics,
		chTrend:         chAnalytics,
	}
}

// trendImpl 按 OLAP 引擎选择返回可用的趋势查询实现。
func (s *AlertEvaluator) trendImpl() trendAnalytics {
	if data.UseClickHouse {
		return s.chTrend
	}
	return s.trend
}

// Start 实现 transport.Server：启动评估循环。
func (s *AlertEvaluator) Start(ctx context.Context) error {
	// 后台评估绕过租户隐私层：以系统上下文执行（与 asynq StartAllTask 同模式）
	sysCtx := appViewer.NewSystemViewerContext(ctx)
	runCtx, cancel := context.WithCancel(sysCtx)
	s.cancel = cancel

	s.wg.Add(1)
	go s.loop(runCtx)

	return nil
}

// Stop 实现 transport.Server：停止评估循环。
func (s *AlertEvaluator) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *AlertEvaluator) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(alertEvaluateInterval)
	defer ticker.Stop()

	// 启动后先跑一轮，避免依赖首个整分钟。
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick 单轮评估：分布式锁防多实例并发。
func (s *AlertEvaluator) tick(ctx context.Context) {
	if s.redis != nil {
		acquired, err := s.redis.SetNX(ctx, alertEvaluateLockKey, time.Now().UnixMilli(), alertEvaluateLockTTL).Result()
		if err != nil {
			s.log.Errorf("acquire evaluator lock failed: %s", err.Error())
			return
		}
		if !acquired {
			return
		}
	}

	s.evaluateAlertRules(ctx)
	s.refreshDueSegments(ctx)
}

// evaluateAlertRules 评估全部到期规则并触发通知。
func (s *AlertEvaluator) evaluateAlertRules(ctx context.Context) {
	rules, err := s.alertRepo.ListDueRules(ctx, time.Now())
	if err != nil {
		s.log.Errorf("list due alert rules failed: %s", err.Error())
		return
	}

	now := time.Now()
	for _, rule := range rules {
		value, hit, err := s.evaluateRule(ctx, rule, now)
		if err != nil {
			s.log.Errorf("evaluate alert rule [%d] failed: %s", rule.ID, err.Error())
			continue
		}
		if !hit {
			continue
		}

		s.notifyTriggered(ctx, rule, value, now)
		if err = s.alertRepo.UpdateTriggerState(ctx, rule.ID, now); err != nil {
			s.log.Errorf("record alert rule trigger state failed: %s", err.Error())
		}
	}
}

// evaluateRule 计算单条规则的窗口指标并判断是否越过阈值。
func (s *AlertEvaluator) evaluateRule(ctx context.Context, rule *ent.AlertRule, now time.Time) (float64, bool, error) {
	window := time.Duration(rule.WindowMinutes) * time.Minute
	if window <= 0 {
		window = time.Hour
	}

	granularity := ubaV1.AnalyticsGranularity_HOUR
	if rule.WindowMinutes >= 24*60 {
		granularity = ubaV1.AnalyticsGranularity_DAY
	}

	req := &ubaV1.EventTrendRequest{
		TimeRange: &ubaV1.TimeRange{
			StartMs: now.Add(-window).UnixMilli(),
			EndMs:   now.UnixMilli(),
		},
		Granularity: granularity,
	}
	if rule.AppID != nil && *rule.AppID != 0 {
		appID := *rule.AppID
		req.AppId = &appID
	}
	if rule.EventName != nil && *rule.EventName != "" {
		req.EventName = rule.EventName
	}

	resp, err := s.trendImpl().EventTrend(ctx, req)
	if err != nil {
		return 0, false, err
	}

	value := float64(resp.GetTotal())
	return value, compareThreshold(value, rule.Comparison, rule.Threshold), nil
}

// compareThreshold 按 GT/GTE/LT/LTE 比较指标值与阈值，未知算子回退 GT。
func compareThreshold(value float64, comparison string, threshold float64) bool {
	switch comparison {
	case "GTE":
		return value >= threshold
	case "LT":
		return value < threshold
	case "LTE":
		return value <= threshold
	default:
		return value > threshold
	}
}

// notifyTriggered 告警触发：按规则配置走站内信与网络钩子。
func (s *AlertEvaluator) notifyTriggered(ctx context.Context, rule *ent.AlertRule, value float64, now time.Time) {
	detail := fmt.Sprintf("规则[%s]触发：窗口%d分钟内指标值 %.2f %s 阈值 %.2f。",
		derefStr(rule.Name), rule.WindowMinutes, value, comparisonText(rule.Comparison), rule.Threshold)

	channels := rule.Channels
	if len(channels) == 0 {
		// 未配置渠道时默认站内信，避免"配了告警但永远收不到"。
		channels = []string{"INTERNAL_MESSAGE"}
	}

	for _, channel := range channels {
		switch channel {
		case "INTERNAL_MESSAGE":
			s.sendInternalMessage(ctx, rule, detail, now)
		case "WEBHOOK":
			// 应用维度不传：webhook 的应用过滤由其自身配置决定。
			s.webhookSender.Send(ctx, SendPayload{
				TenantID:  derefU32(rule.TenantID),
				AppID:     "",
				EventType: alertEventType,
				Data: map[string]any{
					"ruleId":        rule.ID,
					"ruleName":      derefStr(rule.Name),
					"eventName":     derefStr(rule.EventName),
					"aggregator":    rule.Aggregator,
					"comparison":    rule.Comparison,
					"threshold":     rule.Threshold,
					"value":         value,
					"windowMinutes": rule.WindowMinutes,
					"triggeredAt":   now.Format(time.RFC3339),
				},
			})
		}
	}
}

// sendInternalMessage 站内信通知规则创建者。
func (s *AlertEvaluator) sendInternalMessage(ctx context.Context, rule *ent.AlertRule, detail string, now time.Time) {
	tenantID := derefU32(rule.TenantID)
	title := fmt.Sprintf("【指标告警】%s", derefStr(rule.Name))

	msg, err := s.internalMsgRepo.Create(ctx, &internalMessageV1.CreateInternalMessageRequest{
		Data: &internalMessageV1.InternalMessage{
			TenantId:   &tenantID,
			Title:      &title,
			Content:    &detail,
			CategoryId: &alertInternalMessageCategory,
			CreatedAt:  timestamppb.New(now),
		},
	})
	if err != nil {
		s.log.Errorf("create alert internal message failed: %s", err.Error())
		return
	}

	if rule.CreatedBy == nil {
		return
	}

	recipient := *rule.CreatedBy
	if _, err = s.recipientRepo.Create(ctx, &internalMessageV1.InternalMessageRecipient{
		MessageId:       msg.Id,
		RecipientUserId: &recipient,
		TenantId:        &tenantID,
	}); err != nil {
		s.log.Errorf("create alert message recipient failed: %s", err.Error())
	}
}

// refreshDueSegments 刷新到期的定时分群。
func (s *AlertEvaluator) refreshDueSegments(ctx context.Context) {
	segments, err := s.segmentService.ListDueSegments(ctx, time.Now())
	if err != nil {
		s.log.Errorf("list due segments failed: %s", err.Error())
		return
	}

	for _, segment := range segments {
		if _, err = s.segmentService.RefreshEntity(ctx, segment); err != nil {
			s.log.Errorf("refresh segment [%d] failed: %s", segment.ID, err.Error())
		}
	}
}

// derefStr 安全解引用字符串指针。
func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// derefU32 安全解引用 int64 指针。
func derefU32(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// comparisonText 比较算子的中文描述，用于通知文案。
func comparisonText(comparison string) string {
	switch comparison {
	case "GTE":
		return ">="
	case "LT":
		return "<"
	case "LTE":
		return "<="
	default:
		return ">"
	}
}

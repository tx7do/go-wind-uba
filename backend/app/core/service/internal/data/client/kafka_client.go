package client

import (
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"
	kafkaBroker "github.com/tx7do/kratos-transport/broker/kafka"

	"go-wind-uba/pkg/middleware/metadata"
	"go-wind-uba/pkg/topic"
)

// NewKafkaBroker 连接 core 消费风险事件所用的 Kafka。
//
// 返回 nil broker 是合法的：data.kafka 未配置或集群不可达时，服务仍应启动，
// 只是风险事件的 Webhook 推送不生效（查询、分析等主链路不依赖它）。
func NewKafkaBroker(ctx *bootstrap.Context) broker.Broker {
	cfg := ctx.GetConfig()
	l := ctx.NewLoggerHelper("kafka/data/core-service")

	kafka := cfg.GetData().GetKafka()
	if kafka == nil || len(kafka.GetEndpoints()) == 0 {
		l.Warn("data.kafka.endpoints is empty: risk event webhook delivery is disabled")
		return nil
	}

	if kafka.GetAllowAutoTopicCreation() {
		// collector 是主题的属主，这里只为"core 先于 collector 启动"的部署兜底，
		// 避免订阅一个不存在的主题持续报错。
		for _, endpoint := range kafka.GetEndpoints() {
			if err := kafkaBroker.CreateTopic(endpoint, topic.UbaEventRisk, 3, 1); err != nil {
				l.Warnf("create kafka topic %s at %s: %v", topic.UbaEventRisk, endpoint, err)
			}
		}
	}

	b := kafkaBroker.NewBroker(
		broker.WithAddress(kafka.GetEndpoints()...),
		broker.WithCodec(kafka.GetCodec()),
		broker.WithGlobalTracerProvider(),
		broker.WithGlobalPropagator(),
		broker.WithSubscriberMiddlewares(
			metadata.Subscriber(),
		),
	)
	if b == nil {
		l.Warn("kafka broker could not be built: risk event webhook delivery is disabled")
		return nil
	}

	_ = b.Init()

	if err := b.Connect(); err != nil {
		l.Warnf("connect kafka at %v failed: %v — risk event webhook delivery is disabled", kafka.GetEndpoints(), err)
		return nil
	}

	return b
}

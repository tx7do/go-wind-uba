package data

import (
	"github.com/redis/go-redis/v9"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-transport/broker"
	kafkaBroker "github.com/tx7do/kratos-transport/broker/kafka"

	authzEngine "github.com/tx7do/kratos-authz/engine"
	"github.com/tx7do/kratos-authz/engine/noop"

	"github.com/go-kratos/kratos/v2/registry"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	redisClient "github.com/tx7do/kratos-bootstrap/cache/redis"
	bRegistry "github.com/tx7do/kratos-bootstrap/registry"
	"github.com/tx7do/kratos-bootstrap/rpc"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"go-wind-uba/pkg/middleware/metadata"
	"go-wind-uba/pkg/serviceid"
	"go-wind-uba/pkg/topic"
)

const (
	defaultKafkaPartitions        = 32
	defaultKafkaReplicationFactor = 1
)

func NewClientType() authenticationV1.ClientType {
	return authenticationV1.ClientType_collector
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(ctx *bootstrap.Context) (*redis.Client, func(), error) {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	l := ctx.NewLoggerHelper("redis/data/collector-service")

	cli := redisClient.NewClient(cfg.Data, l)

	return cli, func() {
		if err := cli.Close(); err != nil {
			l.Error(err)
		}
	}, nil
}

// NewDiscovery 创建服务发现客户端
func NewDiscovery(ctx *bootstrap.Context) registry.Discovery {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil
	}

	discovery, err := bRegistry.NewDiscovery(cfg.Registry)
	if err != nil {
		return nil
	}

	return discovery
}

// NewAuthorizer 创建权鉴器
func NewAuthorizer() authzEngine.Engine {
	return noop.State{}
}

// NewKafkaBroker 连接 collector 上报所用的 Kafka。
//
// 返回 nil broker 是合法的：data.kafka 未配置或集群不可达时，服务仍应启动（健康检查、
// 已有的重传逻辑都依赖它），但 ReportService 会把上报明确拒绝掉而不是 panic —— 静默丢事件
// 比启动失败更难发现。
func NewKafkaBroker(ctx *bootstrap.Context) broker.Broker {
	cfg := ctx.GetConfig()
	l := ctx.NewLoggerHelper("kafka/data/collector-service")

	kafka := cfg.GetData().GetKafka()
	if kafka == nil || len(kafka.GetEndpoints()) == 0 {
		l.Warn("data.kafka.endpoints is empty: reported events have nowhere to go")
		return nil
	}

	// 主题必须在 Doris 的作业建立之前就存在，否则 Routine Load 会因主题不可见而 PAUSED。
	// 建主题用的是客户端地址（data.kafka.endpoints），不是 server.kafka —— 后者是本服务作为
	// 消费者时的传输层配置，这里并没有订阅者。
	if kafka.GetAllowAutoTopicCreation() {
		ensureTopics(l, kafka.GetEndpoints())
	}

	b := kafkaBroker.NewBroker(
		broker.WithAddress(kafka.GetEndpoints()...),
		broker.WithCodec(kafka.GetCodec()),
		broker.WithGlobalTracerProvider(),
		broker.WithGlobalPropagator(),
		broker.WithPublishMiddlewares(
			metadata.Publish(),
		),
		broker.WithSubscriberMiddlewares(
			metadata.Subscriber(),
		),
	)
	if b == nil {
		l.Warn("kafka broker could not be built: reported events will be rejected")
		return nil
	}

	_ = b.Init()

	if err := b.Connect(); err != nil {
		l.Warnf("connect kafka at %v failed: %v — reported events will be rejected", kafka.GetEndpoints(), err)
		return nil
	}

	return b
}

// ensureTopics 建出 Doris Routine Load 订阅的两个主题。CreateTopic 对已存在的主题返回 nil，
// 所以每次启动都能安全执行；失败只告警，因为 broker 可能还没开 admin 端口，
// 而事件仍可能靠 Kafka 的 auto-create 落下来。
func ensureTopics(l *log.Helper, endpoints []string) {
	for _, endpoint := range endpoints {
		for _, name := range []string{topic.UbaEventRaw, topic.UbaEventRisk} {
			if err := kafkaBroker.CreateTopic(endpoint, name, defaultKafkaPartitions, defaultKafkaReplicationFactor); err != nil {
				l.Warnf("create kafka topic %s at %s: %v", name, endpoint, err)
			}
		}
	}
}

func NewApplicationServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ApplicationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewApplicationServiceClient(cli)
}

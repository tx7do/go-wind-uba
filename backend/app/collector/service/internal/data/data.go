package data

import (
	"github.com/redis/go-redis/v9"

	"github.com/tx7do/kratos-transport/broker"
	"github.com/tx7do/kratos-transport/broker/kafka"

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

func NewKafkaBroker(ctx *bootstrap.Context) broker.Broker {
	cfg := ctx.GetConfig()

	if cfg == nil || cfg.Data.Kafka == nil {
		return nil
	}

	b := kafka.NewBroker(
		broker.WithAddress(cfg.Data.Kafka.Endpoints...),
		broker.WithCodec(cfg.Data.Kafka.Codec),
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
		return nil
	}

	_ = b.Init()

	if err := b.Connect(); err != nil {
		return nil
	}

	return b
}

func NewApplicationServiceClient(ctx *bootstrap.Context, r registry.Discovery) ubaV1.ApplicationServiceClient {
	cli, err := rpc.CreateGrpcClient(ctx.Context(), r, serviceid.NewDiscoveryName(serviceid.CoreService), ctx.GetConfig())
	if err != nil {
		return nil
	}

	return ubaV1.NewApplicationServiceClient(cli)
}

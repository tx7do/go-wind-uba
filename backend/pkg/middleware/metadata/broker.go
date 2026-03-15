package metadata

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-transport/broker"

	"go-wind-uba/pkg/metadata"
)

const (
	MetadataHeaderKey = "x-metadata-bin"
)

func Publish(opts ...Option) broker.PublishMiddleware {
	op := options{
		log: log.NewHelper(log.With(log.DefaultLogger, "module", "publish/metadata/middleware")),

		extractMetadataFromServer: false,
	}
	for _, o := range opts {
		o(&op)
	}

	return func(handler broker.PublishHandler) broker.PublishHandler {
		return func(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
			//op.log.Debugf("metadata broker middleware: publishing message to topic=%s", topic)

			md, err := metadata.FromContext(ctx, op.extractMetadataFromServer)
			if err != nil {
				op.log.Warnf("metadata broker middleware: failed to get metadata from context: %v; topic=%s", err, topic)
				return handler(ctx, topic, msg, opts...)
			}
			if md == nil {
				op.log.Warnf("metadata broker middleware: no metadata found in context; topic=%s", topic)
				return handler(ctx, topic, msg, opts...)
			}

			mdStr, err := metadata.EncodeOperatorMetadata(md)
			if err != nil {
				op.log.Warnf("metadata broker middleware: failed to encode metadata: %v", err)
			} else {
				msg.SetHeader(MetadataHeaderKey, mdStr)
				//op.log.Debugf("metadata broker middleware: added metadata header to message: %s", mdStr)
			}

			return handler(ctx, topic, msg, opts...)
		}
	}
}

func Subscriber(opts ...Option) broker.SubscriberMiddleware {
	op := options{
		log: log.NewHelper(log.With(log.DefaultLogger, "module", "subscriber/metadata/middleware")),

		extractMetadataFromServer: true,
	}
	for _, o := range opts {
		o(&op)
	}

	return func(handler broker.Handler) broker.Handler {
		return func(ctx context.Context, evt broker.Event) error {
			op.log.Debugf("metadata broker middleware: handling event from topic=%s", evt.Topic())

			if len(evt.Message().Headers) == 0 {
				op.log.Debugf("metadata broker middleware: no headers found in message")
				return handler(ctx, evt)
			}

			for k, v := range evt.Message().Headers {
				op.log.Debugf("metadata broker middleware: message header: %s=%s", k, v)
			}

			mdStr := evt.Message().GetHeader(MetadataHeaderKey)
			if mdStr != "" {
				md, err := metadata.DecodeOperatorMetadata(mdStr)
				if err != nil {
					op.log.Warnf("metadata broker middleware: failed to decode metadata: %v", err)
				} else {
					ctx, err = metadata.NewContext(ctx, md)
					if err != nil {
						op.log.Warnf("metadata broker middleware: failed to create new context with metadata: %v", err)
					}
				}
			} else {
				op.log.Debugf("metadata broker middleware: no metadata header found in message")
			}

			return handler(ctx, evt)
		}
	}
}

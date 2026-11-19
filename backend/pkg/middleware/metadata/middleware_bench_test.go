package metadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	broker "github.com/tx7do/kratos-transport/broker"

	"go-wind-uba/pkg/metadata"
)

// BenchmarkPublishWithMetadata 每条 Kafka 消息发布都会走的元数据注入路径
// (编码 + header 写入)。
func BenchmarkPublishWithMetadata(b *testing.B) {
	mw := Publish()
	ctx, err := metadata.NewContext(context.Background(), testOperator())
	require.NoError(b, err)
	msg := broker.NewMessage(nil)
	handler := mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
		return nil
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := handler(ctx, "uba_events_raw", msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSubscriberWithMetadata 每条 Kafka 消息消费都会走的元数据还原路径
// (header 读取 + 解码 + context 注入)。
func BenchmarkSubscriberWithMetadata(b *testing.B) {
	mw := Subscriber()
	mdStr, err := metadata.EncodeOperatorMetadata(testOperator())
	require.NoError(b, err)
	msg := broker.NewMessage(nil)
	msg.SetHeader(MetadataHeaderKey, mdStr)
	evt := &fakeEvent{topic: "uba_events_raw", msg: msg}
	handler := mw(func(_ context.Context, _ broker.Event) error {
		return nil
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := handler(context.Background(), evt); err != nil {
			b.Fatal(err)
		}
	}
}

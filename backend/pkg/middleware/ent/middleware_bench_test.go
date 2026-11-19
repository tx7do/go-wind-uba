package ent

import (
	"context"
	"testing"

	kratosMD "github.com/go-kratos/kratos/v2/metadata"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/require"
	broker "github.com/tx7do/kratos-transport/broker"

	appViewer "go-wind-uba/pkg/entgo/viewer"
	"go-wind-uba/pkg/metadata"
)

// BenchmarkPublishSystemViewer 系统上下文发布消息的头注入路径。
// 用户 viewer 路径当前因 DataScope proto 编组 panic 不可 bench(见
// TestPublishUserViewerDataScopePanic),修复后应补充用户 viewer 基准。
func BenchmarkPublishSystemViewer(b *testing.B) {
	mw := Publish()
	ctx := appViewer.NewSystemViewerContext(context.Background())
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

// BenchmarkSubscriberUserViewer 消费侧身份还原路径(header 解析 + viewer 注入)。
func BenchmarkSubscriberUserViewer(b *testing.B) {
	mw := Subscriber()
	msg := broker.NewMessage(nil)
	msg.SetHeader(UserIDHeaderKey, "1001")
	msg.SetHeader(TenantIDHeaderKey, "42")
	msg.SetHeader(OrgUnitIDHeaderKey, "7")
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

// BenchmarkEntServerUnary unary 请求过 Server 中间件(元数据→UserViewer)的成本。
func BenchmarkEntServerUnary(b *testing.B) {
	mw := Server()
	ctx := kratosMDServerContextWithOperator(b)
	req := &struct{}{}
	handler := mw(func(_ context.Context, _ any) (any, error) {
		return nil, nil
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := handler(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInjectViewerForStream 流式入口的 viewer 注入成本。
func BenchmarkInjectViewerForStream(b *testing.B) {
	ctx := kratosMDServerContextWithOperator(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = InjectViewerForStream(ctx)
	}
}

// kratosMDServerContextWithOperator 构造带操作者元数据的 server 上下文。
func kratosMDServerContextWithOperator(b *testing.B) context.Context {
	b.Helper()
	mdStr, err := metadata.EncodeOperatorMetadata(testOperator())
	require.NoError(b, err)
	return kratosMD.NewServerContext(
		kratosTransport.NewServerContext(context.Background(), &fakeServerTransport{}),
		kratosMD.Metadata{"x-md-global-operator": []string{mdStr}},
	)
}

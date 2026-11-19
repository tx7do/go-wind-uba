package ent

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/go-kratos/kratos/v2/encoding"
	_ "github.com/go-kratos/kratos/v2/encoding/proto" // 与生产链路一致:注册 proto codec
	kratosMD "github.com/go-kratos/kratos/v2/metadata"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-crud/viewer"
	broker "github.com/tx7do/kratos-transport/broker"
	"go.opentelemetry.io/otel/trace"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"

	appViewer "go-wind-uba/pkg/entgo/viewer"
	"go-wind-uba/pkg/metadata"
)

// fakeEvent 实现 broker.Event 的最小事件载体。
type fakeEvent struct {
	topic string
	msg   *broker.Message
}

func (e *fakeEvent) Topic() string            { return e.topic }
func (e *fakeEvent) Message() *broker.Message { return e.msg }
func (e *fakeEvent) RawMessage() any          { return nil }
func (e *fakeEvent) Ack() error               { return nil }
func (e *fakeEvent) Error() error             { return nil }

// testOperator 构造典型用户操作者元数据。
func testOperator() *authenticationV1.OperatorMetadata {
	return metadata.NewUserOperator(1001, 42, 7, identityV1.DataScope_ALL)
}

// validTraceIDHex 返回一个合法的 32 位十六进制 trace id 字符串。
func validTraceIDHex() string {
	return hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))[:32]
}

func TestPublishNoViewer(t *testing.T) {
	// 上下文无 viewer(如内部无身份任务)时直接透传,消息头不被污染。
	mw := Publish()
	msg := broker.NewMessage(nil)
	err := mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
		assert.Empty(t, m.GetHeader(UserIDHeaderKey))
		assert.Empty(t, m.GetHeader(TenantIDHeaderKey))
		assert.Empty(t, m.GetHeader(SystemContextHeaderKey))
		return nil
	})(context.Background(), "uba_events_raw", msg)
	require.NoError(t, err)
}

func TestPublishSystemViewer(t *testing.T) {
	// 系统上下文:x-system=true、身份头为零值 —— 下游 Subscriber 据此还原系统上下文。
	mw := Publish()
	ctx := appViewer.NewSystemViewerContext(context.Background())
	msg := broker.NewMessage(nil)
	err := mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
		return nil
	})(ctx, "uba_events_raw", msg)
	require.NoError(t, err)

	assert.Equal(t, "true", msg.GetHeader(SystemContextHeaderKey))
	assert.Equal(t, "0", msg.GetHeader(UserIDHeaderKey))
	assert.Equal(t, "0", msg.GetHeader(TenantIDHeaderKey))
	assert.Equal(t, "0", msg.GetHeader(OrgUnitIDHeaderKey))
	assert.Empty(t, msg.GetHeader(TraceIDHeaderKey), "无 trace 时不应写 trace 头")
}

func TestPublishUserViewer(t *testing.T) {
	// 用户 viewer:身份三元组应原样写进消息头。
	// 注:DataScope 头的编组见 TestPublishUserViewerDataScopePanic —— 当前实现会 panic,
	// 因此这里构造 dataScopes 为空的 viewer(NewUserViewerWithDataScopes 传 nil)绕开该路径。
	mw := Publish()
	vCtx := appViewer.NewUserViewerWithDataScopes(1001, 42, 7, "", nil)
	ctx := viewer.WithContext(context.Background(), vCtx)

	msg := broker.NewMessage(nil)
	err := mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
		return nil
	})(ctx, "uba_events_raw", msg)
	require.NoError(t, err)

	assert.Equal(t, "1001", msg.GetHeader(UserIDHeaderKey))
	assert.Equal(t, "42", msg.GetHeader(TenantIDHeaderKey))
	assert.Equal(t, "7", msg.GetHeader(OrgUnitIDHeaderKey))
	assert.Empty(t, msg.GetHeader(SystemContextHeaderKey), "用户上下文不应带 x-system")
}

func TestPublishTraceIDPropagation(t *testing.T) {
	// 请求带 OTel trace 时,trace id 应透传到消息头,供下游串联链路。
	mw := Publish()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
	})
	ctx := trace.ContextWithSpanContext(
		viewer.WithContext(context.Background(), appViewer.NewSystemViewer()), sc)

	msg := broker.NewMessage(nil)
	err := mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
		return nil
	})(ctx, "uba_events_raw", msg)
	require.NoError(t, err)
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", msg.GetHeader(TraceIDHeaderKey))
}

func TestPublishUserViewerDataScopePanic(t *testing.T) {
	// 【现状钉死】Publish 对 viewer.DataScope 调 proto codec Marshal:
	// viewer.DataScope 是普通 struct 而非 proto.Message,kratos proto codec 的
	// Marshal 首行就是 v.(proto.Message) 类型断言 —— NewUserViewer 恒带一个
	// DataScope 元素(len>0 必走该分支),因此用户 viewer 一旦 Publish 就 panic。
	// 该中间件当前未被接线,属于接线前必须修复的潜在崩溃点;修复后本用例
	// 失败属预期,请同步改写为正常往返断言。
	mw := Publish()
	ctx := viewer.WithContext(context.Background(), appViewer.NewUserViewer(1, 1, 1, "", identityV1.DataScope_ALL))

	msg := broker.NewMessage(nil)
	assert.Panics(t, func() {
		_ = mw(func(_ context.Context, _ string, m *broker.Message, _ ...broker.PublishOption) error {
			return nil
		})(ctx, "uba_events_raw", msg)
	}, "当前实现对非 proto 的 DataScope 调 proto codec Marshal,类型断言必 panic")
	_ = encoding.GetCodec("proto") // 显式引用,保证 proto codec 注册意图可见
}

func TestSubscriberNoHeaders(t *testing.T) {
	// 无 header 消息直接透传,不得凭空注入 viewer。
	mw := Subscriber()
	evt := &fakeEvent{topic: "uba_events_raw", msg: broker.NewMessage(nil)}
	err := mw(func(ctx context.Context, e broker.Event) error {
		_, ok := viewer.FromContext(ctx)
		assert.False(t, ok, "无 header 时不应注入 viewer")
		return nil
	})(context.Background(), evt)
	require.NoError(t, err)
}

func TestSubscriberIdsToUserViewer(t *testing.T) {
	// 身份头 → UserViewer 还原:三元组一致,trace id 同时恢复到 OTel 上下文。
	mw := Subscriber()
	msg := broker.NewMessage(nil)
	msg.SetHeader(UserIDHeaderKey, "1001")
	msg.SetHeader(TenantIDHeaderKey, "42")
	msg.SetHeader(OrgUnitIDHeaderKey, "7")
	msg.SetHeader(TraceIDHeaderKey, validTraceIDHex())
	evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

	err := mw(func(ctx context.Context, e broker.Event) error {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "应注入 user viewer")
		assert.False(t, vc.IsSystemContext())
		assert.Equal(t, int64(1001), vc.UserID())
		assert.Equal(t, int64(42), vc.TenantID())
		assert.Equal(t, int64(7), vc.OrgUnitID())
		assert.True(t, trace.SpanContextFromContext(ctx).HasTraceID(),
			"合法 trace id 应恢复进 OTel 上下文")
		return nil
	})(context.Background(), evt)
	require.NoError(t, err)
}

func TestSubscriberSystemHeader(t *testing.T) {
	// x-system=true:应还原为系统上下文,跳过用户身份还原。
	mw := Subscriber()
	msg := broker.NewMessage(nil)
	msg.SetHeader(SystemContextHeaderKey, "true")
	evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

	err := mw(func(ctx context.Context, e broker.Event) error {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "应注入 system viewer")
		assert.True(t, vc.IsSystemContext(), "x-system=true 应还原系统上下文")
		return nil
	})(context.Background(), evt)
	require.NoError(t, err)
}

func TestSubscriberInvalidSystemHeader(t *testing.T) {
	// x-system 非布尔值:ParseBool 失败仅告警,继续按用户上下文路径还原。
	mw := Subscriber()
	msg := broker.NewMessage(nil)
	msg.SetHeader(SystemContextHeaderKey, "not-a-bool")
	evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

	err := mw(func(ctx context.Context, e broker.Event) error {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "非法 system 头也应走用户 viewer 路径")
		assert.False(t, vc.IsSystemContext())
		return nil
	})(context.Background(), evt)
	require.NoError(t, err)
}

func TestSubscriberInvalidIdHeaders(t *testing.T) {
	// 非数字身份头:解析失败仅告警,viewer 以零值注入,消息处理不得中断。
	mw := Subscriber()
	msg := broker.NewMessage(nil)
	msg.SetHeader(UserIDHeaderKey, "not-a-number")
	msg.SetHeader(TenantIDHeaderKey, "junk")
	evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

	err := mw(func(ctx context.Context, e broker.Event) error {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, int64(0), vc.UserID(), "非法 user id 应落零值")
		assert.Equal(t, int64(0), vc.TenantID(), "非法 tenant id 应落零值")
		return nil
	})(context.Background(), evt)
	require.NoError(t, err)
}

func TestEntServerNoMetadata(t *testing.T) {
	// 内部服务间调用不带操作者元数据:回落系统上下文(与注释中的信任模型一致)。
	mw := Server()
	_, err := mw(func(ctx context.Context, _ any) (any, error) {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "缺元数据时应回落系统上下文")
		assert.True(t, vc.IsSystemContext())
		return nil, nil
	})(context.Background(), &struct{}{})
	require.NoError(t, err)
}

func TestEntServerWithMetadata(t *testing.T) {
	// 请求带操作者元数据:应还原为对应身份的 UserViewer。
	mw := Server()

	mdStr, err := metadata.EncodeOperatorMetadata(testOperator())
	require.NoError(t, err)

	ctx := kratosMD.NewServerContext(
		kratosTransport.NewServerContext(context.Background(), &fakeServerTransport{}),
		kratosMD.Metadata{"x-md-global-operator": []string{mdStr}},
	)

	_, err = mw(func(ctx context.Context, _ any) (any, error) {
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "应注入 user viewer")
		assert.False(t, vc.IsSystemContext())
		assert.Equal(t, int64(1001), vc.UserID())
		assert.Equal(t, int64(42), vc.TenantID())
		assert.Equal(t, int64(7), vc.OrgUnitID())
		return nil, nil
	})(ctx, &struct{}{})
	require.NoError(t, err)
}

func TestInjectViewerForStream(t *testing.T) {
	t.Run("无元数据回落系统上下文", func(t *testing.T) {
		// 流式 RPC 入口对匿名/异常请求的兜底:system viewer,绝不裸 ctx。
		ctx := InjectViewerForStream(context.Background())
		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok, "流式入口必须注入 viewer")
		assert.True(t, vc.IsSystemContext())
	})

	t.Run("有元数据还原UserViewer", func(t *testing.T) {
		mdStr, err := metadata.EncodeOperatorMetadata(testOperator())
		require.NoError(t, err)

		ctx := kratosMD.NewServerContext(
			kratosTransport.NewServerContext(context.Background(), &fakeServerTransport{}),
			kratosMD.Metadata{"x-md-global-operator": []string{mdStr}},
		)
		ctx = InjectViewerForStream(ctx)

		vc, ok := viewer.FromContext(ctx)
		require.True(t, ok)
		assert.False(t, vc.IsSystemContext())
		assert.Equal(t, int64(1001), vc.UserID())
	})
}

func TestMetaDataToUserViewerContextNil(t *testing.T) {
	// nil 元数据直接返回 nil,由调用方决定回落策略。
	assert.Nil(t, metaDataToUserViewerContext(nil, ""))
}

// fakeServerTransport 供 kratos server 上下文挂载的最小传输器。
type fakeServerTransport struct{}

func (f *fakeServerTransport) Kind() kratosTransport.Kind            { return kratosTransport.KindGRPC }
func (f *fakeServerTransport) Endpoint() string                      { return "fake://ent-test" }
func (f *fakeServerTransport) Operation() string                     { return "/fake/Op" }
func (f *fakeServerTransport) RequestHeader() kratosTransport.Header { return nil }
func (f *fakeServerTransport) ReplyHeader() kratosTransport.Header   { return nil }

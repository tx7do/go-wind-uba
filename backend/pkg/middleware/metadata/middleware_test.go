package metadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	broker "github.com/tx7do/kratos-transport/broker"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"

	"go-wind-uba/pkg/metadata"
)

// fakeEvent 实现 broker.Event 的最小事件载体,只承载内存中的 Message。
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

func TestPublishMiddleware(t *testing.T) {
	t.Run("无元数据透传", func(t *testing.T) {
		// 上下文没有操作者元数据(如系统后台任务直接 Publish)时,
		// 消息不加头直接透传,不得 panic。
		mw := Publish()
		msg := broker.NewMessage(nil)
		called := false
		err := mw(func(ctx context.Context, topic string, m *broker.Message, _ ...broker.PublishOption) error {
			called = true
			assert.Empty(t, m.GetHeader(MetadataHeaderKey), "无元数据时不应注入 header")
			return nil
		})(context.Background(), "uba_events_raw", msg)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("有元数据注入Header", func(t *testing.T) {
		// 上下文带操作者元数据时,应编码后写进 x-metadata-bin 头,
		// 且下游可解码回同一身份。
		mw := Publish()
		ctx, err := metadata.NewContext(context.Background(), testOperator())
		require.NoError(t, err)

		msg := broker.NewMessage(nil)
		err = mw(func(_ context.Context, topic string, m *broker.Message, _ ...broker.PublishOption) error {
			return nil
		})(ctx, "uba_events_raw", msg)
		require.NoError(t, err)

		mdStr := msg.GetHeader(MetadataHeaderKey)
		require.NotEmpty(t, mdStr, "应注入元数据 header")

		// 下游消费者侧解码回环:身份三元组必须原样到达。
		md, err := metadata.DecodeOperatorMetadata(mdStr)
		require.NoError(t, err)
		assert.Equal(t, int64(1001), md.GetUserId())
		assert.Equal(t, int64(42), md.GetTenantId())
		assert.Equal(t, int64(7), md.GetOrgUnitId())
	})

	t.Run("metadata为nil透传", func(t *testing.T) {
		// FromContext 成功但返回 nil 的路径:同样透传不注入。
		mw := Publish()
		msg := broker.NewMessage(nil)
		err := mw(func(ctx context.Context, topic string, m *broker.Message, _ ...broker.PublishOption) error {
			return nil
		})(context.Background(), "t", msg)
		require.NoError(t, err)
		assert.Empty(t, msg.GetHeader(MetadataHeaderKey))
	})
}

func TestSubscriberMiddleware(t *testing.T) {
	t.Run("无headers透传", func(t *testing.T) {
		// 消息没有任何 header(如历史消息/内部事件)时,直接透传,
		// 不得凭空构造空 viewer。
		mw := Subscriber()
		evt := &fakeEvent{topic: "uba_events_raw", msg: broker.NewMessage(nil)}
		called := false
		err := mw(func(ctx context.Context, e broker.Event) error {
			called = true
			_, err2 := metadata.FromContext(ctx, true)
			assert.Error(t, err2, "无 header 时不应注入元数据")
			return nil
		})(context.Background(), evt)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("有元数据header注入", func(t *testing.T) {
		// 消息带 x-metadata-bin 时应解码并注入 context,
		// handler 侧通过 FromContext 拿到发布者身份。
		mw := Subscriber()

		mdStr, err := metadata.EncodeOperatorMetadata(testOperator())
		require.NoError(t, err)
		msg := broker.NewMessage(nil)
		msg.SetHeader(MetadataHeaderKey, mdStr)
		evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

		var got *authenticationV1.OperatorMetadata
		err = mw(func(ctx context.Context, e broker.Event) error {
			md, mdErr := metadata.FromContext(ctx, false)
			if mdErr == nil {
				got = md
			}
			return nil
		})(context.Background(), evt)
		require.NoError(t, err)
		require.NotNil(t, got, "应从消息 header 还原操作者元数据")
		assert.Equal(t, int64(1001), got.GetUserId())
		assert.Equal(t, int64(42), got.GetTenantId())
	})

	t.Run("损坏的元数据header降级透传", func(t *testing.T) {
		// header 存在但内容损坏:解码失败应降级为无身份透传,
		// 消息不能被吞掉、handler 必须仍然被调用。
		mw := Subscriber()
		msg := broker.NewMessage(nil)
		msg.SetHeader(MetadataHeaderKey, "not-a-valid-metadata!!!")
		evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

		called := false
		err := mw(func(ctx context.Context, e broker.Event) error {
			called = true
			_, err2 := metadata.FromContext(ctx, false)
			assert.Error(t, err2, "解码失败时不应注入元数据")
			return nil
		})(context.Background(), evt)
		require.NoError(t, err)
		assert.True(t, called, "解码失败不得阻断消息处理")
	})

	t.Run("无关headers不注入", func(t *testing.T) {
		// 只有其他业务 header 时不应注入任何元数据。
		mw := Subscriber()
		msg := broker.NewMessage(nil)
		msg.SetHeader("x-other", "value")
		evt := &fakeEvent{topic: "uba_events_raw", msg: msg}

		err := mw(func(ctx context.Context, e broker.Event) error {
			_, err2 := metadata.FromContext(ctx, false)
			assert.Error(t, err2, "无关 header 不应注入元数据")
			return nil
		})(context.Background(), evt)
		require.NoError(t, err)
	})
}

func TestWithExtractMetadataFromServerOption(t *testing.T) {
	// Publish 侧的 extractMetadataFromServer 开关决定读 client 还是 server
	// 方向的元数据;gRPC handler 内直接 Publish 时元数据在 server 侧,
	// 必须开该开关才能注入消息头。
	mw := Publish(WithExtractMetadataFromServer(true))
	assert.NotNil(t, mw)
}

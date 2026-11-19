package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

func TestAppAuthenticator_CorruptCacheFallsBackToGrpc(t *testing.T) {
	// 缓存快照损坏(Redis 脏数据/版本不兼容):反序列化失败必须当作缓存未命中
	// 回退 gRPC 反查,而不是直接拒绝或 500。
	auth, mr := newTestAuthenticator(t, testEnabledApp(), nil)
	require.NoError(t, mr.Set(appAuthCacheKey("app-001"), "{corrupted-json"))

	app, err := auth.Authenticate(context.Background(), "app-001", "secret-abc")
	require.NoError(t, err)
	assert.Equal(t, int64(42), app.TenantID, "损坏缓存应回退 gRPC 并成功鉴权")
}

func TestAppAuthenticator_WorksWithoutRedis(t *testing.T) {
	// redisClient 为 nil(data.redis 未配置):缓存层整体旁路,每次直查 gRPC,
	// 正/负缓存分支都必须安全跳过,不得 panic。
	auth, _ := newTestAuthenticator(t, testEnabledApp(), nil)
	auth.redisClient = nil
	ctx := context.Background()

	app, err := auth.Authenticate(ctx, "app-001", "secret-abc")
	require.NoError(t, err)
	assert.Equal(t, int64(42), app.TenantID)

	// 查无此应用时没有负缓存兜底,仍应正确返回 401(而非 panic 或 500)。
	auth2, _ := newTestAuthenticator(t, nil, nil)
	auth2.redisClient = nil
	_, err = auth2.Authenticate(ctx, "ghost-app", "any")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestAppAuthenticator_LookupApp_UnknownAppWritesNegCache(t *testing.T) {
	// 只读旁路同样要写负缓存:可视化埋点等高频轮询场景下查无应用不能穿透 gRPC/DB。
	auth, mr := newTestAuthenticator(t, nil, nil)

	_, err := auth.LookupApp(context.Background(), "ghost-app")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
	assert.True(t, mr.Exists(appAuthNegCacheKey("ghost-app")))
}

func TestAppAuthenticator_DisabledAppSnapshotCached(t *testing.T) {
	// 禁用应用的快照(含禁用状态)也会回写缓存:后续请求命中缓存仍拒绝 401,
	// 不会穿透 gRPC —— 换成必挂的 gRPC 客户端验证第二次确实走了缓存。
	app := testEnabledApp()
	app.Status = ubaV1.Application_OFF.Enum()
	auth, _ := newTestAuthenticator(t, app, nil)
	ctx := context.Background()

	_, err := auth.Authenticate(ctx, "app-001", "secret-abc")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))

	auth.applicationServiceClient = &fakeApplicationClient{err: errors.New("grpc down")}
	_, err = auth.Authenticate(ctx, "app-001", "secret-abc")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err),
		"禁用快照已缓存,第二次应命中缓存拒绝(401 而非 gRPC 故障的 500)")
}

func TestAppAuthenticator_CacheWriteFailureNotFatal(t *testing.T) {
	// Redis 整体不可用(如实例重启窗口):正/负缓存写失败只记日志,不阻断鉴权主流程。
	auth, mr := newTestAuthenticator(t, testEnabledApp(), nil)
	ctx := context.Background()
	mr.Close() // 之后所有 redis 读写都会失败

	app, err := auth.Authenticate(ctx, "app-001", "secret-abc")
	require.NoError(t, err, "正缓存写失败不得影响鉴权结果")
	assert.Equal(t, int64(42), app.TenantID)

	// 负缓存写失败同样只降级:查无应用仍正确返回 401。
	auth2, mr2 := newTestAuthenticator(t, nil, nil)
	mr2.Close()
	_, err = auth2.Authenticate(ctx, "ghost-app", "any")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

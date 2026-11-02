package service

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"google.golang.org/grpc"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// fakeApplicationClient 只覆写 Get，其余方法不会被 AppAuthenticator 调用到。
type fakeApplicationClient struct {
	ubaV1.ApplicationServiceClient

	app *ubaV1.Application
	err error
}

func (f *fakeApplicationClient) Get(_ context.Context, _ *ubaV1.GetApplicationRequest, _ ...grpc.CallOption) (*ubaV1.Application, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.app == nil {
		return nil, nil
	}
	return f.app, nil
}

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	require.NotNil(t, mr)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr
}

func newTestAuthenticator(t *testing.T, app *ubaV1.Application, appErr error) (*AppAuthenticator, *miniredis.Miniredis) {
	t.Helper()

	rc, mr := newTestRedis(t)

	bctx := bootstrap.NewContextWithParam(
		context.Background(),
		&conf.AppInfo{}, &conf.Bootstrap{},
		log.DefaultLogger,
	)

	auth := NewAppAuthenticator(bctx, &fakeApplicationClient{app: app, err: appErr}, rc)
	require.NotNil(t, auth)
	return auth, mr
}

func testEnabledApp() *ubaV1.Application {
	return &ubaV1.Application{
		AppId:     trans.Ptr("app-001"),
		AppSecret: trans.Ptr("secret-abc"),
		Status:    ubaV1.Application_ON.Enum(),
		TenantId:  trans.Ptr(uint32(42)),
	}
}

func TestAppAuthenticator_Authenticate_MissingParams(t *testing.T) {
	auth, _ := newTestAuthenticator(t, testEnabledApp(), nil)
	ctx := context.Background()

	for _, tc := range [][2]string{{"", "s"}, {"a", ""}, {"", ""}} {
		_, err := auth.Authenticate(ctx, tc[0], tc[1])
		require.Error(t, err)
		assert.True(t, collectorV1.IsUnauthorized(err), "empty params should be unauthorized")
	}
}

func TestAppAuthenticator_Authenticate_GrpcLookupAndCache(t *testing.T) {
	ctx := context.Background()
	rc, mr := newTestRedis(t)

	bctx := bootstrap.NewContextWithParam(
		context.Background(),
		&conf.AppInfo{}, &conf.Bootstrap{},
		log.DefaultLogger,
	)

	// 首次：走 gRPC 反查 + 回写缓存。
	first := NewAppAuthenticator(bctx, &fakeApplicationClient{app: testEnabledApp()}, rc)
	app, err := first.Authenticate(ctx, "app-001", "secret-abc")
	require.NoError(t, err)
	assert.Equal(t, uint32(42), app.TenantID)
	require.True(t, mr.Exists(appAuthCacheKey("app-001")))

	// 二次：换一个 gRPC 必挂的客户端，命中缓存仍应通过 —— 证明确实走了缓存。
	second := NewAppAuthenticator(bctx, &fakeApplicationClient{err: errors.New("grpc down")}, rc)
	app2, err := second.Authenticate(ctx, "app-001", "secret-abc")
	require.NoError(t, err, "cached app should authenticate even when grpc is down")
	assert.Equal(t, uint32(42), app2.TenantID)
}

func TestAppAuthenticator_Authenticate_WrongSecret(t *testing.T) {
	auth, _ := newTestAuthenticator(t, testEnabledApp(), nil)

	_, err := auth.Authenticate(context.Background(), "app-001", "wrong-secret")
	require.Error(t, err)
	assert.True(t, collectorV1.IsIncorrectAppSecret(err))
}

func TestAppAuthenticator_Authenticate_DisabledApp(t *testing.T) {
	app := testEnabledApp()
	app.Status = ubaV1.Application_OFF.Enum()
	auth, _ := newTestAuthenticator(t, app, nil)

	_, err := auth.Authenticate(context.Background(), "app-001", "secret-abc")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestAppAuthenticator_Authenticate_UnknownAppNegCache(t *testing.T) {
	// gRPC 返回 nil（应用不存在）→ 401 + 负缓存回写。
	auth, mr := newTestAuthenticator(t, nil, nil)
	ctx := context.Background()

	_, err := auth.Authenticate(ctx, "ghost-app", "any")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
	assert.True(t, mr.Exists(appAuthNegCacheKey("ghost-app")))

	// 负缓存命中后，即便 gRPC 恢复能查到，TTL 内仍直接拒绝（防穿透）。
	auth.applicationServiceClient = &fakeApplicationClient{app: testEnabledApp()}
	_, err = auth.Authenticate(ctx, "ghost-app", "any")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestAppAuthenticator_Authenticate_GrpcErrorIs500Not401(t *testing.T) {
	// 瞬时故障不判为鉴权失败：返回 InternalServerError 而非 Unauthorized。
	auth, _ := newTestAuthenticator(t, nil, errors.New("connection refused"))

	_, err := auth.Authenticate(context.Background(), "app-001", "any")
	require.Error(t, err)
	assert.True(t, collectorV1.IsInternalServerError(err), "grpc outage should not masquerade as 401")
}

func TestAppAuthenticator_LookupApp(t *testing.T) {
	// 只读旁路：启用应用返回信息；禁用应用拒绝（不校验密钥）。
	auth, _ := newTestAuthenticator(t, testEnabledApp(), nil)
	app, err := auth.LookupApp(context.Background(), "app-001")
	require.NoError(t, err)
	assert.Equal(t, uint32(42), app.TenantID)

	disabled := testEnabledApp()
	disabled.Status = ubaV1.Application_OFF.Enum()
	auth2, _ := newTestAuthenticator(t, disabled, nil)
	_, err = auth2.LookupApp(context.Background(), "app-001")
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestAppAuthenticator_CacheSnapshotHasNoPlaintextSecret(t *testing.T) {
	// 缓存快照只落哈希：Redis 数据泄露不应导致密钥外泄。
	auth, mr := newTestAuthenticator(t, testEnabledApp(), nil)

	_, err := auth.Authenticate(context.Background(), "app-001", "secret-abc")
	require.NoError(t, err)

	raw, err := mr.Get(appAuthCacheKey("app-001"))
	require.NoError(t, err)
	assert.NotContains(t, raw, "secret-abc", "app_secret plaintext must not be cached")
}

func TestSha256HexAndCacheKeys(t *testing.T) {
	// SHA-256("abc") 的标准十六进制值。
	assert.Equal(t,
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		sha256Hex("abc"))

	assert.Equal(t, appAuthCacheKeyPrefix+"app-1", appAuthCacheKey("app-1"))
	assert.Equal(t, appAuthNegCacheKeyPrefix+"app-1", appAuthNegCacheKey("app-1"))
}

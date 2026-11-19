package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/trans"
	authzEngine "github.com/tx7do/kratos-authz/engine"
	authzMW "github.com/tx7do/kratos-authz/middleware"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	identityV1 "go-wind-uba/api/gen/go/identity/service/v1"

	"go-wind-uba/pkg/metadata"
)

// fakeTransporter 最小实现的 kratos 服务端传输器,用于模拟 gRPC 形态请求
// (auth 中间件的非 HTTP 分支)。
type fakeTransporter struct {
	operation string
	header    kratosTransport.Header
}

func (f *fakeTransporter) Kind() kratosTransport.Kind            { return kratosTransport.KindGRPC }
func (f *fakeTransporter) Endpoint() string                      { return "fake://auth-test" }
func (f *fakeTransporter) Operation() string                     { return f.operation }
func (f *fakeTransporter) RequestHeader() kratosTransport.Header { return f.header }
func (f *fakeTransporter) ReplyHeader() kratosTransport.Header   { return nil }

// testHeader 实现 kratos transport.Header 接口的简易载体。
type testHeader struct{ m map[string][]string }

func (h *testHeader) Get(key string) string {
	if vs, ok := h.m[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return ""
}
func (h *testHeader) Set(key, value string) {
	if h.m == nil {
		h.m = map[string][]string{}
	}
	h.m[key] = []string{value}
}
func (h *testHeader) Add(key, value string) {
	if h.m == nil {
		h.m = map[string][]string{}
	}
	h.m[key] = append(h.m[key], value)
}
func (h *testHeader) Keys() []string {
	keys := make([]string, 0, len(h.m))
	for k := range h.m {
		keys = append(keys, k)
	}
	return keys
}
func (h *testHeader) Values(key string) []string { return h.m[key] }

// newFakeTransportContext 构造带 gRPC 形态传输上下文的 context,Authorization
// 头按需填入(格式 "Bearer <token>")。header 始终非 nil:authn 引擎对
// RequestHeader() 无 nil 防御,生产实现也恒非 nil。
func newFakeTransportContext(authHeader string) context.Context {
	tr := &fakeTransporter{operation: "/gw.v1.Analytics/EventTrend", header: &testHeader{}}
	if authHeader != "" {
		tr.header.Set("Authorization", authHeader)
	}
	return kratosTransport.NewServerContext(context.Background(), tr)
}

// testPayload 构造一个典型登录态载荷:租户 42、组织单元 7、角色 admin/analyst。
func testPayload() *authenticationV1.UserTokenPayload {
	return &authenticationV1.UserTokenPayload{
		UserId:    1001,
		TenantId:  trans.Ptr(int64(42)),
		OrgUnitId: trans.Ptr(int64(7)),
		Roles:     []string{"admin", "analyst"},
		DataScope: identityV1.DataScope_ALL.Enum(),
	}
}

// alwaysValidChecker 恒返回有效与固定载荷的检查器。
// AccessTokenCheckerFunc 只带 valid 方法,须经组合器补齐 blocker 才满足接口。
func alwaysValidChecker() AccessTokenChecker {
	return NewAccessTokenCheckerFromFuncs(
		func(_ context.Context, _ string, _ bool) (bool, *authenticationV1.UserTokenPayload) {
			return true, testPayload()
		},
		nil,
	)
}

// simpleHandler 返回直接成功的最简 handler。
func simpleHandler() func(ctx context.Context, req any) (any, error) {
	return func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}
}

func TestFromContextRoundtrip(t *testing.T) {
	// 注入后应原样取回同一载荷。
	payload := testPayload()
	ctx := NewContext(context.Background(), payload)
	got, err := FromContext(ctx)
	require.NoError(t, err)
	assert.Same(t, payload, got)

	// 未注入时应返回 ErrMissingJwtToken(401)。
	_, err = FromContext(context.Background())
	require.Error(t, err)
	assert.True(t, errors.IsUnauthorized(err), "缺失 claims 应为 401 未授权错误")
}

func TestErrorsAreUnauthorized(t *testing.T) {
	// 全部错误必须落在 401/UNAUTHORIZED 上,前端依赖该码做统一跳登录。
	errs := map[string]error{
		"ErrWrongContext":                    ErrWrongContext,
		"ErrMissingBearerToken":              ErrMissingBearerToken,
		"ErrValidateTokenFailed":             ErrValidateTokenFailed,
		"ErrMissingJwtToken":                 ErrMissingJwtToken,
		"ErrExtractUserInfoFailed":           ErrExtractUserInfoFailed,
		"ErrExtractSubjectFailed":            ErrExtractSubjectFailed,
		"ErrAccessTokenExpired":              ErrAccessTokenExpired,
		"ErrInvalidRequest":                  ErrInvalidRequest,
		"ErrAccessTokenCheckerNotConfigured": ErrAccessTokenCheckerNotConfigured,
	}
	for name, err := range errs {
		assert.True(t, errors.IsUnauthorized(err), "%s 应为 401 UNAUTHORIZED", name)
	}
}

func TestSetRequestOperationId(t *testing.T) {
	// nil 请求防御:应返回 ErrInvalidRequest 而不是 panic。
	if err := setRequestOperationId(nil, testPayload()); !errors.IsUnauthorized(err) {
		t.Fatalf("nil request 应返回 401, got %v", err)
	}

	// 字段存在且为 *int64 时应注入 UserId。
	payload := testPayload()
	req := &struct{ OperatorId *int64 }{}
	require.NoError(t, setRequestOperationId(req, payload))
	require.NotNil(t, req.OperatorId)
	assert.Equal(t, payload.UserId, *req.OperatorId)

	// 字段不存在时应静默跳过(不是所有请求都需要操作者字段)。
	assert.NoError(t, setRequestOperationId(&struct{ Name string }{}, payload))

	// 字段类型不匹配(非指针)时静默跳过,不得 panic。
	assert.NoError(t, setRequestOperationId(&struct{ OperatorId int64 }{}, payload))
}

func TestSetRequestTenantId(t *testing.T) {
	payload := testPayload()

	// nil 请求防御。
	if err := setRequestTenantId(nil, payload); !errors.IsUnauthorized(err) {
		t.Fatalf("nil request 应返回 401, got %v", err)
	}

	// 字段存在且为 *int64 时应注入租户 ID。
	req := &struct{ TenantId *int64 }{}
	require.NoError(t, setRequestTenantId(req, payload))
	require.NotNil(t, req.TenantId)
	assert.Equal(t, int64(42), *req.TenantId)

	// 载荷租户为 nil 时字段也应为 nil(显式"无租户"语义透传)。
	payloadNoTenant := testPayload()
	payloadNoTenant.TenantId = nil
	reqNil := &struct{ TenantId *int64 }{}
	require.NoError(t, setRequestTenantId(reqNil, payloadNoTenant))
	assert.Nil(t, reqNil.TenantId)

	// 字段不存在时静默跳过。
	assert.NoError(t, setRequestTenantId(&struct{ Name string }{}, payload))

	// 字段为值类型(非指针)时静默跳过 —— FieldByName 找到但 Kind 不符,
	// 注入被静默跳过,不得 panic(与源码注释一致)。
	assert.NoError(t, setRequestTenantId(&struct{ TenantId int64 }{}, payload))
}

func TestComposedChecker(t *testing.T) {
	// valid/blocker 函数都缺失时:默认放行且不阻止。
	c := NewAccessTokenCheckerFromFuncs(nil, nil)
	valid, payload := c.IsValidAccessToken(context.Background(), "tok", false)
	assert.True(t, valid)
	assert.Nil(t, payload)
	assert.False(t, c.IsBlockedAccessToken(context.Background(), "tok"))

	// 两个函数都提供时应原样委托,skipRedis 参数透传。
	var gotToken string
	var gotSkip bool
	calledBlocker := false
	c = NewAccessTokenCheckerFromFuncs(
		func(_ context.Context, token string, skipRedis bool) (bool, *authenticationV1.UserTokenPayload) {
			gotToken, gotSkip = token, skipRedis
			return true, testPayload()
		},
		func(_ context.Context, token string) bool {
			calledBlocker = true
			return token == "blocked"
		},
	)
	valid, payload = c.IsValidAccessToken(context.Background(), "tok-1", true)
	assert.True(t, valid)
	require.NotNil(t, payload)
	assert.Equal(t, "tok-1", gotToken)
	assert.True(t, gotSkip, "skipRedis 参数应透传给 valid 函数")
	assert.True(t, c.IsBlockedAccessToken(context.Background(), "blocked"))
	assert.True(t, calledBlocker)
	assert.False(t, c.IsBlockedAccessToken(context.Background(), "normal"))
}

func TestCheckerFuncAdapters(t *testing.T) {
	// AccessTokenCheckerFunc 适配器:接口方法应调用底层函数。
	called := false
	var f AccessTokenCheckerFunc = func(_ context.Context, _ string, _ bool) (bool, *authenticationV1.UserTokenPayload) {
		called = true
		return true, nil
	}
	_, _ = f.IsValidAccessToken(context.Background(), "t", false)
	assert.True(t, called)

	// AccessTokenBlockerFunc 适配器同上。
	calledBlock := false
	var b AccessTokenBlockerFunc = func(_ context.Context, _ string) bool {
		calledBlock = true
		return false
	}
	assert.False(t, b.IsBlockedAccessToken(context.Background(), "t"))
	assert.True(t, calledBlock)
}

func TestProcessAuthzGRPC(t *testing.T) {
	// gRPC 形态:Resource 取 Operation、Action 取默认 ANY、Subjects 取角色列表。
	tr := &fakeTransporter{operation: "/gw.v1.Analytics/EventTrend"}
	ctx := kratosTransport.NewServerContext(context.Background(), tr)

	ctx2, err := processAuthz(ctx, tr, testPayload())
	require.NoError(t, err)

	claims, ok := authzMW.FromContext(ctx2)
	require.True(t, ok, "processAuthz 后应能取到鉴权 claims")
	require.NotNil(t, claims.Resource)
	assert.Equal(t, "/gw.v1.Analytics/EventTrend", string(*claims.Resource))
	require.NotNil(t, claims.Action)
	assert.Equal(t, "ANY", string(*claims.Action))
	require.NotNil(t, claims.Subjects)
	assert.Equal(t, []string{"admin", "analyst"}, *claims.Subjects)
}

func TestServerMiddleware(t *testing.T) {
	t.Run("无传输上下文", func(t *testing.T) {
		// 非 kratos 传输上下文直接拒绝,不得放行。
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()), WithEnableAuthority(false))
		_, err := mw(simpleHandler())(context.Background(), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("缺BearerToken", func(t *testing.T) {
		// 无 Authorization 头 → 401。
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
		_, err := mw(simpleHandler())(newFakeTransportContext(""), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("Bearer方案不符", func(t *testing.T) {
		// Basic 等其他方案不是 Bearer,应拒绝。
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
		_, err := mw(simpleHandler())(newFakeTransportContext("Basic dXNlcjpwYXNz"), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("Bearer缺token值", func(t *testing.T) {
		// "Bearer" 单词无值,授权串格式非法,应拒绝。
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
		_, err := mw(simpleHandler())(newFakeTransportContext("Bearer"), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("检查器未配置", func(t *testing.T) {
		// 携带合法 Bearer 头但装配漏了 checker → 显式报错,不得静默放行。
		mw := Server()
		_, err := mw(simpleHandler())(newFakeTransportContext("Bearer some-token"), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("令牌无效", func(t *testing.T) {
		// checker 判定无效 → ErrAccessTokenExpired。
		invalid := NewAccessTokenCheckerFromFuncs(
			func(_ context.Context, _ string, _ bool) (bool, *authenticationV1.UserTokenPayload) {
				return false, nil
			},
			nil,
		)
		mw := Server(WithAccessTokenChecker(invalid))
		_, err := mw(simpleHandler())(newFakeTransportContext("Bearer bad-token"), &struct{}{})
		require.Error(t, err)
		assert.True(t, errors.IsUnauthorized(err))
	})

	t.Run("HappyPath默认注入", func(t *testing.T) {
		// 默认开启 authz + injectEnt + injectMetadata:三类上下文应全部就位。
		var (
			gotPayload  *authenticationV1.UserTokenPayload
			gotViewer   bool
			viewerUID   int64
			viewerTID   int64
			gotOperator *authenticationV1.OperatorMetadata
			gotClaims   bool
		)
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
		handler := mw(func(ctx context.Context, req any) (any, error) {
			p, err := FromContext(ctx)
			if err != nil {
				return nil, err
			}
			gotPayload = p
			if vc, ok := viewer.FromContext(ctx); ok {
				gotViewer = true
				viewerUID, viewerTID = vc.UserID(), vc.TenantID()
			}
			// NewContext 注入的是 client 侧 metadata,业务消费方向用 false 读取;
			// (server=true 只读请求头注入的 server metadata,与本注入路径不同向)
			if md, mdErr := metadata.FromContext(ctx, false); mdErr == nil {
				gotOperator = md
			}
			_, gotClaims = authzMW.FromContext(ctx)
			return "ok", nil
		})
		resp, err := handler(newFakeTransportContext("Bearer good-token"), &struct{}{})
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)

		require.NotNil(t, gotPayload)
		assert.Equal(t, int64(1001), gotPayload.UserId)
		assert.True(t, gotViewer, "默认应注入 ent viewer")
		assert.Equal(t, int64(1001), viewerUID)
		assert.Equal(t, int64(42), viewerTID)
		require.NotNil(t, gotOperator, "默认应注入操作者元数据")
		assert.Equal(t, int64(42), gotOperator.GetTenantId())
		assert.True(t, gotClaims, "默认应注入鉴权 claims")
	})

	t.Run("注入OperatorId与TenantId", func(t *testing.T) {
		// 请求结构体的 OperatorId/TenantId 字段应被反射注入。
		type reqWithIDs struct {
			OperatorId *int64
			TenantId   *int64
		}
		mw := Server(
			WithAccessTokenChecker(alwaysValidChecker()),
			WithInjectOperatorId(true),
			WithInjectTenantId(true),
			WithInjectEnt(false),
			WithInjectMetadata(false),
			WithEnableAuthority(false),
		)
		var req *reqWithIDs
		handler := mw(func(ctx context.Context, r any) (any, error) {
			req = r.(*reqWithIDs)
			return nil, nil
		})
		in := &reqWithIDs{}
		_, err := handler(newFakeTransportContext("Bearer good-token"), in)
		require.NoError(t, err)
		require.NotNil(t, req.OperatorId)
		assert.Equal(t, int64(1001), *req.OperatorId)
		require.NotNil(t, req.TenantId)
		assert.Equal(t, int64(42), *req.TenantId)
	})
}

func TestServerMiddlewareViaRealHTTPContext(t *testing.T) {
	// 走真实 kratos HTTP server 上下文(进程内直连 ServeHTTP,无网络监听):
	// 验证 HTTP 形态下 authz 的 Resource/Action 取 PathTemplate/HTTP 方法,
	// 以及 injectMetadata 的 HTTP 分支会把 operator 回写进请求头。
	srv := khttp.NewServer()
	var (
		gotResource string
		gotAction   string
		headerMD    string
		callErr     error
	)
	srv.HandleFunc("/uba/v1/event-trend", func(w http.ResponseWriter, r *http.Request) {
		mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
		_, callErr = mw(func(ctx context.Context, _ any) (any, error) {
			if claims, ok := authzMW.FromContext(ctx); ok {
				if claims.Resource != nil {
					gotResource = string(*claims.Resource)
				}
				if claims.Action != nil {
					gotAction = string(*claims.Action)
				}
			}
			// injectMetadata 的 HTTP 分支会把 operator 写回请求头。
			// 键名与 pkg/metadata 的 mdOperatorKey 保持一致,漂移即测试失败报警。
			headerMD = r.Header.Get("x-md-global-operator")
			return nil, nil
		})(r.Context(), &struct{}{})
	})

	req := httptest.NewRequest(http.MethodGet, "/uba/v1/event-trend", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	require.NoError(t, callErr)
	assert.Equal(t, "/uba/v1/event-trend", gotResource, "HTTP 下 Resource 应为 PathTemplate")
	assert.Equal(t, http.MethodGet, gotAction, "HTTP 下 Action 应为请求方法")
	assert.NotEmpty(t, headerMD, "HTTP 下操作者元数据应回写进请求头")
}

func TestAuthzClaimsDefaultsShape(t *testing.T) {
	// 断言默认 Action 常量:非 HTTP 传输一律 ANY(读/写/删共用一个宽动作,
	// 细粒度交给 Resource 上的策略)。
	assert.Equal(t, authzEngine.Action("ANY"), defaultAction)
}

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	kratosTransport "github.com/go-kratos/kratos/v2/transport"
	grpctrans "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// 典型 Chrome 桌面 UA,多处用例与 benchmark 共用。
const testDesktopUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// withKratosHTTPContext 用真实 kratos HTTP server 的中间件链构造与生产一致的
// *khttp.Transport 上下文,并在注册的 handler 里执行 run。
//
// 为什么要绕真实 server:enrich.go 的 requestFromContext 只认 *khttp.Transport,
// 该类型的 request 字段未导出,包外无法直接构造;直连 ServeHTTP(无网络监听,
// 纯进程内)即可拿到与生产 handler 完全一致的上下文,同时顺带验证 kratos
// 中间件链下的真实行为。run 中的 panic 会被捕获并转为错误,保证「不 panic」
// 断言成立。
func withKratosHTTPContext(t testing.TB, req *http.Request, run func(ctx context.Context)) {
	t.Helper()

	srv := khttp.NewServer()

	var runErr error
	srv.HandleFunc("/uba/v1/report", func(w http.ResponseWriter, r *http.Request) {
		runErr = func() (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					err = assert.AnError
					t.Logf("panic in enrich under kratos context: %v", rec)
				}
			}()
			run(r.Context())
			return nil
		}()
	})

	srv.ServeHTTP(httptest.NewRecorder(), req)
	require.NoError(t, runErr)
}

// newReportRequest 构造打到注册路由的上报请求,并按需填头。
func newReportRequest(headers map[string]string, remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/uba/v1/report", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	return req
}

func TestRequestFromContext_NonHTTPTransportReturnsNil(t *testing.T) {
	// gRPC 等非 HTTP 传输的服务端上下文:类型断言失败应安全返回 nil,不得 panic。
	// 空 Composite 字面量即可(字段均未导出,不需要也不允许填充)。
	tr := &grpctrans.Transport{}
	ctx := kratosTransport.NewServerContext(context.Background(), tr)
	assert.Nil(t, requestFromContext(ctx))
}

func TestEnrichBehaviorFromRequest_FullChainViaHTTPContext(t *testing.T) {
	// 全链路:XFF 链首补真实 IP、请求头 UA 解析 os/browser/device_type、
	// 请求头 referer 兜底 —— ci 为 nil,全部字段均由服务端从 HTTP 请求补出。
	// geo:203.0.113.7 为公网段,测试环境查不查得出结果都不断言具体值,
	// 仅要求流程不 panic(由 withKratosHTTPContext 保证)。
	ev := &ubaV1.BehaviorEvent{}
	req := newReportRequest(map[string]string{
		"X-Forwarded-For": "203.0.113.7, 10.0.0.1, 198.51.100.2",
		"User-Agent":      testDesktopUA,
		"Referer":         "https://example.com/products/list?page=2",
	}, "10.0.0.5:1234")

	withKratosHTTPContext(t, req, func(ctx context.Context) {
		enrichBehaviorFromRequest(ctx, ev, nil)
	})

	assert.Equal(t, "203.0.113.7", ev.GetIp(), "IP 应取 XFF 链首")
	assert.NotEmpty(t, ev.GetOs(), "os 应从请求头 UA 解析")
	assert.NotEmpty(t, ev.GetContext()["browser"], "browser 应解析进 context")
	assert.Equal(t, "desktop", ev.GetContext()["device_type"])
	assert.Equal(t, "https://example.com/products/list?page=2", ev.GetReferer(), "referer 应从请求头兜底")
}

func TestEnrichBehaviorFromRequest_ExplicitIPAndRefererWin(t *testing.T) {
	// SDK 显式上报的 IP 与 referer 不能被请求头覆盖;UA 仍用 ci 自报值。
	ev := &ubaV1.BehaviorEvent{
		Ip:      trans.Ptr("198.51.100.99"),
		Referer: trans.Ptr("https://sdk.example.com/landing"),
	}
	ci := &ubaV1.ClientInfo{UserAgent: testDesktopUA}
	req := newReportRequest(map[string]string{
		"X-Forwarded-For": "203.0.113.8",
		"User-Agent":      "curl/8.0",
		"Referer":         "https://header.example.com/page",
	}, "10.0.0.5:1234")

	withKratosHTTPContext(t, req, func(ctx context.Context) {
		enrichBehaviorFromRequest(ctx, ev, ci)
	})

	assert.Equal(t, "198.51.100.99", ev.GetIp(), "已有 IP 不得被 XFF 覆盖")
	assert.Equal(t, "https://sdk.example.com/landing", ev.GetReferer(), "已有 referer 不得被请求头覆盖")
	assert.NotEmpty(t, ev.GetOs(), "os 仍应按 ci 的 UA 解析")
}

func TestEnrichBehaviorFromRequest_GeoQueryOnPublicIPDoesNotPanicNorOverwrite(t *testing.T) {
	// 公网 IP 触发 geo 查询:测试环境 GeoLite 库是否存在未知,不断言具体国家/城市,
	// 只钉两条不变量 —— 不 panic;且已有值(客户端自报)绝不被查询结果覆盖。
	ev := &ubaV1.BehaviorEvent{
		Ip:      trans.Ptr("198.51.100.7"),
		Country: trans.Ptr("ClientCountry"),
		IpCity:  trans.Ptr("ClientCity"),
	}
	enrichBehaviorFromRequest(context.Background(), ev, nil)

	if ev.Country != nil {
		assert.Equal(t, "ClientCountry", ev.GetCountry())
	}
	if ev.IpCity != nil {
		assert.Equal(t, "ClientCity", ev.GetIpCity())
	}

	// 变体:country/city 全空 → 需要查询;同样只要求不 panic(值或 nil 或非空)。
	evEmpty := &ubaV1.BehaviorEvent{Ip: trans.Ptr("198.51.100.8")}
	enrichBehaviorFromRequest(context.Background(), evEmpty, nil)
	if evEmpty.Country != nil {
		assert.NotEmpty(t, evEmpty.GetCountry(), "查询成功才写入,不应落空串")
	}
	if evEmpty.IpCity != nil {
		assert.NotEmpty(t, evEmpty.GetIpCity())
	}
}

func TestEnrichBehaviorFromRequest_NoUserAgentAnywhereLeavesContextNil(t *testing.T) {
	// 无 ci UA、无请求头 UA:不应创建 context(不写 browser/device_type),
	// os/referer/ip 也不得凭空生成。
	ev := &ubaV1.BehaviorEvent{}
	enrichBehaviorFromRequest(context.Background(), ev, nil)

	assert.Nil(t, ev.Context, "没有 UA 来源时不应创建 context")
	assert.Nil(t, ev.Os)
	assert.Nil(t, ev.Referer)
	assert.Nil(t, ev.Ip)
}

func TestEnrichBehaviorFromRequest_UATruncationUnderHTTPContext(t *testing.T) {
	// 请求头 UA 补进 context 的 browser 值也应走 trimAndLimit 上限(64 字节)。
	longBrowserUA := "Mozilla/5.0 (" + longString(120) + ") Chrome/120.0 Safari/537.36"
	ev := &ubaV1.BehaviorEvent{}
	req := newReportRequest(map[string]string{"User-Agent": longBrowserUA}, "10.0.0.5:1234")

	withKratosHTTPContext(t, req, func(ctx context.Context) {
		enrichBehaviorFromRequest(ctx, ev, nil)
	})

	if browser, ok := ev.GetContext()["browser"]; ok {
		assert.LessOrEqual(t, len(browser), 64, "browser 值应截断到 64 字节")
	}
}

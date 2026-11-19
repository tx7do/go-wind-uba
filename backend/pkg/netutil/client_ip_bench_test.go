package netutil

// 本文件为 client_ip.go 的三个 context 便捷函数补 benchmark。
// 功能覆盖（非 HTTP 形态零值、正常提取路径）已由 context_transport_test.go
// 完成，这里只补每请求热路径的性能基线，复用其测试桩。

import (
	"context"
	"net/http"
	"testing"

	gonetutil "github.com/tx7do/go-utils/netutil"

	"github.com/go-kratos/kratos/v2/transport"
)

// benchHTTPCtx 构造带 HTTP 传输层的 server context（每请求形态）。
func benchHTTPCtx(b *testing.B, req *http.Request) context.Context {
	b.Helper()
	gonetutil.SetTrustedProxies(trustedV4CIDRs())
	return transport.NewServerContext(context.Background(), &stubHTTPTransporter{req: req})
}

// BenchmarkClientIPFromContext 采集网关按事件取客户端 IP 的热路径。
func BenchmarkClientIPFromContext(b *testing.B) {
	req := buildReq("203.0.113.7:54321", map[string]string{
		"X-Forwarded-For": "198.51.100.9",
	})
	ctx := benchHTTPCtx(b, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ip := ClientIPFromContext(ctx); ip == "" {
			b.Fatal("expected non-empty ip")
		}
	}
}

// BenchmarkHeaderFromContext service 层读取请求头参数的热路径。
func BenchmarkHeaderFromContext(b *testing.B) {
	req := buildReq("203.0.113.7:54321", map[string]string{"X-Marker": "present"})
	ctx := benchHTTPCtx(b, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := HeaderFromContext(ctx)
		if h == nil {
			b.Fatal("expected non-nil header")
		}
	}
}

// BenchmarkCookieFromContext 刷新令牌接口读 HttpOnly cookie 的热路径。
func BenchmarkCookieFromContext(b *testing.B) {
	req := buildReq("203.0.113.7:54321", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "rt-bench"})
	ctx := benchHTTPCtx(b, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v := CookieFromContext(ctx, "refresh_token"); v != "rt-bench" {
			b.Fatalf("unexpected cookie value: %q", v)
		}
	}
}

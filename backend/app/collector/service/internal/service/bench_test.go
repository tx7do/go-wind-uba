package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/mileusna/useragent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// captureKratosHTTPContext 构造一次与生产一致的 *khttp.Transport 上下文供循环复用:
// 通过真实 kratos HTTP server 中间件(直连 ServeHTTP,无网络监听)拿到 handler 视角的
// context,开销只发生在 benchmark 预热阶段,计时区间内只跑被测函数。
func captureKratosHTTPContext(b *testing.B, headers map[string]string, remoteAddr string) context.Context {
	b.Helper()

	srv := khttp.NewServer()
	var captured context.Context
	srv.HandleFunc("/uba/v1/report", func(w http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})

	req := httptest.NewRequest(http.MethodPost, "/uba/v1/report", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = remoteAddr

	srv.ServeHTTP(httptest.NewRecorder(), req)
	if captured == nil {
		b.Fatal("failed to capture kratos http context")
	}
	return captured
}

// BenchmarkEnrichBehaviorFromRequest 行为事件富化全流程(IP/XFF 提取 + UA 解析 + geo 兜底)。
// 典型 Web 事件:SDK 自报 UA,请求带完整 XFF/UA/Referer 头。
// 注:事件 IP 落在公网段时 geo 查询会随迭代执行(库缺失则每次 Query 返回错误),
// 与生产行为一致,数字即真实开销。
func BenchmarkEnrichBehaviorFromRequest(b *testing.B) {
	ctx := captureKratosHTTPContext(b, map[string]string{
		"X-Forwarded-For": "203.0.113.7, 10.0.0.1, 198.51.100.2",
		"User-Agent":      testDesktopUA,
		"Referer":         "https://example.com/products/list?page=2",
	}, "10.0.0.5:1234")
	ci := &ubaV1.ClientInfo{UserAgent: testDesktopUA}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 每次迭代新事件:Context map 会被写入,复用会污染测量。
		ev := &ubaV1.BehaviorEvent{}
		enrichBehaviorFromRequest(ctx, ev, ci)
	}
}

// BenchmarkRealClientIP 真实 IP 提取的两条主路径:XFF 链 3 项(需逐项解析+去空格)与
// 仅 RemoteAddr(无任何代理头,最快路径)。
func BenchmarkRealClientIP(b *testing.B) {
	xffReq := httptest.NewRequest(http.MethodPost, "/report", nil)
	xffReq.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1, 198.51.100.2")
	xffReq.RemoteAddr = "10.0.0.5:1234"

	remoteReq := httptest.NewRequest(http.MethodPost, "/report", nil)
	remoteReq.RemoteAddr = "198.51.100.77:5678"

	b.Run("xff_chain_3", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = realClientIP(xffReq)
		}
	})

	b.Run("remote_addr_only", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = realClientIP(remoteReq)
		}
	})
}

// BenchmarkUseragentParse UA 解析单独成层:enrich 的主要开销来源,便于定位耗时构成。
func BenchmarkUseragentParse(b *testing.B) {
	cases := []struct {
		name string
		ua   string
	}{
		{
			name: "chrome_desktop",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		{
			name: "iphone_safari",
			ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		},
		{
			name: "bot",
			ua:   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = useragent.Parse(c.ua)
			}
		})
	}
}

// BenchmarkTrimAndLimit 截断函数的三种典型形状:短串(不触发截断)、
// 超长串(触发按字节截断)、接近真实 referer 上限的输入。
func BenchmarkTrimAndLimit(b *testing.B) {
	short := "hello"
	over64 := strings.Repeat("a", 96)                            // 96B 输入,超出 city/geo 的 64 上限
	referer := "https://example.com/" + strings.Repeat("x", 300) // 320B 输入,超出 referer 的 256 上限

	b.Run("short_under_limit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = trimAndLimit(short, 256)
		}
	})

	b.Run("over_64b_limit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = trimAndLimit(over64, 64)
		}
	})

	b.Run("referer_256b_limit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = trimAndLimit(referer, 256)
		}
	})
}

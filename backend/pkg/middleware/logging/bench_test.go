package logging

import (
	"net/http"
	"testing"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/go-utils/trans"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
)

// BenchmarkGetClientRealIP 逐请求都会执行的真实 IP 提取(XFF 链解析)。
func BenchmarkGetClientRealIP(b *testing.B) {
	req := newHeaderReq(map[string]string{
		"X-Forwarded-For": "203.0.113.7, 10.0.0.1, 198.51.100.2",
	}, "10.0.0.5:1234")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if ClientRealIP(req) == "" {
			b.Fatal("expected ip")
		}
	}
}

// BenchmarkGetRequestId 请求 ID 提取(有头 vs 需生成)。
func BenchmarkGetRequestId(b *testing.B) {
	withHeader := newHeaderReq(map[string]string{"X-Request-ID": "rid-bench"}, "")
	withoutHeader := newHeaderReq(nil, "")
	b.Run("header_present", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if getRequestId(withHeader) == "" {
				b.Fatal("expected id")
			}
		}
	})
	b.Run("generated", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if getRequestId(withoutHeader) == "" {
				b.Fatal("expected id")
			}
		}
	})
}

// BenchmarkDetectPlatformFromUA UA 平台启发式判定(审计逐条调用)。
func BenchmarkDetectPlatformFromUA(b *testing.B) {
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
		"okhttp/4.9.0",
		"CFNetwork/1410 Darwin/22.4.0",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		detectPlatformFromUA(uas[i%len(uas)])
	}
}

// BenchmarkTokenHash 审计存储的 token 摘要计算。
func BenchmarkTokenHash(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(tokenHash("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.bench")) != 64 {
			b.Fatal("bad hash")
		}
	}
}

// BenchmarkComputeRiskScore 登录风险评分(登录请求逐条计算)。
func BenchmarkComputeRiskScore(b *testing.B) {
	lm := NewLoginAuditLogMiddleware(&options{})
	la := &auditLoginLog{
		Status:    trans.Ptr(auditV1.LoginAuditLog_FAILED),
		Username:  trans.Ptr("alice"),
		IpAddress: trans.Ptr("203.0.113.7"),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lm.computeRiskScore(la)
	}
}

// BenchmarkComputeRiskFactors 风险因子计算。
func BenchmarkComputeRiskFactors(b *testing.B) {
	lm := NewLoginAuditLogMiddleware(&options{})
	la := &auditLoginLog{
		Status:        trans.Ptr(auditV1.LoginAuditLog_FAILED),
		Username:      trans.Ptr("alice"),
		IpAddress:     trans.Ptr("203.0.113.7"),
		MfaStatus:     trans.Ptr("MFA_UNVERIFIED"),
		FailureReason: trans.Ptr("incorrect password"),
		RiskScore:     trans.Ptr(uint32(85)),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lm.computeRiskFactors(la)
	}
}

// BenchmarkFillDeviceInfo 设备信息拼装(含 UA 解析,登录/API 审计逐条调用)。
func BenchmarkFillDeviceInfo(b *testing.B) {
	withKratosTransport(b, http.MethodGet, "/bench", map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
	}, "", func(htr *khttp.Transport) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if info := fillDeviceInfo(htr, nil); info == nil {
				b.Fatal("nil device info")
			}
		}
	})
}

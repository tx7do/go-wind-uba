package auth

import (
	"context"
	"testing"
)

// BenchmarkFromContext 每个业务 handler 都会调用一次的 claims 读取路径。
func BenchmarkFromContext(b *testing.B) {
	ctx := NewContext(context.Background(), testPayload())
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := FromContext(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSetRequestTenantId 反射注入是每请求开销,量化其成本。
func BenchmarkSetRequestTenantId(b *testing.B) {
	payload := testPayload()
	req := &struct{ TenantId *int64 }{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := setRequestTenantId(req, payload); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkServerMiddlewareHappyPath 完整中间件链的每请求成本
// (默认注入 ent viewer + 操作者元数据 + authz claims)。
func BenchmarkServerMiddlewareHappyPath(b *testing.B) {
	mw := Server(WithAccessTokenChecker(alwaysValidChecker()))
	handler := mw(simpleHandler())
	ctx := newFakeTransportContext("Bearer bench-token")
	req := &struct{}{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := handler(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkServerMiddlewareNoInjection 只校验令牌、关闭一切注入的最小链路,
// 与 HappyPath 对比可以拆出注入成本。
func BenchmarkServerMiddlewareNoInjection(b *testing.B) {
	mw := Server(
		WithAccessTokenChecker(alwaysValidChecker()),
		WithInjectEnt(false),
		WithInjectMetadata(false),
		WithEnableAuthority(false),
	)
	handler := mw(simpleHandler())
	ctx := newFakeTransportContext("Bearer bench-token")
	req := &struct{}{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := handler(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

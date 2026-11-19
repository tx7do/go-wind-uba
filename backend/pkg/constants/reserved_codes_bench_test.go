package constants

// 保留权限码/角色码判定在鉴权链路逐请求调用（登录态管理员标志、越权闸门），
// 提供 benchmark 基线。实现为前缀/常量比较，预期零分配。

import (
	"testing"
)

func BenchmarkIsReservedPermissionCode(b *testing.B) {
	cases := []struct {
		name string
		code string
	}{
		{"reserved", "sys:access_backend"},
		{"plain", "biz:user:create"},
		{"empty", ""},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = IsReservedPermissionCode(tc.code)
			}
		})
	}
}

func BenchmarkIsReservedRoleCode(b *testing.B) {
	cases := []struct {
		name string
		code string
	}{
		{"builtin", "platform:admin"},
		{"tenant-plain", "tenant:sales"},
		{"empty", ""},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = IsReservedRoleCode(tc.code)
			}
		})
	}
}

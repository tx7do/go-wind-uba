package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsReservedPermissionCode 保留权限码判定：sys: 命名空间全数命中，
// 业务码放行。该判定是权限 CRUD 的越权闸门——sys:access_backend 在登录
// 链路按码值精确匹配，误放行可让自建权限伪造登录资格。
func TestIsReservedPermissionCode(t *testing.T) {
	cases := []struct {
		name string
		code string
		want bool
	}{
		{"access backend", SystemAccessBackendPermissionCode, true},
		{"manage tenants", SystemManageTenantsPermissionCode, true},
		{"platform admin", SystemPlatformAdminPermissionCode, true},
		{"sys namespace other", "sys:foo", true},
		{"sys namespace bare prefix", SystemPermissionCodePrefix, true},
		{"biz code allowed", "biz:user:create", false},
		{"plain code allowed", "user:create", false},
		{"sys lookalike not prefix", "xsys:foo", false},
		{"case sensitive", "Sys:foo", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsReservedPermissionCode(tc.code))
		})
	}
}

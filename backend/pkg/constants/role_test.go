package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsReservedRoleCode 保留角色码判定：内置身份码与保留命名空间全数命中，
// 普通码（含非内置的 tenant: 前缀值）放行。该判定是角色 CRUD 的越权闸门，
// 误放行会让自建角色伪装成内置身份（登录态管理员标志按码值精确匹配）。
func TestIsReservedRoleCode(t *testing.T) {
	cases := []struct {
		name string
		code string
		want bool
	}{
		{"exact platform admin", PlatformAdminRoleCode, true},
		{"exact tenant manager", TenantAdminRoleCode, true},
		{"exact tenant admin template", TenantAdminTemplateRoleCode, true},
		{"platform namespace", "platform:foo", true},
		{"platform namespace bare prefix", PlatformRoleCodePrefix, true},
		{"template namespace", "template:foo", true},
		{"template namespace bare prefix", TemplateRoleCodePrefix, true},
		{"tenant non-builtin allowed", "tenant:sales", false},
		{"tenant namespace bare prefix", TenantRoleCodePrefix, false},
		{"tenant prefix lookalike", "xtenant:manager", false},
		{"plain code", "admin", false},
		{"case sensitive", "Platform:admin", false},
		{"case sensitive builtin", "Tenant:Manager", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsReservedRoleCode(tc.code))
		})
	}
}

package constants

// default_data.go 是空库部署的种子数据：权限组/权限/角色/角色元数据/用户/
// 凭证/用户角色/语言/菜单。种子数据在 core 启动时一次性写入，写入后不再
// 校验，因此"数据自洽"只能靠这里的静态不变量测试兜底：
//   - 引用完整性：角色引用权限 ID、菜单引用父菜单、权限引用菜单/权限组
//     （ID 按切片顺序隐式生成，顺序漂移 = 引用错位）；
//   - 语义不变量：种子凭证必须满足等保口令复杂度（否则被 prepareCredential
//     静默拒绝，admin 永久无法登录）；
//   - 权限闸门：菜单 Authority 与种子权限/角色码全部落在保留命名空间。

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authenticationV1 "go-wind-uba/api/gen/go/authentication/service/v1"
	permissionV1 "go-wind-uba/api/gen/go/permission/service/v1"
	resourceV1 "go-wind-uba/api/gen/go/resource/service/v1"
)

// ---------------------------------------------------------------------------
// 口令复杂度（种子凭证能否落库的前提）
// ---------------------------------------------------------------------------

// TestDefaultUserPasswordComplexity 等保口令复杂度：≥8 位且至少含三类字符。
// 不满足时 prepareCredential 会静默拒绝种子凭证 → 空库部署后 admin 永久无法登录。
func TestDefaultUserPasswordComplexity(t *testing.T) {
	pw := DefaultUserPassword
	assert.GreaterOrEqual(t, len(pw), DefaultPasswordMinLen, "口令长度必须达到最小阈值")

	classes := 0
	hasLower := strings.ContainsAny(pw, "abcdefghijklmnopqrstuvwxyz")
	hasUpper := strings.ContainsAny(pw, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasDigit := strings.ContainsAny(pw, "0123456789")
	hasSpecial := strings.ContainsFunc(pw, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", r)
	})
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSpecial} {
		if ok {
			classes++
		}
	}
	assert.GreaterOrEqual(t, classes, 3, "口令必须至少包含三类字符")
}

// TestPlatformTenantID 平台租户 ID 恒为 0（种子用户/凭证/角色关系都以 0 挂靠平台租户）。
func TestPlatformTenantID(t *testing.T) {
	assert.Equal(t, int64(0), PlatformTenantID)
}

// ---------------------------------------------------------------------------
// 权限组 / 权限（隐式 ID 按切片顺序生成）
// ---------------------------------------------------------------------------

// TestDefaultPermissionGroups 权限组结构：数量、命名空间、状态、父子层级。
func TestDefaultPermissionGroups(t *testing.T) {
	require.Len(t, DefaultPermissionGroups, 5)

	seenNames := map[string]bool{}
	for i, g := range DefaultPermissionGroups {
		require.NotNil(t, g.Name)
		assert.False(t, seenNames[g.GetName()], "权限组重名: %s", g.GetName())
		seenNames[g.GetName()] = true

		assert.Equal(t, SystemPermissionModule, g.GetModule(), "种子权限组必须挂在 sys 模块, idx=%d", i)
		assert.Equal(t, permissionV1.PermissionGroup_ON, g.GetStatus(), "种子权限组必须启用, idx=%d", i)
		assert.True(t, strings.HasPrefix(g.GetPath(), "/"), "路径应以 / 开头, idx=%d", i)

		if i == 0 {
			assert.Nil(t, g.ParentId, "首个权限组必须是根, idx=0")
		} else {
			assert.Equal(t, int64(1), g.GetParentId(), "其余权限组必须挂在第 1 组之下, idx=%d", i)
		}
	}
}

// TestDefaultPermissions_ImplicitIdConvention 角色的 Permissions 引用的是
// "切片顺序隐式 ID"，前五个 ID 对应的权限码必须按此顺序钉死，
// 顺序漂移会让 DefaultRoles 的引用错位（把审计权限发给错误角色）。
func TestDefaultPermissions_ImplicitIdConvention(t *testing.T) {
	require.GreaterOrEqual(t, len(DefaultPermissions), 5)
	expected := []string{
		SystemAccessBackendPermissionCode, // ID=1
		SystemPlatformAdminPermissionCode, // ID=2
		SystemTenantManagerPermissionCode, // ID=3
		SystemManageTenantsPermissionCode, // ID=4
		SystemAuditLogsPermissionCode,     // ID=5
	}
	for i, want := range expected {
		assert.Equal(t, want, DefaultPermissions[i].GetCode(),
			"隐式 ID=%d 的权限码漂移，DefaultRoles 引用将错位", i+1)
	}
}

// TestDefaultPermissions 权限数据自洽：码唯一、状态启用、权限组引用合法、
// 受保护权限码全部出现在种子中、MenuIds 引用真实存在的菜单。
func TestDefaultPermissions(t *testing.T) {
	menuIDs := collectMenuIDs(t)
	seenCodes := map[string]bool{}

	for i, p := range DefaultPermissions {
		code := p.GetCode()
		assert.False(t, seenCodes[code], "权限码重复: %s", code)
		seenCodes[code] = true

		assert.Equal(t, permissionV1.Permission_ON, p.GetStatus(), "种子权限必须启用, idx=%d", i)
		assert.True(t, IsReservedPermissionCode(code), "种子权限码必须为系统保留命名空间: %s", code)

		gid := p.GetGroupId()
		assert.GreaterOrEqual(t, gid, int64(1), "权限组引用非法, idx=%d", i)
		assert.LessOrEqual(t, gid, int64(len(DefaultPermissionGroups)),
			"权限组引用越界（隐式 ID 只到 %d）, idx=%d", len(DefaultPermissionGroups), i)

		for _, mid := range p.GetMenuIds() {
			assert.Contains(t, menuIDs, mid, "权限 %s 引用了不存在的菜单 ID %d", code, mid)
		}
	}

	for _, protected := range ProtectedPermissionCodes {
		assert.Contains(t, seenCodes, protected,
			"受保护权限码 %s 必须出现在种子权限中（否则删除后无重建来源）", protected)
	}
}

// ---------------------------------------------------------------------------
// 角色 / 角色元数据
// ---------------------------------------------------------------------------

// TestDefaultRoles 种子角色：平台管理员（SYSTEM）+ 租户管理员模板（TEMPLATE），
// 代码/名称/保护位/权限引用逐一钉死。
func TestDefaultRoles(t *testing.T) {
	require.Len(t, DefaultRoles, 2)

	admin := DefaultRoles[0]
	assert.Equal(t, DefaultPlatformAdminRoleName, admin.GetName())
	assert.Equal(t, PlatformAdminRoleCode, admin.GetCode())
	assert.Equal(t, permissionV1.Role_SYSTEM, admin.GetType())
	assert.True(t, admin.GetIsProtected(), "内置角色必须受保护")
	// Permissions=[1,2,4]（隐式 ID）：access_backend / platform_admin / manage_tenants。
	assert.Equal(t, []int64{1, 2, 4}, admin.GetPermissions())
	assertRolesReferenceValidPermissions(t, admin.GetPermissions())

	tpl := DefaultRoles[1]
	assert.Equal(t, DefaultTenantManagerRoleName+"模板", tpl.GetName())
	assert.Equal(t, TenantAdminTemplateRoleCode, tpl.GetCode())
	assert.Equal(t, permissionV1.Role_TEMPLATE, tpl.GetType())
	assert.True(t, tpl.GetIsProtected())
	assert.Equal(t, []int64{1, 3}, tpl.GetPermissions())
	assertRolesReferenceValidPermissions(t, tpl.GetPermissions())
}

// assertRolesReferenceValidPermissions 角色 Permissions 的隐式 ID 必须落在
// DefaultPermissions 范围内。
func assertRolesReferenceValidPermissions(t *testing.T, permIDs []int64) {
	t.Helper()
	for _, id := range permIDs {
		assert.GreaterOrEqual(t, id, int64(1))
		assert.LessOrEqual(t, id, int64(len(DefaultPermissions)), "角色引用了越界权限 ID %d", id)
	}
}

// TestDefaultRoleMetadata 角色元数据与角色按索引一一对应：平台角色 =
// 非模板 + PLATFORM 域；模板角色 IsTemplate + TemplateFor 指向实例角色码。
func TestDefaultRoleMetadata(t *testing.T) {
	require.Len(t, DefaultRoleMetadata, len(DefaultRoles))

	platform := DefaultRoleMetadata[0]
	assert.Equal(t, int64(1), platform.GetRoleId(), "元数据第 1 条必须对应第 1 个角色（平台管理员）")
	assert.False(t, platform.GetIsTemplate())
	assert.Equal(t, permissionV1.RoleMetadata_PLATFORM, platform.GetScope())
	assert.Equal(t, permissionV1.RoleMetadata_AUTO, platform.GetSyncPolicy())

	tpl := DefaultRoleMetadata[1]
	assert.Equal(t, int64(2), tpl.GetRoleId())
	assert.True(t, tpl.GetIsTemplate())
	assert.Equal(t, TenantAdminRoleCode, tpl.GetTemplateFor(), "模板必须声明实例化为租户管理员")
	assert.Equal(t, permissionV1.RoleMetadata_TENANT, tpl.GetScope())
	assert.Equal(t, permissionV1.RoleMetadata_AUTO, tpl.GetSyncPolicy())
}

// ---------------------------------------------------------------------------
// 用户 / 凭证 / 用户角色（三表联动）
// ---------------------------------------------------------------------------

// TestDefaultUsersAndCredentialsAndRoles 种子管理员三表一致性：
// users(1) ↔ credentials(UserId=1, Identifier=admin) ↔ user_roles(UserId=1, RoleId=1=平台管理员)。
func TestDefaultUsersAndCredentialsAndRoles(t *testing.T) {
	require.Len(t, DefaultUsers, 1)
	u := DefaultUsers[0]
	assert.Equal(t, DefaultAdminUserName, u.GetUsername())
	assert.Equal(t, PlatformTenantID, u.GetTenantId())
	assert.NotEmpty(t, u.GetEmail())
	assert.NotEmpty(t, u.GetRealname())

	require.Len(t, DefaultUserCredentials, 1)
	c := DefaultUserCredentials[0]
	assert.Equal(t, int64(1), c.GetUserId(), "凭证必须挂在第 1 个用户上")
	assert.Equal(t, PlatformTenantID, c.GetTenantId())
	assert.Equal(t, DefaultAdminUserName, c.GetIdentifier())
	assert.Equal(t, DefaultUserPassword, c.GetCredential(), "种子凭证必须与默认口令一致")
	assert.Equal(t, authenticationV1.UserCredential_USERNAME, c.GetIdentityType())
	assert.Equal(t, authenticationV1.UserCredential_PASSWORD_HASH, c.GetCredentialType())
	assert.True(t, c.GetIsPrimary())
	assert.Equal(t, authenticationV1.UserCredential_ENABLED, c.GetStatus())

	require.Len(t, DefaultUserRoles, 1)
	ur := DefaultUserRoles[0]
	assert.Equal(t, int64(1), ur.GetUserId())
	assert.Equal(t, int64(1), ur.GetRoleId(), "第 1 个用户必须绑第 1 个角色（平台管理员）")
	assert.Equal(t, PlatformTenantID, ur.GetTenantId())
	assert.True(t, ur.GetIsPrimary())
	assert.Equal(t, permissionV1.UserRole_ACTIVE, ur.GetStatus())
}

// ---------------------------------------------------------------------------
// 语言
// ---------------------------------------------------------------------------

// TestDefaultLanguages 语言种子：码唯一、全部启用、默认语言唯一且为 zh-CN。
func TestDefaultLanguages(t *testing.T) {
	seenCodes := map[string]bool{}
	defaults := 0
	for i, l := range DefaultLanguages {
		code := l.GetLanguageCode()
		assert.False(t, seenCodes[code], "语言码重复: %s", code)
		seenCodes[code] = true

		assert.NotEmpty(t, l.GetLanguageName(), "语言名缺失, idx=%d", i)
		assert.NotEmpty(t, l.GetNativeName(), "母语名缺失, idx=%d", i)
		assert.True(t, l.GetIsEnabled(), "种子语言必须启用, idx=%d", i)
		if l.GetIsDefault() {
			defaults++
		}
	}
	assert.Equal(t, 1, defaults, "默认语言必须恰好一个")
	assert.True(t, DefaultLanguages[0].GetIsDefault(), "zh-CN 必须是默认语言")
	assert.Equal(t, "zh-CN", DefaultLanguages[0].GetLanguageCode())
}

// ---------------------------------------------------------------------------
// 菜单
// ---------------------------------------------------------------------------

// collectMenuIDs 收集全部菜单 ID（含编译期对种子数量的宽松 sanity check）。
func collectMenuIDs(t *testing.T) map[int64]bool {
	t.Helper()
	ids := make(map[int64]bool, len(DefaultMenus))
	for _, m := range DefaultMenus {
		id := m.GetId()
		assert.Greater(t, id, int64(0), "菜单 ID 必须为正")
		assert.False(t, ids[id], "菜单 ID 重复: %d", id)
		ids[id] = true
	}
	return ids
}

// TestDefaultMenus 菜单种子自洽：ID 唯一、父引用完整、类型合法、
// Meta 齐备、Authority 全部为保留权限码。
func TestDefaultMenus(t *testing.T) {
	require.NotEmpty(t, DefaultMenus)
	ids := collectMenuIDs(t)

	for i, m := range DefaultMenus {
		if pid := m.GetParentId(); pid != 0 {
			assert.Contains(t, ids, pid, "菜单 %s 引用了不存在的父菜单 %d", m.GetName(), pid)
			assert.NotEqual(t, m.GetId(), pid, "菜单不能以自己为父, idx=%d", i)
		}
		assert.NotEmpty(t, m.GetName(), "菜单名缺失, idx=%d", i)
		assert.NotEmpty(t, m.GetPath(), "菜单路径缺失, idx=%d", i)
		assert.Contains(t, []resourceV1.Menu_Type{resourceV1.Menu_CATALOG, resourceV1.Menu_MENU}, m.GetType(),
			"种子菜单类型只能是目录/菜单, idx=%d", i)
		if m.GetType() == resourceV1.Menu_MENU {
			assert.NotEmpty(t, m.GetComponent(), "叶子菜单必须带组件路径, idx=%d", i)
		}
		require.NotNil(t, m.Meta, "种子菜单必须带 Meta, idx=%d", i)
		assert.NotEmpty(t, m.GetMeta().GetTitle(), "Meta.Title（i18n key）缺失, idx=%d", i)

		for _, auth := range m.GetMeta().GetAuthority() {
			assert.True(t, IsReservedPermissionCode(auth),
				"菜单 %s 的 Authority %s 必须为系统保留权限码", m.GetName(), auth)
		}
	}
}

// TestDefaultMenus_NoDuplicatePathUnderParent 同一父菜单下路径不得重复
// （前端路由注册会互相覆盖）。
func TestDefaultMenus_NoDuplicatePathUnderParent(t *testing.T) {
	seen := map[string]bool{} // key: parentID|path
	for _, m := range DefaultMenus {
		key := fmt.Sprintf("%d|%s", m.GetParentId(), m.GetPath())
		assert.False(t, seen[key], "同一父菜单下路径重复: parent=%d path=%s", m.GetParentId(), m.GetPath())
		seen[key] = true
	}
}

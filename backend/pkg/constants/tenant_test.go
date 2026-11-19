package constants

// tenant.go 的租户模式常量契约：is iota 布局被序列化到 DB 与 API 两侧，
// 数值漂移 = 老数据语义错乱。

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUserTenantRelationTypeValues 关系类型 iota 布局钉死。
func TestUserTenantRelationTypeValues(t *testing.T) {
	assert.Equal(t, UserTenantRelationType(0), UserTenantRelationNone, "iota 起点：未启用多租户")
	assert.Equal(t, UserTenantRelationType(1), UserTenantRelationOneToOne, "一对一 = 1")
	assert.Equal(t, UserTenantRelationType(2), UserTenantRelationOneToMany, "一对多 = 2")
}

// TestDefaultUserTenantRelationType 默认关联关系必须是一对一
// （注释承诺：启用多租户时推荐一对一）。
func TestDefaultUserTenantRelationType(t *testing.T) {
	assert.Equal(t, UserTenantRelationOneToOne, DefaultUserTenantRelationType)
}

// TestIsTenantModeEnabled 默认值下租户模式必须处于启用态
// （平台管理/租户管理链路依赖该语义）。
func TestIsTenantModeEnabled(t *testing.T) {
	assert.True(t, IsTenantModeEnabled,
		"DefaultUserTenantRelationType 为 OneToOne 时租户模式应启用")
}

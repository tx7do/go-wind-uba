package serviceid

// serviceid 包是服务注册发现的命名契约：服务名常量 + 两个地址构造函数。
// 命名一旦漂移（改常量、改前缀格式），注册中心里就会出现新旧两套服务名，
// 导致服务间 gRPC 发现失败，因此这里把"契约值"逐字钉死。

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestServiceNameConstants 服务名常量契约：值逐字钉死，改动即测试失败
// （改前必须评估注册中心里旧名的下线）。
func TestServiceNameConstants(t *testing.T) {
	assert.Equal(t, "core-service", CoreService, "核心分析服务")
	assert.Equal(t, "syncer-service", SyncerService, "同步服务")
	assert.Equal(t, "admin-service", AdminService, "后台网关")
	assert.Equal(t, "collector-service", CollectorService, "埋点网关")

	// 四个服务名彼此互异，防止复制粘贴改名造成的撞名。
	names := []string{CoreService, SyncerService, AdminService, CollectorService}
	seen := map[string]bool{}
	for _, n := range names {
		assert.False(t, seen[n], "服务名重复: %s", n)
		seen[n] = true
	}
}

// TestProjectName 项目发现命名空间前缀。
func TestProjectName(t *testing.T) {
	assert.Equal(t, "gowind-uba", ProjectName)
}

// TestNewDiscoveryName 发现名 = 项目名/服务名（etcd key 的第一段）。
func TestNewDiscoveryName(t *testing.T) {
	cases := []struct {
		name    string
		service string
		want    string
	}{
		{"core 服务", CoreService, "gowind-uba/core-service"},
		{"admin 服务", AdminService, "gowind-uba/admin-service"},
		{"collector 服务", CollectorService, "gowind-uba/collector-service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NewDiscoveryName(tc.service))
		})
	}

	// 空服务名只产生尾随斜杠（不 panic）；契约上调用方总是传常量。
	assert.Equal(t, ProjectName+"/", NewDiscoveryName(""))
}

// TestMakeDiscoveryAddress 发现地址 = discovery:/// 服务名
// （kratos etcd registrar 的 scheme 约定，格式漂移会导致注册/发现不匹配）。
func TestMakeDiscoveryAddress(t *testing.T) {
	cases := []struct {
		name    string
		service string
		want    string
	}{
		{"core 服务", CoreService, "discovery:///core-service"},
		{"syncer 服务", SyncerService, "discovery:///syncer-service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MakeDiscoveryAddress(tc.service))
		})
	}

	// 稳定性：同输入重复调用结果一致。
	assert.Equal(t, MakeDiscoveryAddress(CoreService), MakeDiscoveryAddress(CoreService))
}

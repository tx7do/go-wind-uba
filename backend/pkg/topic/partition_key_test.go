package topic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tx7do/go-utils/trans"
)

// BehaviorEventKey 决定事件在 Kafka 分区上的分布：同一登录用户必须恒定落同一分区
// （下游按 user 维度聚合/去重依赖这个），匿名用户回退设备维度。
func TestBehaviorEventKey(t *testing.T) {
	// 登录用户：tenant:user 维度，且是哈希值（非明文）。
	k1 := BehaviorEventKey("t1", trans.Ptr(uint32(7)), "dev-a")
	assert.Equal(t, k1, BehaviorEventKey("t1", trans.Ptr(uint32(7)), "dev-b"),
		"同一用户换设备也应同分区（user 优先于 device）")
	assert.Len(t, k1, 16, "sha256 前 8 字节的十六进制应为 16 字符")

	// userID=0 视为匿名：回退 device 维度。
	k2 := BehaviorEventKey("t1", trans.Ptr(uint32(0)), "dev-a")
	assert.Equal(t, k2, BehaviorEventKey("t1", nil, "dev-a"),
		"userID=0 与 nil 应同走设备维度")
	assert.NotEqual(t, k1, k2)

	// 不同租户必须不同分区键（防跨租户串扰）。
	assert.NotEqual(t,
		BehaviorEventKey("t1", trans.Ptr(uint32(7)), ""),
		BehaviorEventKey("t2", trans.Ptr(uint32(7)), ""))

	// 稳定性：同输入重复调用结果一致。
	assert.Equal(t, k1, BehaviorEventKey("t1", trans.Ptr(uint32(7)), "dev-a"))
}

func TestRiskEventKey(t *testing.T) {
	assert.Equal(t, RiskEventKey("t1", "HIGH"), RiskEventKey("t1", "HIGH"))
	assert.NotEqual(t, RiskEventKey("t1", "HIGH"), RiskEventKey("t1", "LOW"))
	assert.NotEqual(t, RiskEventKey("t1", "HIGH"), RiskEventKey("t2", "HIGH"))
}

func TestSyncConfigKey(t *testing.T) {
	// 配置同步 key 不做哈希：保证相同 key 到相同分区，且可读。
	assert.Equal(t, "table:pk-1", SyncConfigKey("table", "pk-1"))
	assert.Equal(t, SyncConfigKey("t", "1"), SyncConfigKey("t", "1"))
}

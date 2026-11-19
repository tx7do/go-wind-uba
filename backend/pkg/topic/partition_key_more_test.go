package topic

// 本文件补齐 partition_key_test.go 的缺口，并覆盖高频采集路径的 benchmark：
//   1. 负数 userID 的分支归属（userID <= 0 一律回退设备维度）；
//   2. hashKey 的输出形态（16 位小写十六进制 = sha256 前 8 字节）与
//      采样规模下的唯一性（分区键冲突会把不同用户的事件并到同分区）；
//   3. partition key 计算是采集网关逐事件调用的热路径，需性能基线。

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tx7do/go-utils/trans"
)

// TestBehaviorEventKey_NegativeUserID 负数 userID 视为无效（非合法登录态），
// 与 0/nil 一样回退设备维度。
func TestBehaviorEventKey_NegativeUserID(t *testing.T) {
	neg := BehaviorEventKey("t1", trans.Ptr(int64(-5)), "dev-a")
	dev := BehaviorEventKey("t1", nil, "dev-a")
	zero := BehaviorEventKey("t1", trans.Ptr(int64(0)), "dev-a")

	assert.Equal(t, dev, neg, "负数 userID 应与 nil 同走设备维度")
	assert.Equal(t, dev, zero, "0 与 nil 同走设备维度")
}

// TestBehaviorEventKey_HashFormat 哈希输出形态：16 字符小写十六进制。
// 下游 Doris/ClickHouse 分桶表达式依赖该长度稳定性。
func TestBehaviorEventKey_HashFormat(t *testing.T) {
	k := BehaviorEventKey("t1", trans.Ptr(int64(1)), "d")
	assert.Len(t, k, 16)
	for _, c := range k {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"哈希输出应为小写十六进制，含 %q", c)
	}
}

// TestHashKey_DeterministicAndDistinct hashKey 白盒测试：确定性 + 采样唯一性。
// 10000 个不同输入在 8 字节空间内应零碰撞（生日界远未触达）。
func TestHashKey_DeterministicAndDistinct(t *testing.T) {
	assert.Equal(t, hashKey("same"), hashKey("same"), "同输入必须同哈希")
	assert.NotEqual(t, hashKey("a"), hashKey("b"), "不同输入必须不同哈希")

	seen := map[string]bool{}
	for i := 0; i < 10000; i++ {
		h := hashKey(fmt.Sprintf("tenant:%d", i))
		assert.False(t, seen[h], "采样哈希出现碰撞: %s", h)
		seen[h] = true
	}
}

// TestRiskEventKey_HashOutput 风险事件键同样走哈希形态（明文风险级别不出现在分区键中）。
func TestRiskEventKey_HashOutput(t *testing.T) {
	k := RiskEventKey("t1", "HIGH")
	assert.Len(t, k, 16, "风险事件键应为 16 字符哈希")
	assert.NotEqual(t, "t1:HIGH", k)
}

// ---------------------------------------------------------------------------
// benchmark：采集网关逐事件热路径
// ---------------------------------------------------------------------------

// BenchmarkBehaviorEventKey_UserID 登录用户事件（采集路径最常见形态）。
func BenchmarkBehaviorEventKey_UserID(b *testing.B) {
	uid := trans.Ptr(int64(42))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if k := BehaviorEventKey("tenant-1", uid, "device-1"); len(k) != 16 {
			b.Fatal("unexpected key length")
		}
	}
}

// BenchmarkBehaviorEventKey_DeviceID 匿名设备事件（首访/未登录形态）。
func BenchmarkBehaviorEventKey_DeviceID(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if k := BehaviorEventKey("tenant-1", nil, "device-1"); len(k) != 16 {
			b.Fatal("unexpected key length")
		}
	}
}

// BenchmarkRiskEventKey 风控事件入队路径。
func BenchmarkRiskEventKey(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if k := RiskEventKey("tenant-1", "HIGH"); len(k) != 16 {
			b.Fatal("unexpected key length")
		}
	}
}

// BenchmarkSyncConfigKey 配置同步路径（不哈希、直接拼接）。
func BenchmarkSyncConfigKey(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if k := SyncConfigKey("sys_users", "42"); k != "sys_users:42" {
			b.Fatal("unexpected key")
		}
	}
}

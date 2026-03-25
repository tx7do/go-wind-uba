package topic

import (
	"crypto/sha256"
	"fmt"
)

// BehaviorEventKey 行为事件分区键
func BehaviorEventKey(tenantID string, userID *uint32, deviceID string) string {
	var key string
	if userID != nil && *userID > 0 {
		key = fmt.Sprintf("%s:%d", tenantID, *userID)
	} else {
		key = fmt.Sprintf("%s:%s", tenantID, deviceID)
	}
	return hashKey(key)
}

// RiskEventKey 风险事件分区键
func RiskEventKey(tenantID, riskLevel string) string {
	return hashKey(fmt.Sprintf("%s:%s", tenantID, riskLevel))
}

// SyncConfigKey 配置同步分区键
func SyncConfigKey(tableName, primaryKey string) string {
	return fmt.Sprintf("%s:%s", tableName, primaryKey) // 不用 hash，保证相同 key 到相同分区
}

// hashKey SHA256 哈希（取前 8 字节作为 int64）
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:8])
}

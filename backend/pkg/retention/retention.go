// Package retention 承载按表的数据留存(TTL)策略:配置加载、校验与两个
// OLAP 引擎的留存语义转换。留存的实际执行归引擎自身——Doris 由动态分区
// 历史窗口自动删除过期分区,ClickHouse 由 TTL 表达式清理——本包只生成
// 幂等的对账 DDL,由 uba-ingest 的 `retention` 命令下发(企业能力,需 license)。
//
// 默认留存已固化在开仓 DDL 里(Doris dynamic_partition.start=-180 /
// ClickHouse TTL INTERVAL 180 DAY);本包只在策略显式覆盖时对账,未列出的表
// 保持 DDL 默认。
package retention

import (
	"encoding/json"
	"fmt"
	"os"
)

// Rule 单表留存规则。
type Rule struct {
	// Table 表名(不含库名,如 events_fact)。
	Table string `json:"table"`
	// Days 留存天数,≥1。
	Days int `json:"days"`
	// TimeColumn ClickHouse TTL 依据的日期列(如 event_date);Doris 动态分区
	// 表可留空。ClickHouse 规则必填。
	TimeColumn string `json:"time_column,omitempty"`
}

// Policy 按表留存策略。
type Policy []Rule

// LoadFromFile 从 JSON 文件加载并校验;文件不存在视为未启用(nil, nil)。
func LoadFromFile(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("retention: read %s: %w", path, err)
	}
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("retention: parse %s: %w", path, err)
	}
	seen := map[string]bool{}
	for i, r := range policy {
		if r.Table == "" {
			return nil, fmt.Errorf("retention: rule #%d: table required", i)
		}
		if r.Days < 1 {
			return nil, fmt.Errorf("retention: rule #%d(%s): days must be >= 1", i, r.Table)
		}
		if seen[r.Table] {
			return nil, fmt.Errorf("retention: rule #%d(%s): duplicate table", i, r.Table)
		}
		seen[r.Table] = true
	}
	return policy, nil
}

// DorisSetDynamicPartitionStart 生成对账动态分区历史窗口的 DDL:
// 起始 = "-<days>",引擎据此自动删除更旧的分区。
func DorisSetDynamicPartitionStart(qualifiedTable string, days int) string {
	return fmt.Sprintf("ALTER TABLE %s SET (\"dynamic_partition.start\" = \"-%d\")", qualifiedTable, days)
}

// ClickHouseModifyTTL 生成对账 TTL 的 DDL,镜像开仓 DDL 的
// `TTL <date_col> + INTERVAL n DAY` 语义(默认删除过期行,配合
// ttl_only_drop_parts 只落整分区)。
func ClickHouseModifyTTL(qualifiedTable, timeColumn string, days int) string {
	return fmt.Sprintf("ALTER TABLE %s MODIFY TTL %s + INTERVAL %d DAY", qualifiedTable, timeColumn, days)
}

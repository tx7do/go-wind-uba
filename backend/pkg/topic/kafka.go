package topic

const (
	UbaEventRaw      = "uba.events.raw"      // 原始行为事件上报
	UbaEventEnriched = "uba.events.enriched" // 丰富后的行为事件上报
	UbaEventRisk     = "uba.risk.events"     // 风险事件上报
	UbaEventPath     = "uba.path.events"     // 行为路径事件上报

	UbaSyncMysql2Doris           = "uba.sync.mysql2doris" // MySQL 配置 → Doris 同步
	UbaSyncMysql2Clickhouse      = "uba.sync.mysql2ch"    // MySQL 配置 → ClickHouse 同步
	UbaSyncPostgresql2Doris      = "uba.sync.pg2doris"    // Postgresql 配置 → Doris 同步
	UbaSyncPostgresql2Clickhouse = "uba.sync.pg2ch"       // Postgresql 配置 → ClickHouse 同步
	UbaSyncDoris2Elasticsearch   = "uba.sync.doris2es"    // Doris 分析结果 → ES 同步

	UbaAlerts   = "uba.alerts"     // 风控告警通知
	UbaAuditLog = "uba.audit.logs" // 操作审计日志
	UbaDlq      = "uba.dlq.events" // 死信队列（处理失败事件）
)

const (
	UbaGroupIngestDoris      = "uba-ingest-doris"  // 事件入湖 Doris 处理组
	UbaGroupIngestClickHouse = "uba-ingest-ch"     // 事件入湖 ClickHouse 处理组
	UbaGroupEnrichGeo        = "uba-enrich-geo"    // 地理位置丰富处理组
	UbaGroupRiskDetector     = "uba-risk-detector" // 风险检测处理组
	UbaGroupPathComputer     = "uba-path-computer" // 行为路径计算处理组
	UbaGroupAlertSender      = "uba-alert-sender"  // 告警发送处理组
	UbaGroupSyncDoris        = "uba-sync-doris"    // 配置同步 Doris 处理组
	UbaGroupDlqProcessor     = "uba-dlq-processor" // 死信队列处理组
)

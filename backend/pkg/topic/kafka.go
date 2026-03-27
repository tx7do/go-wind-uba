package topic

const (
	UbaEventRaw      = "uba_events_raw"      // 原始行为事件上报
	UbaEventEnriched = "uba_events_enriched" // 丰富后的行为事件上报
	UbaEventRisk     = "uba_risk_events"     // 风险事件上报
	UbaEventPath     = "uba_path_events"     // 行为路径事件上报

	UbaSyncMysql2Doris           = "uba_sync_mysql2doris" // MySQL 配置 → Doris 同步
	UbaSyncMysql2Clickhouse      = "uba_sync_mysql2ch"    // MySQL 配置 → ClickHouse 同步
	UbaSyncPostgresql2Doris      = "uba_sync_pg2doris"    // Postgresql 配置 → Doris 同步
	UbaSyncPostgresql2Clickhouse = "uba_sync_pg2ch"       // Postgresql 配置 → ClickHouse 同步
	UbaSyncDoris2Elasticsearch   = "uba_sync_doris2es"    // Doris 分析结果 → ES 同步

	UbaAlerts   = "uba_alerts"     // 风控告警通知
	UbaAuditLog = "uba_audit_logs" // 操作审计日志
	UbaDlq      = "uba_dlq_events" // 死信队列（处理失败事件）
)

const (
	UbaGroupIngestDoris      = "uba_ingest_doris"  // 事件入湖 Doris 处理组
	UbaGroupIngestClickHouse = "uba_ingest_ch"     // 事件入湖 ClickHouse 处理组
	UbaGroupEnrichGeo        = "uba_enrich_geo"    // 地理位置丰富处理组
	UbaGroupRiskDetector     = "uba_risk_detector" // 风险检测处理组
	UbaGroupPathComputer     = "uba_path_computer" // 行为路径计算处理组
	UbaGroupAlertSender      = "uba_alert_sender"  // 告警发送处理组
	UbaGroupSyncDoris        = "uba_sync_doris"    // 配置同步 Doris 处理组
	UbaGroupDlqProcessor     = "uba_dlq_processor" // 死信队列处理组
)

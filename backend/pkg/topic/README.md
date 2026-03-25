# Kafka Topic 设计文档

## Topic 列表设计

### 1. 核心业务 Topic

| Topic 名称              | 用途                   | 分区数   | 副本数 | 保留策略        | 消息格式     |
|-----------------------|----------------------|-------|-----|-------------|----------|
| `uba.events.raw`      | 原始行为事件上报             | 32-64 | 3   | 7d / 50GB   | Protobuf |
| `uba.events.enriched` | enriched 事件（补全地理位置等） | 32-64 | 3   | 3d / 30GB   | Protobuf |
| `uba.risk.events`     | 风险事件（高优先级）           | 8-16  | 3   | 30d / 100GB | Protobuf |
| `uba.path.events`     | 路径/会话事件              | 16-32 | 3   | 7d / 30GB   | Protobuf |

### 2. 数据同步 Topic

| Topic 名称               | 用途                            | 分区数 | 副本数 | 保留策略      | 消息格式 |
|------------------------|-------------------------------|-----|-----|-----------|------|
| `uba.sync.mysql2doris` | MySQL 配置 → Doris 同步           | 8   | 3   | 1d / 10GB | JSON |
| `uba.sync.mysql2ch`    | MySQL 配置 → ClickHouse 同步      | 8   | 3   | 1d / 10GB | JSON |
| `uba.sync.pg2doris`    | Postgresql 配置 → Doris 同步      | 8   | 3   | 1d / 10GB | JSON |
| `uba.sync.pg2ch`       | Postgresql 配置 → ClickHouse 同步 | 8   | 3   | 1d / 10GB | JSON |
| `uba.sync.doris2es`    | Doris 分析结果 → ES 同步            | 8   | 3   | 1d / 10GB | JSON |

### 3. 系统运维 Topic

| Topic 名称         | 用途           | 分区数 | 副本数 | 保留策略        | 消息格式     |
|------------------|--------------|-----|-----|-------------|----------|
| `uba.alerts`     | 风控告警通知       | 4   | 3   | 7d / 5GB    | JSON     |
| `uba.audit.logs` | 操作审计日志       | 8   | 3   | 90d / 200GB | JSON     |
| `uba.dlq.events` | 死信队列（处理失败事件） | 8   | 3   | 30d / 50GB  | Protobuf |

## 消费者组（Consumer Group）设计

| Consumer Group      | 订阅 Topic               | 用途                 | 并发度       |
|---------------------|------------------------|--------------------|-----------|
| `uba-ingest-doris`  | `uba.events.raw`       | 写入 Doris 事实表       | 48（= 分区数） |
| `uba-ingest-ch`     | `uba.events.raw`       | 写入 ClickHouse 事实表  | 48        |
| `uba-enrich-geo`    | `uba.events.raw`       | IP → 地理位置解析        | 16        |
| `uba-risk-detector` | `uba.events.enriched`  | 实时风控规则匹配           | 32        |
| `uba-path-computer` | `uba.events.enriched`  | 路径特征计算             | 16        |
| `uba-alert-sender`  | `uba.risk.events`      | 高风险告警通知            | 4         |
| `uba-sync-doris`    | `uba.sync.mysql2doris` | MySQL → Doris 配置同步 | 8         |
| `uba-dlq-processor` | `uba.dlq.events`       | 死信队列重试/归档          | 4         |

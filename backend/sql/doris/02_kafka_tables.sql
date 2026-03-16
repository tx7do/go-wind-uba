-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
-- ============================================================


-- ============================================================
-- 1. Kafka 引擎表 - 行为事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_events_raw (
    event_id           STRING COMMENT '全局唯一事件ID',
    tenant_id          INT COMMENT '租户ID',
    user_id            INT COMMENT '登录用户ID',
    device_id          STRING COMMENT '设备ID',
    account_id         STRING COMMENT '业务账号ID',
    global_user_id     STRING DEFAULT '',
    event_time         DATETIMEV2(3) COMMENT '客户端事件时间',
    event_category     STRING COMMENT '事件大类',
    event_name         STRING COMMENT '事件名称',
    event_action       STRING COMMENT '事件动作',
    object_type        STRING COMMENT '对象类型',
    object_id          STRING COMMENT '对象ID',
    object_name        STRING DEFAULT '',
    session_id         INT DEFAULT 0,
    session_seq        INT DEFAULT 0,
    platform           STRING DEFAULT '',
    os                 STRING DEFAULT '',
    app_version        STRING DEFAULT '',
    channel            STRING DEFAULT '',
    ip                 STRING DEFAULT '',
    ip_city            STRING DEFAULT '',
    country            STRING DEFAULT '',
    network            STRING DEFAULT '',
    context            STRING DEFAULT '{}',
    duration_ms        INT DEFAULT 0,
    amount             DECIMAL(18,2) DEFAULT 0,
    quantity           INT DEFAULT 0,
    score              INT DEFAULT 0,
    metrics            STRING DEFAULT '{}',
    properties         STRING DEFAULT '{}',
    op_result          STRING DEFAULT '',
    error_code         STRING DEFAULT '',
    risk_level         STRING DEFAULT 'normal',
    trace_id           STRING DEFAULT ''
)
    ENGINE=KAFKA
    PROPERTIES (
                   "kafka.broker.list" = "kafka:9092",
                   "kafka.topic" = "uba_events",
                   "kafka.group.id" = "uba_doris_consumer_events",
                   "kafka.scan.mode" = "earliest",
                   "kafka.format" = "json",
                   "kafka.consumer.num" = "3",
                   "json.strip_outer_array" = "false"
               );


-- ============================================================
-- 2. Kafka 引擎表 - 风险事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_risk_events_raw (
    risk_id            STRING COMMENT '风险事件唯一ID',
    tenant_id          INT COMMENT '租户ID',
    user_id            INT COMMENT '登录用户ID',
    device_id          STRING COMMENT '设备ID',
    global_user_id     STRING DEFAULT '',
    risk_type          STRING COMMENT '风险类型',
    risk_level         STRING COMMENT '风险等级',
    risk_score         FLOAT COMMENT '风险评分',
    rule_id            INT COMMENT '触发规则ID',
    rule_name          STRING COMMENT '规则名称',
    rule_context       STRING DEFAULT '{}',
    related_event_ids  ARRAY<STRING> COMMENT '关联事件ID列表',
    session_id         INT DEFAULT 0,
    description        STRING COMMENT '风险描述',
    evidence           STRING DEFAULT '{}',
    status             STRING DEFAULT 'pending',
    handler_id         STRING DEFAULT '',
    handled_time       DATETIMEV2(3) NULL,
    handle_remark      STRING DEFAULT '',
    occur_time         DATETIMEV2(3) COMMENT '风险发生时间',
    report_time        DATETIMEV2(3) DEFAULT CURRENT_TIMESTAMP(3)
)
    ENGINE=KAFKA
    PROPERTIES (
                   "kafka.broker.list" = "kafka:9092",
                   "kafka.topic" = "uba_risk_events",
                   "kafka.group.id" = "uba_doris_consumer_risk",
                   "kafka.scan.mode" = "earliest",
                   "kafka.format" = "json",
                   "kafka.consumer.num" = "2",
                   "json.strip_outer_array" = "false"
               );


-- ============================================================
-- 3. 异步导入任务 - Kafka 行为事件 → 事实表
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_events_to_fact
ON events_fact
COLUMNS(event_date=date(event_time)),
COLUMNS(event_ts=unix_timestamp(event_time)*1000),
COLUMNS(server_time=CURRENT_TIMESTAMP(3)),
COLUMNS(created_at=CURRENT_TIMESTAMP),
COLUMNS(updated_at=CURRENT_TIMESTAMP)
PROPERTIES
(
    "desired_concurrent_number" = "3",
    "max_batch_interval" = "10",
    "max_batch_rows" = "100000",
    "format" = "json"
)
FROM KAFKA
(
    "kafka_broker_list" = "kafka:9092",
    "kafka_topic" = "uba_events",
    "kafka_group_id" = "uba_doris_routine_events",
    "kafka_consumer_offset_reset" = "earliest"
);


-- ============================================================
-- 4. 异步导入任务 - Kafka 风险事件 → 风险表
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_risk_events_to_fact
ON risk_events
COLUMNS(event_date=date(occur_time)),
COLUMNS(created_at=CURRENT_TIMESTAMP),
COLUMNS(updated_at=CURRENT_TIMESTAMP)
PROPERTIES
(
    "desired_concurrent_number" = "2",
    "max_batch_interval" = "10",
    "format" = "json"
)
FROM KAFKA
(
    "kafka_broker_list" = "kafka:9092",
    "kafka_topic" = "uba_risk_events",
    "kafka_group_id" = "uba_doris_routine_risk",
    "kafka_consumer_offset_reset" = "earliest"
);

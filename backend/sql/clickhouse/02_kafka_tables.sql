-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
-- ============================================================


--- ============================================================
-- 1. Kafka 引擎表 - 行为事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_events_raw
(
    event_id       String,
    tenant_id      UInt32,
    user_id        UInt32,
    device_id      String,
    account_id     String,
    global_user_id String,
    event_time     DateTime64(3),
    event_category String,
    event_name     String,
    event_action   String,
    object_type    String,
    object_id      String,
    object_name    String,
    session_id     UInt64,
    session_seq    UInt32,
    platform       String,
    os             String,
    app_version    String,
    channel        String,
    user_agent     String,
    ip             String,
    ip_city        String,
    country        String,
    network        String,
    geo            String,
    referer        String,

    context        String,
    duration_ms    UInt32,
    amount         Decimal(18, 2),
    quantity       UInt32,
    score          Int32,
    metrics        String,
    properties     String,
    op_result      String,
    error_code     String,
    risk_level     String,
    trace_id       String
    )
    ENGINE = Kafka
    SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'uba.events.raw',
    kafka_group_name = 'uba-ingest-ch',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_skip_broken_messages = 1;


-- ============================================================
-- 2. Kafka 引擎表 - 风险事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_risk_events_raw
(
    id                UInt64,
    tenant_id         UInt32,
    user_id           UInt32,
    device_id         String,
    global_user_id    String,
    risk_type         String,
    risk_level        String,
    risk_score        Float32,
    rule_id           UInt32,
    rule_name         String,
    rule_context      String,
    related_event_ids Array(String),
    session_id        UInt64,
    description       String,
    evidence          String,
    status            String,
    handler_id        String,
    handled_time      Nullable(DateTime64(3)),
    handle_remark     String,
    occur_time        Nullable(DateTime64(3)),
    report_time       Nullable(DateTime64(3))
    )
    ENGINE = Kafka
    SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'uba.risk.events',
    kafka_group_name = 'uba-alert-sender',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536,
    kafka_skip_broken_messages = 1,
    kafka_commit_on_select = 1,
    input_format_json_read_numbers_as_strings = 0;


-- ============================================================
-- 3. 物化视图 - Kafka 行为事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_events_to_fact
    TO gw_uba.events_fact
AS
SELECT event_id,
       tenant_id,
       user_id,
       device_id,
       account_id,
       global_user_id,
       event_time,
       now64(3)                                       AS server_time,
       event_category,
       event_name,
       event_action,
       object_type,
       object_id,
       object_name,
       session_id,
       session_seq,
       platform,
       os,
       app_version,
       channel,
       ip,
       ip_city,
       country,
       network,
       geo,
       referer,
       JSONExtract(context, 'Map(String, String)')    AS context,
       duration_ms,
       amount,
       quantity,
       score,
       JSONExtract(metrics, 'Map(String, Float64)')   AS metrics,
       JSONExtract(properties, 'Map(String, String)') AS properties,
       op_result,
       error_code,
       risk_level,
       trace_id,
       now()                                          AS created_at,
       now()                                          AS updated_at
FROM gw_uba.kafka_events_raw;


-- ============================================================
-- 4. 物化视图 - Kafka 风险事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_risk_events_to_fact
    TO gw_uba.risk_events
AS
SELECT id,
       tenant_id,
       user_id,
       device_id,
       global_user_id,
       risk_type,
       risk_level,
       risk_score,
       rule_id,
       rule_name,
       JSONExtract(rule_context, 'Map(String, String)')           AS rule_context,
       related_event_ids,
       session_id,
       description,
       JSONExtract(evidence, 'Map(String, String)')               AS evidence,
       status,
       handler_id,
       handled_time,
       handle_remark,
       occur_time,
       if(report_time = '0000-00-00 00:00:00', NULL, report_time) AS report_time,
       now()                                                      AS created_at,
       now()                                                      AS updated_at
FROM gw_uba.kafka_risk_events_raw;

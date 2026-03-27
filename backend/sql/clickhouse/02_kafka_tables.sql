-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
-- ============================================================


--- ============================================================
-- 1. Kafka 引擎表 - 行为事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_events_raw
(
    raw String
)
    ENGINE = Kafka
    SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'uba_events_raw',
    kafka_group_name = 'uba_ingest_ch',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 3,
    kafka_skip_broken_messages = 1,
    kafka_commit_on_select = 1;


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
SELECT JSONExtractString(raw, 'event_id')                                  AS event_id,
       toUInt32(JSONExtract(raw, 'tenant_id', 'UInt32'))                   AS tenant_id,
       toUInt32(JSONExtract(raw, 'user_id', 'UInt32'))                     AS user_id,
       JSONExtractString(raw, 'device_id')                                 AS device_id,
       JSONExtractString(raw, 'account_id')                                AS account_id,
       JSONExtractString(raw, 'global_user_id')                            AS global_user_id,

       parseDateTime64BestEffort(JSONExtractString(raw, 'event_time'), 3)  AS event_time,
       parseDateTime64BestEffort(JSONExtractString(raw, 'server_time'), 3) AS server_time,
--        now64(3)                                                            AS server_time,

       JSONExtractString(raw, 'event_category')                            AS event_category,
       JSONExtractString(raw, 'event_name')                                AS event_name,
       JSONExtractString(raw, 'event_action')                              AS event_action,

       JSONExtractString(raw, 'object_type')                               AS object_type,
       JSONExtractString(raw, 'object_id')                                 AS object_id,
       JSONExtractString(raw, 'object_name')                               AS object_name,

       toUInt64(JSONExtract(raw, 'session_id', 'UInt64'))                  AS session_id,
       toUInt32(JSONExtract(raw, 'session_seq', 'UInt32'))                 AS session_seq,

       JSONExtractString(raw, 'platform')                                  AS platform,
       JSONExtractString(raw, 'os')                                        AS os,
       JSONExtractString(raw, 'app_version')                               AS app_version,
       JSONExtractString(raw, 'channel')                                   AS channel,
       JSONExtractString(raw, 'user_agent')                                AS user_agent,

       JSONExtractString(raw, 'ip')                                        AS ip,
       JSONExtractString(raw, 'ip_city')                                   AS ip_city,
       JSONExtractString(raw, 'country')                                   AS country,
       JSONExtractString(raw, 'network')                                   AS network,
       JSONExtractString(raw, 'geo')                                       AS geo,
       JSONExtractString(raw, 'referer')                                   AS referer,

       JSONExtract(raw, 'context', 'Map(String, String)')                  AS context,

       toUInt32(JSONExtract(raw, 'duration_ms', 'UInt32'))                 AS duration_ms,
       toDecimal128(JSONExtract(raw, 'amount', 'Decimal(18,2)'), 2)        AS amount,
       toUInt32(JSONExtract(raw, 'quantity', 'UInt32'))                    AS quantity,
       toInt32(JSONExtract(raw, 'score', 'Int32'))                         AS score,

       JSONExtract(raw, 'metrics', 'Map(String, Float64)')                 AS metrics,
       JSONExtract(raw, 'properties', 'Map(String, String)')               AS properties,

       JSONExtractString(raw, 'op_result')                                 AS op_result,
       JSONExtractString(raw, 'error_code')                                AS error_code,
       JSONExtractString(raw, 'risk_level')                                AS risk_level,
       JSONExtractString(raw, 'trace_id')                                  AS trace_id,

       now()                                                               AS created_at,
       now()                                                               AS updated_at

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

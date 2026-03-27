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
    raw String
)
    ENGINE = Kafka
        SETTINGS
            kafka_broker_list = 'kafka:9092',
            kafka_topic_list = 'uba_risk_events',
            kafka_group_name = 'uba_alert_sender',
            kafka_format = 'JSONAsString',
            kafka_num_consumers = 3,
            kafka_skip_broken_messages = 1,
            kafka_commit_on_select = 1;


-- ============================================================
-- 3. 物化视图 - Kafka 行为事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_events_to_fact
    TO gw_uba.events_fact
AS
SELECT JSONExtractString(raw, 'eventId')                                  AS event_id,
       toUInt32(JSONExtract(raw, 'tenantId', 'UInt32'))                   AS tenant_id,
       toUInt32(JSONExtract(raw, 'userId', 'UInt32'))                     AS user_id,
       JSONExtractString(raw, 'deviceId')                                 AS device_id,
       JSONExtractString(raw, 'accountId')                                AS account_id,
       JSONExtractString(raw, 'globalUserId')                            AS global_user_id,

       parseDateTime64BestEffort(JSONExtractString(raw, 'eventTime'), 3)  AS event_time,
       parseDateTime64BestEffort(JSONExtractString(raw, 'serverTime'), 3) AS server_time,

       JSONExtractString(raw, 'eventCategory')                            AS event_category,
       JSONExtractString(raw, 'eventName')                                AS event_name,
       JSONExtractString(raw, 'eventAction')                              AS event_action,

       JSONExtractString(raw, 'objectType')                               AS object_type,
       JSONExtractString(raw, 'objectId')                                 AS object_id,
       JSONExtractString(raw, 'objectName')                               AS object_name,

       toUInt64(JSONExtract(raw, 'sessionId', 'UInt64'))                  AS session_id,
       toUInt32(JSONExtract(raw, 'sessionSeq', 'UInt32'))                 AS session_seq,

       JSONExtractString(raw, 'platform')                                  AS platform,
       JSONExtractString(raw, 'os')                                        AS os,
       JSONExtractString(raw, 'appVersion')                               AS app_version,
       JSONExtractString(raw, 'channel')                                   AS channel,
       JSONExtractString(raw, 'userAgent')                                AS user_agent,

       JSONExtractString(raw, 'ip')                                        AS ip,
       JSONExtractString(raw, 'ipCity')                                   AS ip_city,
       JSONExtractString(raw, 'country')                                   AS country,
       JSONExtractString(raw, 'network')                                   AS network,
       JSONExtractString(raw, 'geo')                                       AS geo,
       JSONExtractString(raw, 'referer')                                   AS referer,

       JSONExtract(raw, 'context', 'Map(String, String)')                  AS context,

       toUInt32(JSONExtract(raw, 'durationMs', 'UInt32'))                 AS duration_ms,
       toDecimal128(JSONExtract(raw, 'amount', 'Decimal(18,2)'), 2)        AS amount,
       toUInt32(JSONExtract(raw, 'quantity', 'UInt32'))                    AS quantity,
       toInt32(JSONExtract(raw, 'score', 'Int32'))                         AS score,

       JSONExtract(raw, 'metrics', 'Map(String, Float64)')                 AS metrics,
       JSONExtract(raw, 'properties', 'Map(String, String)')               AS properties,

       JSONExtractString(raw, 'opResult')                                 AS op_result,
       JSONExtractString(raw, 'errorCode')                                AS error_code,
       JSONExtractString(raw, 'riskLevel')                                AS risk_level,
       JSONExtractString(raw, 'traceId')                                  AS trace_id,

       now()                                                               AS created_at,
       now()                                                               AS updated_at
FROM gw_uba.kafka_events_raw;


-- ============================================================
-- 4. 物化视图 - Kafka 风险事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW gw_uba.mv_kafka_risk_events_to_fact
    TO gw_uba.risk_events
AS
SELECT toUInt64(JSONExtractString(raw, 'id'))                              AS id,
       coalesce(toUInt32OrNull(JSONExtractString(raw, 'tenantId')), 0)     AS tenant_id,

       coalesce(toUInt32OrNull(JSONExtractString(raw, 'userId')), 0)       AS user_id,
       JSONExtractString(raw, 'deviceId')                                  AS device_id,
       JSONExtractString(raw, 'globalUserId')                              AS global_user_id,

       JSONExtractString(raw, 'riskType')                                  AS risk_type,
       JSONExtractString(raw, 'riskLevel')                                 AS risk_level,
       coalesce(toFloat32OrNull(JSONExtractString(raw, 'riskScore')), 0.0) AS risk_score,

       coalesce(toUInt32OrNull(JSONExtractString(raw, 'ruleId')), 0)       AS rule_id,
       JSONExtractString(raw, 'ruleName')                                  AS rule_name,
       JSONExtract(raw, 'ruleContext', 'Map(String, String)')              AS rule_context,

       JSONExtract(raw, 'relatedEventIds', 'Array(String)')                AS related_event_ids,
       coalesce(toUInt64OrNull(JSONExtractString(raw, 'session_id')), 0)   AS session_id,

       JSONExtractString(raw, 'description')                               AS description,
       JSONExtract(raw, 'evidence', 'Map(String, String)')                 AS evidence,

       JSONExtractString(raw, 'status')                                    AS status,
       coalesce(JSONExtractString(raw, 'handlerId'), '')                   AS handler_id,

       parseDateTime64BestEffortOrNull(
               nullIf(JSONExtractString(raw, 'handledTime'), ''),
               3
       )                                                                   AS handled_time,
       coalesce(JSONExtractString(raw, 'handleRemark'), '')                AS handle_remark,

       parseDateTime64BestEffortOrNull(
               nullIf(JSONExtractString(raw, 'occurTime'), ''), 3
       )                                                                   AS occur_time,
       parseDateTime64BestEffortOrNull(
               nullIf(JSONExtractString(raw, 'reportTime'), ''), 3
       )                                                                   AS report_time,


       now()                                                               AS created_at,
       now()                                                               AS updated_at

FROM gw_uba.kafka_risk_events_raw;

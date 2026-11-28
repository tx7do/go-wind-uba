-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
--
-- 执行方式：由 `uba-ingest ch-apply` 渲染并执行（见 backend/tools/uba-ingest），
--          占位符 {{.KafkaBrokerList}} 取自 configs 的 data.kafka.endpoints。
--          本地手工执行前先用 `uba-ingest ch-apply --dry-run` 输出替换后的 SQL。
--
-- 注意：kafka_group_name 每张 Kafka 引擎表必须独占一个消费组——同一 topic 上
--      同组消费会被 Kafka 均分分区。风险事件表（uba_risk_events）同时被 Go 侧
--      RiskEventNotifier 以 uba_alert_sender 组订阅做 webhook 通知，因此这里
--      必须用独立的 uba_ingest_ch_risk 组。
--      已按旧配置（误用 uba_alert_sender）建过表的部署，需要 DROP 后重建该表
--      才会换组（Kafka 引擎表的 SETTINGS 建表后不可变）。
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
            kafka_broker_list = '{{.KafkaBrokerList}}',
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
            kafka_broker_list = '{{.KafkaBrokerList}}',
            kafka_topic_list = 'uba_risk_events',
            kafka_group_name = 'uba_ingest_ch_risk',
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
       JSONExtractString(raw, 'globalUserId')                             AS global_user_id,

       parseDateTime64BestEffort(JSONExtractString(raw, 'eventTime'), 3)  AS event_time,
       parseDateTime64BestEffort(JSONExtractString(raw, 'serverTime'), 3) AS server_time,

       JSONExtractString(raw, 'eventCategory')                            AS event_category,
       JSONExtractString(raw, 'eventName')                                AS event_name,
       JSONExtractString(raw, 'eventAction')                              AS event_action,

       JSONExtractString(raw, 'objectType')                               AS object_type,
       JSONExtractString(raw, 'objectId')                                 AS object_id,
       JSONExtractString(raw, 'objectName')                               AS object_name,

       JSONExtractString(raw, 'sessionId')                                AS session_id,
       toUInt32(JSONExtract(raw, 'sessionSeq', 'UInt32'))                 AS session_seq,

       JSONExtractString(raw, 'platform')                                 AS platform,
       JSONExtractString(raw, 'os')                                       AS os,
       JSONExtractString(raw, 'appVersion')                               AS app_version,
       JSONExtractString(raw, 'channel')                                  AS channel,
       JSONExtractString(raw, 'userAgent')                                AS user_agent,

       JSONExtractString(raw, 'ip')                                       AS ip,
       JSONExtractString(raw, 'ipCity')                                   AS ip_city,
       JSONExtractString(raw, 'country')                                  AS country,
       JSONExtractString(raw, 'network')                                  AS network,
       JSONExtractString(raw, 'geo')                                      AS geo,
       JSONExtractString(raw, 'referer')                                  AS referer,

       JSONExtract(raw, 'context', 'Map(String, String)')                 AS context,

       toUInt32(JSONExtract(raw, 'durationMs', 'UInt32'))                 AS duration_ms,
       toDecimal128(JSONExtract(raw, 'amount', 'Decimal(18,2)'), 2)       AS amount,
       toUInt32(JSONExtract(raw, 'quantity', 'UInt32'))                   AS quantity,
       toInt32(JSONExtract(raw, 'score', 'Int32'))                        AS score,

       JSONExtract(raw, 'metrics', 'Map(String, Float64)')                AS metrics,
       JSONExtract(raw, 'properties', 'Map(String, String)')              AS properties,

       JSONExtractString(raw, 'opResult')                                 AS op_result,
       JSONExtractString(raw, 'errorCode')                                AS error_code,
       JSONExtractString(raw, 'riskLevel')                                AS risk_level,
       JSONExtractString(raw, 'traceId')                                  AS trace_id,

       -- 点击热力图字段（缺失/非法时落默认值，不影响其他事件入库）
       toUInt16OrZero(JSONExtractString(raw, 'clickX'))             AS click_x,
       toUInt16OrZero(JSONExtractString(raw, 'clickY'))             AS click_y,
       JSONExtractString(raw, 'elementXpath')                            AS element_xpath,
       JSONExtractString(raw, 'pageUrl')                                 AS page_url,
       toUInt16OrZero(JSONExtractString(raw, 'viewportWidth'))      AS viewport_width,
       JSONExtractString(raw, 'serverId')                                AS server_id,
       toUInt16OrZero(JSONExtractString(raw, 'level'))             AS level,

       now()                                                              AS created_at,
       now()                                                              AS updated_at
FROM gw_uba.kafka_events_raw;


-- ============================================================
-- 4. 物化视图 - Kafka 风险事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_risk_events_to_fact
    TO gw_uba.risk_events
AS
SELECT JSONExtractString(raw, 'riskEventId')                               AS risk_event_id,
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
       JSONExtractString(raw, 'sessionId')                                 AS session_id,

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

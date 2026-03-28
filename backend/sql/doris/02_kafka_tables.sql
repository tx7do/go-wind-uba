-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
-- ============================================================

USE gw_uba;

-- 停止现有任务（如果存在）
STOP ROUTINE LOAD FOR gw_uba.job_events_to_fact;
STOP ROUTINE LOAD FOR gw_uba.job_risk_events_to_fact;

-- 删除现有任务（如果需要重新创建）
DROP ROUTINE LOAD IF EXISTS gw_uba.job_events_to_fact;
DROP ROUTINE LOAD IF EXISTS gw_uba.job_risk_events_to_fact;


-- ============================================================
-- 1. 行为事件流入任务
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_events_to_fact
ON events_fact
COLUMNS(
    event_id, tenant_id, user_id, device_id, account_id, global_user_id,
    temp_event_time, event_category, event_name, event_action, object_type,
    object_id, object_name, session_id, session_seq, platform, ip,
    ip_city, country, user_agent, referer, context, duration_ms,
    temp_amount, quantity, score, metrics, properties, error_code, trace_id,

    event_time = str_to_date(replace(replace(temp_event_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s'),
    amount = if(temp_amount = '', 0, temp_amount),

    server_time = now(3),
    created_at = now(),
    updated_at = now()
)
PROPERTIES
(
    "desired_concurrent_number" = "3",
    "max_batch_interval" = "10",
    "max_batch_rows" = "200000",
    "format" = "json",
    "jsonpaths" = "[\"$.eventId\", \"$.tenantId\", \"$.userId\", \"$.deviceId\", \"$.accountId\", \"$.globalUserId\", \"$.eventTime\", \"$.eventCategory\", \"$.eventName\", \"$.eventAction\", \"$.objectType\", \"$.objectId\", \"$.objectName\", \"$.sessionId\", \"$.sessionSeq\", \"$.platform\", \"$.ip\", \"$.ipCity\", \"$.country\", \"$.userAgent\", \"$.referer\", \"$.context\", \"$.durationMs\", \"$.amount\", \"$.quantity\", \"$.score\", \"$.metrics\", \"$.properties\", \"$.errorCode\", \"$.traceId\"]",
    "strip_outer_array" = "false",
    "num_as_string" = "true"
)
FROM KAFKA
(
    "kafka_broker_list" = "kafka:9092",
    "kafka_topic" = "uba_events_raw",
    "property.group.id" = "uba_ingest_doris",
    "property.kafka_default_offsets" = "OFFSET_BEGINNING"
);

-- ============================================================
-- 2. 风险事件流入任务
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_risk_events_to_fact
ON risk_events
COLUMNS(
    risk_event_id, tenant_id, user_id, device_id, global_user_id,
    risk_type, risk_level, risk_score, rule_id, rule_name,
    rule_context, related_event_ids, session_id, description,
    evidence, status, handler_id, temp_handled_time, handle_remark,
    temp_occur_time, temp_report_time,

    occur_time = str_to_date(replace(replace(temp_occur_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s'),
    handled_time = str_to_date(replace(replace(temp_handled_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s'),

    report_time = if(temp_report_time is null or temp_report_time = '', now(3),
                     str_to_date(replace(replace(temp_report_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s')),

    event_date = date(occur_time),
    created_at = now(),
    updated_at = now()
)
PROPERTIES
(
    "desired_concurrent_number" = "3",
    "max_batch_interval" = "10",
    "max_batch_rows" = "200000",
    "format" = "json",
    "jsonpaths" = "[\"$.riskEventId\", \"$.tenantId\", \"$.userId\", \"$.deviceId\", \"$.globalUserId\", \"$.riskType\", \"$.riskLevel\", \"$.riskScore\", \"$.ruleId\", \"$.ruleName\", \"$.ruleContext\", \"$.relatedEventIds\", \"$.sessionId\", \"$.description\", \"$.evidence\", \"$.status\", \"$.handlerId\", \"$.handledTime\", \"$.handleRemark\", \"$.occurTime\", \"$.reportTime\"]",
    "strip_outer_array" = "false",
    "num_as_string" = "true"
)
FROM KAFKA
(
    "kafka_broker_list" = "kafka:9092",
    "kafka_topic" = "uba_risk_events",
    "property.group.id" = "uba_risk_detector",
    "property.kafka_default_offsets" = "OFFSET_BEGINNING"
);

SHOW ROUTINE LOAD;

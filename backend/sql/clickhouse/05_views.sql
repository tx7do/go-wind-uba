-- ============================================================
-- UBA 系统 - 物化视图设计
-- 执行顺序：5
-- ============================================================


-- ============================================================
-- 1. 物化视图 - 路径聚合
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_path_features
    TO gw_uba.path_features
AS
SELECT concat(toString(session_id), '_', toString(min(event_ts))) AS id,
       tenant_id,
       user_id,
       session_id,

       hex(MD5(arrayStringConcat(groupArray(event_name), '->')))  AS path_hash,
       arrayElement(groupArray(event_name), 1)                    AS first_event,
       arrayElement(groupArray(event_name), -1)                   AS last_event,

       length(groupArray(event_name))                             AS path_length,
       arraySlice(groupArray(event_name), 1, 3)                   AS first_3_events,
       arraySlice(groupArray(event_name), -3)                     AS last_3_events,

       maxIf(1, event_name = 'purchase_success')                  AS is_converted,
       'purchase_success'                                         AS conversion_event,
       maxIf(event_time, event_name = 'purchase_success')         AS conversion_time,

       min(event_time)                                            AS start_time,
       max(event_time)                                            AS end_time,

       max(event_ts) - min(event_ts)                              AS total_duration_ms,
       count()                                                    AS step_count

FROM gw_uba.events_fact
WHERE session_id <> 0
GROUP BY tenant_id, user_id, session_id;


-- ============================================================
-- 2. 物化视图 - 会话聚合
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_sessions_fact
    TO gw_uba.sessions_fact
AS
SELECT session_id                              AS id,
       tenant_id,
       user_id,
       device_id,
       global_user_id,

       min(event_time)                         AS start_time,
       max(event_time)                         AS end_time,
       max(event_ts) - min(event_ts)           AS duration_ms,

       count()                                 AS event_count,
       sumIf(1, event_name = 'page_view')      AS page_view_count,
       sumIf(1, event_name != 'page_view')     AS action_count,

       arrayElement(groupArray(object_id), 1)  AS entry_page,
       arrayElement(groupArray(object_id), -1) AS exit_page,

       if(count() = 1, 1, 0)                   AS is_bounce,

       any(platform)                           AS platform,
       any(os)                                 AS os,
       any(app_version)                        AS app_version,
       any(ip_city)                            AS ip_city,
       any(country)                            AS country,

       sum(amount)                             AS total_amount,
       sumIf(1, event_category = 'pay')        AS pay_event_count,

       any(risk_level)                         AS risk_level,
       array('')                               AS risk_tags,
       any(context)                            AS context,

       now()                                   AS created_at,
       now()                                   AS updated_at

FROM gw_uba.events_fact
WHERE session_id <> 0
GROUP BY tenant_id, user_id, device_id, global_user_id, session_id;


-- ============================================================
-- 3. 物化视图 - 用户画像聚合
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_users_dim
    TO gw_uba.users_dim
AS
SELECT tenant_id,
       user_id,

       toDateTime('1970-01-01')                 AS register_time,
       ''                                       AS register_channel,
       min(event_date)                          AS first_active_date,
       max(event_date)                          AS last_active_date,

       0                                        AS user_level,
       0                                        AS vip_level,
       ''                                       AS user_role,

       count()                                  AS total_events,
       uniqExact(session_id)                    AS total_sessions,
       sum(amount)                              AS total_pay_amount,
       toDateTime64('1970-01-01 00:00:00', 3)   AS last_pay_time,

       cast([], 'Array(String)')                AS prefer_categories,
       cast([], 'Array(String)')                AS prefer_objects,

       0                                        AS risk_score,
       ''                                       AS risk_level,
       cast([], 'Array(String)')                AS risk_tags,
       toDateTime('1970-01-01')                 AS last_risk_time,

       cast(map('', ''), 'Map(String, String)') AS geo,
       any(platform)                            AS platform,
       any(country)                             AS country,
       ''                                       AS device_type,

       cast(map('', ''), 'Map(String, String)') AS profile,

       1                                        AS ver,
       now()                                    AS created_at,
       now()                                    AS updated_at
FROM gw_uba.events_fact
WHERE user_id <> 0
GROUP BY tenant_id, user_id;


-- ============================================================
-- 3. 物化视图：风险事件 → 用户风险画像
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_risk_to_user_risk_profile
    TO gw_uba.user_risk_profile
AS
SELECT
    -- 主键
    tenant_id,
    user_id,

    -- 风险指标聚合（按用户聚合）
    max(risk_score)                AS risk_score,
    argMax(risk_level, occur_time) AS risk_level,
    groupUniqArray(risk_type)      AS risk_tags,
    max(occur_time)                AS last_risk_time,

    -- 版本控制
    now()                          AS updated_at

FROM gw_uba.risk_events
WHERE user_id != 0
GROUP BY tenant_id, user_id;
id <> 0;

-- ============================================================
-- UBA 系统 - 物化视图设计
-- 用途：基于聚合表构建预计算视图，支持快速查询和报表分析
-- 执行顺序：5
-- ============================================================

USE gw_uba;

-- ============================================================
-- 1. UBA 物化视图 (预聚合层)
-- ============================================================

-- 1.1 事件事实表 - 日维度物化视图 (同步刷新：基表变更即触发)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_events_daily
BUILD IMMEDIATE
REFRESH AUTO ON COMMIT
COMMENT '事件日预聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1"
)
AS
SELECT
    tenant_id,
    to_date(event_time) AS stat_date,
    event_category,
    event_name,
    platform,
    country,
    HLL_UNION(HLL_HASH(user_id)) AS uv,
    COUNT(*) AS pv,
    COUNT(DISTINCT session_id) AS session_count,
    SUM(amount) AS total_amount,
    SUM(duration_ms) AS duration_sum,
    COUNT(duration_ms) AS duration_count,

    SUM(IF(risk_level IS NOT NULL, 1, 0)) AS risk_event_count,
    SUM(IF(event_name = 'level_up', 1, 0)) AS level_up_count,
    HLL_UNION(HLL_HASH(IF(amount > 0, user_id, NULL))) AS pay_user_count

FROM events_fact
GROUP BY tenant_id, stat_date, event_category, event_name, platform, country;


-- 1.2 会话事实表 - 日维度物化视图 (异步定时刷新：每日凌晨1点)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_sessions_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '会话日预聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300",
    "workload_group" = "default"
)
AS
SELECT
    tenant_id,
    session_date AS stat_date,
    platform,
    COUNT(*) AS session_count,
    HLL_UNION(HLL_HASH(user_id)) AS unique_users,
    SUM(duration_ms) AS duration_sum,
    COUNT(duration_ms) AS duration_count,
    SUM(is_bounce) AS bounce_sum,
    COUNT(is_bounce) AS bounce_count,
    SUM(total_amount) AS total_amount
FROM sessions_fact
GROUP BY tenant_id, stat_date, platform;


-- 1.3 风险事件表 - 日维度物化视图 (异步定时刷新：每日凌晨1点)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_risk_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '风险日预聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300"
)
AS
SELECT
    tenant_id,
    event_date AS stat_date,
    risk_type,
    risk_level,
    status,
    COUNT(*) AS event_count,
    HLL_UNION(HLL_HASH(user_id)) AS unique_users,

    SUM(IF(status = 'confirmed', 1, 0)) AS confirmed_count,
    SUM(risk_score) AS risk_score_sum,
    COUNT(risk_score) AS risk_score_count

FROM risk_events
GROUP BY tenant_id, stat_date, risk_type, risk_level, status;


-- 1.4 事件事实表 - 小时维度物化视图 (异步定时刷新：每小时)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_events_hourly
REFRESH AUTO ON SCHEDULE EVERY 1 HOUR
PROPERTIES (
    "replication_num" = "1"
)
AS
SELECT
    tenant_id,
    date_trunc('hour', event_time) AS stat_hour,
    event_name,
    HLL_UNION(HLL_HASH(user_id)) AS uv,
    COUNT(*) AS pv
FROM events_fact
GROUP BY tenant_id, stat_hour, event_name;


-- 1.5 用户留存视图 - 基于事件事实表计算用户留存 (异步定时刷新：每日凌晨1点)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_user_retention
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
PROPERTIES (
    "replication_num" = "1"
)
AS
SELECT
    tenant_id,
    first_date,
    DATEDIFF(first_date, event_date) AS retention_day,
    HLL_UNION(HLL_HASH(user_id)) AS retained_users
FROM (
         SELECT tenant_id, user_id,
                MIN(to_date(event_time)) AS first_date,
                to_date(event_time) AS event_date
         FROM events_fact
         GROUP BY tenant_id, user_id, to_date(event_time)
     ) t
GROUP BY tenant_id, first_date, retention_day;


-- ============================================================
-- 2. UBA 业务视图 (报表应用层)
-- ============================================================

-- 2.1 日活视图 DAU
CREATE VIEW IF NOT EXISTS v_daily_active_users AS
SELECT
    tenant_id,
    stat_date,
    HLL_CARDINALITY(uv) AS dau,
    pv AS event_count
FROM mv_events_daily;

-- 2.2 会话分析视图（含跳出率）
CREATE VIEW IF NOT EXISTS v_daily_session_analysis AS
SELECT
    tenant_id,
    stat_date,
    platform,
    session_count,
    HLL_CARDINALITY(unique_users) AS uv,
    ROUND(duration_sum / 1000 / 60, 2) AS total_minutes,
    ROUND(IF(session_count > 0, bounce_sum / session_count * 100, 0), 2) AS bounce_rate
FROM mv_sessions_daily;

-- 2.3 风险概览视图
CREATE VIEW IF NOT EXISTS v_daily_risk_overview AS
SELECT
    tenant_id,
    stat_date,
    risk_level,
    SUM(event_count) AS total_risk_events,
    HLL_CARDINALITY(HLL_UNION(unique_users)) AS risk_user_count
FROM mv_risk_daily
GROUP BY tenant_id, stat_date, risk_level;

-- 2.4 用户标签用户数视图
CREATE VIEW IF NOT EXISTS v_user_tag_count AS
SELECT
    tenant_id,
    stat_date,
    tag_id,
    tag_value,
    user_count
FROM user_tags_agg;

-- 2.5 热门转化路径视图（排序在查询时进行）
CREATE VIEW IF NOT EXISTS v_popular_conversion_paths AS
SELECT
    tenant_id,
    stat_date,
    event_sequence,
    support_count,
    HLL_CARDINALITY(unique_users) AS user_count,
    ROUND(IF(support_count > 0, conversion_sum / support_count * 100, 0), 2) AS conversion_rate
FROM popular_paths_daily;


-- ============================================================
-- 3. UBA 扩展物化视图 (补充预聚合层)
-- ============================================================

-- 3.1 对象交互日聚合（商品/道具/关卡等曝光、点击、转化）
--     场景：对象分析 Top N、对象转化漏斗。基于事实表 object_type/object_id。
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_objects_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '对象交互日聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300"
)
AS
SELECT
    tenant_id,
    to_date(event_time) AS stat_date,
    object_type,
    object_id,
    MAX(object_name) AS object_name,
    COUNT(*)                                            AS event_cnt,
    HLL_UNION(HLL_HASH(user_id))                        AS uv,
    SUM(IF(event_action = 'click',    1, 0))            AS click_cnt,
    SUM(IF(event_action = 'purchase', 1, 0))            AS purchase_cnt,
    ROUND(SUM(amount), 2)                               AS total_amount,
    SUM(duration_ms)                                    AS duration_sum,
    COUNT(duration_ms)                                  AS duration_count
FROM events_fact
WHERE object_type IS NOT NULL
  AND object_id   IS NOT NULL
GROUP BY tenant_id, stat_date, object_type, object_id;


-- 3.2 地域分布日聚合（国家/城市维度的流量与金额）
--     场景：地域分析、投放效果地域对比。基于事实表 country/ip_city。
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_geo_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '地域分布日聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300"
)
AS
SELECT
    tenant_id,
    to_date(event_time) AS stat_date,
    country,
    ip_city,
    platform,
    COUNT(*)                              AS pv,
    HLL_UNION(HLL_HASH(user_id))          AS uv,
    COUNT(DISTINCT session_id)            AS session_count,
    ROUND(SUM(amount), 2)                 AS total_amount,
    SUM(IF(risk_level IS NOT NULL, 1, 0)) AS risk_event_count
FROM events_fact
WHERE country IS NOT NULL OR ip_city IS NOT NULL
GROUP BY tenant_id, stat_date, country, ip_city, platform;


-- 3.3 渠道与版本分布日聚合（投放渠道、APP 版本发布监测）
--     场景：渠道 ROI 对比、新版发布后的指标变化、版本渗透率。
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_channel_version_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '渠道与版本分布日聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300"
)
AS
SELECT
    tenant_id,
    to_date(event_time) AS stat_date,
    channel,
    app_version,
    platform,
    COUNT(*)                              AS pv,
    HLL_UNION(HLL_HASH(user_id))          AS uv,
    COUNT(DISTINCT session_id)            AS session_count,
    ROUND(SUM(amount), 2)                 AS total_amount,
    SUM(IF(risk_level IS NOT NULL, 1, 0)) AS risk_event_count
FROM events_fact
WHERE channel IS NOT NULL OR app_version IS NOT NULL
GROUP BY tenant_id, stat_date, channel, app_version, platform;


-- 3.4 用户首末次活跃日聚合（基于事件事实表实时计算，不依赖 users_dim 离线更新）
--     场景：新增/留存校验、用户活跃度判断、冷启动用户识别。
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_user_active_daily
BUILD IMMEDIATE
REFRESH AUTO ON SCHEDULE EVERY 1 DAY
COMMENT '用户首末次活跃日聚合视图'
DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "grace_period" = "300"
)
AS
SELECT
    tenant_id,
    to_date(event_time)   AS stat_date,
    user_id,
    MIN(event_time)       AS first_event_time,
    MAX(event_time)       AS last_event_time,
    COUNT(*)              AS event_cnt,
    COUNT(DISTINCT session_id) AS session_cnt
FROM events_fact
WHERE user_id > 0
GROUP BY tenant_id, stat_date, user_id;


-- ============================================================
-- 4. UBA 扩展业务视图 (报表应用层)
-- ============================================================

-- 4.1 对象分析视图（含点击率、转化率）
CREATE VIEW IF NOT EXISTS v_daily_object_analysis AS
SELECT
    tenant_id,
    stat_date,
    object_type,
    object_id,
    object_name,
    event_cnt,
    HLL_CARDINALITY(uv) AS uv,
    click_cnt,
    purchase_cnt,
    ROUND(IF(event_cnt > 0, click_cnt    / event_cnt * 100, 0), 2) AS click_rate,
    ROUND(IF(event_cnt > 0, purchase_cnt / event_cnt * 100, 0), 2) AS purchase_rate,
    total_amount,
    ROUND(duration_sum / NULLIF(duration_count, 0), 2) AS avg_duration_ms
FROM mv_objects_daily;


-- 4.2 地域分析视图（含人均事件数、人均金额）
CREATE VIEW IF NOT EXISTS v_daily_geo_analysis AS
SELECT
    tenant_id,
    stat_date,
    country,
    ip_city,
    platform,
    pv,
    HLL_CARDINALITY(uv) AS uv,
    session_count,
    ROUND(IF(HLL_CARDINALITY(uv) > 0, pv / HLL_CARDINALITY(uv), 0), 2) AS events_per_user,
    total_amount,
    ROUND(IF(HLL_CARDINALITY(uv) > 0, total_amount / HLL_CARDINALITY(uv), 0), 2) AS amount_per_user,
    risk_event_count
FROM mv_geo_daily;


-- 4.3 渠道版本分析视图
CREATE VIEW IF NOT EXISTS v_daily_channel_version_analysis AS
SELECT
    tenant_id,
    stat_date,
    channel,
    app_version,
    platform,
    pv,
    HLL_CARDINALITY(uv) AS uv,
    session_count,
    total_amount,
    risk_event_count
FROM mv_channel_version_daily;


-- 4.4 风险规则触发排行视图（风控规则调优，按 rule_id + status 聚合）
--     按处置状态(status)分组，避免硬编码具体状态值（status 取值由业务定义）。
--     使用方按实际 status 值过滤即可，如：WHERE status = 'confirmed'。
CREATE VIEW IF NOT EXISTS v_risk_rule_ranking AS
SELECT
    tenant_id,
    event_date AS stat_date,
    rule_id,
    rule_name,
    risk_type,
    status,
    COUNT(*)                                            AS status_count,
    HLL_CARDINALITY(HLL_HASH(user_id))                 AS affected_users,
    ROUND(AVG(risk_score), 2)                           AS avg_risk_score
FROM risk_events
WHERE rule_id > 0
GROUP BY tenant_id, event_date, rule_id, rule_name, risk_type, status;


-- 4.5 用户活跃汇总视图（首末次活跃、活跃天数，配合 users_dim 使用）
CREATE VIEW IF NOT EXISTS v_user_active_summary AS
SELECT
    tenant_id,
    user_id,
    MIN(first_event_time) AS first_active_time,
    MAX(last_event_time)  AS last_active_time,
    COUNT(DISTINCT stat_date) AS active_days,
    SUM(event_cnt)        AS total_events,
    SUM(session_cnt)      AS total_sessions
FROM mv_user_active_daily
GROUP BY tenant_id, user_id;


-- 4.6 设备聚合视图（设备维度流量、账号数，养号/共享识别）
--     distinct_users 高表示同设备多账号，需配合人工复核。
CREATE VIEW IF NOT EXISTS v_daily_device_summary AS
SELECT
    tenant_id,
    to_date(event_time)   AS stat_date,
    device_id,
    platform,
    COUNT(*)                              AS event_cnt,
    HLL_CARDINALITY(HLL_HASH(user_id))    AS user_uv,
    COUNT(DISTINCT user_id)               AS distinct_users,
    COUNT(DISTINCT session_id)            AS session_count,
    MIN(event_time)                       AS first_seen,
    MAX(event_time)                       AS last_seen
FROM events_fact
WHERE device_id IS NOT NULL AND device_id != ''
  AND user_id > 0
GROUP BY tenant_id, stat_date, device_id, platform;


-- 4.7 支付分析视图（金额、支付人数、ARPPU、客单价）
--     基于 mv_events_daily 的 pay_user_count，计算人均付费指标。
CREATE VIEW IF NOT EXISTS v_daily_payment_analysis AS
SELECT
    tenant_id,
    stat_date,
    SUM(pv)                                  AS total_events,
    HLL_CARDINALITY(HLL_UNION(uv))           AS uv,
    SUM(total_amount)                        AS total_amount,
    HLL_CARDINALITY(HLL_UNION(pay_user_count)) AS pay_users,
    ROUND(SUM(total_amount), 2)              AS amount,
    ROUND(
        SUM(total_amount) / NULLIF(HLL_CARDINALITY(HLL_UNION(pay_user_count)), 0), 2
    ) AS arppu,
    ROUND(
        SUM(total_amount) / NULLIF(HLL_CARDINALITY(HLL_UNION(uv)), 0), 2
    ) AS arpu
FROM mv_events_daily
GROUP BY tenant_id, stat_date;
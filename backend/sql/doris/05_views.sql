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
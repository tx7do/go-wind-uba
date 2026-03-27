-- ============================================================
-- UBA 系统 - 聚合表设计
-- 用途：存储预聚合指标，支持快速查询和报表分析
-- 执行顺序：3
-- ============================================================


-- ============================================================
-- 1. 事件聚合表（按日统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.events_agg_daily
(
    tenant_id        UInt32,
    stat_date        Date,
    event_category   LowCardinality(String),
    event_name       LowCardinality(String),
    platform         LowCardinality(String),
    country          LowCardinality(String),
    channel          LowCardinality(String),
    uv               AggregateFunction(uniqCombined, UInt32),
    pv               SimpleAggregateFunction(sum, UInt64),
    session_count    AggregateFunction(uniqCombined, String),
    total_amount     SimpleAggregateFunction(sum, Decimal(38, 2)),
    duration_sum     SimpleAggregateFunction(sum, Float64),
    duration_count   SimpleAggregateFunction(sum, UInt64),
    risk_event_count SimpleAggregateFunction(sum, UInt64),
    level_up_count   SimpleAggregateFunction(sum, UInt64),
    pay_user_count   AggregateFunction(uniqCombined, UInt32)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, event_category, event_name, platform, country, channel)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 2. 事件聚合表（按小时统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.events_agg_hourly
(
    tenant_id    UInt32,
    stat_hour    DateTime,
    event_name   LowCardinality(String),
    platform     LowCardinality(String),
    uv           AggregateFunction(uniqCombined, UInt32),
    pv           SimpleAggregateFunction(sum, UInt64),
    total_amount SimpleAggregateFunction(sum, Decimal(38, 2))
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMMDD(stat_hour)
        ORDER BY (tenant_id, stat_hour, event_name)
        TTL stat_hour + INTERVAL 7 DAY;


-- ============================================================
-- 3. 会话聚合表（按日统计，支持分平台分析）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.sessions_agg_daily
(
    tenant_id      UInt32,
    stat_date      Date,
    platform       LowCardinality(String),
    session_count  SimpleAggregateFunction(sum, UInt64),
    unique_users   AggregateFunction(uniqCombined, UInt32),
    duration_sum   SimpleAggregateFunction(sum, Float64),
    duration_count SimpleAggregateFunction(sum, UInt64),
    bounce_sum     SimpleAggregateFunction(sum, UInt64),
    bounce_count   SimpleAggregateFunction(sum, UInt64),
    total_amount   SimpleAggregateFunction(sum, Decimal(38, 2)),
    p50_duration   AggregateFunction(quantileTiming(0.5), UInt64),
    p90_duration   AggregateFunction(quantileTiming(0.9), UInt64),
    p99_duration   AggregateFunction(quantileTiming(0.99), UInt64)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, platform)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 4. 风险统计聚合表（按日统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.risk_stats_daily
(
    tenant_id        UInt32,
    stat_date        Date,
    risk_type        LowCardinality(String),
    risk_level       LowCardinality(String),
    status           LowCardinality(String),
    event_count      SimpleAggregateFunction(sum, UInt64),
    unique_users     AggregateFunction(uniqCombined, UInt32),
    confirmed_count  SimpleAggregateFunction(sum, UInt64),
    risk_score_sum   SimpleAggregateFunction(sum, Float64),
    risk_score_count SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, risk_type, risk_level)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 5. 用户标签聚合表（用于运营圈选）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_tags_agg
(
    tenant_id    UInt32,
    tag_id       UInt32,
    tag_value    String,
    stat_date    Date,
    user_count   AggregateFunction(uniqCombined, UInt32),
    sample_users AggregateFunction(groupArraySample(1000), UInt32)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, tag_id, tag_value, stat_date)
        TTL stat_date + INTERVAL 365 DAY;


-- ============================================================
-- 6. 热门路径聚合表（用于路径挖掘）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.popular_paths_daily
(
    tenant_id        UInt32,
    stat_date        Date,
    event_sequence   Array(String),
    sequence_hash    String,
    support_count    SimpleAggregateFunction(sum, UInt64),
    unique_users     AggregateFunction(uniqCombined, UInt32),
    duration_sum     SimpleAggregateFunction(sum, Float64),
    duration_count   SimpleAggregateFunction(sum, UInt64),
    conversion_sum   SimpleAggregateFunction(sum, Float64),
    conversion_count SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, sequence_hash)
        TTL stat_date + INTERVAL 180 DAY;


-- ============================================================
-- 7. 用户日活聚合表（DAU/MAU 核心）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_activity_daily
(
    tenant_id      UInt32,
    stat_date      Date,
    platform       LowCardinality(String),
    country        LowCardinality(String),
    user_level     LowCardinality(String),
    active_users   AggregateFunction(uniqCombined, UInt32),
    pay_users      AggregateFunction(uniqCombined, UInt32),
    risk_users     AggregateFunction(uniqCombined, UInt32),
    total_sessions AggregateFunction(uniqCombined, String),
    total_events   SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, platform, country, user_level)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 8. 用户留存日表（次日 / 7 日留存分析）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_retention_daily
(
    tenant_id      UInt32,
    register_date  Date,
    stat_date      Date,
    platform       LowCardinality(String),
    country        LowCardinality(String),
    register_users UInt64,
    retained_users UInt64,
    retention_days UInt8,
    INDEX idx_register (tenant_id, register_date) TYPE minmax GRANULARITY 1
)
    ENGINE = ReplacingMergeTree
        PARTITION BY toYYYYMM(register_date)
        ORDER BY (tenant_id, register_date, stat_date, platform, country)
        TTL register_date + INTERVAL 730 DAY;


-- ============================================================
-- 9. 付费日聚合表（收入 / 付费人数 / 客单价）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.pay_agg_daily
(
    tenant_id       UInt32,
    stat_date       Date,
    platform        LowCardinality(String),
    country         LowCardinality(String),
    pay_level       LowCardinality(String),
    pay_user_count  AggregateFunction(uniqCombined, UInt32),
    pay_order_count SimpleAggregateFunction(sum, UInt64),
    total_amount    SimpleAggregateFunction(sum, Decimal(38, 2)),
    refund_count    SimpleAggregateFunction(sum, UInt64),
    refund_amount   SimpleAggregateFunction(sum, Decimal(38, 2)),
    INDEX idx_stat (tenant_id, stat_date) TYPE minmax GRANULARITY 1
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, platform, country, pay_level)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 10. 页面访问聚合表（页面热度 & 漏斗）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.page_visit_daily
(
    tenant_id      UInt32,
    stat_date      Date,
    page_id        String,
    page_type      LowCardinality(String),
    platform       LowCardinality(String),
    pv             SimpleAggregateFunction(sum, UInt64),
    uv             AggregateFunction(uniqCombined, UInt32),
    session_count  AggregateFunction(uniqCombined, String),
    duration_sum   SimpleAggregateFunction(sum, Float64),
    duration_count SimpleAggregateFunction(sum, UInt64),
    enter_count    SimpleAggregateFunction(sum, UInt64),
    exit_count     SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, page_id, page_type, platform)
        TTL stat_date + INTERVAL 365 DAY;


-- ============================================================
-- 11. 漏斗分析聚合表（支持多步骤漏斗分析）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.funnel_steps_daily
(
    tenant_id      UInt32,
    stat_date      Date,
    funnel_id      String,
    step_index     UInt8,
    step_name      LowCardinality(String),
    enter_users    AggregateFunction(uniqCombined, UInt32),
    complete_users AggregateFunction(uniqCombined, UInt32)
)
    ENGINE = AggregatingMergeTree
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, funnel_id, step_index)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 物化视图（自动聚合）
-- ============================================================


-- -----------------------------------------------------------
-- 1. 物化视图：会话日聚合
-- 源表：gw_uba.sessions_fact
-- 目标表：gw_uba.sessions_agg_daily
-- 触发：sessions_fact 有新数据时自动执行
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_sessions_agg_daily
    TO gw_uba.sessions_agg_daily
AS
SELECT tenant_id,
       toDate(start_time)                     as stat_date,
       platform,
       count()                                as session_count,
       uniqCombinedState(user_id)             as unique_users,

       -- 平均时长（存储 sum + count，查询时计算 avg）
       sum(duration_ms)                       as duration_sum,
       count()                                as duration_count,
       sum(is_bounce)                         as bounce_sum,
       count()                                as bounce_count,

       sum(total_amount)                      as total_amount,

       -- 分位数（使用 State 函数）
       quantileTimingState(0.5)(duration_ms)  as p50_duration,
       quantileTimingState(0.9)(duration_ms)  as p90_duration,
       quantileTimingState(0.99)(duration_ms) as p99_duration
FROM gw_uba.sessions_fact
GROUP BY tenant_id, stat_date, platform;


-- -----------------------------------------------------------
-- 2. 物化视图：风险统计日聚合
-- 源表：gw_uba.risk_events
-- 目标表：gw_uba.risk_stats_daily
-- 触发：risk_events 有新数据时自动执行
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_risk_stats_daily
    TO gw_uba.risk_stats_daily
AS
SELECT tenant_id,
       event_date                    as stat_date,
       risk_type,
       risk_level,
       status,
       count()                       as event_count,
       uniqCombinedState(user_id)    as unique_users,

       -- 平均风险分（存储 sum + count，查询时计算 avg）
       sum(risk_score)               as risk_score_sum,
       count()                       as risk_score_count,

       countIf(status = 'confirmed') as confirmed_count
FROM gw_uba.risk_events
GROUP BY tenant_id, event_date, risk_type, risk_level, status;


-- -----------------------------------------------------------
-- 3. 物化视图：用户标签聚合
-- 源表：gw_uba.user_tags
-- 目标表：gw_uba.user_tags_agg
-- 触发：user_tags 有新数据时自动执行
-- 过滤：只统计有效标签（is_active = 1 且未过期）
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_user_tags_agg
    TO gw_uba.user_tags_agg
AS
SELECT tenant_id,
       tag_id,
       tag_value,
       toDate(updated_at)                   as stat_date,
       uniqCombinedState(user_id)           as user_count,
       groupArraySampleState(1000)(user_id) as sample_users
FROM gw_uba.user_tags
WHERE is_active = 1
  AND (expire_time > now64(3) OR expire_time = '1970-01-01') -- 过滤过期标签
GROUP BY tenant_id, tag_id, tag_value, toDate(updated_at);


-- -----------------------------------------------------------
-- 4. 物化视图：热门路径日聚合
-- 源表：gw_uba.path_features
-- 目标表：gw_uba.popular_paths_daily
-- 触发：path_features 有新数据时自动执行
-- 过滤：只统计有效路径（path_length >= 3 且 first_event 不为空）
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_popular_paths_daily
    TO gw_uba.popular_paths_daily
AS
SELECT tenant_id,
       event_date                                                                      as stat_date,
       [first_event, arrayElement(first_3_events, 2), arrayElement(first_3_events, 3)] as event_sequence,
       cityHash64(event_sequence)                                                      as sequence_hash,
       count()                                                                         as support_count,
       uniqCombinedState(user_id)                                                      as unique_users,
       sum(total_duration_ms)                                                          as duration_sum,
       count()                                                                         as duration_count,
       sum(is_converted)                                                               as conversion_sum,
       count()                                                                         as conversion_count
FROM gw_uba.path_features
WHERE path_length >= 3
  AND first_event != ''
GROUP BY tenant_id, event_date, event_sequence;


-- -----------------------------------------------------------
-- 5. 物化视图：事件日聚合
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.events_agg_daily
-- 触发：events_fact 有新数据时自动执行
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_events_agg_daily
    TO gw_uba.events_agg_daily
AS
SELECT tenant_id,
       toDate(event_time)                       as stat_date,
       event_category,
       event_name,
       platform,
       country,
       uniqCombinedState(user_id)               as uv,
       count()                                  as pv,
       uniqCombinedState(session_id)            as session_count,
       sum(amount)                              as total_amount,
       sum(duration_ms)                         as duration_sum,
       count()                                  as duration_count,
       countIf(risk_level != 'normal')          as risk_event_count,
       countIf(event_name = 'level_up')         as level_up_count,
       uniqCombinedStateIf(user_id, amount > 0) as pay_user_count
FROM gw_uba.events_fact
GROUP BY tenant_id, stat_date, event_category, event_name, platform, country;


-- -----------------------------------------------------------
-- 6. 物化视图：用户日活聚合
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.user_activity_daily
-- 触发：events_fact 有新数据时自动执行
-- 过滤：只统计有效用户（user_id != 0）
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_user_activity_daily
    TO gw_uba.user_activity_daily
AS
SELECT tenant_id,
       toDate(event_time)                                   AS stat_date,
       platform,
       country,
       ''                                                   AS user_level,
       uniqCombinedState(user_id)                           AS active_users,
       --uniqCombinedStateIf(user_id, 0)                      AS new_users,
       uniqCombinedStateIf(user_id, amount > 0)             AS pay_users,
       uniqCombinedStateIf(user_id, risk_level != 'normal') AS risk_users,
       uniqCombinedState(session_id)                        AS total_sessions,
       count()                                              AS total_events
FROM gw_uba.events_fact
WHERE user_id != 0
GROUP BY tenant_id, stat_date, platform, country, user_level;


-- -----------------------------------------------------------
-- 7. 物化视图：用户留存日聚合
-- 源表：gw_uba.users_dim
-- 目标表：gw_uba.user_retention_daily
-- 触发：users_dim 有新数据时自动执行
-- 过滤：只统计有效用户（user_id != 0）
-- ✅ 使用定时任务计算替换
-- -----------------------------------------------------------
-- CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_user_retention_daily
--     TO gw_uba.user_retention_daily
-- AS
-- SELECT tenant_id,
--        toDate(first_active_date)                 AS register_date,
--        toDate(now())                             AS stat_date,
--        platform,
--        country,
--        uniqCombinedState(user_id)                AS register_users,
--        uniqCombinedState(user_id)                AS retained_users,
--        dateDiff('day', register_date, stat_date) AS retention_days
-- FROM gw_uba.users_dim
-- WHERE user_id != 0
-- GROUP BY tenant_id, register_date, stat_date, platform, country;


-- -----------------------------------------------------------
-- 8. 物化视图：付费日聚合（修复版）
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.pay_agg_daily
-- 触发：events_fact 有新数据时自动执行
-- 过滤：只统计付费事件（event_category = 'pay' 或 event_name 包含 pay/purchase）
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_pay_agg_daily
    TO gw_uba.pay_agg_daily
AS
SELECT tenant_id,
       toDate(event_time)                       AS stat_date,
       platform,
       country,
       ''                                       AS pay_level,

       -- 付费用户数（去重）
       uniqCombinedStateIf(user_id, amount > 0) AS pay_user_count,

       -- 订单数（付费事件数）
       countIf(amount > 0)                      AS pay_order_count,

       -- 总金额
       sum(amount)                              AS total_amount,

       -- 退款相关
       0                                        AS refund_count,
       0                                        AS refund_amount

FROM gw_uba.events_fact
WHERE event_category = 'pay'
   OR event_name IN ('purchase', 'pay', 'recharge', 'top_up')
GROUP BY tenant_id, stat_date, platform, country, pay_level;


-- -----------------------------------------------------------
-- 9. 物化视图：页面访问日聚合
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.page_visit_daily
-- 触发：events_fact 有新数据时自动执行
-- 过滤：只统计页面访问事件（event_name = 'page_view'）
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_page_visit_daily
    TO gw_uba.page_visit_daily
AS
SELECT tenant_id,
       toDate(event_time)            AS stat_date,
       object_id                     AS page_id,
       'page'                        AS page_type,
       platform,
       count()                       AS pv,
       uniqCombinedState(user_id)    AS uv,
       uniqCombinedState(session_id) AS session_count,
       sum(duration_ms)              AS duration_sum,
       count()                       AS duration_count,
       0                             AS enter_count,
       0                             AS exit_count
FROM gw_uba.events_fact
WHERE event_name = 'page_view'
GROUP BY tenant_id, stat_date, page_id, page_type, platform;


-- -----------------------------------------------------------
-- 10. 物化视图：事件日聚合查询视图
-- 说明：对 events_agg_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.events_agg_daily_view AS
SELECT tenant_id,
       stat_date,
       event_category,
       event_name,
       platform,
       country,

       uniqCombinedMerge(uv)                   AS uv,
       uniqCombinedMerge(pay_user_count)       AS pay_user_count,
       uniqCombinedMerge(session_count)        AS session_count,

       sum(pv)                                 AS pv,
       sum(total_amount)                       AS total_amount,
       sum(duration_sum) / sum(duration_count) AS avg_duration,
       sum(risk_event_count)                   AS risk_event_count,
       sum(level_up_count)                     AS level_up_count

FROM gw_uba.events_agg_daily
GROUP BY tenant_id, stat_date, event_category, event_name, platform, country;


-- -----------------------------------------------------------
-- 11. 物化视图：付费日聚合查询视图
-- 说明：对 pay_agg_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.pay_agg_daily_view AS
SELECT tenant_id,
       stat_date,
       platform,
       country,
       pay_level,

       uniqCombinedMerge(pay_user_count) AS total_pay_user_count,
       sum(pay_order_count)              AS total_pay_order_count,
       sum(total_amount)                 AS grand_total_amount,
       sum(refund_count)                 AS total_refund_count,
       sum(refund_amount)                AS grand_refund_amount,

       if(total_pay_user_count > 0,
          grand_total_amount / total_pay_user_count,
          0)                             AS per_user_amount,

       if(grand_total_amount > 0,
          grand_refund_amount / grand_total_amount,
          0)                             AS refund_rate

FROM gw_uba.pay_agg_daily
GROUP BY tenant_id, stat_date, platform, country, pay_level;


-- -----------------------------------------------------------
-- 12. 物化视图：会话日聚合查询视图
-- 说明：对 sessions_agg_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.sessions_agg_daily_view AS
SELECT tenant_id,
       stat_date,
       platform,
       sum(session_count)                      AS session_count,
       uniqCombinedMerge(unique_users)         AS unique_users,
       sum(duration_sum) / sum(duration_count) AS avg_duration,
       sum(bounce_sum) / sum(bounce_count)     AS bounce_rate,
       sum(total_amount)                       AS total_amount,
       quantileTimingMerge(0.5)(p50_duration)  AS p50_duration,
       quantileTimingMerge(0.9)(p90_duration)  AS p90_duration,
       quantileTimingMerge(0.99)(p99_duration) AS p99_duration
FROM gw_uba.sessions_agg_daily
GROUP BY tenant_id, stat_date, platform;


-- -----------------------------------------------------------
-- 13. 物化视图：风险统计日聚合查询视图
-- 说明：对 risk_stats_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.risk_stats_daily_view AS
SELECT tenant_id,
       stat_date,
       risk_type,
       risk_level,
       status,

       uniqCombinedMerge(unique_users) AS unique_users,
       sum(event_count)                AS total_event_count,
       sum(confirmed_count)            AS total_confirmed_count,
       sum(risk_score_sum)             AS total_risk_score_sum,
       sum(risk_score_count)           AS total_risk_score_count,

       if(total_risk_score_count > 0,
          total_risk_score_sum / total_risk_score_count,
          0)                           AS avg_risk_score,

       if(total_event_count > 0,
          total_confirmed_count / total_event_count,
          0)                           AS confirm_rate

FROM gw_uba.risk_stats_daily
GROUP BY tenant_id, stat_date, risk_type, risk_level, status;


-- -----------------------------------------------------------
-- 14. 物化视图：用户标签聚合查询视图
-- 说明：对 user_tags_agg 的聚合结果进行二次聚合，支持更灵活的查询（如按标签值汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.user_tags_agg_view AS
SELECT tenant_id,
       tag_id,
       tag_value,
       stat_date,
       uniqCombinedMerge(user_count)             AS user_count,
       groupArraySampleMerge(1000)(sample_users) AS sample_users
FROM gw_uba.user_tags_agg
GROUP BY tenant_id, tag_id, tag_value, stat_date;


-- -----------------------------------------------------------
-- 15. 物化视图：热门路径日聚合查询视图
-- 说明：对 popular_paths_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.popular_paths_daily_view AS
SELECT tenant_id,
       stat_date,
       event_sequence,
       sequence_hash,
       sum(support_count)                      AS support_count,
       uniqCombinedMerge(unique_users)         AS unique_users,
       sum(duration_sum) / sum(duration_count) AS avg_duration,
       sum(conversion_sum)                     AS total_conversion_sum,
       sum(conversion_count)                   AS total_conversion_count,
       if(total_conversion_count > 0,
          total_conversion_sum / total_conversion_count,
          0)                                   AS conversion_rate
FROM gw_uba.popular_paths_daily
GROUP BY tenant_id, stat_date, event_sequence, sequence_hash;


-- -----------------------------------------------------------
-- 16. 物化视图：事件日聚合查询视图（按小时）
-- 说明：对 events_agg_hourly 的聚合结果进行二次聚合，支持按小时的趋势分析
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.events_agg_hourly_view AS
SELECT tenant_id,
       stat_hour,
       event_name,
       platform,
       uniqCombinedMerge(uv) AS uv,
       sum(pv)               AS pv,
       sum(total_amount)     AS total_amount
FROM gw_uba.events_agg_hourly
GROUP BY tenant_id, stat_hour, event_name, platform;


-- -----------------------------------------------------------
-- 17. 物化视图：漏斗分析日聚合查询视图
-- 说明：对 funnel_steps_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.funnel_steps_daily_view AS
SELECT tenant_id,
       stat_date,
       funnel_id,
       step_index,
       step_name,
       uniqCombinedMerge(enter_users)                       AS enter_users,
       uniqCombinedMerge(complete_users)                    AS complete_users,
       if(enter_users > 0, complete_users / enter_users, 0) AS conversion_rate
FROM gw_uba.funnel_steps_daily
GROUP BY tenant_id, stat_date, funnel_id, step_index, step_name;


-- -----------------------------------------------------------
-- 18. 物化视图：用户日活聚合查询视图
-- 说明：对 user_activity_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.user_activity_daily_view AS
SELECT tenant_id,
       stat_date,
       platform,
       country,
       user_level,
       uniqCombinedMerge(active_users)   AS active_users,
       uniqCombinedMerge(pay_users)      AS pay_users,
       uniqCombinedMerge(risk_users)     AS risk_users,
       uniqCombinedMerge(total_sessions) AS total_sessions,
       sum(total_events)                 AS total_events
FROM gw_uba.user_activity_daily
GROUP BY tenant_id, stat_date, platform, country, user_level;


-- -----------------------------------------------------------
-- 19. 物化视图：事件小时聚合
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.events_agg_hourly
-- 触发：events_fact 有新数据时自动执行
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_events_agg_hourly
    TO gw_uba.events_agg_hourly
AS
SELECT tenant_id,
       toStartOfHour(event_time)  AS stat_hour,
       event_name,
       platform,
       uniqCombinedState(user_id) AS uv,
       count()                    AS pv,
       sum(amount)                AS total_amount
FROM gw_uba.events_fact
WHERE user_id != 0
GROUP BY tenant_id, stat_hour, event_name, platform;


-- -----------------------------------------------------------
-- 20. 物化视图：漏斗步骤聚合
-- 源表：gw_uba.events_fact
-- 目标表：gw_uba.funnel_steps_daily
-- 触发：events_fact 有新数据时自动执行
-- 说明：实际生产环境建议服务层计算漏斗，此处为简化示例
-- -----------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_funnel_steps_daily
    TO gw_uba.funnel_steps_daily
AS
SELECT tenant_id,
       toDate(event_time)         AS stat_date,
       'default_funnel'           AS funnel_id,
       1                          AS step_index,
       event_name                 AS step_name,
       uniqCombinedState(user_id) AS enter_users,
       uniqCombinedState(user_id) AS complete_users
FROM gw_uba.events_fact
WHERE event_name IN ('login', 'browse', 'add_cart', 'purchase')
  AND user_id != 0
GROUP BY tenant_id, stat_date, funnel_id, step_index, step_name;


-- -----------------------------------------------------------
-- 21. 物化视图：页面访问日聚合查询视图
-- 说明：对 page_visit_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.page_visit_daily_view AS
SELECT tenant_id,
       stat_date,
       page_id,
       page_type,
       platform,
       sum(pv)                                 AS pv,
       uniqCombinedMerge(uv)                   AS uv,
       uniqCombinedMerge(session_count)        AS session_count,
       sum(duration_sum) / sum(duration_count) AS avg_duration,
       sum(enter_count)                        AS enter_count,
       sum(exit_count)                         AS exit_count
FROM gw_uba.page_visit_daily
GROUP BY tenant_id, stat_date, page_id, page_type, platform;


-- -----------------------------------------------------------
-- 22. 物化视图：用户留存日聚合查询视图
-- 说明：对 user_retention_daily 的聚合结果进行二次聚合，支持更灵活的查询（如按国家/平台汇总）
-- 注意：聚合函数字段必须使用 Merge 进行二次聚合，普通 sum 字段直接 sum
-- 说明：由于留存表的特殊性，register_users 和 retained_users 不能直接 sum，需要先 Merge 后再计算留存率
-- -----------------------------------------------------------
CREATE VIEW IF NOT EXISTS gw_uba.user_retention_daily_view AS
SELECT tenant_id,
       register_date,
       stat_date,
       platform,
       country,
       retention_days,
       register_users,
       retained_users,
       if(register_users > 0,
          retained_users / register_users,
          0) AS retention_rate
FROM gw_uba.user_retention_daily;

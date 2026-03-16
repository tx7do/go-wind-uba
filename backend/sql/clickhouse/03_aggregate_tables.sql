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
    -- ========== 分区 & 路由字段 ==========
    tenant_id        UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',
    stat_date        Date COMMENT '统计日期（按日聚合，用于时间范围查询）',

    -- ========== 维度组合（按查询模式设计）==========
    event_category   LowCardinality(String) COMMENT '事件大类（auth 认证/pay 支付/game 游戏/content 内容/security 安全）',
    event_name       LowCardinality(String) COMMENT '事件名称（login 登录/level_up 升级/purchase 购买/click 点击）',
    platform         LowCardinality(String) COMMENT '平台类型（iOS/Android/Web/H5/小程序）',
    country          LowCardinality(String) COMMENT '国家/地区（用于国际化分析）',

    -- ========== 核心指标（使用 AggregateFunction 支持精确去重）==========
    uv               AggregateFunction(uniqCombined, UInt32) COMMENT '去重用户数（使用 uniqCombined 状态函数，支持精确去重）',
    pv               SimpleAggregateFunction(sum, UInt64) COMMENT '事件总数（页面浏览量/事件量，直接累加）',
    session_count    SimpleAggregateFunction(sum, UInt64) COMMENT '会话总数（独立会话数量）',

    -- ========== 业务指标 ==========
    total_amount     SimpleAggregateFunction(sum, Decimal(38, 2)) COMMENT '总金额（充值/订单金额总和，Decimal(38,2) 防止溢出）',

    -- 平均时长（用 sum + count 代替 avg，因为 SimpleAggregateFunction 不支持 avg）
    duration_sum     SimpleAggregateFunction(sum, Float64) COMMENT '时长总和（用于计算平均时长）',
    duration_count   SimpleAggregateFunction(sum, UInt64) COMMENT '时长计数（用于计算平均时长）',

    risk_event_count SimpleAggregateFunction(sum, UInt64) COMMENT '风险事件数（risk_level != normal 的事件数量）',

    -- ========== 游戏特有指标（示例）==========
    level_up_count   SimpleAggregateFunction(sum, UInt64) COMMENT '升级次数（event_name = level_up 的事件数量）',
    pay_user_count   AggregateFunction(uniqCombined, UInt32) COMMENT '付费用户数（amount > 0 的去重用户数）'
)
    ENGINE = AggregatingMergeTree -- 聚合树引擎，支持 AggregateFunction 状态合并
        PARTITION BY toYYYYMM(stat_date) -- 按月分区，平衡管理粒度和查询性能
        ORDER BY (tenant_id, stat_date, event_category, event_name) -- 按租户 + 日期 + 分类 + 事件名排序，优化常见查询
        TTL stat_date + INTERVAL 730 DAY -- 730 天（2 年）前的聚合数据自动清理，节省存储空间
        COMMENT '事件日聚合表（用于行为分析报表、漏斗分析、趋势分析）';


-- ============================================================
-- 2. 会话聚合表（按日统计，支持分平台分析）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.sessions_agg_daily
(
    -- ========== 分区 & 路由字段 ==========
    tenant_id      UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',
    stat_date      Date COMMENT '统计日期（按日聚合）',
    platform       LowCardinality(String) COMMENT '平台类型（iOS/Android/Web/H5/小程序，用于分平台分析）',

    -- ========== 核心指标 ==========
    session_count  SimpleAggregateFunction(sum, UInt64) COMMENT '会话总数',
    unique_users   AggregateFunction(uniqCombined, UInt32) COMMENT '去重用户数',

    -- 平均时长（用 sum + count 代替 avg）
    duration_sum   SimpleAggregateFunction(sum, Float64) COMMENT '时长总和',
    duration_count SimpleAggregateFunction(sum, UInt64) COMMENT '时长计数',

    -- 跳出率（用 sum + count 代替 avg）
    bounce_sum     SimpleAggregateFunction(sum, Float64) COMMENT '跳出次数总和（is_bounce = 1 的累加）',
    bounce_count   SimpleAggregateFunction(sum, UInt64) COMMENT '跳出计数（用于计算跳出率）',

    total_amount   SimpleAggregateFunction(sum, Decimal(38, 2)) COMMENT '会话内总金额',

    -- ========== 分位数（需使用 AggregateFunction）==========
    p50_duration   AggregateFunction(quantileTiming(0.5), UInt64) COMMENT '50 分位时长（中位数，50% 用户会话时长低于此值）',
    p90_duration   AggregateFunction(quantileTiming(0.9), UInt64) COMMENT '90 分位时长（90% 用户会话时长低于此值）',
    p99_duration   AggregateFunction(quantileTiming(0.99), UInt64) COMMENT '99 分位时长（99% 用户会话时长低于此值，用于识别异常长会话）'
)
    ENGINE = AggregatingMergeTree -- 聚合树引擎
        PARTITION BY toYYYYMM(stat_date) -- 按月分区
        ORDER BY (tenant_id, stat_date, platform) -- 按租户 + 日期 + 平台排序，优化分平台查询
        TTL stat_date + INTERVAL 730 DAY -- 730 天（2 年）前的聚合数据自动清理
        COMMENT '会话日聚合表（用于会话分析、跳出率分析、分平台对比）';


-- ============================================================
-- 3. 风险统计聚合表（按日统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.risk_stats_daily
(
    -- ========== 分区 & 路由字段 ==========
    tenant_id        UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',
    stat_date        Date COMMENT '统计日期（按日聚合）',
    risk_type        LowCardinality(String) COMMENT '风险类型（login_anomaly 登录异常/fraud_payment 欺诈支付/brute_force 暴力破解）',
    risk_level       LowCardinality(String) COMMENT '风险等级（normal/suspicious/high/critical）',
    status           LowCardinality(String) COMMENT '处置状态（pending 待处理/confirmed 已确认/false_positive 误报/ignored 已忽略）',

    -- ========== 计数指标 ==========
    event_count      SimpleAggregateFunction(sum, UInt64) COMMENT '风险事件总数',
    unique_users     AggregateFunction(uniqCombined, UInt32) COMMENT '去重用户数（触发风险的去重用户）',

    confirmed_count  SimpleAggregateFunction(sum, UInt64) COMMENT '已确认风险数（status = confirmed 的事件数量）',

    -- 平均风险分（用 sum + count 代替 avg）
    risk_score_sum   SimpleAggregateFunction(sum, Float64) COMMENT '风险分总和（用于计算平均风险分）',
    risk_score_count SimpleAggregateFunction(sum, UInt64) COMMENT '风险分计数（用于计算平均风险分）'
)
    ENGINE = AggregatingMergeTree -- 聚合树引擎
        PARTITION BY toYYYYMM(stat_date) -- 按月分区
        ORDER BY (tenant_id, stat_date, risk_type, risk_level) -- 按租户 + 日期 + 风险类型 + 风险等级排序，优化风控报表查询
        TTL stat_date + INTERVAL 730 DAY -- 730 天（2 年）前的聚合数据自动清理
        COMMENT '风险统计日聚合表（用于风控报表、规则效果分析、误报率分析）';


-- ============================================================
-- 4. 用户标签聚合表（用于运营圈选）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_tags_agg
(
    -- ========== 分区 & 路由字段 ==========
    tenant_id    UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',
    tag_id       UInt32 COMMENT '标签定义 ID（关联 uba_tag_definitions.id）',
    tag_value    String COMMENT '标签值（用于按标签值分群）',
    stat_date    Date COMMENT '统计日期（按日聚合）',

    -- ========== 统计指标 ==========
    user_count   SimpleAggregateFunction(sum, UInt64) COMMENT '标签用户数（拥有该标签的去重用户数）',
    sample_users AggregateFunction(groupArraySample(1000), UInt32) COMMENT '抽样用户 ID 数组（用于运营预览，最多 1000 个用户）'
)
    ENGINE = AggregatingMergeTree -- 聚合树引擎
        PARTITION BY toYYYYMM(stat_date) -- 按月分区
        ORDER BY (tenant_id, tag_id, tag_value, stat_date) -- 按租户 + 标签 + 标签值 + 日期排序，优化标签圈选查询
        TTL stat_date + INTERVAL 365 DAY -- 365 天（1 年）前的聚合数据自动清理（标签统计保留周期较短）
        COMMENT '用户标签聚合表（用于运营圈选、标签效果分析、用户分群）';


-- ============================================================
-- 5. 热门路径聚合表（用于路径挖掘）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.popular_paths_daily
(
    -- ========== 分区 & 路由字段 ==========
    tenant_id        UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',
    stat_date        Date COMMENT '统计日期（按日聚合）',

    -- ========== 路径序列（截取前 3 步）==========
    event_sequence   Array(String) COMMENT '事件序列数组（如["login", "browse", "cart"]）',
    sequence_hash    String COMMENT '序列哈希值（用于去重和快速匹配）',

    -- ========== 统计指标 ==========
    support_count    SimpleAggregateFunction(sum, UInt64) COMMENT '支持度（该路径出现的次数）',
    unique_users     AggregateFunction(uniqCombined, UInt32) COMMENT '去重用户数（走该路径的去重用户）',

    -- 平均时长（用 sum + count 代替 avg）
    duration_sum     SimpleAggregateFunction(sum, Float64) COMMENT '时长总和',
    duration_count   SimpleAggregateFunction(sum, UInt64) COMMENT '时长计数',

    -- 转化率（用 sum + count 代替 avg）
    conversion_sum   SimpleAggregateFunction(sum, Float64) COMMENT '转化次数总和（is_converted = 1 的累加）',
    conversion_count SimpleAggregateFunction(sum, UInt64) COMMENT '转化计数（用于计算转化率）'
)
    ENGINE = AggregatingMergeTree -- 聚合树引擎
        PARTITION BY toYYYYMM(stat_date) -- 按月分区
        ORDER BY (tenant_id, stat_date, sequence_hash) -- 按租户 + 日期 + 序列哈希排序（不能用 support_count，聚合字段不能在 ORDER BY 中）
        TTL stat_date + INTERVAL 180 DAY -- 180 天前的聚合数据自动清理（路径数据保留周期较短）
        COMMENT '热门路径聚合表（用于路径分析、漏斗优化、转化分析）';


-- ============================================================
-- 物化视图（自动聚合）
-- ============================================================


-- -----------------------------------------------------------
-- 物化视图：会话日聚合
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
-- 物化视图：风险统计日聚合
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
-- 物化视图：用户标签聚合
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
       count(DISTINCT user_id)              as user_count,
       groupArraySampleState(1000)(user_id) as sample_users
FROM gw_uba.user_tags
WHERE is_active = 1
  AND (expire_time > now64(3) OR expire_time = '1970-01-01') -- 过滤过期标签
GROUP BY tenant_id, tag_id, tag_value, toDate(updated_at);


-- -----------------------------------------------------------
-- 物化视图：热门路径日聚合
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
-- 物化视图：事件日聚合
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
       count(DISTINCT session_id)               as session_count,
       sum(amount)                              as total_amount,
       sum(duration_ms)                         as duration_sum,
       count()                                  as duration_count,
       countIf(risk_level != 'normal')          as risk_event_count,
       countIf(event_name = 'level_up')         as level_up_count,
       uniqCombinedStateIf(user_id, amount > 0) as pay_user_count
FROM gw_uba.events_fact
GROUP BY tenant_id, stat_date, event_category, event_name, platform, country;

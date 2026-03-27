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
    session_count    AggregateFunction(uniqCombined, UInt64) COMMENT '会话总数（独立会话数量）',

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
    bounce_sum     SimpleAggregateFunction(sum, UInt64) COMMENT '跳出次数总和（is_bounce = 1 的累加）',
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
    user_count   AggregateFunction(uniqCombined, UInt32) COMMENT '标签用户数（拥有该标签的去重用户数）',
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
-- 6. 用户日活聚合表（DAU/MAU 核心）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_activity_daily
(
    tenant_id      UInt32,
    stat_date      Date,
    platform       LowCardinality(String),
    country        LowCardinality(String),
    user_level     LowCardinality(String),

    active_users   AggregateFunction(uniqCombined, UInt32),
    --new_users      AggregateFunction(uniqCombined, UInt32),
    pay_users      AggregateFunction(uniqCombined, UInt32),
    risk_users     AggregateFunction(uniqCombined, UInt32),

    total_sessions AggregateFunction(uniqCombined, UInt64),
    total_events   SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree()
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, platform, country, user_level)
        TTL stat_date + INTERVAL 730 DAY;


-- ============================================================
-- 7. 用户留存日表（次日 / 7 日留存分析）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_retention_daily
(
    -- 主键 & 分区字段
    tenant_id      UInt32 COMMENT '租户 ID',
    register_date  Date COMMENT '注册日期（用户首次活跃日期）',
    stat_date      Date COMMENT '统计日期（留存检查日期）',

    -- 维度
    platform       LowCardinality(String) COMMENT '平台',
    country        LowCardinality(String) COMMENT '国家',

    -- 留存指标
    register_users UInt64 COMMENT '注册人数（当日新增用户数）',
    retained_users UInt64 COMMENT '留存人数（在 stat_date 活跃的用户数）',
    retention_days UInt8 COMMENT '留存天数（stat_date - register_date）',

    -- 计算字段（查询时计算）
    -- retention_rate = retained_users / register_users

    INDEX idx_register (tenant_id, register_date) TYPE minmax GRANULARITY 1
)
    ENGINE = ReplacingMergeTree()
        PARTITION BY toYYYYMM(register_date)
        ORDER BY (tenant_id, register_date, stat_date, platform, country)
        TTL register_date + INTERVAL 730 DAY COMMENT '用户留存日表（通过定时任务计算，支持次日/7 日/30 日留存分析）';


-- ============================================================
-- 8. 付费日聚合表（收入 / 付费人数 / 客单价）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.pay_agg_daily
(
    -- 主键 & 分区字段
    tenant_id       UInt32 COMMENT '租户 ID',
    stat_date       Date COMMENT '统计日期',
    platform        LowCardinality(String) COMMENT '平台',
    country         LowCardinality(String) COMMENT '国家',
    pay_level       LowCardinality(String) COMMENT '付费等级（free/paying/vip）',

    -- 核心指标
    pay_user_count  AggregateFunction(uniqCombined, UInt32) COMMENT '付费用户数（去重）',
    pay_order_count SimpleAggregateFunction(sum, UInt64) COMMENT '订单数（付费事件数）',
    total_amount    SimpleAggregateFunction(sum, Decimal(38, 2)) COMMENT '总金额',

    -- 辅助指标（用于查询时计算客单价）
    -- 客单价 = total_amount / uniqCombinedMerge(pay_user_count)

    -- 退款相关
    refund_count    SimpleAggregateFunction(sum, UInt64) COMMENT '退款订单数',
    refund_amount   SimpleAggregateFunction(sum, Decimal(38, 2)) COMMENT '退款金额',

    -- 新增用户付费
    -- new_pay_users   AggregateFunction(uniqCombined, UInt32) COMMENT '新付费用户（当日注册且付费）',

    INDEX idx_stat (tenant_id, stat_date) TYPE minmax GRANULARITY 1
)
    ENGINE = AggregatingMergeTree()
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, platform, country, pay_level)
        TTL stat_date + INTERVAL 730 DAY
        COMMENT '付费日聚合表（用于收入分析、付费转化、客单价分析）';


-- ============================================================
-- 9. 页面 / 功能访问聚合表（页面热度 & 漏斗）
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
    session_count  AggregateFunction(uniqCombined, UInt64),

    --avg_duration   SimpleAggregateFunction(sum, Float64),
    duration_sum   SimpleAggregateFunction(sum, Float64),
    duration_count SimpleAggregateFunction(sum, UInt64),

    enter_count    SimpleAggregateFunction(sum, UInt64),
    exit_count     SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree()
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (tenant_id, stat_date, page_id, page_type, platform)
        TTL stat_date + INTERVAL 365 DAY;



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

       -- 聚合函数字段：用 Merge
       uniqCombinedMerge(uv)                   AS uv,
       uniqCombinedMerge(pay_user_count)       AS pay_user_count,

       -- 普通 sum 字段：直接 sum，不能用 Merge
       sum(pv)                                 AS pv,
       sum(session_count)                      AS session_count,
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
       sum(user_count)                           AS user_count,
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
       if(sum(conversion_count) > 0,
          sum(conversion_sum) / sum(conversion_count),
          0)                                   AS conversion_rate
FROM gw_uba.popular_paths_daily
GROUP BY tenant_id, stat_date, event_sequence, sequence_hash;

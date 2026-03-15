-- 事件事实表
CREATE TABLE ubd.events_fact
(
    -- ========== 主键 & 路由 ==========
    event_id       String COMMENT '全局唯一事件ID (ULID/Snowflake)',
    tenant_id      UInt32 COMMENT '租户ID (SaaS隔离)',

    -- ========== 主体: Who ==========
    user_id        UInt32 COMMENT '登录用户ID (可为空)',
    device_id      String COMMENT '设备指纹/匿名ID',
    account_id     String COMMENT '业务账号ID (游戏角色/子账号)',
    global_user_id String   DEFAULT '' COMMENT 'ID-Mapping后统一ID',

    -- ========== 时间 ==========
    event_time     DateTime64(3) COMMENT '客户端事件时间',
    event_date     Date MATERIALIZED toDate(event_time),
    event_ts       Int64 MATERIALIZED toUnixTimestamp64Milli(event_time),
    server_time    DateTime64(3) DEFAULT now64(3) COMMENT '服务端接收时间',

    -- ========== 行为: What (两级分类) ==========
    event_category LowCardinality(String) COMMENT '事件大类: auth/pay/game/content/security',
    event_name     LowCardinality(String) COMMENT '事件名称: login/level_up/purchase/click',
    event_action   LowCardinality(String) COMMENT '动作: start/success/fail/retry',

    -- ========== 客体: Object (统一引用) ==========
    object_type    LowCardinality(String) COMMENT '对象类型: product/item/level/page/api',
    object_id      String COMMENT '对象ID: 商品ID/关卡ID/页面URL',
    object_name    String COMMENT '对象名称 (冗余, 便于查询)',

    -- ========== 上下文: Context ==========
    -- 会话
    session_id     UInt32 COMMENT '会话ID (写入层生成)',
    session_seq    UInt32 COMMENT '会话内事件序号',

    -- 环境
    platform       LowCardinality(String),
    os             LowCardinality(String),
    app_version    LowCardinality(String),
    channel        String COMMENT '渠道: app_store/google_play/huawei',

    -- 网络 & 位置
    ip             String,
    ip_city        LowCardinality(String),
    country        LowCardinality(String),
    network        LowCardinality(String),

    -- 业务上下文 (关键通用设计)
    context        Map(String, String) COMMENT '通用上下文: {server_id: s1, zone: cn-east, ab_group: B}',

    -- ========== 指标: Metrics (数值型, 便于聚合) ==========
    -- 通用数值指标 (用固定列 + Map 混合)
    duration_ms    UInt32 COMMENT '耗时 (页面停留/接口响应)',
    amount         Decimal(18, 2) COMMENT '金额 (充值/订单)',
    quantity       UInt32 COMMENT '数量 (道具/商品)',
    score          Int32 COMMENT '分数/积分/风险分',

    metrics        Map(String, Float64) COMMENT '扩展指标: {damage: 1200, exp_gain: 50, fps: 59.8}',

    -- ========== 扩展: Properties (业务自定义) ==========
    properties     Map(String, String) COMMENT '扩展属性: {item_rarity: SSR, payment_method: alipay}',

    -- ========== 企业级字段 ==========
    op_result      LowCardinality(String) COMMENT '执行结果: success/failed/timeout',
    error_code     String COMMENT '错误码',
    risk_level     LowCardinality(String) COMMENT '风险等级: normal/suspicious/high',
    trace_id       String COMMENT '链路追踪ID',

    -- ========== 系统字段 ==========
    ingest_time    DateTime DEFAULT now(),
    version        UInt32   DEFAULT 1 COMMENT 'Schema版本 (便于演进)',

    -- ========== 索引优化 ==========
    INDEX          idx_object_id object_id TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX          idx_context_keys mapKeys(context) TYPE bloom_filter(0.01) GRANULARITY 2,
    INDEX          idx_risk risk_level TYPE set(4) GRANULARITY 1
) ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)  -- 按月分区，平衡管理粒度
ORDER BY (tenant_id, event_category, event_date, event_name, event_ts)
TTL event_time + INTERVAL 180 DAY TO VOLUME 'cold_storage'  -- 冷热分离
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    min_bytes_for_wide_part = 10485760;


-- 用户维度表
CREATE TABLE ubd.users_dim
(
    tenant_id         UInt32,
    user_id           UInt32,
    ver               UInt64 COMMENT '版本号 (ReplacingMergeTree去重用)',

    -- 基础属性
    register_time     DateTime,
    register_channel  String,
    first_active_date Date,
    last_active_date  Date,

    -- 身份属性 (通用)
    user_level        UInt16 COMMENT '用户等级/玩家等级',
    vip_level         UInt8,
    user_role         LowCardinality(String) COMMENT '角色: player/admin/guest',

    -- 行为画像 (预计算)
    total_events      UInt64,
    total_sessions    UInt32,
    total_pay_amount  Decimal(18, 2),
    last_pay_time     DateTime,

    -- 偏好标签
    prefer_categories Array(String) COMMENT '偏好事件分类: [game, social]',
    prefer_objects    Array(String) COMMENT '偏好对象: [pvp, rpg, shooter]',

    -- 风险画像
    risk_score        UInt8    DEFAULT 0, -- 0-100
    risk_tags         Array(String),      -- ['frequent_login_fail', 'abnormal_location']

    -- 扩展属性
    profile           Map(String, String) COMMENT '自定义画像: {guild_id: 1001, server: cn-1}',

    update_time       DateTime DEFAULT now(),

    INDEX             idx_risk_score risk_score TYPE minmax GRANULARITY 1,
    INDEX             idx_last_active last_active_date TYPE minmax GRANULARITY 1
) ENGINE = ReplacingMergeTree(ver)
ORDER BY (tenant_id, user_id)
SETTINGS replace_out_of_order = 1;


-- 对象维度表
CREATE TABLE ubd.objects_dim
(
    tenant_id     UInt32,
    object_type   LowCardinality(String), -- 'game_item', 'product', 'article'
    object_id     String,

    -- 基础信息
    object_name   String,
    category_path String COMMENT '分类路径: game/equipment/weapon',

    -- 属性 (结构化 + 扩展)
    price         Decimal(18, 2),
    currency      LowCardinality(String),
    rarity        LowCardinality(String) COMMENT '稀有度: N/R/SR/SSR',

    -- 扩展属性
    attributes    Map(String, String) COMMENT '自定义: {attack: 120, durability: 100}',

    -- 状态
    status        LowCardinality(String) COMMENT 'online/offline/discontinued',
    valid_from    DateTime,
    valid_to      DateTime,

    update_time   DateTime DEFAULT now(),

    UNIQUE KEY (tenant_id, object_type, object_id)
) ENGINE = ReplacingMergeTree(update_time)
ORDER BY (tenant_id, object_type, object_id);


-- ID-Mapping 表
CREATE TABLE ubd.id_mapping
(
    global_user_id String COMMENT '打通后的全局用户ID',
    tenant_id      UInt32,

    id_type        LowCardinality(String) COMMENT 'user_id/device_id/cookie/email/phone',
    id_value       String,

    -- 关联信息
    confidence     Float32  DEFAULT 1.0 COMMENT '关联置信度',
    link_source    LowCardinality(String) COMMENT '关联来源: login/bind/algorithm',

    -- 时效
    first_seen     DateTime,
    last_seen      DateTime,
    is_active      UInt8    DEFAULT 1,

    update_time    DateTime DEFAULT now(),

    INDEX          idx_active is_active TYPE set(2) GRANULARITY 1
) ENGINE = ReplacingMergeTree(update_time)
ORDER BY (tenant_id, id_type, id_value)
TTL update_time + INTERVAL 365 DAY;


-- 聚合表
CREATE TABLE ubd.events_agg_daily
(
    tenant_id        UInt32,
    stat_date        Date,

    -- 维度组合 (按查询模式设计)
    event_category   LowCardinality(String),
    event_name       LowCardinality(String),
    platform         LowCardinality(String),
    country          LowCardinality(String),

    -- 核心指标 (使用 AggregateFunction 支持精确去重)
    uv               AggregateFunction(uniqCombined, String), -- 去重用户
    pv               SimpleAggregateFunction(sum, UInt64),    -- 事件数
    session_count    SimpleAggregateFunction(sum, UInt32),

    -- 业务指标
    total_amount     SimpleAggregateFunction(sum, Decimal(18,2)),
    avg_duration     SimpleAggregateFunction(avg, Float64),
    risk_event_count SimpleAggregateFunction(sum, UInt32),

    -- 游戏特有指标 (示例)
    level_up_count   SimpleAggregateFunction(sum, UInt32),
    pay_user_count   AggregateFunction(uniqCombined, String),

    update_time      DateTime DEFAULT now()
) ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(stat_date)
ORDER BY (tenant_id, stat_date, event_category, event_name)
TTL stat_date + INTERVAL 730 DAY; -- 聚合数据保留2年


-- 会话事实表
CREATE TABLE uba.sessions_fact
(
    -- ========== 主键 & 路由 ==========
    session_id      UInt32 COMMENT '会话唯一ID',
    tenant_id       UInt32 COMMENT '租户ID',

    -- ========== 主体: Who ==========
    user_id         UInt32 COMMENT '登录用户ID',
    device_id       String COMMENT '设备指纹',
    global_user_id  String DEFAULT '' COMMENT 'ID-Mapping后统一ID',

    -- ========== 时间 ==========
    start_time      DateTime64(3) COMMENT '会话开始时间',
    end_time        DateTime64(3) COMMENT '会话结束时间',
    session_date    Date MATERIALIZED toDate(start_time) COMMENT '分区键',
    duration_ms     UInt64 COMMENT '会话时长(毫秒)',

    -- ========== 会话指标 ==========
    event_count     UInt32 COMMENT '事件总数',
    page_view_count UInt32 COMMENT '页面浏览数',
    action_count    UInt32 COMMENT '交互操作数',

    entry_page      String COMMENT '入口页面',
    exit_page       String COMMENT '出口页面',
    is_bounce       UInt8 COMMENT '是否跳出(0/1)',

    -- ========== 环境快照 ==========
    platform        LowCardinality(String),
    os              LowCardinality(String),
    app_version     LowCardinality(String),
    ip_city         LowCardinality(String),
    country         LowCardinality(String),

    -- ========== 业务指标 ==========
    total_amount    Decimal(18, 2) COMMENT '会话内总金额',
    pay_event_count UInt32 COMMENT '支付事件数',

    -- ========== 风险标记 ==========
    risk_level      LowCardinality(String),
    risk_tags       Array(String),

    -- ========== 扩展属性 ==========
    context         Map(String, String) COMMENT '会话上下文',

    update_time     DateTime DEFAULT now()
)
    ENGINE = ReplacingMergeTree(update_time)  -- 支持会话状态更新
PARTITION BY toYYYYMM(session_date)
ORDER BY (tenant_id, session_date, user_id, start_time)
TTL start_time + INTERVAL 90 DAY TO VOLUME 'cold_storage'
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1;

-- ========== 跳数索引 ==========
ALTER TABLE uba.sessions_fact ADD INDEX idx_duration duration_ms TYPE minmax GRANULARITY 2;
ALTER TABLE uba.sessions_fact ADD INDEX idx_risk risk_level TYPE set(4) GRANULARITY 1;
ALTER TABLE uba.sessions_fact ADD INDEX idx_bounce is_bounce TYPE set(2) GRANULARITY 1;
ALTER TABLE uba.sessions_fact ADD INDEX idx_entry_page entry_page TYPE ngrambf_v1(3, 1024, 3, 0) GRANULARITY 2;


-- 聚合表设计：按日统计会话指标，支持分平台分析
CREATE TABLE uba.sessions_agg_daily
(
    tenant_id       UInt32,
    stat_date       Date,
    platform        LowCardinality(String),

    -- 核心指标
    session_count   SimpleAggregateFunction(sum, UInt64),
    unique_users    AggregateFunction(uniqCombined, String),
    avg_duration    SimpleAggregateFunction(avg, Float64),
    bounce_rate     SimpleAggregateFunction(avg, Float64),  -- 存储 0-1 小数
    total_amount    SimpleAggregateFunction(sum, Decimal(18,2)),

    -- 分位数（需手动计算或近似）
    p50_duration    SimpleAggregateFunction(quantileTiming(0.5), UInt64),
    p90_duration    SimpleAggregateFunction(quantileTiming(0.9), UInt64),
    p99_duration    SimpleAggregateFunction(quantileTiming(0.99), UInt64)
)
    ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(stat_date)
ORDER BY (tenant_id, stat_date, platform)
TTL stat_date + INTERVAL 730 DAY;

-- 物化视图自动聚合
CREATE MATERIALIZED VIEW uba.mv_sessions_agg_daily TO uba.sessions_agg_daily AS
SELECT
    tenant_id,
    toDate(start_time) as stat_date,
    platform,
    count() as session_count,
    uniqCombinedState(user_id) as unique_users,
    avg(duration_ms) as avg_duration,
    avg(is_bounce) as bounce_rate,
    sum(total_amount) as total_amount,
    quantileTimingState(0.5)(duration_ms) as p50_duration,
    quantileTimingState(0.9)(duration_ms) as p90_duration,
    quantileTimingState(0.99)(duration_ms) as p99_duration
FROM uba.sessions_fact
GROUP BY tenant_id, stat_date, platform;


-- 风险事件表
CREATE TABLE uba.risk_events
(
    -- ========== 主键 ==========
    risk_id         String COMMENT '风险事件唯一ID',
    tenant_id       UInt32,

    -- ========== 关联主体 ==========
    user_id         UInt32,
    device_id       String,
    global_user_id  String DEFAULT '',

    -- ========== 风险类型 & 等级 ==========
    risk_type       LowCardinality(String) COMMENT 'risk_type enum: login_anomaly, fraud_payment...',
    risk_level      LowCardinality(String) COMMENT 'normal/suspicious/high/critical',
    risk_score      Float32 COMMENT '0-100 风险评分',

    -- ========== 触发信息 ==========
    rule_id         UInt32 COMMENT '触发规则ID',
    rule_name       String,
    rule_context    Map(String, String) COMMENT '规则触发上下文',

    -- ========== 关联行为事件 ==========
    related_event_ids Array(String),
    session_id      UInt32,

    -- ========== 风险详情 ==========
    description     String,
    evidence        Map(String, String) COMMENT '证据键值对',

    -- ========== 处置状态 ==========
    status          LowCardinality(String) COMMENT 'pending/confirmed/false_positive/ignored',
    handler_id      String,
    handled_time    DateTime64(3),
    handle_remark   String,

    -- ========== 时间 ==========
    occur_time      DateTime64(3) COMMENT '风险发生时间',
    report_time     DateTime64(3) DEFAULT now64(3) COMMENT '上报时间',
    event_date      Date MATERIALIZED toDate(occur_time),

    update_time     DateTime DEFAULT now()
)
    ENGINE = ReplacingMergeTree(update_time)  -- 支持处置状态更新
PARTITION BY toYYYYMM(event_date)
ORDER BY (tenant_id, event_date, risk_level, occur_time)
TTL occur_time + INTERVAL 180 DAY TO VOLUME 'cold_storage'
SETTINGS
    index_granularity = 8192,
    enable_mixed_granularity_parts = 1;

-- ========== 跳数索引（加速风控查询）==========
ALTER TABLE uba.risk_events ADD INDEX idx_risk_score risk_score TYPE minmax GRANULARITY 1;
ALTER TABLE uba.risk_events ADD INDEX idx_status status TYPE set(10) GRANULARITY 1;
ALTER TABLE uba.risk_events ADD INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE uba.risk_events ADD INDEX idx_rule_id rule_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE uba.risk_events ADD INDEX idx_description description TYPE tokenbf_v1(1024, 3, 0) GRANULARITY 2;

-- ========== 物化视图：风险统计 ==========
CREATE TABLE uba.risk_stats_daily
(
    tenant_id       UInt32,
    stat_date       Date,
    risk_type       LowCardinality(String),
    risk_level      LowCardinality(String),
    status          LowCardinality(String),

    event_count     SimpleAggregateFunction(sum, UInt64),
    unique_users    AggregateFunction(uniqCombined, String),
    avg_risk_score  SimpleAggregateFunction(avg, Float32),
    confirmed_count SimpleAggregateFunction(sum, UInt64)
)
    ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(stat_date)
ORDER BY (tenant_id, stat_date, risk_type, risk_level)
TTL stat_date + INTERVAL 730 DAY;

CREATE MATERIALIZED VIEW uba.mv_risk_stats_daily TO uba.risk_stats_daily AS
SELECT
    tenant_id,
    event_date as stat_date,
    risk_type,
    risk_level,
    status,
    count() as event_count,
    uniqCombinedState(user_id) as unique_users,
    avg(risk_score) as avg_risk_score,
    countIf(status = 'confirmed') as confirmed_count
FROM uba.risk_events
GROUP BY tenant_id, event_date, risk_type, risk_level, status;


-- 用户标签表
CREATE TABLE uba.user_tags
(
    tenant_id       UInt32,
    user_id         UInt32,
    tag_id          UInt32 COMMENT '标签定义ID',

    -- 标签值（统一用 String，数值/枚举在应用层解析）
    tag_value       String,
    value_label     String COMMENT '枚举值显示名称',

    -- 置信度（算法打标）
    confidence      Float32 DEFAULT 1.0,

    -- 来源
    source          LowCardinality(String) COMMENT 'manual/rule/model/import',
    source_rule_id  UInt32,

    -- 时效
    effective_time  DateTime64(3),
    expire_time     DateTime64(3),
    is_active       UInt8 DEFAULT 1,

    update_time     DateTime DEFAULT now(),
    version         UInt64 COMMENT '版本号'
)
    ENGINE = ReplacingMergeTree(version)  -- 支持标签更新/过期
PARTITION BY toYYYYMM(effective_time)
ORDER BY (tenant_id, user_id, tag_id, effective_time)
TTL expire_time + INTERVAL 1 DAY DELETE  -- 过期标签自动清理
SETTINGS
    index_granularity = 8192,
    replace_out_of_order = 1;

-- ========== 跳数索引 ==========
ALTER TABLE uba.user_tags ADD INDEX idx_active is_active TYPE set(2) GRANULARITY 1;
ALTER TABLE uba.user_tags ADD INDEX idx_source source TYPE set(10) GRANULARITY 1;
ALTER TABLE uba.user_tags ADD INDEX idx_tag_value tag_value TYPE bloom_filter(0.01) GRANULARITY 4;

-- ========== 物化视图：标签统计（用于运营圈选）==========
CREATE TABLE uba.user_tags_agg
(
    tenant_id       UInt32,
    tag_id          UInt32,
    tag_value       String,
    stat_date       Date,

    user_count      SimpleAggregateFunction(sum, UInt64),
    sample_users    AggregateFunction(groupArraySample(1000), String) COMMENT '抽样用户用于预览'
)
    ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(stat_date)
ORDER BY (tenant_id, tag_id, tag_value, stat_date)
TTL stat_date + INTERVAL 365 DAY;

CREATE MATERIALIZED VIEW uba.mv_user_tags_agg TO uba.user_tags_agg AS
SELECT
    tenant_id,
    tag_id,
    tag_value,
    toDate(update_time) as stat_date,
    count(DISTINCT user_id) as user_count,
    groupArraySampleState(1000)(user_id) as sample_users
FROM uba.user_tags
WHERE is_active = 1 AND (expire_time > now() OR expire_time = '1970-01-01')
GROUP BY tenant_id, tag_id, tag_value, toDate(update_time);


-- 路径特征表
CREATE TABLE uba.path_features
(
    -- ========== 主键 ==========
    path_id         String COMMENT '路径特征ID (hash of sequence)',
    tenant_id       UInt32,

    -- ========== 关联主体 ==========
    user_id         UInt32,
    session_id      UInt32,

    -- ========== 路径摘要 ==========
    path_hash       String COMMENT '事件序列的 hash (用于去重/聚合)',
    first_event     LowCardinality(String) COMMENT '入口事件',
    last_event      LowCardinality(String) COMMENT '出口事件',
    path_length     UInt8 COMMENT '路径步数',

    -- 关键节点（前3步 + 后3步，用于快速匹配）
    first_3_events  Array(String),
    last_3_events   Array(String),

    -- ========== 转化标记 ==========
    is_converted    UInt8 DEFAULT 0,
    conversion_event LowCardinality(String),
    conversion_time DateTime64(3),

    -- ========== 时间 ==========
    start_time      DateTime64(3),
    end_time        DateTime64(3),
    event_date      Date MATERIALIZED toDate(start_time),

    -- ========== 指标 ==========
    total_duration_ms UInt64,
    step_count      UInt8,

    update_time     DateTime DEFAULT now()
)
    ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (tenant_id, event_date, path_hash, start_time)
TTL start_time + INTERVAL 90 DAY TO VOLUME 'cold_storage'
SETTINGS
    index_granularity = 8192;

-- ========== 跳数索引 ==========
ALTER TABLE uba.path_features ADD INDEX idx_first_event first_event TYPE set(100) GRANULARITY 2;
ALTER TABLE uba.path_features ADD INDEX idx_converted is_converted TYPE set(2) GRANULARITY 1;
ALTER TABLE uba.path_features ADD INDEX idx_path_length path_length TYPE minmax GRANULARITY 1;
ALTER TABLE uba.path_features ADD INDEX idx_first_3 first_3_events TYPE bloom_filter(0.01) GRANULARITY 4;

-- ========== 物化视图：热门路径挖掘 ==========
CREATE TABLE uba.popular_paths_daily
(
    tenant_id       UInt32,
    stat_date       Date,

    -- 路径序列（截取前 5 步）
    event_sequence  Array(String),
    sequence_hash   String,

    -- 统计指标
    support_count   SimpleAggregateFunction(sum, UInt64),
    unique_users    AggregateFunction(uniqCombined, String),
    avg_duration    SimpleAggregateFunction(avg, Float64),
    conversion_rate SimpleAggregateFunction(avg, Float32)
)
    ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(stat_date)
ORDER BY (tenant_id, stat_date, support_count DESC)
TTL stat_date + INTERVAL 180 DAY;

-- 物化视图：每日聚合热门路径（简化版：只统计前 3 步序列）
CREATE MATERIALIZED VIEW uba.mv_popular_paths_daily TO uba.popular_paths_daily AS
SELECT
    tenant_id,
    event_date as stat_date,
    [first_event, arrayElement(first_3_events, 2), arrayElement(first_3_events, 3)] as event_sequence,
    cityHash64(event_sequence) as sequence_hash,
    count() as support_count,
    uniqCombinedState(user_id) as unique_users,
    avg(total_duration_ms) as avg_duration,
    avg(is_converted) as conversion_rate
FROM uba.path_features
WHERE path_length >= 3 AND first_event != ''
GROUP BY tenant_id, event_date, event_sequence;


-- 异步物化视图
CREATE
MATERIALIZED VIEW ubd.mv_events_agg_daily TO ubd.events_agg_daily AS
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
       avg(duration_ms)                         as avg_duration,
       countIf(risk_level != 'normal')          as risk_event_count,
       countIf(event_name = 'level_up')         as level_up_count,
       uniqCombinedStateIf(user_id, amount > 0) as pay_user_count
FROM ubd.events_fact
GROUP BY tenant_id, stat_date, event_category, event_name, platform, country;


CREATE TABLE ubd.kafka_events_raw
(
    event_id       String,
    tenant_id      UInt32,
    user_id        UInt32,
    device_id      String,
    account_id     String,
    global_user_id String,
    event_time     DateTime64(3),
    event_category String,
    event_name     String,
    event_action   String,
    object_type    String,
    object_id      String,
    object_name    String,
    session_id     UInt32,
    session_seq    UInt32,
    platform       String,
    os             String,
    app_version    String,
    channel        String,
    ip             String,
    ip_city        String,
    country        String,
    network        String,
    context        String,              -- JSON 字符串
    duration_ms    UInt32,
    amount         Decimal(18, 2),
    quantity       UInt32,
    score          Int32,
    metrics        String,              -- JSON 字符串
    properties     String,              -- JSON 字符串
    op_result      String,
    error_code     String,
    risk_level     String,
    trace_id       String
)
    ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka-1:9092,kafka-2:9092,kafka-3:9092',
    kafka_topic_list = 'uba-events-raw',
    kafka_group_name = 'ch_uba_consumer_001',
    kafka_format = 'JSONEachRow',
    kafka_max_block_size = 100000,
    kafka_skip_broken_messages = 100,    -- 跳过错误消息
    kafka_commit_on_block_write = 1,
    kafka_thread_per_consumer = 0,       -- 自动调整线程
    kafka_poll_max_batch_size = 100000,
    kafka_poll_timeout_ms = 500,
    kafka_max_wait_ms = 5000,
    kafka_security_protocol = 'PLAINTEXT',  -- SASL_SSL 如果开启认证
    kafka_sasl_mechanism = 'PLAIN';

CREATE MATERIALIZED VIEW ubd.mv_kafka_to_events_fact
TO ubd.events_fact
AS SELECT
              event_id,
              tenant_id,
              user_id,
              device_id,
              account_id,
              global_user_id,
              event_time,
              toDate(event_time) AS event_date,           -- 衍生列
              toUnixTimestamp64Milli(event_time) AS event_ts,
              now64(3) AS server_time,
              event_category,
              event_name,
              event_action,
              object_type,
              object_id,
              object_name,
              session_id,
              session_seq,
              platform,
              os,
              app_version,
              channel,
              ip,
              ip_city,
              country,
              network,
              context,
              duration_ms,
              amount,
              quantity,
              score,
              metrics,
              properties,
              op_result,
              error_code,
              risk_level,
              trace_id,
              now() AS ingest_time,
              1 AS version
   FROM ubd.kafka_events_raw;

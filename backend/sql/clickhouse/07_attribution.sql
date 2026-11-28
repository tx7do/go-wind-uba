-- ============================================================
-- UBA 系统 - 买量归因接入层
-- 数据库：gw_uba
-- 用途：归因记录表(attribution_records)与渠道日花费表(channel_cost_daily),
--      以及两条 Kafka 消费链:collector PostAttribution / PostChannelCost
--      → uba_attribution / uba_channel_cost → Kafka 引擎表 → 物化视图。
-- 执行顺序：7
--
-- 注意：kafka_group_name 每张 Kafka 引擎表独占一个消费组(与 02_kafka_tables.sql
--      同一约束)。清理语义:TTL 到期自动删除,attribution 365 天、花费 3 年。
-- ============================================================


-- ============================================================
-- 1. 归因记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.attribution_records
(
    attribution_id   String,
    tenant_id        Int64,
    attribution_time DateTime64(3),
    attribution_type LowCardinality(String),
    user_id          Int64 DEFAULT 0,
    device_id        String DEFAULT '',
    platform         String DEFAULT '',
    media_source     String,
    campaign         String DEFAULT '',
    ad_group         String DEFAULT '',
    creative         String DEFAULT '',
    is_organic       UInt8 DEFAULT 0,
    click_time       Nullable(DateTime64(3)),
    extras           String DEFAULT '',
    created_at       DateTime DEFAULT now()
)
    ENGINE = MergeTree
        PARTITION BY toYYYYMM(attribution_time) -- 按月分区
        ORDER BY (tenant_id, attribution_time, attribution_id)
        TTL attribution_time + INTERVAL 365 DAY -- 留存一年,与 Doris 侧动态分区窗口对齐
        SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;


-- ============================================================
-- 2. 渠道日花费表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.channel_cost_daily
(
    stat_date    Date,
    tenant_id    Int64,
    media_source String,
    campaign     String DEFAULT '',
    currency     String DEFAULT 'USD',
    cost         Decimal(18, 2) DEFAULT 0,
    clicks       Int64 DEFAULT 0,
    impressions  Int64 DEFAULT 0,
    installs     Int64 DEFAULT 0,
    updated_at   DateTime DEFAULT now()
)
    ENGINE = ReplacingMergeTree(updated_at) -- 同键重传以最新一条为准(与 Doris UNIQUE KEY 语义对齐)
        PARTITION BY toYYYYMM(stat_date)
        ORDER BY (stat_date, tenant_id, media_source, campaign)
        TTL stat_date + INTERVAL 1095 DAY
        SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;


-- ============================================================
-- 3. Kafka 引擎表 - 归因 / 花费接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_attribution_raw
(
    raw String
)
    ENGINE = Kafka
        SETTINGS
            kafka_broker_list = '{{.KafkaBrokerList}}',
            kafka_topic_list = 'uba_attribution',
            kafka_group_name = 'uba_attribution_ch',
            kafka_format = 'JSONAsString',
            kafka_num_consumers = 1,
            kafka_skip_broken_messages = 1,
            kafka_commit_on_select = 1;

CREATE TABLE IF NOT EXISTS gw_uba.kafka_channel_cost_raw
(
    raw String
)
    ENGINE = Kafka
        SETTINGS
            kafka_broker_list = '{{.KafkaBrokerList}}',
            kafka_topic_list = 'uba_channel_cost',
            kafka_group_name = 'uba_cost_ch',
            kafka_format = 'JSONAsString',
            kafka_num_consumers = 1,
            kafka_skip_broken_messages = 1,
            kafka_commit_on_select = 1;


-- ============================================================
-- 4. 物化视图 - Kafka → 目标表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_attribution
    TO gw_uba.attribution_records
AS
SELECT JSONExtractString(raw, 'attributionId')                                     AS attribution_id,
       toInt64(JSONExtract(raw, 'tenantId', 'Int64'))                              AS tenant_id,
       parseDateTime64BestEffort(JSONExtractString(raw, 'attributionTime'), 3)     AS attribution_time,
       JSONExtractString(raw, 'type')                                              AS attribution_type,
       toInt64(JSONExtract(raw, 'userId', 'Int64'))                                AS user_id,
       JSONExtractString(raw, 'deviceId')                                          AS device_id,
       JSONExtractString(raw, 'platform')                                          AS platform,
       JSONExtractString(raw, 'mediaSource')                                       AS media_source,
       JSONExtractString(raw, 'campaign')                                          AS campaign,
       JSONExtractString(raw, 'adGroup')                                           AS ad_group,
       JSONExtractString(raw, 'creative')                                          AS creative,
       if(JSONExtractString(raw, 'isOrganic') = 'true', 1, 0)                      AS is_organic,
       if(JSONExtractString(raw, 'clickTime') = '', NULL,
          parseDateTime64BestEffortOrNull(JSONExtractString(raw, 'clickTime'), 3)) AS click_time,
       if(JSONHas(raw, 'extras'), JSONExtractRaw(raw, 'extras'), '')               AS extras,
       now()                                                                       AS created_at;

CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_channel_cost
    TO gw_uba.channel_cost_daily
AS
SELECT toDate(JSONExtractString(raw, 'date'))                     AS stat_date,
       toInt64(JSONExtract(raw, 'tenantId', 'Int64'))             AS tenant_id,
       JSONExtractString(raw, 'mediaSource')                      AS media_source,
       JSONExtractString(raw, 'campaign')                         AS campaign,
       JSONExtractString(raw, 'currency')                         AS currency,
       toDecimal64(JSONExtract(raw, 'cost', 'Float64'), 2)        AS cost,
       toInt64(JSONExtract(raw, 'clicks', 'Int64'))               AS clicks,
       toInt64(JSONExtract(raw, 'impressions', 'Int64'))          AS impressions,
       toInt64(JSONExtract(raw, 'installs', 'Int64'))             AS installs,
       now()                                                      AS updated_at;

-- ============================================================
-- UBA 系统 - 买量归因接入层
-- 数据库：gw_uba
-- 用途：归因记录表(attribution_records)与渠道日花费表(channel_cost_daily),
--      以及两条 Kafka 消费链:collector PostAttribution / PostChannelCost
--      → uba_attribution / uba_channel_cost → Routine Load。
-- 执行顺序：7
--
-- 数据来源:MMP(AppsFlyer / Adjust 等)服务端回传,经 collector 的
-- /uba/v1/attribution 与 /uba/v1/channel-cost 接口入 Kafka(见
-- app/collector/service/internal/service/attribution_service.go)。
-- 渠道 ROI = channel_cost_daily.cost ÷ 归因用户的留存/付费/LTV(分析模型消费)。
-- ============================================================

USE gw_uba;


-- ============================================================
-- 1. 归因记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS attribution_records (
    attribution_id   VARCHAR(128)  NOT NULL COMMENT '归因记录唯一 ID(MMP 侧,幂等去重键)',
    tenant_id        BIGINT        NOT NULL COMMENT '租户 ID',
    attribution_time DATETIME      NOT NULL COMMENT '归因时间(分区键)',
    attribution_type VARCHAR(32)   NOT NULL DEFAULT 'INSTALL' COMMENT '归因类型:INSTALL/RE_ATTRIBUTION/IN_APP_EVENT',
    user_id          BIGINT        NULL COMMENT '平台用户 ID(未注册安装为空,按 device_id 关联)',
    device_id        VARCHAR(128)  NULL COMMENT '设备标识:GAID/IDFA/自建指纹',
    platform         VARCHAR(32)   NULL COMMENT 'android/ios/web/harmonyos',
    media_source     VARCHAR(128)  NOT NULL COMMENT '媒体渠道:organic/facebook/tiktok…',
    campaign         VARCHAR(256)  NULL COMMENT '广告计划',
    ad_group         VARCHAR(256)  NULL COMMENT '广告组',
    creative         VARCHAR(256)  NULL COMMENT '素材',
    is_organic       BOOLEAN       NOT NULL DEFAULT false COMMENT '是否自然量(非买量)',
    click_time       DATETIME      NULL COMMENT '广告点击时间',
    extras           TEXT          NULL COMMENT 'MMP 原始字段透传(JSON)',
    created_at       DATETIME      NULL,
    updated_at       DATETIME      NULL
)
    UNIQUE KEY(attribution_id, tenant_id, attribution_time)
    COMMENT "买量归因记录表"
    PARTITION BY RANGE(attribution_time) ()
    DISTRIBUTED BY HASH(attribution_id) BUCKETS 8
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-365",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );


-- ============================================================
-- 2. 渠道日花费表(按 渠道×计划×日 汇总,币种不换算——换算在分析层)
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_cost_daily (
    stat_date    DATE          NOT NULL COMMENT '统计日期',
    tenant_id    BIGINT        NOT NULL COMMENT '租户 ID',
    media_source VARCHAR(128)  NOT NULL COMMENT '媒体渠道',
    campaign     VARCHAR(256)  NOT NULL DEFAULT '' COMMENT '广告计划(空串=渠道整体)',
    currency     VARCHAR(8)    NOT NULL DEFAULT 'USD' COMMENT '币种 ISO 4217',
    cost         DECIMAL(18,2) NOT NULL DEFAULT '0' COMMENT '当日花费',
    clicks       BIGINT        NOT NULL DEFAULT '0' COMMENT '当日点击数',
    impressions  BIGINT        NOT NULL DEFAULT '0' COMMENT '当日曝光数',
    installs     BIGINT        NOT NULL DEFAULT '0' COMMENT '当日安装数(MMP 口径)',
    created_at   DATETIME      NULL,
    updated_at   DATETIME      NULL
)
    UNIQUE KEY(stat_date, tenant_id, media_source, campaign)
    COMMENT "渠道日花费表"
    PARTITION BY RANGE(stat_date) ()
    DISTRIBUTED BY HASH(media_source) BUCKETS 3
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-1095",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );


-- ============================================================
-- 3. 归因记录流入任务
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_attribution_to_records
ON attribution_records
COLUMNS(
    attribution_id, tenant_id, temp_attribution_time, attribution_type, user_id,
    device_id, platform, media_source, campaign, ad_group, creative, is_organic,
    temp_click_time, extras,

    attribution_time = str_to_date(replace(replace(temp_attribution_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s'),
    click_time = if(temp_click_time = '' OR temp_click_time IS NULL, NULL,
                    str_to_date(replace(replace(temp_click_time, 'T', ' '), 'Z', ''), '%Y-%m-%d %H:%i:%s')),
    created_at = now(),
    updated_at = now()
)
PROPERTIES
(
    "desired_concurrent_number" = "1",
    "max_batch_interval" = "10",
    "max_batch_rows" = "200000",
    "format" = "json",
    "jsonpaths" = "[\"$.attributionId\", \"$.tenantId\", \"$.attributionTime\", \"$.type\", \"$.userId\", \"$.deviceId\", \"$.platform\", \"$.mediaSource\", \"$.campaign\", \"$.adGroup\", \"$.creative\", \"$.isOrganic\", \"$.clickTime\", \"$.extras\"]",
    "strip_outer_array" = "false",
    "num_as_string" = "true"
)
FROM KAFKA
(
    "kafka_broker_list" = "{{.KafkaBrokerList}}",
    "kafka_topic" = "uba_attribution",
    "property.group.id" = "uba_ingest_doris",
    "property.kafka_default_offsets" = "OFFSET_BEGINNING"
);


-- ============================================================
-- 4. 渠道花费流入任务
-- ============================================================
CREATE ROUTINE LOAD gw_uba.job_channel_cost_to_daily
ON channel_cost_daily
COLUMNS(
    stat_date, tenant_id, media_source, campaign, currency, cost,
    clicks, impressions, installs,
    created_at = now(),
    updated_at = now()
)
PROPERTIES
(
    "desired_concurrent_number" = "1",
    "max_batch_interval" = "10",
    "max_batch_rows" = "200000",
    "format" = "json",
    "jsonpaths" = "[\"$.date\", \"$.tenantId\", \"$.mediaSource\", \"$.campaign\", \"$.currency\", \"$.cost\", \"$.clicks\", \"$.impressions\", \"$.installs\"]",
    "strip_outer_array" = "false",
    "num_as_string" = "true"
)
FROM KAFKA
(
    "kafka_broker_list" = "{{.KafkaBrokerList}}",
    "kafka_topic" = "uba_channel_cost",
    "property.group.id" = "uba_ingest_doris",
    "property.kafka_default_offsets" = "OFFSET_BEGINNING"
);

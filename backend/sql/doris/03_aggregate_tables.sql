-- ============================================================
-- UBA 系统 - 聚合表设计
-- 用途：存储预聚合指标，支持快速查询和报表分析
-- 执行顺序：3
-- ============================================================

CREATE DATABASE IF NOT EXISTS gw_uba;
USE gw_uba;


-- ============================================================
-- 1. 会话日聚合表
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions_agg_daily (
    tenant_id          INT NOT NULL COMMENT '租户ID',
    stat_date          DATE NOT NULL COMMENT '统计日期',
    platform           VARCHAR(64) NOT NULL COMMENT '平台类型',
    session_count      BIGINT SUM DEFAULT '0' COMMENT '会话总数',
    unique_users       HLL REPLACE COMMENT '去重用户数',
    duration_sum       BIGINT SUM DEFAULT '0' COMMENT '时长总和',
    duration_count     BIGINT SUM DEFAULT '0' COMMENT '时长计数',
    bounce_sum         BIGINT SUM DEFAULT '0' COMMENT '跳出次数',
    bounce_count       BIGINT SUM DEFAULT '0' COMMENT '跳出计数',
    total_amount       DECIMAL(38,2) SUM DEFAULT '0' COMMENT '总金额',
    p50_duration       QUANTILE_STATE REPLACE COMMENT '50分位时长',
    p90_duration       QUANTILE_STATE REPLACE COMMENT '90分位时长',
    p99_duration       QUANTILE_STATE REPLACE COMMENT '99分位时长'
)
AGGREGATE KEY (tenant_id, stat_date, platform)
COMMENT '会话日聚合表（会话分析、跳出率、分平台）'
PARTITION BY RANGE (stat_date) ()
DISTRIBUTED BY HASH(tenant_id, stat_date) BUCKETS 16
PROPERTIES (
    "replication_num" = "1",
    "dynamic_partition.enable" = "true",
    "dynamic_partition.time_unit" = "DAY",
    "dynamic_partition.start" = "-90",
    "dynamic_partition.end" = "3",
    "dynamic_partition.prefix" = "p",
    "light_schema_change" = "true"
);


-- ============================================================
-- 2. 用户标签聚合表
-- ============================================================
CREATE TABLE IF NOT EXISTS user_tags_agg (
    tenant_id    INT NOT NULL COMMENT '租户ID',
    tag_id       INT NOT NULL COMMENT '标签ID',
    tag_value    VARCHAR(256) NOT NULL COMMENT '标签值',
    stat_date    DATE NOT NULL COMMENT '统计日期',
    user_count   BIGINT SUM DEFAULT '0' COMMENT '标签用户数'
)
AGGREGATE KEY (tenant_id, tag_id, tag_value, stat_date)
COMMENT '用户标签聚合表（运营圈选、用户分群）'
PARTITION BY RANGE (stat_date) ()
DISTRIBUTED BY HASH(tenant_id, stat_date) BUCKETS 10
PROPERTIES (
    "replication_num" = "1",
    "dynamic_partition.enable" = "true",
    "dynamic_partition.time_unit" = "DAY",
    "dynamic_partition.start" = "-90",
    "dynamic_partition.end" = "3",
    "dynamic_partition.prefix" = "p",
    "light_schema_change" = "true"
);

-- ============================================================
-- 3. 热门路径日聚合表
-- ============================================================
CREATE TABLE IF NOT EXISTS popular_paths_daily (
    tenant_id        INT NOT NULL COMMENT '租户ID',
    stat_date        DATE NOT NULL COMMENT '统计日期',
    sequence_hash    VARCHAR(128) NOT NULL COMMENT '路径哈希',
    event_sequence   ARRAY<STRING> REPLACE COMMENT '事件序列',
    support_count    BIGINT SUM DEFAULT '0' COMMENT '路径出现次数',
    unique_users     HLL REPLACE COMMENT '去重用户数',
    duration_sum     BIGINT SUM DEFAULT '0' COMMENT '时长总和',
    duration_count   BIGINT SUM DEFAULT '0' COMMENT '时长计数',
    conversion_sum   BIGINT SUM DEFAULT '0' COMMENT '转化次数',
    conversion_count BIGINT SUM DEFAULT '0' COMMENT '转化计数'
)
AGGREGATE KEY (tenant_id, stat_date, sequence_hash)
COMMENT '热门路径聚合表（路径挖掘、漏斗优化）'
PARTITION BY RANGE (stat_date) ()
DISTRIBUTED BY HASH(tenant_id, stat_date) BUCKETS 16
PROPERTIES (
    "replication_num" = "1",
    "dynamic_partition.enable" = "true",
    "dynamic_partition.time_unit" = "DAY",
    "dynamic_partition.start" = "-90",
    "dynamic_partition.end" = "3",
    "dynamic_partition.prefix" = "p",
    "light_schema_change" = "true"
);

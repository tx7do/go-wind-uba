-- ============================================================
-- UBA 系统 - 聚合表设计
-- 用途：存储预聚合指标，支持快速查询和报表分析
-- 执行顺序：3
-- ============================================================

CREATE DATABASE IF NOT EXISTS gw_uba;
USE gw_uba;


-- ============================================================
-- 1. 事件聚合表（按日统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS events_agg_daily (
    tenant_id            INT NOT NULL COMMENT '租户ID',
    stat_date            DATE NOT NULL COMMENT '统计日期',
    event_category       VARCHAR(64) NOT NULL COMMENT '事件大类',
    event_name           VARCHAR(128) NOT NULL COMMENT '事件名称',
    platform             VARCHAR(64) COMMENT '平台类型',
    country              VARCHAR(64) COMMENT '国家/地区',
    uv                   HLL REPLACE COMMENT '去重用户数',
    pv                   BIGINT SUM DEFAULT '0' COMMENT '事件总数',
    session_count        BIGINT SUM DEFAULT '0' COMMENT '会话总数',
    total_amount         DECIMAL(38,2) SUM DEFAULT '0' COMMENT '总金额',
    duration_sum         BIGINT SUM DEFAULT '0' COMMENT '时长总和',
    duration_count       BIGINT SUM DEFAULT '0' COMMENT '时长计数',
    risk_event_count     BIGINT SUM DEFAULT '0' COMMENT '风险事件数',
    level_up_count       BIGINT SUM DEFAULT '0' COMMENT '升级次数',
    pay_user_count       HLL REPLACE COMMENT '付费用户数'
)
    AGGREGATE KEY (tenant_id, stat_date, event_category, event_name, platform, country)
    PARTITION BY RANGE (stat_date) ()
    DISTRIBUTED BY HASH(tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p"
               );
ALTER TABLE events_agg_daily MODIFY COMMENT '事件日聚合表（用于行为分析报表、漏斗分析、趋势分析）';


-- ============================================================
-- 2. 会话聚合表（按日统计，支持分平台分析）
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
    PARTITION BY RANGE (stat_date) ()
    DISTRIBUTED BY HASH(tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p"
               );
ALTER TABLE sessions_agg_daily MODIFY COMMENT '会话日聚合表（用于会话分析、跳出率、分平台对比）';


-- ============================================================
-- 3. 风险统计聚合表（按日统计）
-- ============================================================
CREATE TABLE IF NOT EXISTS risk_stats_daily (
    tenant_id          INT NOT NULL COMMENT '租户ID',
    stat_date          DATE NOT NULL COMMENT '统计日期',
    risk_type          VARCHAR(64) NOT NULL COMMENT '风险类型',
    risk_level         VARCHAR(64) NOT NULL COMMENT '风险等级',
    status             VARCHAR(64) NOT NULL COMMENT '处置状态',
    event_count        BIGINT SUM DEFAULT '0' COMMENT '风险事件总数',
    unique_users       HLL REPLACE COMMENT '去重用户数',
    confirmed_count    BIGINT SUM DEFAULT '0' COMMENT '已确认风险数',
    risk_score_sum     BIGINT SUM DEFAULT '0' COMMENT '风险分总和',
    risk_score_count   BIGINT SUM DEFAULT '0' COMMENT '风险分计数'
)
    AGGREGATE KEY (tenant_id, stat_date, risk_type, risk_level, status)
    PARTITION BY RANGE (stat_date) ()
    DISTRIBUTED BY HASH(tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p"
               );
ALTER TABLE risk_stats_daily MODIFY COMMENT '风险统计日聚合表（风控报表、规则效果分析）';


-- ============================================================
-- 4. 用户标签聚合表（用于运营圈选）
-- ============================================================
CREATE TABLE IF NOT EXISTS user_tags_agg (
    tenant_id    INT NOT NULL COMMENT '租户ID',
    tag_id       INT NOT NULL COMMENT '标签ID',
    tag_value    VARCHAR(256) NOT NULL COMMENT '标签值',
    stat_date    DATE NOT NULL COMMENT '统计日期',
    user_count   BIGINT SUM DEFAULT '0' COMMENT '标签用户数'
)
    AGGREGATE KEY (tenant_id, tag_id, tag_value, stat_date)
    PARTITION BY RANGE (stat_date) ()
    DISTRIBUTED BY HASH(tenant_id) BUCKETS 10
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p"
               );
ALTER TABLE user_tags_agg MODIFY COMMENT '用户标签聚合表（运营圈选、用户分群）';


-- ============================================================
-- 5. 热门路径聚合表（用于路径挖掘）
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.popular_paths_daily (
    tenant_id        INT NOT NULL COMMENT '租户ID',
    stat_date        DATE NOT NULL COMMENT '统计日期',
    sequence_hash    VARCHAR(128) NOT NULL COMMENT '序列哈希（用于唯一标识路径）',
    event_sequence   ARRAY<STRING> REPLACE COMMENT '事件序列',
    support_count    BIGINT SUM DEFAULT '0' COMMENT '路径出现次数',
    unique_users     HLL REPLACE COMMENT '去重用户数',
    duration_sum     BIGINT SUM DEFAULT '0' COMMENT '时长总和',
    duration_count   BIGINT SUM DEFAULT '0' COMMENT '时长计数',
    conversion_sum   BIGINT SUM DEFAULT '0' COMMENT '转化次数',
    conversion_count BIGINT SUM DEFAULT '0' COMMENT '转化计数'
)
    AGGREGATE KEY (tenant_id, stat_date, sequence_hash)
    PARTITION BY RANGE (stat_date) ()
    DISTRIBUTED BY HASH(tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p"
               );
ALTER TABLE popular_paths_daily MODIFY COMMENT '热门路径聚合表（路径分析、漏斗优化、转化分析）';

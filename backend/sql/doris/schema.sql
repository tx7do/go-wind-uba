-- 事件事实表
CREATE TABLE IF NOT EXISTS gw_uba.events_fact (
    -- ========== 主键 & 路由 ==========
                                               event_id       VARCHAR(64) COMMENT '全局唯一事件 ID',
    tenant_id      INT UNSIGNED NOT NULL COMMENT '租户 ID',

    -- ========== 主体: Who ==========
    user_id        INT UNSIGNED COMMENT '登录用户 ID',
    device_id      VARCHAR(64) COMMENT '设备指纹',
    account_id     VARCHAR(64) COMMENT '业务账号 ID',
    global_user_id VARCHAR(64) DEFAULT '' COMMENT 'ID-Mapping 后统一 ID',

    -- ========== 时间 ==========
    event_time     DATETIMEv2(3) COMMENT '客户端事件时间',
    event_date     DATE NOT NULL COMMENT '分区列',
    event_ts       BIGINT COMMENT 'Unix 时间戳 (毫秒)',
    server_time    DATETIMEv2(3) DEFAULT CURRENT_TIMESTAMP(3),

    -- ========== 行为: What ==========
    event_category VARCHAR(32) COMMENT '事件大类',
    event_name     VARCHAR(64) NOT NULL COMMENT '事件名称',
    event_action   VARCHAR(32) COMMENT '动作',

    -- ========== 客体: Object ==========
    object_type    VARCHAR(32) COMMENT '对象类型',
    object_id      VARCHAR(128) COMMENT '对象 ID',
    object_name    VARCHAR(255) COMMENT '对象名称',

    -- ========== 上下文: Context ==========
    session_id     INT UNSIGNED COMMENT '会话 ID',
    session_seq    INT COMMENT '会话内序号',

    -- 环境
    platform       VARCHAR(32),
    os             VARCHAR(32),
    app_version    VARCHAR(32),
    channel        VARCHAR(64),

    -- 网络 & 位置
    ip             VARCHAR(64),
    ip_city        VARCHAR(64),
    country        VARCHAR(64),
    network        VARCHAR(32),

    -- 业务上下文 (Doris 2.0 JSON 类型，支持索引)
    context        JSON COMMENT '通用上下文',

    -- ========== 指标: Metrics ==========
    duration_ms    INT COMMENT '耗时',
    amount         DECIMALV3(18, 2) COMMENT '金额',
    quantity       INT COMMENT '数量',
    score          INT COMMENT '分数',

    -- 扩展指标 (JSON 类型)
    metrics        JSON COMMENT '扩展指标',

    -- ========== 扩展: Properties (JSON 类型) ==========
    properties     JSON COMMENT '扩展属性',

    -- ========== 企业级字段 ==========
    op_result      VARCHAR(32) COMMENT '执行结果',
    error_code     VARCHAR(64) COMMENT '错误码',
    risk_level     VARCHAR(32) COMMENT '风险等级',
    trace_id       VARCHAR(64) COMMENT '链路追踪 ID',

    -- ========== 系统字段 ==========
    ingest_time    DATETIME DEFAULT CURRENT_TIMESTAMP,
    version        INT DEFAULT 1
    )
    ENGINE=OLAP
    DUPLICATE KEY(event_id)  -- 日志数据，允许重复（重试场景），或改为 UNIQUE KEY(event_id)
    PARTITION BY RANGE(event_date) ()  -- 动态分区，自动创建
    DISTRIBUTED BY HASH(tenant_id, event_date) BUCKETS AUTO  -- 自动分桶
    PROPERTIES (
                   "replication_num" = "3",
                   "storage_format" = "DEFAULT",
                   "enable_light_schema_change" = "true",
                   "storage_ttl" = "180 DAY",  -- 180 天后自动删除
                   "in_memory" = "false"
               );

-- ========== 倒排索引 (替代 CH 的 LowCardinality + Index) ==========
-- 加速高频过滤字段
ALTER TABLE gw_uba.events_fact ADD INDEX idx_event_name (event_name) USING INVERTED;
ALTER TABLE gw_uba.events_fact ADD INDEX idx_user_id (user_id) USING INVERTED;
ALTER TABLE gw_uba.events_fact ADD INDEX idx_object_id (object_id) USING INVERTED;
ALTER TABLE gw_uba.events_fact ADD INDEX idx_risk_level (risk_level) USING INVERTED;
-- JSON 字段内部键的索引 (Doris 2.0 特性)
ALTER TABLE gw_uba.events_fact ADD INDEX idx_props_server (properties) USING INVERTED PROPERTIES ("parser" = "json", "jsonpaths" = "[\"server_id\"]");


-- 用户维度表
CREATE TABLE IF NOT EXISTS gw_uba.users_dim (
                                             tenant_id         INT UNSIGNED NOT NULL,
    user_id           INT UNSIGNED NOT NULL,
    ver               BIGINT COMMENT '版本号',

    -- 基础属性
    register_time     DATETIME,
    register_channel  VARCHAR(64),
    first_active_date DATE,
    last_active_date  DATE,

    -- 身份属性
    user_level        SMALLINT,
    vip_level         TINYINT,
    user_role         VARCHAR(32),

    -- 行为画像
    total_events      BIGINT,
    total_sessions    INT,
    total_pay_amount  DECIMALV3(18, 2),
    last_pay_time     DATETIME,

    -- 偏好标签 (Doris 支持 Array)
    prefer_categories ARRAY<VARCHAR>,
    prefer_objects    ARRAY<VARCHAR>,

    -- 风险画像
    risk_score        TINYINT DEFAULT 0,
    risk_tags         ARRAY<VARCHAR>,

    -- 扩展属性
    profile           JSON,

    update_time       DATETIME DEFAULT CURRENT_TIMESTAMP
    )
    ENGINE=OLAP
    UNIQUE KEY(tenant_id, user_id)  -- 主键去重
    DISTRIBUTED BY HASH(user_id) BUCKETS AUTO
    PROPERTIES (
                   "replication_num" = "3",
                   "enable_merge_on_write" = "true"  -- 写入时合并，查询无需 FINAL，等价于 CH ReplacingMergeTree
               );


-- 对象维度表
CREATE TABLE IF NOT EXISTS gw_uba.objects_dim (
                                               tenant_id     INT UNSIGNED NOT NULL,
    object_type   VARCHAR(32) NOT NULL,
    object_id     VARCHAR(128) NOT NULL,

    object_name   VARCHAR(255),
    category_path VARCHAR(255),

    price         DECIMALV3(18, 2),
    currency      VARCHAR(32),
    rarity        VARCHAR(32),

    attributes    JSON,

    status        VARCHAR(32),
    valid_from    DATETIME,
    valid_to      DATETIME,

    update_time   DATETIME DEFAULT CURRENT_TIMESTAMP
    )
    ENGINE=OLAP
    UNIQUE KEY(tenant_id, object_type, object_id)
    DISTRIBUTED BY HASH(object_id) BUCKETS AUTO
    PROPERTIES (
                   "replication_num" = "3",
                   "enable_merge_on_write" = "true"
               );


-- ID-Mapping 表
CREATE TABLE IF NOT EXISTS gw_uba.id_mapping (
                                              global_user_id VARCHAR(64) NOT NULL,
    tenant_id      INT UNSIGNED NOT NULL,

    id_type        VARCHAR(32) NOT NULL,
    id_value       VARCHAR(128) NOT NULL,

    confidence     FLOAT DEFAULT 1.0,
    link_source    VARCHAR(32),

    first_seen     DATETIME,
    last_seen      DATETIME,
    is_active      TINYINT DEFAULT 1,

    update_time    DATETIME DEFAULT CURRENT_TIMESTAMP
    )
    ENGINE=OLAP
    UNIQUE KEY(tenant_id, id_type, id_value)  -- 同一 ID 类型值唯一，更新覆盖
    DISTRIBUTED BY HASH(id_value) BUCKETS AUTO
    PROPERTIES (
                   "replication_num" = "3",
                   "enable_merge_on_write" = "true",
                   "storage_ttl" = "365 DAY"
               );


-- 聚合表
CREATE TABLE IF NOT EXISTS gw_uba.events_agg_daily (
                                                    tenant_id        INT UNSIGNED NOT NULL,
    stat_date        DATE NOT NULL,

    event_category   VARCHAR(32),
    event_name       VARCHAR(64),
    platform         VARCHAR(32),
    country          VARCHAR(64),

    -- Doris Bitmap 类型，用于精确去重 (UV)
    uv               BITMAP BITMAP_UNION,
    pv               BIGINT SUM,
    session_count    BIGINT SUM,

    total_amount     DECIMALV3(18, 2) SUM,
    avg_duration     DOUBLE AVG,
    risk_event_count BIGINT SUM,

    level_up_count   BIGINT SUM,
    -- 付费用户 UV
    pay_user_count   BITMAP BITMAP_UNION,

    update_time      DATETIME DEFAULT CURRENT_TIMESTAMP
    )
    ENGINE=OLAP
    AGGREGATE KEY(tenant_id, stat_date, event_category, event_name, platform, country)
    DISTRIBUTED BY HASH(tenant_id, stat_date) BUCKETS AUTO
    PARTITION BY RANGE(stat_date) ()
    PROPERTIES (
                   "replication_num" = "3",
                   "storage_ttl" = "730 DAY"
               );


-- 风险事件表（独立存储，便于快速检索）
CREATE TABLE gw_uba.risk_events (
                                 risk_id VARCHAR(64),
                                 tenant_id INT UNSIGNED,
                                 user_id INT UNSIGNED,
                                 risk_type VARCHAR(32),
                                 risk_level VARCHAR(16),
                                 risk_score FLOAT,
                                 rule_id INT UNSIGNED,
                                 status VARCHAR(32),
                                 occur_time DATETIMEv2,

    -- 全文检索字段（用于告警搜索）
                                 description VARCHAR(512),
                                 evidence JSON,

                                 INDEX idx_risk_score risk_score TYPE minmax GRANULARITY 1,
                                 INDEX idx_status status TYPE set(10) GRANULARITY 1,
                                 INDEX idx_occur_time occur_time TYPE minmax GRANULARITY 1
)
    ENGINE=OLAP
UNIQUE KEY(risk_id)
DISTRIBUTED BY HASH(tenant_id, occur_time) BUCKETS AUTO
PARTITION BY RANGE(occur_time) ();

-- 用户标签聚合表（用于快速圈选）
CREATE TABLE gw_uba.user_tags_agg (
                                   tenant_id INT UNSIGNED,
                                   user_id INT UNSIGNED,
                                   tag_id INT UNSIGNED,
                                   tag_value VARCHAR(255),

                                   update_time DATETIME
)
    ENGINE=OLAP
UNIQUE KEY(tenant_id, user_id, tag_id)
DISTRIBUTED BY HASH(user_id) BUCKETS AUTO;

-- 物化视图：按标签统计用户数（用于运营仪表盘）
CREATE MATERIALIZED VIEW gw_uba.mv_tag_stats AS
SELECT
    tenant_id,
    tag_id,
    tag_value,
    count(DISTINCT user_id) as user_count,
    toDate(update_time) as stat_date
FROM gw_uba.user_tags_agg
GROUP BY tenant_id, tag_id, tag_value, stat_date;

-- 不存储完整路径，而是存储路径特征用于加速查询
CREATE TABLE gw_uba.path_features (
                                   tenant_id INT UNSIGNED,
                                   user_id INT UNSIGNED,
                                   session_id INT UNSIGNED,

    -- 路径摘要（用于快速匹配）
                                   path_hash VARCHAR(64),           -- 路径序列的 hash
                                   first_event VARCHAR(64),         -- 入口事件
                                   last_event VARCHAR(64),          -- 出口事件
                                   path_length INT,                 -- 步数

    -- 转化标记
                                   is_converted BOOLEAN,
                                   conversion_event VARCHAR(64),

                                   event_time DATETIMEv2
)
    ENGINE=OLAP
DUPLICATE KEY(tenant_id, session_id)
DISTRIBUTED BY HASH(tenant_id, session_id) BUCKETS AUTO;

-- 物化视图：预计算漏斗步骤到达人数
CREATE MATERIALIZED VIEW gw_uba.mv_funnel_steps AS
SELECT
    tenant_id,
    event_date,
    event_name,
    count(DISTINCT user_id) as step_users
FROM gw_uba.events_fact
WHERE event_name IN ('register', 'activate', 'pay')  -- 漏斗事件
GROUP BY tenant_id, event_date, event_name;

-- Doris: 会话事实表（预计算）
CREATE TABLE gw_uba.sessions_fact (
                                   session_id INT UNSIGNED,
                                   tenant_id INT UNSIGNED,
                                   user_id INT UNSIGNED,
                                   start_time DATETIMEv2,
                                   end_time DATETIMEv2,
                                   duration_ms INT,
                                   event_count INT,
                                   is_bounce BOOLEAN,
)
    ENGINE=OLAP
UNIQUE KEY(session_id)
DISTRIBUTED BY HASH(tenant_id, start_time) BUCKETS AUTO
PARTITION BY RANGE(start_time) ();

-- 物化视图：按日聚合会话统计
CREATE MATERIALIZED VIEW gw_uba.mv_session_daily_stats AS
SELECT
    tenant_id,
    toDate(start_time) as stat_date,
    count() as session_count,
    count(DISTINCT user_id) as unique_user_count,
    avg(duration_ms) as avg_duration,
    countIf(is_bounce) * 1.0 / count() as bounce_rate
FROM gw_uba.sessions_fact
GROUP BY tenant_id, stat_date;

-- 异步物化视图
CREATE ASYNC MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_events_agg_daily
REFRESH ASYNC EVERY (INTERVAL 1 MINUTE)  -- 每分钟刷新一次
PROPERTIES (
    "replication_num" = "3",
    "storage_ttl" = "730 DAY"
)
AS
SELECT
    tenant_id,
    event_date AS stat_date,
    event_category,
    event_name,
    platform,
    country,
    -- Doris Bitmap 去重
    BITMAP_UNION(to_bitmap(user_id)) AS uv,
    COUNT(*) AS pv,
    COUNT(DISTINCT session_id) AS session_count,
    SUM(amount) AS total_amount,
    AVG(duration_ms) AS avg_duration,
    SUM(CASE WHEN risk_level != 'normal' THEN 1 ELSE 0 END) AS risk_event_count,
    SUM(CASE WHEN event_name = 'level_up' THEN 1 ELSE 0 END) AS level_up_count,
    BITMAP_UNION_IF(to_bitmap(user_id), amount > 0) AS pay_user_count
FROM gw_uba.events_fact
GROUP BY tenant_id, event_date, event_category, event_name, platform, country;


-- 创建 Routine Load 任务
CREATE ROUTINE LOAD gw_uba.load_events_fact
ON gw_uba.events_fact
COLUMNS TERMINATED BY "\n"
WITH FORMAT JSON
STRICT_MODE = false
JSONPATHS = "[\"event_id\", \"tenant_id\", \"user_id\", \"device_id\", \"account_id\", \"global_user_id\", \"event_time\", \"event_category\", \"event_name\", \"event_action\", \"object_type\", \"object_id\", \"object_name\", \"session_id\", \"session_seq\", \"platform\", \"os\", \"app_version\", \"channel\", \"ip\", \"ip_city\", \"country\", \"network\", \"context\", \"duration_ms\", \"amount\", \"quantity\", \"score\", \"metrics\", \"properties\", \"op_result\", \"error_code\", \"risk_level\", \"trace_id\"]"
PROPERTIES (
    "desired_concurrent_number" = "3",           -- 并发消费数
    "max_batch_interval" = "10",                 -- 最大批次间隔 (秒)
    "max_batch_rows" = "100000",                 -- 最大批次行数
    "max_batch_size" = "104857600",              -- 最大批次大小 (100MB)
    "strict_mode" = "false",                     -- 非严格模式，允许部分字段为空
    "timezone" = "Asia/Shanghai",
    "exec_mem_limit" = "2GB",
    "load_to_timeout_second" = "600",
    "enable_auto_scale" = "true"                 -- 自动扩缩容
)
FROM KAFKA (
    "kafka_broker_list" = "kafka-1:9092,kafka-2:9092,kafka-3:9092",
    "kafka_topic" = "uba-events-raw",
    "client_id" = "doris_uba_consumer_001",
    "kafka_default_timeout" = "60000",
    "property.client.id" = "doris_uba_consumer_001",
    "property.session.timeout.ms" = "60000",
    "property.max.poll.interval.ms" = "300000",
    "property.auto.offset.reset" = "latest",     -- 或 earliest
    "property.security.protocol" = "PLAINTEXT",  -- SASL_SSL 如果开启认证
    "property.sasl.mechanism" = "PLAIN"
);

CREATE ROUTINE LOAD gw_uba.load_events_fact
ON gw_uba.events_fact
COLUMNS TERMINATED BY "\n"
WITH FORMAT JSON
-- 映射 Kafka JSON 字段到表列
COLUMNS(
    event_id,
    tenant_id,
    user_id,
    device_id,
    account_id,
    global_user_id,
    event_time,
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
    -- 衍生列计算
    event_date = toDate(event_time),
    event_ts = toUnixTimestamp64Milli(event_time),
    server_time = now64(3),
    ingest_time = now(),
    version = 1
)
STRICT_MODE = false
JSONPATHS = "[\"event_id\", \"tenant_id\", \"user_id\", \"device_id\", \"account_id\", \"global_user_id\", \"event_time\", \"event_category\", \"event_name\", \"event_action\", \"object_type\", \"object_id\", \"object_name\", \"session_id\", \"session_seq\", \"platform\", \"os\", \"app_version\", \"channel\", \"ip\", \"ip_city\", \"country\", \"network\", \"context\", \"duration_ms\", \"amount\", \"quantity\", \"score\", \"metrics\", \"properties\", \"op_result\", \"error_code\", \"risk_level\", \"trace_id\"]"
PROPERTIES (
    "desired_concurrent_number" = "3",
    "max_batch_interval" = "10",
    "max_batch_rows" = "100000",
    "strict_mode" = "false",
    "timezone" = "Asia/Shanghai",
    "enable_auto_scale" = "true"
)
FROM KAFKA (
    "kafka_broker_list" = "kafka-1:9092,kafka-2:9092,kafka-3:9092",
    "kafka_topic" = "uba-events-raw",
    "client_id" = "doris_uba_consumer_001",
    "property.auto.offset.reset" = "latest"
);

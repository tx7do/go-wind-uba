-- ============================================================
-- UBA 系统 - 基础表（事实表 + 维度表）
-- 数据库：gw_uba
-- 用途：存储原始行为数据、会话数据、风险数据及维度数据
-- 执行顺序：1
-- 兼容版本：Apache Doris 2.0+
-- ============================================================

CREATE DATABASE IF NOT EXISTS gw_uba;
USE gw_uba;


-- ============================================================
-- 1. 事件事实表
-- ============================================================
CREATE TABLE IF NOT EXISTS events_fact (
    event_id VARCHAR(64) NOT NULL COMMENT '全局唯一事件ID',
    tenant_id INT NOT NULL COMMENT '租户ID',
    event_time DATETIMEV2(3) NOT NULL COMMENT '客户端事件时间',

    user_id INT DEFAULT 0 COMMENT '登录用户ID',
    device_id VARCHAR(128) COMMENT '设备ID',
    account_id VARCHAR(128) COMMENT '业务账号ID',
    global_user_id VARCHAR(128) DEFAULT '' COMMENT '全局用户ID',
    event_ts BIGINT GENERATED ALWAYS AS (UNIX_TIMESTAMP(event_time)*1000) COMMENT '事件时间戳',
    server_time DATETIMEV2(3) DEFAULT CURRENT_TIMESTAMP(3) COMMENT '服务端接收时间',
    event_category VARCHAR(64) COMMENT '事件大类',
    event_name VARCHAR(128) COMMENT '事件名称',
    event_action VARCHAR(64) COMMENT '事件动作',
    object_type VARCHAR(64) COMMENT '对象类型',
    object_id VARCHAR(128) COMMENT '对象ID',
    object_name VARCHAR(256) COMMENT '对象名称',
    session_id BIGINT DEFAULT 0 COMMENT '会话ID',
    session_seq INT DEFAULT 0 COMMENT '会话内序号',
    platform VARCHAR(64) COMMENT '平台',
    os VARCHAR(128) COMMENT '系统',
    app_version VARCHAR(64) COMMENT 'APP版本',
    channel VARCHAR(128) COMMENT '渠道',
    user_agent VARCHAR(1024) COMMENT 'UA',
    ip VARCHAR(64) COMMENT 'IP',
    ip_city VARCHAR(128) COMMENT '城市',
    country VARCHAR(128) COMMENT '国家',
    geo VARCHAR(128) COMMENT '地理位置',
    network VARCHAR(64) COMMENT '网络',
    referer VARCHAR(1024) COMMENT '来源页',
    context MAP<STRING,STRING> COMMENT '上下文',
    duration_ms INT DEFAULT 0 COMMENT '耗时',
    amount DECIMAL(18,2) DEFAULT 0 COMMENT '金额',
    quantity INT DEFAULT 0 COMMENT '数量',
    score INT DEFAULT 0 COMMENT '分数',
    metrics MAP<STRING,DOUBLE> COMMENT '数值指标',
    properties MAP<STRING,STRING> COMMENT '扩展属性',
    op_result VARCHAR(64) COMMENT '执行结果',
    error_code VARCHAR(128) COMMENT '错误码',
    risk_level VARCHAR(64) COMMENT '风险等级',
    trace_id VARCHAR(128) COMMENT '链路ID',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间'
)
    UNIQUE KEY(event_id, tenant_id, event_time) -- 顺序必须和顶部字段一致
    PARTITION BY RANGE(event_time) ()
    DISTRIBUTED BY HASH(event_id, tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-180",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE events_fact MODIFY COMMENT "用户行为事件事实表";
ALTER TABLE events_fact ADD INDEX idx_object_id (object_id) USING INVERTED;
ALTER TABLE events_fact ADD INDEX idx_referer (referer) USING INVERTED;
ALTER TABLE events_fact ADD INDEX idx_user_agent (user_agent) USING INVERTED;
ALTER TABLE events_fact ADD INDEX idx_geo (geo) USING INVERTED;
ALTER TABLE events_fact ADD INDEX idx_risk (risk_level) USING INVERTED;
ALTER TABLE events_fact ADD INDEX idx_event_name (event_name) USING INVERTED;


-- ============================================================
-- 2. 用户会话事实表
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions_fact (
    id              BIGINT NOT NULL COMMENT '会话唯一ID',
    tenant_id       INT NOT NULL COMMENT '租户ID',
    session_date    DATE NOT NULL COMMENT '会话日期',
    user_id         INT DEFAULT 0 COMMENT '登录用户ID',
    device_id       VARCHAR(128) COMMENT '设备指纹',
    global_user_id  VARCHAR(128) DEFAULT '' COMMENT '全局用户ID',
    start_time      DATETIMEV2(3) NOT NULL COMMENT '会话开始时间',
    end_time        DATETIMEV2(3) COMMENT '会话结束时间',
    duration_ms     BIGINT DEFAULT 0 COMMENT '会话时长(ms)',
    event_count     INT DEFAULT 0 COMMENT '事件总数',
    page_view_count INT DEFAULT 0 COMMENT '页面浏览数',
    action_count    INT DEFAULT 0 COMMENT '交互操作数',
    entry_page      VARCHAR(1024) COMMENT '入口页面',
    exit_page       VARCHAR(1024) COMMENT '出口页面',
    is_bounce       TINYINT DEFAULT 0 COMMENT '是否跳出',
    platform        VARCHAR(64) COMMENT '平台类型',
    os              VARCHAR(128) COMMENT '操作系统',
    app_version     VARCHAR(64) COMMENT '应用版本',
    ip_city         VARCHAR(128) COMMENT 'IP所在城市',
    country         VARCHAR(128) COMMENT '国家/地区',
    total_amount    DECIMAL(18,2) DEFAULT 0 COMMENT '会话内总金额',
    pay_event_count INT DEFAULT 0 COMMENT '支付事件数',
    risk_level      VARCHAR(64) COMMENT '风险等级',
    risk_tags       ARRAY<STRING> COMMENT '风险标签数组',
    context         MAP<STRING,STRING> COMMENT '会话上下文',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间'
)
    UNIQUE KEY(id, tenant_id, session_date)
    PARTITION BY RANGE(session_date) ()
    DISTRIBUTED BY HASH(id, tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-90",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE events_fact MODIFY COMMENT "会话事实表（存储用户会话级聚合指标）";
ALTER TABLE sessions_fact ADD INDEX idx_entry_page (entry_page) USING INVERTED;
ALTER TABLE sessions_fact ADD INDEX idx_risk_level (risk_level) USING INVERTED;
ALTER TABLE sessions_fact ADD INDEX idx_is_bounce (is_bounce) USING INVERTED;


-- ============================================================
-- 3. 风险事件表
-- ============================================================
CREATE TABLE IF NOT EXISTS risk_events (
    id              BIGINT NOT NULL COMMENT '风险事件唯一ID',
    tenant_id       INT NOT NULL COMMENT '租户ID',
    event_date      DATE NOT NULL COMMENT '风险日期',
    user_id         INT DEFAULT 0 COMMENT '登录用户ID',
    device_id       VARCHAR(128) COMMENT '设备指纹',
    global_user_id  VARCHAR(128) DEFAULT '' COMMENT '全局用户ID',
    risk_type       VARCHAR(64) COMMENT '风险类型',
    risk_level      VARCHAR(64) COMMENT '风险等级',
    risk_score      FLOAT DEFAULT 0 COMMENT '风险评分',
    rule_id         INT DEFAULT 0 COMMENT '触发规则ID',
    rule_name       VARCHAR(256) COMMENT '触发规则名称',
    rule_context    MAP<STRING,STRING> COMMENT '规则触发上下文',
    related_event_ids ARRAY<STRING> COMMENT '关联行为事件ID数组',
    session_id      BIGINT DEFAULT 0 COMMENT '关联会话ID',
    description     VARCHAR(1024) COMMENT '风险描述',
    evidence        MAP<STRING,STRING> COMMENT '证据键值对',
    status          VARCHAR(64) COMMENT '处置状态',
    handler_id      VARCHAR(128) COMMENT '处置人ID',
    handled_time    DATETIMEV2(3) COMMENT '处置时间',
    handle_remark   VARCHAR(1024) COMMENT '处置备注',
    occur_time      DATETIMEV2(3) NOT NULL COMMENT '风险发生时间',
    report_time     DATETIMEV2(3) DEFAULT CURRENT_TIMESTAMP(3) COMMENT '风险上报时间',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间'
)
    UNIQUE KEY(id, tenant_id, event_date)
    PARTITION BY RANGE(event_date) ()
    DISTRIBUTED BY HASH(id, tenant_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-180",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE risk_events MODIFY COMMENT "风险事件表（存储风控规则触发的风险事件，支持风险处置、误报分析、规则优化）";
ALTER TABLE risk_events ADD INDEX idx_description (description) USING INVERTED;
ALTER TABLE risk_events ADD INDEX idx_status (status) USING INVERTED;
ALTER TABLE risk_events ADD INDEX idx_risk_level (risk_level) USING INVERTED;


-- ============================================================
-- 4. 用户维度表
-- ============================================================
CREATE TABLE IF NOT EXISTS users_dim (
    tenant_id         INT NOT NULL COMMENT '租户ID',
    user_id           INT NOT NULL COMMENT '登录用户ID',

    register_time     DATETIME COMMENT '注册时间',
    register_channel  VARCHAR(128) COMMENT '注册渠道',
    first_active_date DATE COMMENT '首次活跃日期',
    last_active_date  DATE COMMENT '最后活跃日期',

    user_level        SMALLINT DEFAULT 0 COMMENT '用户等级',
    vip_level         TINYINT DEFAULT 0 COMMENT 'VIP等级',
    user_role         VARCHAR(64) COMMENT '用户角色',

    total_events      BIGINT DEFAULT 0 COMMENT '累计事件数',
    total_sessions    INT DEFAULT 0 COMMENT '累计会话数',
    total_pay_amount  DECIMAL(18,2) DEFAULT 0 COMMENT '累计支付金额',
    last_pay_time     DATETIME COMMENT '最后支付时间',

    prefer_categories ARRAY<STRING> COMMENT '偏好事件分类',
    prefer_objects    ARRAY<STRING> COMMENT '偏好对象类型',

    -- 风险字段
    risk_score        TINYINT DEFAULT 0 COMMENT '用户风险评分',
    risk_tags         ARRAY<STRING> COMMENT '用户风险标签数组',
    risk_level        VARCHAR(32) COMMENT '风险等级 low/medium/high/black',
    last_risk_time    DATETIME COMMENT '最后一次风险事件时间',

    geo               MAP<STRING,STRING> COMMENT '地理位置 country/province/city/isp',
    platform          VARCHAR(64) COMMENT '客户端平台 ios/android/web/mini_program',
    device_type       VARCHAR(64) COMMENT '设备类型 mobile/pad/desktop/unknown',

    profile           MAP<STRING,STRING> COMMENT '自定义用户画像',

    -- 版本控制
    ver               BIGINT DEFAULT 1 COMMENT '数据版本号，更新+1',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间'
)
    UNIQUE KEY(tenant_id, user_id)
    DISTRIBUTED BY HASH(tenant_id, user_id) BUCKETS 10
    PROPERTIES (
                   "replication_num" = "1",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE users_dim MODIFY COMMENT '用户维度表（存储用户级别的预计算画像指标，支持用户圈选、分群分析、个性化推荐）';
ALTER TABLE users_dim ADD INDEX idx_risk_score (risk_score) USING INVERTED;
ALTER TABLE users_dim ADD INDEX idx_last_active (last_active_date) USING INVERTED;
ALTER TABLE users_dim ADD INDEX idx_risk_level (risk_level) USING INVERTED;


-- ============================================================
-- 5. 对象维度表
-- ============================================================
CREATE TABLE IF NOT EXISTS objects_dim (
    tenant_id     INT NOT NULL COMMENT '租户ID',
    object_type   VARCHAR(64) NOT NULL COMMENT '对象类型',
    object_id     VARCHAR(128) NOT NULL COMMENT '对象ID',
    object_name   VARCHAR(256) COMMENT '对象名称',
    category_path VARCHAR(512) COMMENT '分类路径',
    price         DECIMAL(18,2) DEFAULT 0 COMMENT '对象价格',
    currency      VARCHAR(32) COMMENT '货币类型',
    rarity        VARCHAR(32) COMMENT '稀有度',
    attributes    MAP<STRING,STRING> COMMENT '自定义属性',
    status        VARCHAR(64) COMMENT '对象状态',
    valid_from    DATETIME COMMENT '生效时间',
    valid_to      DATETIME COMMENT '失效时间',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间'
)
    UNIQUE KEY(tenant_id, object_type, object_id)
    DISTRIBUTED BY HASH(tenant_id, object_type, object_id) BUCKETS 10
    PROPERTIES (
                   "replication_num" = "1",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE objects_dim MODIFY COMMENT '对象维度表（存储业务对象的基本信息，如商品/道具/关卡/页面等，支持对象分析）';
ALTER TABLE objects_dim ADD INDEX idx_object_name (object_name) USING INVERTED;
ALTER TABLE objects_dim ADD INDEX idx_status (status) USING INVERTED;
ALTER TABLE objects_dim ADD INDEX idx_rarity (rarity) USING INVERTED;


-- ============================================================
-- 6. 用户身份 ID 关联映射表
-- ============================================================
CREATE TABLE IF NOT EXISTS id_mapping (
    global_user_id VARCHAR(128) NOT NULL COMMENT '全局用户ID',
    tenant_id      INT NOT NULL COMMENT '租户ID',
    id_type        VARCHAR(64) NOT NULL COMMENT '身份标识类型',
    id_value       VARCHAR(256) NOT NULL COMMENT '身份标识值',
    confidence     FLOAT DEFAULT 1.0 COMMENT '关联置信度',
    link_source    VARCHAR(64) COMMENT '关联来源',
    first_seen     DATETIME COMMENT '首次关联时间',
    last_seen      DATETIME COMMENT '最后活跃时间',
    is_active      TINYINT DEFAULT 1 COMMENT '是否有效',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间',
    updated_date   DATE GENERATED ALWAYS AS (DATE(updated_at)) COMMENT '更新日期'
)
    UNIQUE KEY(global_user_id, tenant_id, id_type, id_value)
    PARTITION BY LIST (tenant_id)
(
    PARTITION p1 VALUES IN (1),
    PARTITION p2 VALUES IN (2),
    PARTITION p3 VALUES IN (3)
)
    DISTRIBUTED BY HASH (tenant_id, id_type, id_value) BUCKETS 10
    PROPERTIES (
                   "replication_num" = "1",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE id_mapping MODIFY COMMENT 'ID关联映射表（打通匿名用户与登录用户的身份关联，支持跨设备/跨账号用户识别）';
ALTER TABLE id_mapping ADD INDEX idx_is_active (is_active) USING INVERTED;


-- ============================================================
-- 7. 用户标签关联表
-- ============================================================
CREATE TABLE IF NOT EXISTS user_tags (
    tenant_id      INT NOT NULL COMMENT '租户ID',
    user_id        INT NOT NULL COMMENT '登录用户ID',
    tag_id         INT NOT NULL COMMENT '标签定义ID',
    expire_date    DATE NOT NULL COMMENT '过期日期',
    tag_value      VARCHAR(256) COMMENT '标签值',
    value_label    VARCHAR(256) COMMENT '标签值显示名',
    confidence     FLOAT DEFAULT 1.0 COMMENT '标签置信度',
    source         VARCHAR(64) COMMENT '标签来源',
    source_rule_id INT DEFAULT 0 COMMENT '来源规则ID',
    effective_time DATETIMEV2(3) COMMENT '标签生效时间',
    expire_time    DATETIMEV2(3) COMMENT '标签过期时间',
    is_active      TINYINT DEFAULT 1 COMMENT '是否有效',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '记录更新时间'
)
    UNIQUE KEY(tenant_id, user_id, tag_id, expire_date)
    PARTITION BY RANGE(expire_date) ()
    DISTRIBUTED BY HASH(tenant_id, user_id, tag_id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-30",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE user_tags MODIFY COMMENT '用户标签关联表（存储用户与标签的关联关系，支持手动/规则/算法打标，支持有效期管理）';
ALTER TABLE user_tags ADD INDEX idx_is_active (is_active) USING INVERTED;
ALTER TABLE user_tags ADD INDEX idx_source (source) USING INVERTED;


-- ============================================================
-- 8. 用户行为路径特征表
-- ============================================================
CREATE TABLE IF NOT EXISTS path_features (
    id                VARCHAR(128) NOT NULL COMMENT '路径特征ID',
    tenant_id         INT NOT NULL COMMENT '租户ID',
    user_id           INT DEFAULT 0 COMMENT '登录用户ID',
    session_id        BIGINT DEFAULT 0 COMMENT '会话ID',
    path_hash         VARCHAR(128) COMMENT '路径序列哈希值',
    first_event       VARCHAR(128) COMMENT '入口事件',
    last_event        VARCHAR(128) COMMENT '出口事件',
    path_length       TINYINT DEFAULT 0 COMMENT '路径步数',
    first_3_events    ARRAY<STRING> COMMENT '前3步事件序列',
    last_3_events     ARRAY<STRING> COMMENT '后3步事件序列',
    is_converted      TINYINT DEFAULT 0 COMMENT '是否转化',
    conversion_event  VARCHAR(128) COMMENT '转化事件名称',
    conversion_time   DATETIMEV2(3) COMMENT '转化时间',
    start_time        DATETIMEV2(3) NOT NULL COMMENT '路径开始时间',
    end_time          DATETIMEV2(3) COMMENT '路径结束时间',
    event_date        DATE NOT NULL COMMENT '路径日期',
    total_duration_ms BIGINT DEFAULT 0 COMMENT '路径总耗时',
    step_count        TINYINT DEFAULT 0 COMMENT '路径步数'
)
    UNIQUE KEY(path_id, tenant_id, event_date)
    PARTITION BY RANGE(event_date) ()
    DISTRIBUTED BY HASH(tenant_id, id) BUCKETS 16
    PROPERTIES (
                   "replication_num" = "1",
                   "dynamic_partition.enable" = "true",
                   "dynamic_partition.time_unit" = "DAY",
                   "dynamic_partition.start" = "-90",
                   "dynamic_partition.end" = "3",
                   "dynamic_partition.prefix" = "p",
                   "light_schema_change" = "true",
                   "enable_unique_key_merge_on_write" = "true"
               );
ALTER TABLE path_features MODIFY COMMENT '用户行为路径特征表（存储用户会话内的行为序列，用于路径分析、漏斗分析、热门路径挖掘，支持转化追踪）';
ALTER TABLE path_features ADD INDEX idx_first_event (first_event) USING INVERTED;
ALTER TABLE path_features ADD INDEX idx_is_converted (is_converted) USING INVERTED;
ALTER TABLE path_features ADD INDEX idx_path_length (path_length) USING INVERTED;

-- ============================================================
-- UBA 系统 - Kafka 接入层
-- 执行顺序：2
-- ============================================================


--- ============================================================
-- 1. Kafka 引擎表 - 行为事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_events_raw
(
    -- ========== 主键字段 ==========
    event_id       String COMMENT '全局唯一事件 ID（ULID/Snowflake 生成，用于事件去重和追踪）',
    tenant_id      UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',

    -- ========== 主体：Who（谁产生的事件）==========
    user_id        UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户）',
    device_id      String COMMENT '设备指纹/匿名 ID（用于匿名用户追踪）',
    account_id     String COMMENT '业务账号 ID（游戏角色 ID/子账号 ID 等）',
    global_user_id String COMMENT '全局用户 ID（ID-Mapping 后统一标识）',

    -- ========== 时间：When（事件发生时间）==========
    event_time     DateTime64(3) COMMENT '客户端事件时间（用户设备上的事件发生时间，毫秒精度）',

    -- ========== 行为：What（发生了什么事件）==========
    event_category String COMMENT '事件大类（auth/pay/game/content/security）',
    event_name     String COMMENT '事件名称（login/level_up/purchase/click）',
    event_action   String COMMENT '事件动作（start/success/fail/retry）',

    -- ========== 客体：Object（事件作用对象）==========
    object_type    String COMMENT '对象类型（product/item/level/page/api）',
    object_id      String COMMENT '对象 ID（商品 ID/关卡 ID/页面 URL 等）',
    object_name    String COMMENT '对象名称（冗余字段，便于查询展示）',

    -- ========== 上下文：Context（事件上下文信息）==========
    session_id     UInt64 COMMENT '会话 ID（写入层生成，用于会话内事件序列分析）',
    session_seq    UInt32 COMMENT '会话内事件序号（事件在会话中的顺序号）',

    platform       String COMMENT '平台类型（iOS/Android/Web/H5/小程序）',
    os             String COMMENT '操作系统（iOS 15.0/Android 12/Windows 11）',
    app_version    String COMMENT '应用版本（1.0.0/2.3.1）',
    channel        String COMMENT '渠道来源（app_store/google_play/huawei）',
    user_agent     String COMMENT '用户代理字符串（用于解析设备型号和浏览器信息）',

    ip             String COMMENT '客户端 IP 地址（用于地理位置解析和风控识别）',
    ip_city        String COMMENT 'IP 所在城市（用于地域分析）',
    country        String COMMENT '国家/地区（用于国际化分析）',
    network        String COMMENT '网络类型（WiFi/4G/5G/以太网）',
    geo            String COMMENT '地理位置信息（经纬度坐标，JSON 格式）',
    referrer       String COMMENT '来源 URL（用户访问来源页面 URL）',

    context        String COMMENT '通用业务上下文（JSON 格式）',

    -- ========== 指标：Metrics（事件数值指标）==========
    duration_ms    UInt32 COMMENT '事件耗时（页面停留时长/接口响应时间，单位毫秒）',
    amount         Decimal(18, 2) COMMENT '事件金额（充值金额/订单金额，单位元）',
    quantity       UInt32 COMMENT '事件数量（道具数量/商品数量）',
    score          Int32 COMMENT '事件分数（游戏得分/信用积分/风险评分）',

    metrics        String COMMENT '扩展数值指标（JSON 格式）',

    -- ========== 扩展：Properties（事件自定义属性）==========
    properties     String COMMENT '扩展业务属性（JSON 格式）',

    -- ========== 企业级字段（运营 & 风控）==========
    op_result      String COMMENT '执行结果（success/failed/timeout）',
    error_code     String COMMENT '错误码（事件失败时的错误码）',
    risk_level     String COMMENT '风险等级（normal/suspicious/high，实时风控标记）',
    trace_id       String COMMENT '链路追踪 ID（关联微服务调用链）',

    -- ========== Kafka 元数据（自动填充）==========
    _topic         String COMMENT 'Kafka Topic 名称（自动填充）',
    _partition     Int32 COMMENT 'Kafka 分区号（自动填充）',
    _offset        Int64 COMMENT 'Kafka 偏移量（自动填充）',
    _timestamp     DateTime COMMENT 'Kafka 消息时间戳（自动填充）',
    _key           String COMMENT 'Kafka 消息 Key（自动填充）'
    ) ENGINE = Kafka
    SETTINGS
    kafka_broker_list = 'host.docker.internal:9092',
    kafka_topic_list = 'uba.events.raw',
    kafka_group_name = 'uba-ingest-ch',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536,
    kafka_skip_broken_messages = 1,
    kafka_commit_on_select = 1;
;


-- ============================================================
-- 2. Kafka 引擎表 - 风险事件接入
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.kafka_risk_events_raw
(
    -- ========== 主键字段 ==========
    id                UInt64 COMMENT '风险事件唯一 ID（ULID/Snowflake 生成）',
    tenant_id         UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',

    -- ========== 关联主体：Who（谁触发风险）==========
    user_id           UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户）',
    device_id         String COMMENT '设备指纹（用于匿名风险追踪）',
    global_user_id    String COMMENT '全局用户 ID（ID-Mapping 后统一标识）',

    -- ========== 风险类型 & 等级：What（什么风险）==========
    risk_type         String COMMENT '风险类型（login_anomaly/fraud_payment/brute_force/frequent_operation）',
    risk_level        String COMMENT '风险等级（normal/suspicious/high/critical）',
    risk_score        Float32 COMMENT '风险评分（0-100，分数越高风险越大）',

    -- ========== 触发信息：Why（为什么触发）==========
    rule_id           UInt32 COMMENT '触发规则 ID（关联 uba_risk_rules.id）',
    rule_name         String COMMENT '触发规则名称（冗余字段，便于查询展示）',
    rule_context      String COMMENT '规则触发上下文（JSON 格式：{"threshold": 5, "window": "300s"}）',

    -- ========== 关联行为事件：Evidence（证据链）==========
    related_event_ids Array(String) COMMENT '关联行为事件 ID 数组（触发风险的行为事件 ID 列表）',
    session_id        UInt64 COMMENT '关联会话 ID（关联 sessions_fact.session_id）',

    -- ========== 风险详情：Detail（风险详细信息）==========
    description       String COMMENT '风险描述（人类可读的风险说明）',
    evidence          String COMMENT '证据键值对（JSON 格式：{"ip": "192.168.1.1", "location": "Beijing"}）',

    -- ========== 处置状态：Status（风险处置流程）==========
    status            String COMMENT '处置状态（pending/confirmed/false_positive/ignored）',
    handler_id        String COMMENT '处置人 ID（处理该风险事件的运营人员 ID）',
    handled_time      DateTime64(3) COMMENT '处置时间（风险事件被处理的时间点）',
    handle_remark     String COMMENT '处置备注（运营人员的处置说明）',

    -- ========== 时间字段：When（风险时间信息）==========
    occur_time        DateTime64(3) COMMENT '风险发生时间（触发风险的行为发生时间）',
    report_time       DateTime64(3) COMMENT '风险上报时间（ClickHouse 接收到风险事件的时间）',

    -- ========== Kafka 元数据（自动填充）==========
    _topic            String COMMENT 'Kafka Topic 名称（自动填充）',
    _partition        Int32 COMMENT 'Kafka 分区号（自动填充）',
    _offset           Int64 COMMENT 'Kafka 偏移量（自动填充）',
    _timestamp        DateTime COMMENT 'Kafka 消息时间戳（自动填充）',
    _key              String COMMENT 'Kafka 消息 Key（自动填充）'
) ENGINE = Kafka -- Kafka 引擎表（虚拟表，实时消费 Kafka 数据，不存储）
      SETTINGS
          kafka_broker_list = 'host.docker.internal:9092', -- Kafka 集群地址
          kafka_topic_list = 'uba.risk.events', -- Kafka Topic 名称
          kafka_group_name = 'uba-alert-sender', -- 消费者组名称
          kafka_format = 'JSONEachRow', -- 数据格式
          kafka_num_consumers = 2, -- 消费者并发数（风险事件量较少）
          kafka_max_block_size = 32768, -- 每次拉取最大消息数
          kafka_skip_broken_messages = 1, -- 跳过解析失败的消息
          kafka_commit_on_select = 1 -- 查询后提交偏移量
;


-- ============================================================
-- 3. 物化视图 - Kafka 行为事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_events_to_fact
    TO gw_uba.events_fact
AS
SELECT
    event_id,
    tenant_id,
    user_id,
    device_id,
    account_id,
    global_user_id,
    event_time,
    now64(3)                                AS server_time,
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

    JSONExtract(context, 'Map(String, String)')        AS context,
    duration_ms,
    amount,
    quantity,
    score,

    JSONExtract(metrics, 'Map(String, Float64)')       AS metrics,
    JSONExtract(properties, 'Map(String, String)')     AS properties,

    op_result,
    error_code,
    risk_level,
    trace_id,
    now()                                   AS created_at,
    now()                                   AS updated_at

FROM gw_uba.kafka_events_raw;


-- ============================================================
-- 4. 物化视图 - Kafka 风险事件 → 事实表
-- ============================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS gw_uba.mv_kafka_risk_events_to_fact
    TO gw_uba.risk_events
AS
SELECT
    id,
    tenant_id,
    user_id,
    device_id,
    global_user_id,
    risk_type,
    risk_level,
    risk_score,
    rule_id,
    rule_name,

    JSONExtract(rule_context, 'Map(String, String)')   AS rule_context,
    related_event_ids,
    session_id,
    description,

    JSONExtract(evidence, 'Map(String, String)')      AS evidence,

    status,
    handler_id,
    handled_time,
    handle_remark,
    occur_time,
    report_time,

    now()                                              AS created_at,
    now()                                              AS updated_at

FROM gw_uba.kafka_risk_events_raw;

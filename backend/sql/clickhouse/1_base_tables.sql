-- ============================================================
-- UBA 系统 - 基础表（事实表 + 维度表）
-- 用途：存储原始行为数据、会话数据、风险数据及维度数据
-- 执行顺序：1
-- ============================================================


-- ============================================================
-- 1. 事件事实表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.events_fact
(
    -- ========== 主键 & 路由字段 ==========
    event_id       String COMMENT '全局唯一事件 ID（ULID/Snowflake 生成，用于事件去重和追踪）',
    tenant_id      UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',

    -- ========== 主体：Who（谁产生的事件）==========
    user_id        UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户，关联 uba_users.id）',
    device_id      String COMMENT '设备指纹/匿名 ID（用于匿名用户追踪，关联 id_mapping.id_value）',
    account_id     String COMMENT '业务账号 ID（游戏角色 ID/子账号 ID 等业务系统账号）',
    global_user_id String        DEFAULT '' COMMENT '全局用户 ID（ID-Mapping 后统一标识，用于跨设备/跨账号关联）',

    -- ========== 时间：When（事件发生时间）==========
    event_time     DateTime64(3) COMMENT '客户端事件时间（用户设备上的事件发生时间，毫秒精度）',
    event_date     Date MATERIALIZED toDate(event_time) COMMENT '事件日期（物化列，用于分区和 TTL 清理）',
    event_ts       Int64 MATERIALIZED toUnixTimestamp64Milli(event_time) COMMENT '事件时间戳（毫秒级 Unix 时间戳，便于时间范围查询和排序）',
    server_time    DateTime64(3) DEFAULT now64(3) COMMENT '服务端接收时间（ClickHouse 接收到事件的时间，用于计算上报延迟）',

    -- ========== 行为：What（发生了什么事件）==========
    event_category LowCardinality(String) COMMENT '事件大类（一级分类：auth 认证/pay 支付/game 游戏/content 内容/security 安全）',
    event_name     LowCardinality(String) COMMENT '事件名称（二级分类：login 登录/level_up 升级/purchase 购买/click 点击）',
    event_action   LowCardinality(String) COMMENT '事件动作（start 开始/success 成功/fail 失败/retry 重试）',

    -- ========== 客体：Object（事件作用对象）==========
    object_type    LowCardinality(String) COMMENT '对象类型（product 商品/item 道具/level 关卡/page 页面/api 接口）',
    object_id      String COMMENT '对象 ID（商品 ID/关卡 ID/页面 URL 等具体对象标识）',
    object_name    String COMMENT '对象名称（冗余字段，便于查询展示，避免关联对象维度表）',

    -- ========== 上下文：Context（事件上下文信息）==========
    -- 会话上下文
    session_id     UInt64 COMMENT '会话 ID（写入层生成，关联 sessions_fact.session_id，用于会话内事件序列分析）',
    session_seq    UInt32 COMMENT '会话内事件序号（事件在会话中的顺序号，用于还原事件序列）',

    -- 环境上下文
    platform       LowCardinality(String) COMMENT '平台类型（iOS/Android/Web/H5/小程序）',
    os             LowCardinality(String) COMMENT '操作系统（iOS 15.0/Android 12/Windows 11）',
    app_version    LowCardinality(String) COMMENT '应用版本（1.0.0/2.3.1，用于版本分析）',
    channel        String COMMENT '渠道来源（app_store/google_play/huawei/oppo 应用商店）',
    user_agent     String COMMENT '用户代理字符串（原始 UA，用于解析详细浏览器版本/爬虫识别/设备型号）',

    -- 网络 & 位置上下文
    ip             String COMMENT '客户端 IP 地址（用于地理位置解析和风控识别）',
    ip_city        LowCardinality(String) COMMENT 'IP 所在城市（用于地域分析）',
    country        LowCardinality(String) COMMENT '国家/地区（用于国际化分析）',
    geo            String COMMENT '地理位置信息（GeoHash 或 经纬度字符串 "lat,lon"，用于地图可视化及附近搜索）',
    network        LowCardinality(String) COMMENT '网络类型（WiFi/4G/5G/以太网）',
    referer        String COMMENT '来源页面 URL（用于流量来源分析、防盗链、漏斗上游分析）',

    -- 业务上下文
    context        Map(String, String) COMMENT '通用业务上下文（扩展字段：{server_id: s1, zone: cn-east, ab_group: B}）',

    -- ========== 指标：Metrics（事件数值指标）==========
    -- 通用数值指标（固定列 + Map 混合设计）
    duration_ms    UInt32 COMMENT '事件耗时（页面停留时长/接口响应时间/关卡通关时间，单位毫秒）',
    amount         Decimal(18, 2) COMMENT '事件金额（充值金额/订单金额/打赏金额，单位元）',
    quantity       UInt32 COMMENT '事件数量（道具数量/商品数量/积分数量）',
    score          Int32 COMMENT '事件分数（游戏得分/信用积分/风险评分）',

    metrics        Map(String, Float64) COMMENT '扩展数值指标（{damage: 1200, exp_gain: 50, fps: 59.8}）',

    -- ========== 扩展：Properties（事件自定义属性）==========
    properties     Map(String, String) COMMENT '扩展业务属性（{item_rarity: SSR, payment_method: alipay, level_difficulty: hard}）',

    -- ========== 企业级字段（运营 & 风控）==========
    op_result      LowCardinality(String) COMMENT '执行结果（success 成功/failed 失败/timeout 超时）',
    error_code     String COMMENT '错误码（事件失败时的错误码，用于错误分析）',
    risk_level     LowCardinality(String) COMMENT '风险等级（normal 正常/suspicious 可疑/high 高风险，实时风控标记）',
    trace_id       String COMMENT '链路追踪 ID（关联微服务调用链，用于问题排查）',

    -- ========== 审计字段（系统管理）==========
    created_at     DateTime      DEFAULT now() COMMENT '记录创建时间（数据写入 ClickHouse 的时间）',
    updated_at     DateTime      DEFAULT now() COMMENT '记录更新时间（用于审计追踪，MergeTree 无需版本控制）',

    -- ========== 跳数索引 ==========
    INDEX idx_object_id object_id TYPE bloom_filter(0.01) GRANULARITY 4,           -- 加速对象 ID 精确查询
    INDEX idx_context_keys mapKeys(context) TYPE bloom_filter(0.01) GRANULARITY 2, -- 加速上下文键名查询
    INDEX idx_risk risk_level TYPE set(4) GRANULARITY 1,                           -- 加速风险等级筛选
    INDEX idx_geo geo TYPE bloom_filter(0.01) GRANULARITY 4,                       -- 加速地理位置/GeoHash 前缀查询
    INDEX idx_referer referer TYPE bloom_filter(0.01) GRANULARITY 4                -- 加速来源域名/URL 过滤查询
) ENGINE = MergeTree -- 使用 MergeTree（事件只追加写入，不可变，无需去重）
      PARTITION BY toYYYYMM(event_date) -- 按月分区，平衡管理粒度和查询性能
      ORDER BY (tenant_id, event_category, event_date, event_name, event_ts) -- 按租户 + 分类 + 日期 + 事件名 + 时间戳排序，优化常见查询
      TTL event_date + INTERVAL 180 DAY -- 180 天前的事件自动清理，节省存储空间
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '事件事实表（存储所有用户行为事件，支持行为分析、漏斗分析、用户画像）';


-- ============================================================
-- 2. 用户会话事实表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.sessions_fact
(
    -- ========== 主键 & 路由字段 ==========
    id              UInt64 COMMENT '会话唯一 ID（写入层生成，用于关联 events_fact.session_id）',
    tenant_id       UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',

    -- ========== 主体：Who（谁的会话）==========
    user_id         UInt32 COMMENT '登录用户 ID（可为 0 表示匿名会话，关联 uba_users.id）',
    device_id       String COMMENT '设备指纹（用于匿名会话追踪，关联 id_mapping.id_value）',
    global_user_id  String   DEFAULT '' COMMENT '全局用户 ID（ID-Mapping 后统一标识，用于跨设备会话关联）',

    -- ========== 时间：When（会话时间范围）==========
    start_time      DateTime64(3) COMMENT '会话开始时间（会话中第一个事件的发生时间）',
    end_time        Nullable(DateTime64(3)) COMMENT '会话结束时间（会话中最后一个事件的发生时间，会话关闭时更新）',
    session_date    Date MATERIALIZED toDate(start_time) COMMENT '会话日期（物化列，用于分区和 TTL 清理）',
    duration_ms     UInt64 COMMENT '会话时长（end_time - start_time 的毫秒数，用于会话质量分析）',

    -- ========== 会话指标：How Many（会话内事件统计）==========
    event_count     UInt32 COMMENT '事件总数（会话内发生的事件数量，用于会话活跃度分析）',
    page_view_count UInt32 COMMENT '页面浏览数（会话内页面浏览事件数量，用于内容消费分析）',
    action_count    UInt32 COMMENT '交互操作数（会话内用户交互事件数量，如点击/滑动/输入）',

    -- ========== 路径：Where（会话入口和出口）==========
    entry_page      String COMMENT '入口页面（会话第一个事件的页面 URL，用于流量来源分析）',
    exit_page       String COMMENT '出口页面（会话最后一个事件的页面 URL，用于流失分析）',
    is_bounce       UInt8 COMMENT '是否跳出（0/1，单页面会话为跳出，用于跳出率计算）',

    -- ========== 环境快照：Context（会话环境信息）==========
    platform        LowCardinality(String) COMMENT '平台类型（会话期间的平台，如 iOS/Android/Web）',
    os              LowCardinality(String) COMMENT '操作系统（会话期间的操作系统版本）',
    app_version     LowCardinality(String) COMMENT '应用版本（会话期间的应用版本号）',
    ip_city         LowCardinality(String) COMMENT 'IP 所在城市（会话期间的地理位置）',
    country         LowCardinality(String) COMMENT '国家/地区（会话期间的国家/地区）',

    -- ========== 业务指标：Business（会话业务价值）==========
    total_amount    Decimal(18, 2) COMMENT '会话内总金额（会话内所有支付事件的金额总和）',
    pay_event_count UInt32 COMMENT '支付事件数（会话内支付事件的数量，用于转化分析）',

    -- ========== 风险标记：Risk（会话风险评估）==========
    risk_level      LowCardinality(String) COMMENT '风险等级（normal 正常/suspicious 可疑/high 高风险，实时风控标记）',
    risk_tags       Array(String) COMMENT '风险标签数组（如["frequent_login_fail", "abnormal_location"]）',

    -- ========== 扩展属性：Extension（会话扩展信息）==========
    context         Map(String, String) COMMENT '会话上下文（扩展字段：{server_id: s1, zone: cn-east, ab_group: B}）',

    -- ========== 审计字段：Audit（系统管理）==========
    created_at      DateTime DEFAULT now() COMMENT '记录创建时间（会话创建时写入 ClickHouse 的时间）',
    updated_at      DateTime DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制，会话结束时会更新）',

    -- ========== 跳数索引：Index（加速查询）==========
    INDEX idx_duration duration_ms TYPE minmax GRANULARITY 2,                    -- 加速会话时长范围查询
    INDEX idx_risk risk_level TYPE set(4) GRANULARITY 1,                         -- 加速风险等级筛选
    INDEX idx_bounce is_bounce TYPE set(2) GRANULARITY 1,                        -- 加速跳出会话筛选
    INDEX idx_entry_page entry_page TYPE ngrambf_v1(3, 1024, 3, 0) GRANULARITY 2 -- 加速入口页面模糊查询
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 ReplacingMergeTree（会话开始时创建，结束时需要更新 end_time 和 duration_ms）
      PARTITION BY toYYYYMM(session_date) -- 按月分区，平衡管理粒度和查询性能
      ORDER BY (tenant_id, id, session_date) -- 按租户 + 日期 + ID 排序，优化用户会话查询
      TTL session_date + INTERVAL 90 DAY -- 90 天前的会话自动清理，节省存储空间
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '会话事实表（存储用户会话级聚合指标，支持会话分析、跳出率分析、转化分析）';


-- ============================================================
-- 3. 风险事件表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.risk_events
(
    -- ========== 主键字段 ==========
    id                UInt64 COMMENT '风险事件唯一 ID（Snowflake 生成，用于风险事件追踪和处置）',
    tenant_id         UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',

    -- ========== 关联主体：Who（谁触发风险）==========
    user_id           UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户，关联 uba_users.id）',
    device_id         String COMMENT '设备指纹（用于匿名风险追踪，关联 id_mapping.id_value）',
    global_user_id    String        DEFAULT '' COMMENT '全局用户 ID（ID-Mapping 后统一标识，用于跨设备风险关联）',

    -- ========== 风险类型 & 等级：What（什么风险）==========
    risk_type         LowCardinality(String) COMMENT '风险类型（login_anomaly 登录异常/fraud_payment 欺诈支付/brute_force 暴力破解/frequent_operation 频繁操作）',
    risk_level        LowCardinality(String) COMMENT '风险等级（normal 正常/suspicious 可疑/high 高/critical 严重，用于处置优先级）',
    risk_score        Float32 COMMENT '风险评分（0-100，分数越高风险越大，用于风险排序和阈值筛选）',

    -- ========== 触发信息：Why（为什么触发）==========
    rule_id           UInt32 COMMENT '触发规则 ID（关联 uba_risk_rules.id，用于规则效果分析）',
    rule_name         String COMMENT '触发规则名称（冗余字段，便于查询展示，避免关联规则表）',
    rule_context      Map(String, String) COMMENT '规则触发上下文（{threshold: 5, window: 300s, current_count: 8}）',

    -- ========== 关联行为事件：Evidence（证据链）==========
    related_event_ids Array(String) COMMENT '关联行为事件 ID 数组（触发风险的行为事件 ID 列表，用于证据追溯）',
    session_id        UInt64 COMMENT '关联会话 ID（关联 sessions_fact.session_id，用于会话内风险分析）',

    -- ========== 风险详情：Detail（风险详细信息）==========
    description       String COMMENT '风险描述（人类可读的风险说明，如"1 小时内登录失败 8 次"）',
    evidence          Map(String, String) COMMENT '证据键值对（{ip: 192.168.1.1, location: Beijing, device: iPhone13}）',

    -- ========== 处置状态：Status（风险处置流程）==========
    status            LowCardinality(String) COMMENT '处置状态（pending 待处理/confirmed 已确认/false_positive 误报/ignored 已忽略）',
    handler_id        String COMMENT '处置人 ID（处理该风险事件的运营人员 ID）',
    handled_time      Nullable(DateTime64(3)) COMMENT '处置时间（风险事件被处理的时间点）',
    handle_remark     String COMMENT '处置备注（运营人员的处置说明和备注）',

    -- ========== 时间字段：When（风险时间信息）==========
    occur_time        Nullable(DateTime64(3)) COMMENT '风险发生时间（触发风险的行为发生时间，毫秒精度）',
    report_time       Nullable(DateTime64(3)) DEFAULT now64(3) COMMENT '风险上报时间（ClickHouse 接收到风险事件的时间）',
    event_date        Date MATERIALIZED toDate(occur_time) COMMENT '风险日期（物化列，用于分区和 TTL 清理）',

    -- ========== 审计字段：Audit（系统管理）==========
    created_at        DateTime      DEFAULT now() COMMENT '记录创建时间（风险事件写入 ClickHouse 的时间）',
    updated_at        DateTime      DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制，处置状态更新时变化）',

    -- ========== 跳数索引 ==========
    INDEX idx_risk_score risk_score TYPE minmax GRANULARITY 1,                  -- 加速风险评分范围查询
    INDEX idx_status status TYPE set(10) GRANULARITY 1,                         -- 加速处置状态筛选
    INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4,            -- 加速用户风险查询
    INDEX idx_rule_id rule_id TYPE bloom_filter(0.01) GRANULARITY 4,            -- 加速规则效果分析
    INDEX idx_description description TYPE tokenbf_v1(1024, 3, 0) GRANULARITY 2 -- 加速风险描述全文搜索
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 ReplacingMergeTree（风险事件处置时需要更新 status、handler_id 等字段）
      PARTITION BY toYYYYMM(event_date) -- 按月分区，平衡管理粒度和查询性能
      ORDER BY (tenant_id, event_date, risk_level, occur_time) -- 按租户 + 日期 + 风险等级 + 发生时间排序，优化待处理风险查询
      TTL event_date + INTERVAL 180 DAY -- 180 天前的风险事件自动清理，节省存储空间
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '风险事件表（存储风控规则触发的风险事件，支持风险处置、误报分析、规则优化）';


-- ============================================================
-- 4. 用户维度表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.users_dim
(
    -- ========== 主键字段 ==========
    tenant_id         UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',
    user_id           UInt32 COMMENT '登录用户 ID（关联 uba_users.id，主键的一部分）',

    -- ========== 基础属性：Basic（用户基本信息）==========
    register_time     Nullable(DateTime) COMMENT '注册时间（用户首次注册的时间点）',
    register_channel  String COMMENT '注册渠道（app_store/google_play/huawei/web 等注册来源）',
    first_active_date Date COMMENT '首次活跃日期（用户第一次产生行为事件的日期）',
    last_active_date  Date COMMENT '最后活跃日期（用户最后一次产生行为事件的日期，用于流失分析）',

    -- ========== 身份属性：Identity（用户身份标识）==========
    user_level        UInt16 COMMENT '用户等级/玩家等级（游戏等级/会员等级，用于分层运营）',
    vip_level         UInt8 COMMENT 'VIP 等级（0-10，VIP 会员等级，用于特权服务）',
    user_role         LowCardinality(String) COMMENT '用户角色（player 普通玩家/admin 管理员/guest 游客/vip 会员）',

    -- ========== 行为画像：Behavior（用户行为预计算）==========
    total_events      UInt64 COMMENT '累计事件数（用户历史产生的所有事件数量）',
    total_sessions    UInt32 COMMENT '累计会话数（用户历史产生的所有会话数量）',
    total_pay_amount  Decimal(18, 2) COMMENT '累计支付金额（用户历史所有支付事件的金额总和，单位元）',
    last_pay_time     Nullable(DateTime) COMMENT '最后支付时间（用户最后一次支付的时间点，用于付费用户分析）',

    -- ========== 偏好标签：Preference（用户偏好预计算）==========
    prefer_categories Array(String) COMMENT '偏好事件分类（用户最常参与的事件分类，如["game", "social", "pay"]）',
    prefer_objects    Array(String) COMMENT '偏好对象类型（用户最常交互的对象类型，如["pvp", "rpg", "shooter"]）',

    -- ========== 风险画像：Risk（用户风险预计算）==========
    risk_score        UInt8    DEFAULT 0 COMMENT '用户风险评分（0-100，综合历史风险事件计算，用于风险用户识别）',
    risk_level        LowCardinality(String) COMMENT '风险等级 low/medium/high/black',
    risk_tags         Array(String) COMMENT '用户风险标签数组（如["frequent_login_fail", "abnormal_location", "suspicious_payment"]）',
    last_risk_time    Nullable(DateTime) COMMENT '最后风险时间',

    -- ========== 设备&环境（新增）==========
    geo               Map(String, String) COMMENT '地理位置 country/province/city/isp',
    platform          LowCardinality(String) COMMENT '平台 ios/android/web/mini_program',
    device_type       LowCardinality(String) COMMENT '设备类型 mobile/pad/desktop',

    -- ========== 扩展属性：Extension（用户扩展信息）==========
    profile           Map(String, String) COMMENT '自定义用户画像（扩展字段：{guild_id: 1001, server: cn-1, ab_group: B}）',

    -- ========== 审计字段：Audit（系统管理）==========
    ver               UInt64 DEFAULT 1 COMMENT '数据版本号，更新+1',
    created_at        DateTime DEFAULT now() COMMENT '记录创建时间（用户画像首次写入 ClickHouse 的时间）',
    updated_at        DateTime DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制，用户画像更新时变化）',

    -- ========== 跳数索引：Index ==========
    INDEX idx_risk_score risk_score TYPE minmax GRANULARITY 1,       -- 加速用户风险评分范围查询
    INDEX idx_last_active last_active_date TYPE minmax GRANULARITY 1 -- 加速最后活跃日期范围查询
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 ReplacingMergeTree（用户画像需要定期更新，如 last_active_date、total_events 等）
      ORDER BY (tenant_id, user_id) -- 按租户 + 用户 ID 排序，优化单用户画像查询
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '用户维度表（存储用户级别的预计算画像指标，支持用户圈选、分群分析、个性化推荐）';


-- ============================================================
-- 5. 对象维度表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.objects_dim
(
    -- ========== 主键字段 ==========
    id            String COMMENT '对象 ID（业务系统中的对象唯一标识，如商品 SKU/道具 ID/关卡 ID）',
    tenant_id     UInt32 COMMENT '租户 ID（SaaS 多租户隔离，所有查询必须带此条件）',
    object_type   LowCardinality(String) COMMENT '对象类型（game_item 游戏道具/product 商品/article 文章/level 关卡/page 页面/api 接口）',

    -- ========== 基础信息：Basic（对象基本信息）==========
    object_name   String COMMENT '对象名称（人类可读的对象名称，如"屠龙刀"/"iPhone 15"/"第一关"）',
    category_path String COMMENT '分类路径（对象的分类层级路径，如"game/equipment/weapon"）',

    -- ========== 属性：Attributes（对象结构化属性）==========
    price         Decimal(18, 2) COMMENT '对象价格（商品/道具的定价，单位元，用于价值分析）',
    currency      LowCardinality(String) COMMENT '货币类型（CNY/USD/GOLD/DIAMOND 等货币/积分类型）',
    rarity        LowCardinality(String) COMMENT '稀有度（N 普通/R 稀有/SR 史诗/SSR 传说，用于游戏道具分级）',

    -- ========== 扩展属性：Extension（对象自定义属性）==========
    attributes    Map(String, String) COMMENT '自定义属性（扩展字段：{attack: 120, durability: 100, color: red, size: L}）',

    -- ========== 状态：Status（对象生命周期状态）==========
    status        LowCardinality(String) COMMENT '对象状态（online 上架/online 在售/offline 下架/discontinued 停产）',
    valid_from    Nullable(DateTime) COMMENT '生效时间（对象信息开始生效的时间点）',
    valid_to      Nullable(DateTime) COMMENT '失效时间（对象信息失效的时间点，NULL 表示长期有效）',

    -- ========== 审计字段：Audit（系统管理）==========
    created_at    DateTime DEFAULT now() COMMENT '记录创建时间（对象信息首次写入 ClickHouse 的时间）',
    updated_at    DateTime DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制，对象信息更新时变化）',

    -- ========== 跳数索引：Index（加速查询）==========
    INDEX idx_object_name object_name TYPE ngrambf_v1(3, 1024, 3, 0) GRANULARITY 2, -- 加速对象名称模糊查询
    INDEX idx_status status TYPE set(10) GRANULARITY 1,                             -- 加速对象状态筛选
    INDEX idx_rarity rarity TYPE set(10) GRANULARITY 1                              -- 加速稀有度筛选
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 ReplacingMergeTree（对象信息需要更新，如价格调整、状态变更）
      ORDER BY (tenant_id, object_type, id) -- 按租户 + 对象类型 + 对象 ID 排序，优化单对象查询
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '对象维度表（存储业务对象的基本信息，如商品/道具/关卡/页面等，支持对象分析）';


-- ============================================================
-- 6. 用户身份 ID 关联映射表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.id_mapping
(
    -- ========== 主键字段 ==========
    global_user_id String COMMENT '全局用户 ID（打通后的统一标识，用于跨设备/跨账号关联）',
    tenant_id      UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',

    -- ========== 身份标识字段 ==========
    id_type        LowCardinality(String) COMMENT '身份标识类型：user_id（登录用户）/device_id（设备指纹）/cookie（浏览器 Cookie）/email（邮箱）/phone（手机号）',
    id_value       String COMMENT '身份标识值（具体 ID 值，如用户 ID 数字、设备 UUID 等）',

    -- ========== 关联信息字段 ==========
    confidence     Float32  DEFAULT 1.0 COMMENT '关联置信度（0-1，1 表示确定关联，用于算法关联时区分可信度）',
    link_source    LowCardinality(String) COMMENT '关联来源：login（用户登录）/bind（手动绑定）/algorithm（算法推荐）/device（同设备识别）',

    -- ========== 时效字段 ==========
    first_seen     Nullable (DateTime) COMMENT '首次关联时间（该身份标识第一次出现的时间）',
    last_seen      Nullable (DateTime) COMMENT '最后活跃时间（该身份标识最后一次活跃的时间，用于 TTL 清理）',
    is_active      UInt8    DEFAULT 1 COMMENT '是否有效：1（有效）/0（已失效，如用户解绑）',

    -- ========== 审计字段 ==========
    created_at     DateTime DEFAULT now() COMMENT '记录创建时间（数据写入 ClickHouse 的时间）',
    updated_at     DateTime DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制）',
    updated_date   Date MATERIALIZED toDate(updated_at) COMMENT '更新日期（物化列，用于 TTL 分区清理）',

    INDEX idx_active is_active TYPE set(2) GRANULARITY 1 -- 加速有效关联关系查询
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 updated_at 作为版本列，支持关联关系更新
      PARTITION BY tenant_id -- 按租户分区，支持多租户隔离
      ORDER BY (tenant_id, id_type, id_value) -- 按租户 + 标识类型 + 标识值排序，支持快速查找
      TTL updated_date + INTERVAL 365 DAY -- 365 天未更新的关联关系自动清理
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT 'ID 关联映射表（打通匿名用户与登录用户的身份关联，支持跨设备/跨账号用户识别）';


-- ============================================================
-- 7. 用户标签关联表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.user_tags
(
    -- ========== 主键字段 ==========
    tenant_id      UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',
    user_id        UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户）',
    tag_id         UInt32 COMMENT '标签定义 ID（关联 uba_tag_definitions.id）',

    -- ========== 标签值字段 ==========
    tag_value      String COMMENT '标签值（统一用 String 存储，数值/枚举在应用层解析）',
    value_label    String COMMENT '标签值显示名称（枚举值的中文/英文显示名，如"高价值"）',

    -- ========== 置信度字段 ==========
    confidence     Float32  DEFAULT 1.0 COMMENT '标签置信度（0-1，算法打标时区分可信度，手动打标为 1.0）',

    -- ========== 来源字段 ==========
    source         LowCardinality(String) COMMENT '标签来源：manual（人工打标）/rule（规则引擎）/model（算法模型）/import（批量导入）',
    source_rule_id UInt32 COMMENT '来源规则 ID（当 source=rule 时，关联触发该标签的规则 ID）',

    -- ========== 时效字段 ==========
    effective_time Nullable(DateTime64(3)) COMMENT '标签生效时间（标签开始生效的时间点）',
    expire_time    Nullable(DateTime64(3)) COMMENT '标签过期时间（标签失效的时间点，NULL 表示永久有效）',
    expire_date    Date MATERIALIZED COALESCE(toDate(expire_time), toDateTime('2100-01-01')) COMMENT '过期日期（物化列，用于 TTL 清理，永久有效标签设为 2100 年）',
    is_active      UInt8    DEFAULT 1 COMMENT '是否有效：1（有效）/0（已失效，如手动取消标签）',

    -- ========== 审计字段 ==========
    created_at     DateTime DEFAULT now() COMMENT '记录创建时间（标签关联创建的时间）',
    updated_at     DateTime DEFAULT now() COMMENT '记录更新时间（用于 ReplacingMergeTree 版本控制）',

    -- ========== 跳数索引 ==========
    INDEX idx_active is_active TYPE set(2) GRANULARITY 1,               -- 加速有效标签查询
    INDEX idx_source source TYPE set(10) GRANULARITY 1,                 -- 加速按来源筛选
    INDEX idx_tag_value tag_value TYPE bloom_filter(0.01) GRANULARITY 4 -- 加速标签值模糊查询
) ENGINE = ReplacingMergeTree(updated_at) -- 使用 updated_at 作为版本列，支持标签更新/覆盖
      PARTITION BY toYYYYMM(effective_time) -- 按标签生效时间月度分区，便于按时间范围查询
      ORDER BY (tenant_id, user_id, tag_id, effective_time) -- 按租户 + 用户 + 标签 + 生效时间排序
      TTL expire_date + INTERVAL 1 DAY
          DELETE -- 过期标签 1 天后自动删除，节省存储空间
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '用户标签关联表（存储用户与标签的关联关系，支持手动打标、规则打标、算法打标，支持标签有效期管理）';


-- ============================================================
-- 8. 用户行为路径特征表
-- ============================================================
CREATE TABLE IF NOT EXISTS gw_uba.path_features
(
    -- ========== 主键字段 ==========
    id                String COMMENT '路径特征 ID（路径的唯一标识，可用 hash 生成）',
    tenant_id         UInt32 COMMENT '租户 ID（SaaS 多租户隔离）',

    -- ========== 关联主体字段 ==========
    user_id           UInt32 COMMENT '登录用户 ID（可为 0 表示匿名用户）',
    session_id        UInt64 COMMENT '会话 ID（关联 sessions_fact.session_id，用于会话内路径分析）',

    -- ========== 路径摘要字段 ==========
    path_hash         String COMMENT '路径序列哈希值（用于去重和聚合，相同路径有相同 hash）',
    first_event       LowCardinality(String) COMMENT '入口事件（路径的第一个事件名称，如"login"）',
    last_event        LowCardinality(String) COMMENT '出口事件（路径的最后一个事件名称，如"purchase"）',
    path_length       UInt8 COMMENT '路径步数（路径中包含的事件数量，用于筛选有效路径）',

    -- ========== 关键节点字段 ==========
    first_3_events    Array(String) COMMENT '前 3 步事件序列（用于快速匹配路径前缀，如["login", "browse", "cart"]）',
    last_3_events     Array(String) COMMENT '后 3 步事件序列（用于快速匹配路径后缀，如["cart", "checkout", "pay"]）',

    -- ========== 转化标记字段 ==========
    is_converted      UInt8    DEFAULT 0 COMMENT '是否转化：0（未转化）/1（已转化，如完成购买）',
    conversion_event  LowCardinality(String) COMMENT '转化事件名称（触发转化的事件，如"purchase_success"）',
    conversion_time   Nullable(DateTime64(3)) COMMENT '转化时间（转化发生的时间点，用于计算转化时长）',

    -- ========== 时间字段 ==========
    start_time        Nullable(DateTime64(3)) COMMENT '路径开始时间（路径中第一个事件的发生时间）',
    end_time          Nullable(DateTime64(3)) COMMENT '路径结束时间（路径中最后一个事件的发生时间）',
    event_date        Date MATERIALIZED toDate(start_time) COMMENT '路径日期（物化列，用于分区和 TTL 清理）',

    -- ========== 指标字段 ==========
    total_duration_ms UInt64 COMMENT '路径总耗时（从 start_time 到 end_time 的毫秒数）',
    step_count        UInt8 COMMENT '路径步数（与 path_length 相同，冗余便于查询）',

    -- ========== 跳数索引 ==========
    INDEX idx_first_event first_event TYPE set(100) GRANULARITY 2,         -- 加速入口事件筛选
    INDEX idx_converted is_converted TYPE set(2) GRANULARITY 1,            -- 加速转化路径筛选
    INDEX idx_path_length path_length TYPE minmax GRANULARITY 1,           -- 加速路径长度范围查询'
    INDEX idx_first_3 first_3_events TYPE bloom_filter(0.01) GRANULARITY 4 -- 加速路径前缀匹配
) ENGINE = MergeTree -- 使用 MergeTree（只追加写入，路径特征不需要更新）
      PARTITION BY toYYYYMM(event_date) -- 按路径开始时间月度分区，便于按时间范围查询
      ORDER BY (tenant_id, event_date, path_hash, start_time) -- 按租户 + 日期 + 路径哈希 + 开始时间排序
      TTL event_date + INTERVAL 90 DAY -- 90 天前的路径自动清理，节省存储空间
      SETTINGS
          index_granularity = 8192, -- 索引粒度，平衡查询性能和存储开销
          enable_mixed_granularity_parts = 1, -- 启用混合粒度分区，支持大文本字段
          ttl_only_drop_parts = 1, -- TTL 只删除完整分区，避免部分删除开销
          min_bytes_for_wide_part = 10485760 -- 10MB，宽分区最小字节数，优化合并策略
      COMMENT '用户行为路径特征表（存储用户会话内的行为序列，用于路径分析、漏斗分析、热门路径挖掘，支持转化追踪）';

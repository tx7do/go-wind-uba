-- 插入 uba_tag_definitions 测试数据
INSERT INTO public.uba_tag_definitions (
    created_at,
    updated_at,
    deleted_at,
    created_by,
    updated_by,
    deleted_by,
    tenant_id,
    name,
    description,
    category,
    tag_type,
    rule,
    allowed_values,
    is_system,
    is_dynamic,
    refresh_interval_seconds,
    code
)
VALUES
    -- 1. 系统基础标签 - 性别（静态标签）
    (NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
     '用户性别', '标识用户的性别信息', 'user', 'string',
     '{}',
     '[]',
     TRUE, FALSE, 0, 'USER_GENDER'),

    -- 2. 动态计算标签 - 近30天活跃用户（自动刷新）
    (NOW(), NOW(), NULL, 1001, 1002, NULL, 0,
     '近30天活跃用户', '最近30天有行为记录的用户', 'behavior', 'list',
     '{"event":"user_active","days":"30","operator":"gte","count":"1"}',
     '[]',
     FALSE, TRUE, 3600, 'ACTIVE_USER_30D'),

    -- 3. 枚举标签 - 会员等级
    (NOW(), NOW(), NULL, 1002, 1002, NULL, 1,
     '会员等级', '用户付费会员等级', 'behavior', 'enum',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'MEMBER_LEVEL'),

    -- 4. 系统标签 - 用户注册渠道
    (NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
     '注册渠道', '用户注册来源渠道', 'risk', 'string',
     '{}',
     '[]',
     TRUE, FALSE, 0, 'REGISTER_CHANNEL'),

    -- 5. 动态标签 - 高消费用户（月消费>1000元）
    (NOW(), NOW(), NULL, 1003, 1003, NULL, 1,
     '高价值消费用户', '近30天累计消费金额大于1000元', 'risk', 'list',
     '{"metric":"order_amount","days":"30","operator":"gt","value":"1000"}',
     '[]',
     FALSE, TRUE, 1800, 'HIGH_VALUE_USER'),

    -- 6. 业务标签 - 商品偏好类型
    (NOW(), NOW(), NULL, 1002, 1004, NULL, 2,
     '商品偏好类型', '用户常购买的商品类型', 'business', 'string',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'GOODS_PREFERENCE'),

    -- 7. 已删除标签（测试软删除）
    (NOW(), NOW(), NOW() - INTERVAL '7 days', 1001, 1005, 1005, 0,
     '旧版测试标签', '已废弃的测试标签', 'business', 'string',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'OLD_TEST_TAG'),

    -- 8. 无刷新间隔静态标签
    (NOW(), NOW(), NULL, 1004, 1004, NULL, 2,
     '用户年龄段', '按年龄划分的用户群体', 'risk', 'string',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'USER_AGE_GROUP');


-- 插入 uba_user_tags 用户标签测试数据
INSERT INTO public.uba_user_tags (
    tenant_id,
    created_at,
    updated_at,
    deleted_at,
    created_by,
    updated_by,
    deleted_by,
    user_id,
    tag_id,
    value,
    value_label,
    confidence,
    source,
    source_rule_id,
    effective_time,
    expire_time,
    is_active
)
VALUES
    -- 1. 用户10001：性别-男（静态标签，tag_id=1）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10001, 1, 'male', '男', 1.0,
     'manual', NULL, NOW() - INTERVAL '365 days', NULL, TRUE),

    -- 2. 用户10001：会员等级-金卡（tag_id=3）
    (1, NOW(), NOW(), NULL, 1002, 1002, NULL,
     10001, 3, 'gold', '金卡会员', 1.0,
     'rule', 1001, NOW() - INTERVAL '180 days', NOW() + INTERVAL '180 days', TRUE),

    -- 3. 用户10001：近30天活跃用户（动态标签，tag_id=2）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10001, 2, 'true', '活跃用户', 0.98,
     'rule', 2001, NOW() - INTERVAL '7 days', NOW() + INTERVAL '30 days', TRUE),

    -- 4. 用户10002：性别-女（tag_id=1）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10002, 1, 'female', '女', 1.0,
     'manual', NULL, NOW() - INTERVAL '200 days', NULL, TRUE),

    -- 5. 用户10002：注册渠道-微信小程序（tag_id=4）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10002, 4, 'wechat_mini', '微信小程序', 1.0,
     'rule', NULL, NOW() - INTERVAL '200 days', NULL, TRUE),

    -- 6. 用户10003：高价值消费用户（动态标签，tag_id=5）
    (1, NOW(), NOW(), NULL, 1003, 1003, NULL,
     10003, 5, 'true', '高价值用户', 0.95,
     'rule', 2002, NOW() - INTERVAL '15 days', NOW() + INTERVAL '15 days', TRUE),

    -- 7. 用户10003：商品偏好-电子产品（tag_id=6）
    (2, NOW(), NOW(), NULL, 1002, 1004, NULL,
     10003, 6, 'electronics', '电子产品', 0.92,
     'model', 3001, NOW() - INTERVAL '60 days', NULL, TRUE),

    -- 8. 用户10004：年龄段-19-30岁（tag_id=8）
    (2, NOW(), NOW(), NULL, 1004, 1004, NULL,
     10004, 8, '19-30', '19-30岁', 1.0,
     'manual', NULL, NOW() - INTERVAL '90 days', NULL, TRUE),

    -- 9. 已过期标签（测试过期逻辑）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10004, 3, 'normal', '普通会员', 1.0,
     'model', 1002, NOW() - INTERVAL '365 days', NOW() - INTERVAL '10 days', FALSE),

    -- 10. 软删除标签（测试软删除）
    (0, NOW(), NOW(), NOW() - INTERVAL '5 days', 1001, 1005, 1005,
     10005, 7, 'test', '测试标签', 1.0,
     'import', NULL, NOW() - INTERVAL '30 days', NOW() - INTERVAL '10 days', FALSE);


-- 插入 uba_id_mappings 用户ID映射测试数据
INSERT INTO public.uba_id_mappings (
    tenant_id,
    created_by,
    updated_by,
    deleted_by,
    created_at,
    updated_at,
    deleted_at,
    global_user_id,
    id_type,
    id_value,
    confidence,
    link_source,
    first_seen,
    last_seen,
    is_active,
    properties
)
VALUES
    -- 1. 用户 10001 - 手机号ID映射（主映射）
    (0, 1001, 1001, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10001',
     'phone',
     '13800138000',
     1.0,
     'login',
     NOW() - INTERVAL '365 days',
     NOW(),
     TRUE,
     '{"operator":"中国移动","city":"北京","register_channel":"APP"}'),

    -- 2. 用户 10001 - 微信OPENID映射
    (0, 1001, 1001, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10001',
     'openid',
     'oVwx1s5XKZLH7aQ8bZ9xY0c1D2e',
     0.99,
     'wechat_auth',
     NOW() - INTERVAL '300 days',
     NOW(),
     TRUE,
     '{"nickname":"追风少年","gender":"male","subscribe_time":"2025-05-20"}'),

    -- 3. 用户 10002 - 手机号ID映射
    (0, 1001, 1001, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10002',
     'phone',
     '13900139000',
     1.0,
     'login',
     NOW() - INTERVAL '200 days',
     NOW() - INTERVAL '1 days',
     TRUE,
     '{"operator":"中国联通","city":"上海","register_channel":"H5"}'),

    -- 4. 用户 10002 - 设备ID映射
    (0, 1001, 1001, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10002',
     'device_id',
     '867530912345678',
     0.95,
     'device_bind',
     NOW() - INTERVAL '200 days',
     NOW() - INTERVAL '2 days',
     TRUE,
     '{"device_type":"mobile","os_version":"Android 14","brand":"Xiaomi"}'),

    -- 5. 用户 10003 - 手机号ID映射（租户1）
    (1, 1003, 1003, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10003',
     'phone',
     '13700137000',
     1.0,
     'login',
     NOW() - INTERVAL '180 days',
     NOW(),
     TRUE,
     '{"operator":"中国电信","city":"广州","register_channel":"微信小程序"}'),

    -- 6. 用户 10004 - 手机号ID映射（租户2）
    (2, 1004, 1004, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10004',
     'phone',
     '13600136000',
     1.0,
     'login',
     NOW() - INTERVAL '90 days',
     NOW() - INTERVAL '5 days',
     TRUE,
     '{"operator":"中国移动","city":"深圳","register_channel":"抖音"}'),

    -- 7. 用户 10004 - 邮箱ID映射
    (2, 1004, 1004, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10004',
     'email',
     'user10004@test.com',
     0.98,
     'email_bind',
     NOW() - INTERVAL '60 days',
     NOW() - INTERVAL '5 days',
     TRUE,
     '{"email_verified":"true","receive_marketing":"true"}'),

    -- 8. 已软删除的废弃ID映射
    (0, 1001, 1005, 1005,
     NOW(), NOW(), NOW() - INTERVAL '7 days',
     'GLOBAL_USER_10005',
     'device_id',
     '00000000123456789',
     0.8,
     'deprecated',
     NOW() - INTERVAL '365 days',
     NOW() - INTERVAL '30 days',
     FALSE,
     '{"reason":"设备更换","status":"invalid"}'),

    -- 9. 用户 10001 - 支付宝ID映射
    (0, 1001, 1001, NULL,
     NOW(), NOW(), NULL,
     'GLOBAL_USER_10001',
     'openid',
     '2088123456789012',
     0.99,
     'alipay_auth',
     NOW() - INTERVAL '100 days',
     NOW(),
     TRUE,
     '{"auth_status":"authorized","last_use_time":"2026-03-20"}');


-- 插入 uba_webhooks 测试数据
INSERT INTO public.uba_webhooks (
    created_at,
    updated_at,
    deleted_at,
    created_by,
    updated_by,
    deleted_by,
    tenant_id,
    name,
    url,
    secret,
    event_types,
    enabled,
    last_triggered_at,
    failure_count,
    app_id
)
VALUES
    -- 1. 租户0：用户标签变更回调（启用）
    (NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
     '用户标签更新通知',
     'https://api.xxx.com/webhook/tag_change',
     'whsec_1234567890abcdef',
     '["tag.created","tag.updated","tag.deleted"]',
     true,
     NOW() - INTERVAL '1 hour',
     0,
     '1001'),

    -- 2. 租户0：用户支付成功回调（启用）
    (NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
     '支付成功通知',
     'https://api.xxx.com/webhook/pay_success',
     'whsec_abcdef1234567890',
     '["order.paid","order.refund"]',
     true,
     NOW() - INTERVAL '30 minutes',
     1,
     '1001'),

    -- 3. 租户1：用户登录事件回调（禁用）
    (NOW(), NOW(), NULL, 1002, 1002, NULL, 1,
     '用户登录通知',
     'https://api.yyy.com/webhook/user_login',
     'whsec_0987654321fedcba',
     '["user.login","user.logout"]',
     false,
     NULL,
     5,
     '1002'),

    -- 4. 租户1：会话结束回调（启用）
    (NOW(), NOW(), NULL, 1002, 1003, NULL, 1,
     '会话结束通知',
     'https://api.yyy.com/webhook/session_end',
     'whsec_fedcba0987654321',
     '["session.start","session.end"]',
     true,
     NOW() - INTERVAL '2 hours',
     0,
     '1002'),

    -- 5. 租户2：风险事件回调（启用）
    (NOW(), NOW(), NULL, 1004, 1004, NULL, 2,
     '用户风险预警通知',
     'https://api.zzz.com/webhook/risk_alert',
     'whsec_risk1234567890abc',
     '["user.risk.high","user.risk.medium"]',
     true,
     NOW() - INTERVAL '15 minutes',
     2,
     '1003'),

    -- 6. 已软删除的废弃回调
    (NOW(), NOW(), NOW() - INTERVAL '7 days', 1001, 1005, 1005, 0,
     '旧版测试回调',
     'https://api.old.com/webhook/test',
     'whsec_old1234567890',
     '["test.event"]',
     false,
     NOW() - INTERVAL '10 days',
     99,
     '1001');


-- ==============================================
-- UBA_APPLICATIONS 测试数据（8条，覆盖所有场景）
-- ==============================================
INSERT INTO public.uba_applications (
    tenant_id,
    name,
    app_id,
    app_key,
    app_secret,
    type,
    status,
    platforms,
    remark,
    desensitize,
    webhook_url,
    webhook_secret,
    created_at,
    updated_at,
    created_by
)
VALUES
    -- 1. 租户0：官方游戏应用
    (0,
     '全民游戏',
     'game_global_001',
     'app_key_game_0001',
     'app_secret_abc123456789',
     'game',
     'ON',
     '["ios","android","web"]',
     '官方游戏大盘，全端数据采集',
     true,
     'https://open.example.com/webhook/game',
     'wh_sec_2025_game',
     now() - interval '90 day',
     now() - interval '10 day',
     1001
    ),
    -- 2. 租户0：电商平台
    (0,
     '优选电商',
     'ecommerce_global_001',
     'app_key_ec_0002',
     'app_secret_def987654321',
     'ecommerce',
     'ON',
     '["ios","android","h5","mini_program"]',
     '电商行为分析：浏览、加购、支付',
     true,
     'https://open.example.com/webhook/ecommerce',
     'wh_sec_2025_ec',
     now() - interval '80 day',
     now() - interval '5 day',
     1001
    ),
    -- 3. 租户1：工具类应用
    (1,
     '清理大师',
     'tool_tenant1_001',
     'app_key_tool_0003',
     'app_secret_xyz1122334455',
     'tool',
     'ON',
     '["android"]',
     '工具类APP，用户行为分析',
     false,
     '',
     '',
     now() - interval '60 day',
     now() - interval '2 day',
     1002
    ),
    -- 4. 租户1：内容资讯
    (1,
     '头条资讯',
     'content_tenant1_001',
     'app_key_ct_0004',
     'app_secret_klm5566778899',
     'content',
     'ON',
     '["ios","android","web"]',
     '内容阅读、停留、点击分析',
     true,
     'https://tenant1.example.com/callback',
     'wh_sec_t1_2025',
     now() - interval '45 day',
     now(),
     1002
    ),
    -- 5. 租户2：社交应用
    (2,
     '附近聊天',
     'social_tenant2_001',
     'app_key_social_0005',
     'app_secret_qwe0099887766',
     'social',
     'ON',
     '["ios","android"]',
     '社交关系、互动、消息行为',
     true,
     'https://social.example.com/webhook',
     'wh_sec_t2_soc',
     now() - interval '30 day',
     now(),
     1003
    ),
    -- 6. 租户2：教育应用
    (2,
     '天天学习',
     'education_tenant2_001',
     'app_key_edu_0006',
     'app_secret_rty123321123',
     'education',
     'ON',
     '["web","mini_program"]',
     '学习时长、课程、完课率分析',
     false,
     '',
     '',
     now() - interval '25 day',
     now(),
     1003
    ),
    -- 7. 已禁用应用（测试状态）
    (0,
     '旧版测试应用',
     'test_old_001',
     'app_key_test_0007',
     'app_secret_test_xxxxxxx',
     'other',
     'OFF',
     '["web"]',
     '已废弃，仅做历史数据保留',
     false,
     '',
     '',
     now() - interval '180 day',
     now() - interval '90 day',
     1001
    ),
    -- 8. 带软删除标记的应用（测试软删）
    (1,
     '已删除演示应用',
     'demo_deleted_001',
     'app_key_del_0008',
     'app_secret_del_123123123',
     'tool',
     'OFF',
     '["android"]',
     '演示软删除功能',
     false,
     '',
     '',
     now() - interval '20 day',
     now() - interval '1 day',
     1002
    );

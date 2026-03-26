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
     '用户性别', '标识用户的性别信息', 'TAG_CATEGORY_USER', 'TAG_TYPE_STRING',
     '{}',
     '[]',
     TRUE, FALSE, 0, 'USER_GENDER'),

    -- 2. 动态计算标签 - 近30天活跃用户（自动刷新）
    (NOW(), NOW(), NULL, 1001, 1002, NULL, 0,
     '近30天活跃用户', '最近30天有行为记录的用户', 'TAG_CATEGORY_BEHAVIOR', 'TAG_TYPE_LIST',
     '{"event":"user_active","days":"30","operator":"gte","count":"1"}',
     '[]',
     FALSE, TRUE, 3600, 'ACTIVE_USER_30D'),

    -- 3. 枚举标签 - 会员等级
    (NOW(), NOW(), NULL, 1002, 1002, NULL, 1,
     '会员等级', '用户付费会员等级', 'TAG_CATEGORY_BEHAVIOR', 'TAG_TYPE_ENUM',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'MEMBER_LEVEL'),

    -- 4. 系统标签 - 用户注册渠道
    (NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
     '注册渠道', '用户注册来源渠道', 'TAG_CATEGORY_RISK', 'TAG_TYPE_STRING',
     '{}',
     '[]',
     TRUE, FALSE, 0, 'REGISTER_CHANNEL'),

    -- 5. 动态标签 - 高消费用户（月消费>1000元）
    (NOW(), NOW(), NULL, 1003, 1003, NULL, 1,
     '高价值消费用户', '近30天累计消费金额大于1000元', 'TAG_CATEGORY_RISK', 'TAG_TYPE_LIST',
     '{"metric":"order_amount","days":"30","operator":"gt","value":"1000"}',
     '[]',
     FALSE, TRUE, 1800, 'HIGH_VALUE_USER'),

    -- 6. 业务标签 - 商品偏好类型
    (NOW(), NOW(), NULL, 1002, 1004, NULL, 2,
     '商品偏好类型', '用户常购买的商品类型', 'TAG_CATEGORY_BUSINESS', 'TAG_TYPE_STRING',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'GOODS_PREFERENCE'),

    -- 7. 已删除标签（测试软删除）
    (NOW(), NOW(), NOW() - INTERVAL '7 days', 1001, 1005, 1005, 0,
     '旧版测试标签', '已废弃的测试标签', 'TAG_CATEGORY_BUSINESS', 'TAG_TYPE_STRING',
     '{}',
     '[]',
     FALSE, FALSE, 0, 'OLD_TEST_TAG'),

    -- 8. 无刷新间隔静态标签
    (NOW(), NOW(), NULL, 1004, 1004, NULL, 2,
     '用户年龄段', '按年龄划分的用户群体', 'TAG_CATEGORY_RISK', 'TAG_TYPE_STRING',
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
     10001, 1, '男', '男', 1.0,
     'TAG_SOURCE_MANUAL', NULL, NOW() - INTERVAL '365 days', NULL, TRUE),

    -- 2. 用户10001：会员等级-金卡（tag_id=3）
    (1, NOW(), NOW(), NULL, 1002, 1002, NULL,
     10001, 3, '金卡会员', '金卡会员', 1.0,
     'TAG_SOURCE_RULE', 1001, NOW() - INTERVAL '180 days', NOW() + INTERVAL '180 days', TRUE),

    -- 3. 用户10001：近30天活跃用户（动态标签，tag_id=2）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10001, 2, 'true', '活跃用户', 0.98,
     'TAG_SOURCE_RULE', 2001, NOW() - INTERVAL '7 days', NOW() + INTERVAL '30 days', TRUE),

    -- 4. 用户10002：性别-女（tag_id=1）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10002, 1, '女', '女', 1.0,
     'TAG_SOURCE_MANUAL', NULL, NOW() - INTERVAL '200 days', NULL, TRUE),

    -- 5. 用户10002：注册渠道-微信小程序（tag_id=4）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10002, 4, '微信小程序', '微信小程序', 1.0,
     'TAG_SOURCE_RULE', NULL, NOW() - INTERVAL '200 days', NULL, TRUE),

    -- 6. 用户10003：高价值消费用户（动态标签，tag_id=5）
    (1, NOW(), NOW(), NULL, 1003, 1003, NULL,
     10003, 5, 'true', '高价值用户', 0.95,
     'TAG_SOURCE_RULE', 2002, NOW() - INTERVAL '15 days', NOW() + INTERVAL '15 days', TRUE),

    -- 7. 用户10003：商品偏好-电子产品（tag_id=6）
    (2, NOW(), NOW(), NULL, 1002, 1004, NULL,
     10003, 6, '电子产品', '电子产品', 0.92,
     'TAG_SOURCE_MODEL', 3001, NOW() - INTERVAL '60 days', NULL, TRUE),

    -- 8. 用户10004：年龄段-19-30岁（tag_id=8）
    (2, NOW(), NOW(), NULL, 1004, 1004, NULL,
     10004, 8, '19-30岁', '19-30岁', 1.0,
     'TAG_SOURCE_MANUAL', NULL, NOW() - INTERVAL '90 days', NULL, TRUE),

    -- 9. 已过期标签（测试过期逻辑）
    (0, NOW(), NOW(), NULL, 1001, 1001, NULL,
     10004, 3, '普通会员', '普通会员', 1.0,
     'TAG_SOURCE_MODEL', 1002, NOW() - INTERVAL '365 days', NOW() - INTERVAL '10 days', FALSE),

    -- 10. 软删除标签（测试软删除）
    (0, NOW(), NOW(), NOW() - INTERVAL '5 days', 1001, 1005, 1005,
     10005, 7, 'test', '测试标签', 1.0,
     'TAG_SOURCE_IMPORT', NULL, NOW() - INTERVAL '30 days', NOW() - INTERVAL '10 days', FALSE);


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
     'ID_TYPE_PHONE',
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
     'ID_TYPE_OPENID',
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
     'ID_TYPE_PHONE',
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
     'ID_TYPE_DEVICE_ID',
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
     'ID_TYPE_PHONE',
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
     'ID_TYPE_PHONE',
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
     'ID_TYPE_EMAIL',
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
     'ID_TYPE_DEVICE_ID',
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
     'ID_TYPE_OPENID',
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


INSERT INTO public.uba_applications (
    created_at,
    updated_at,
    deleted_at,
    created_by,
    updated_by,
    deleted_by,
    tenant_id,
    name,
    app_id,
    app_key,
    app_secret,
    type,
    status,
    remark,
    desensitize,
    webhook_url,
    webhook_secret
)
VALUES
-- 1. 租户0：正式应用（启用，数据脱敏）
(NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
 '电商主站',
 'APP_2026001',
 'key_abcdef1234567890',
 'sec_abcdef1234567890abcdef123456',
 'PLATFORM_WEB',
 'ON',
 '电商线上环境',
 true,
 'https://api.shop.com/webhook',
 'whsec_abc123xyz789'),

-- 2. 租户0：APP应用（启用，不脱敏）
(NOW(), NOW(), NULL, 1001, 1001, NULL, 0,
 '电商APP',
 'APP_2026002',
 'key_xyz9876543210fedcba',
 'sec_xyz9876543210fedcba987654',
 'PLATFORM_IOS',
 'ON',
 '移动端APP',
 false,
 'https://m.shop.com/api/webhook',
 'whsec_xyz987654321'),

-- 3. 租户1：后台管理系统（禁用）
(NOW(), NOW(), NULL, 1002, 1002, NULL, 1,
 '运营后台',
 'APP_2026003',
 'key_admin1234567890',
 'sec_admin1234567890abcdef',
 'PLATFORM_LINUX',
 'OFF',
 '内部管理后台',
 true,
 '',
 ''),

-- 4. 租户1：小程序应用（启用）
(NOW(), NOW(), NULL, 1002, 1003, NULL, 1,
 '微信小程序',
 'APP_2026004',
 'key_mini1234567890abc',
 'sec_mini1234567890abcdef123',
 'PLATFORM_MINI_PROGRAM',
 'ON',
 '微信小程序端',
 false,
 'https://wx.abc.com/webhook',
 'whsec_mini123456'),

-- 5. 租户2：第三方合作应用（软删除）
(NOW(), NOW(), NOW() - INTERVAL '7 days', 1004, 1005, 1005, 2,
 '第三方合作平台',
 'APP_2026005',
 'key_third1234567890abcd',
 'sec_third1234567890abcdef12',
 'PLATFORM_MINI_PROGRAM',
 'OFF',
 '已废弃合作方',
 true,
 NULL,
 NULL);

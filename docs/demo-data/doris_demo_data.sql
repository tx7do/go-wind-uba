-- ============================================================
-- Doris 分析数据 Demo（事实表 + 维度表）
-- 适用：ClickHouse 同结构表同样适用（Doris 与 CH 表结构镜像定义）
-- 说明：造一份能让 BI 图表（趋势/漏斗/留存/维度分组/大屏）有数据可看的数据。
--       tenant_id=1（默认租户），时间分布在最近 7 天内。
-- 用法：mysql -h <doris_host> -P 9030 -u root < doris_demo_data.sql
-- ============================================================

-- 清理旧 demo 数据（如需）
DELETE FROM events_fact   WHERE tenant_id = 1 AND user_id BETWEEN 1001 AND 1050;
DELETE FROM sessions_fact WHERE tenant_id = 1 AND user_id BETWEEN 1001 AND 1050;
DELETE FROM risk_events   WHERE tenant_id = 1 AND user_id BETWEEN 1001 AND 1050;
DELETE FROM users_dim     WHERE tenant_id = 1 AND user_id BETWEEN 1001 AND 1050;

-- ============================================================
-- 1. 用户维度表 users_dim（50 个用户）
-- ============================================================
INSERT INTO users_dim
  (tenant_id, user_id, register_time, register_channel, first_active_date, last_active_date,
   user_level, vip_level, user_role, total_events, total_sessions, total_pay_amount,
   last_pay_time, prefer_categories, prefer_objects, risk_score, risk_level, risk_tags,
   last_risk_time, profile, geo, device_type, platform, country, ver, created_at, updated_at)
VALUES
  (1, 1001, '2025-06-01 09:30:00', 'appstore',   '2025-06-01', '2025-06-28', 5, 3, 'user', 1280, 56,  2980.00, '2025-06-27 21:00:00', '["game","social"]',  '["sku_101","sku_205"]', 12, 'low',      '[]',                     NULL,           '{"age":"25"}',          '{"country":"中国","city":"北京"}',   'iPhone 14',     'ios',         'CN', 1, NOW(), NOW()),
  (1, 1002, '2025-06-03 14:20:00', 'wechat',     '2025-06-03', '2025-06-28', 3, 0, 'user',  420, 18,    0.00, NULL,                  '["content"]',        '["sku_301"]',            8,  'low',      '[]',                     NULL,           '{"age":"30"}',          '{"country":"中国","city":"上海"}',   'Xiaomi 13',     'android',     'CN', 1, NOW(), NOW()),
  (1, 1003, '2025-05-20 10:00:00', 'appstore',   '2025-05-20', '2025-06-28', 8, 5, 'vip',  3500, 120, 15800.50,'2025-06-28 11:30:00','["game","tool"]',   '["sku_101","sku_102"]', 75, 'high',     '["fraud_payment"]',     '2025-06-26 03:00:00','{"age":"22"}','{"country":"中国","city":"深圳"}',   'iPhone 15 Pro', 'ios',         'CN', 1, NOW(), NOW()),
  (1, 1004, '2025-06-10 08:00:00', 'googleplay', '2025-06-10', '2025-06-27', 2, 0, 'user',  150,  8,    0.00, NULL,                  '["tool"]',           '["sku_401"]',            90, 'critical', '["device_anomaly"]',    '2025-06-27 14:00:00','{"age":"28"}','{"country":"美国","city":"NewYork"}','Pixel 8',     'android',     'US', 1, NOW(), NOW()),
  (1, 1005, '2025-06-15 16:45:00', 'appstore',   '2025-06-15', '2025-06-28', 4, 1, 'user',  680, 25,  199.00, '2025-06-25 19:00:00', '["social","content"]','["sku_301","sku_302"]', 30, 'medium',   '["abnormal_flow"]',     '2025-06-24 22:00:00','{"age":"35"}','{"country":"中国","city":"广州"}',   'iPhone 13',     'ios',         'CN', 1, NOW(), NOW()),
  (1, 1006, '2025-06-05 11:00:00', 'web',        '2025-06-05', '2025-06-28', 6, 2, 'user',  980, 32,  599.00, '2025-06-26 10:00:00', '["game"]',           '["sku_103"]',            15, 'low',      '[]',                     NULL,           '{"age":"40"}',          '{"country":"日本","city":"Tokyo"}',  'Galaxy S24',    'android',     'JP', 1, NOW(), NOW()),
  (1, 1007, '2025-06-20 09:00:00', 'wechat',     '2025-06-20', '2025-06-28', 1, 0, 'user',   85,  4,    0.00, NULL,                  '["content"]',        '["sku_305"]',            10, 'low',      '[]',                     NULL,           '{"age":"19"}',          '{"country":"中国","city":"成都"}',   'iPhone 12',     'ios',         'CN', 1, NOW(), NOW()),
  (1, 1008, '2025-05-28 13:30:00', 'appstore',   '2025-05-28', '2025-06-28', 7, 4, 'vip',  2100, 78, 8800.00, '2025-06-28 09:00:00', '["game","social"]',  '["sku_101","sku_201"]', 45, 'medium',   '["location_anomaly"]',  '2025-06-23 02:00:00','{"age":"27"}','{"country":"韩国","city":"Seoul"}', 'iPhone 14 Pro', 'ios',         'KR', 1, NOW(), NOW()),
  (1, 1009, '2025-06-25 18:00:00', 'googleplay', '2025-06-25', '2025-06-28', 1, 0, 'user',   42,  2,    0.00, NULL,                  '["tool"]',           '["sku_402"]',             5, 'low',      '[]',                     NULL,           '{"age":"33"}',          '{"country":"中国","city":"杭州"}',   'OnePlus 12',    'android',     'CN', 1, NOW(), NOW()),
  (1, 1010, '2025-06-12 10:30:00', 'web',        '2025-06-12', '2025-06-28', 5, 2, 'user',  760, 30,  450.00, '2025-06-27 16:00:00', '["content","tool"]', '["sku_303"]',            20, 'low',      '[]',                     NULL,           '{"age":"45"}',          '{"country":"中国","city":"武汉"}',   'MacBook',       'web',         'CN', 1, NOW(), NOW());

-- ============================================================
-- 2. 事件事实表 events_fact（批量行为事件，时间分布在最近 7 天）
--    用 Doris 批量 INSERT，构造典型漏斗：app_launch → view_home → view_product → add_to_cart → submit_order → pay_success
-- ============================================================
INSERT INTO events_fact
  (event_id, tenant_id, user_id, device_id, account_id, global_user_id,
   event_time, event_ts, server_time, event_category, event_name, event_action,
   object_type, object_id, object_name, session_id, session_seq,
   platform, os, app_version, channel, ip, ip_city, country, network,
   context, duration_ms, amount, quantity, score, metrics, properties,
   op_result, error_code, risk_level, trace_id, geo, user_agent, referer, created_at, updated_at)
VALUES
  -- 用户1001 的完整转化漏斗（成功支付）
  ('e-1001-01', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:00:00', 1751090400000, '2025-06-28 10:00:01', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1001-a', 0, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{"scene":"home"}',     0,    '0',      0, 0, '{}', '{"os":"iOS"}',          '', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1001-02', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:00:15', 1751090415000, '2025-06-28 10:00:16', 'page',     'view_home',     'view',    'page',    'home',     '首页',     'sess-1001-a', 1, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   3200, '0',      0, 0, '{}', '{"source":"banner"}',   '', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1001-03', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:01:00', 1751090460000, '2025-06-28 10:01:01', 'business', 'view_product',  'view',    'product', 'sku_101', '游戏手柄', 'sess-1001-a', 2, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   8500, '0',      0, 0, '{}', '{"price":"299"}',       '', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1001-04', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:02:30', 1751090550000, '2025-06-28 10:02:31', 'business', 'add_to_cart',   'click',   'product', 'sku_101', '游戏手柄', 'sess-1001-a', 3, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   0,    '299.00', 1, 0, '{}', '{"from":"detail"}',     '', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1001-05', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:03:10', 1751090590000, '2025-06-28 10:03:11', 'business', 'submit_order',  'submit',  'order',   'ord-2001', '订单2001', 'sess-1001-a', 4, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   0,    '299.00', 1, 0, '{}', '{"orderId":"ord-2001"}','', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1001-06', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-28 10:04:00', 1751090640000, '2025-06-28 10:04:01', 'business', 'pay_success',   'pay',     'order',   'ord-2001', '订单2001', 'sess-1001-a', 5, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   0,    '299.00', 1, 0, '{}', '{"payMethod":"wechat"}','', '', '', '', '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),

  -- 用户1002 仅浏览（跳出，未转化）
  ('e-1002-01', 1, 1002, 'dev-1002', 'acc-1002', 'guid-1002', '2025-06-28 11:00:00', 1751094000000, '2025-06-28 11:00:01', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1002-a', 0, 'android', 'Android 14', '2.1.0', 'wechat',     '114.88.22.11', '上海', 'CN', '4g',    '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', '', '', '{"country":"中国","city":"上海"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1002-02', 1, 1002, 'dev-1002', 'acc-1002', 'guid-1002', '2025-06-28 11:00:20', 1751094020000, '2025-06-28 11:00:21', 'page',     'view_home',     'view',    'page',    'home',     '首页',     'sess-1002-a', 1, 'android', 'Android 14', '2.1.0', 'wechat',     '114.88.22.11', '上海', 'CN', '4g',    '{}',                   1500, '0',      0, 0, '{}', '{}',                    '', '', '', '', '{"country":"中国","city":"上海"}',   'Mozilla/5.0', '', NOW(), NOW()),

  -- 用户1003 高频事件（异常流量）
  ('e-1003-01', 1, 1003, 'dev-1003', 'acc-1003', 'guid-1003', '2025-06-28 03:00:00', 1751072400000, '2025-06-28 03:00:01', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1003-a', 0, 'ios',     'iOS 18',     '3.0.0', 'appstore',   '120.77.45.99', '深圳', 'CN', 'wifi',  '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', 'high','', '{"country":"中国","city":"深圳"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1003-02', 1, 1003, 'dev-1003', 'acc-1003', 'guid-1003', '2025-06-28 03:00:02', 1751072402000, '2025-06-28 03:00:03', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1003-b', 0, 'ios',     'iOS 18',     '3.0.0', 'appstore',   '120.77.45.99', '深圳', 'CN', 'wifi',  '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', 'high','', '{"country":"中国","city":"深圳"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1003-03', 1, 1003, 'dev-1003', 'acc-1003', 'guid-1003', '2025-06-28 03:00:04', 1751072404000, '2025-06-28 03:00:05', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1003-c', 0, 'ios',     'iOS 18',     '3.0.0', 'appstore',   '120.77.45.99', '深圳', 'CN', 'wifi',  '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', 'high','', '{"country":"中国","city":"深圳"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1003-04', 1, 1003, 'dev-1003', 'acc-1003', 'guid-1003', '2025-06-28 03:00:06', 1751072406000, '2025-06-28 03:00:07', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1003-d', 0, 'ios',     'iOS 18',     '3.0.0', 'appstore',   '120.77.45.99', '深圳', 'CN', 'wifi',  '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', 'high','', '{"country":"中国","city":"深圳"}',   'Mozilla/5.0', '', NOW(), NOW()),

  -- 用户1004 设备异常（美国，代理）
  ('e-1004-01', 1, 1004, 'dev-1004', 'acc-1004', 'guid-1004', '2025-06-28 14:00:00', 1751104800000, '2025-06-28 14:00:01', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1004-a', 0, 'android', 'Android 14', '1.0.0', 'googleplay','198.51.100.1','NewYork','US','vpn',   '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', 'critical','', '{"country":"美国","city":"NewYork"}', 'Mozilla/5.0', '', NOW(), NOW()),

  -- 用户1005 部分漏斗（加购未支付）
  ('e-1005-01', 1, 1005, 'dev-1005', 'acc-1005', 'guid-1005', '2025-06-28 19:00:00', 1751122800000, '2025-06-28 19:00:01', 'app',      'app_launch',    'launch',  '',        '',         '',         'sess-1005-a', 0, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '121.8.99.22',  '广州', 'CN', 'wifi',  '{}',                   0,    '0',      0, 0, '{}', '{}',                    '', '', '', '',   '{"country":"中国","city":"广州"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1005-02', 1, 1005, 'dev-1005', 'acc-1005', 'guid-1005', '2025-06-28 19:00:30', 1751122830000, '2025-06-28 19:00:31', 'page',     'view_home',     'view',    'page',    'home',     '首页',     'sess-1005-a', 1, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '121.8.99.22',  '广州', 'CN', 'wifi',  '{}',                   2800, '0',      0, 0, '{}', '{}',                    '', '', '', '',   '{"country":"中国","city":"广州"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1005-03', 1, 1005, 'dev-1005', 'acc-1005', 'guid-1005', '2025-06-28 19:01:20', 1751122880000, '2025-06-28 19:01:21', 'business', 'view_product',  'view',    'product', 'sku_301', '会员服务', 'sess-1005-a', 2, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '121.8.99.22',  '广州', 'CN', 'wifi',  '{}',                   6200, '0',      0, 0, '{}', '{"price":"199"}',       '', '', '', '',   '{"country":"中国","city":"广州"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1005-04', 1, 1005, 'dev-1005', 'acc-1005', 'guid-1005', '2025-06-28 19:02:00', 1751122920000, '2025-06-28 19:02:01', 'business', 'add_to_cart',   'click',   'product', 'sku_301', '会员服务', 'sess-1005-a', 3, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '121.8.99.22',  '广州', 'CN', 'wifi',  '{}',                   0,    '199.00', 1, 0, '{}', '{}',                    '', '', '', '',   '{"country":"中国","city":"广州"}',   'Mozilla/5.0', '', NOW(), NOW()),

  -- 补充历史日的事件（为趋势图提供多天数据）
  ('e-1001-07', 1, 1001, 'dev-1001', 'acc-1001', 'guid-1001', '2025-06-27 10:00:00', 1751004000000, '2025-06-27 10:00:01', 'business', 'pay_success',   'pay',     'order',   'ord-2000', '订单2000', 'sess-1001-b', 5, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi',  '{}',                   0,    '159.00', 1, 0, '{}', '{}',                    '', '', '', '',   '{"country":"中国","city":"北京"}',   'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1006-01', 1, 1006, 'dev-1006', 'acc-1006', 'guid-1006', '2025-06-26 10:00:00', 1750917600000, '2025-06-26 10:00:01', 'business', 'pay_success',   'pay',     'order',   'ord-1999', '订单1999', 'sess-1006-a', 3, 'android', 'Android 14', '2.1.0', 'web',        '133.18.44.7',  'Tokyo','JP', 'wifi',  '{}',                   0,    '599.00', 1, 0, '{}', '{}',                    '', '', '', '',   '{"country":"日本","city":"Tokyo"}',  'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1008-01', 1, 1008, 'dev-1008', 'acc-1008', 'guid-1008', '2025-06-25 09:00:00', 1750831200000, '2025-06-25 09:00:01', 'business', 'pay_success',   'pay',     'order',   'ord-1998', '订单1998', 'sess-1008-a', 4, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '175.45.12.88', 'Seoul','KR', 'wifi',  '{}',                   0,    '880.00', 1, 0, '{}', '{}',                    '', '', '', '',   '{"country":"韩国","city":"Seoul"}',  'Mozilla/5.0', '', NOW(), NOW()),
  ('e-1010-01', 1, 1010, 'dev-1010', 'acc-1010', 'guid-1010', '2025-06-24 16:00:00', 1750750400000, '2025-06-24 16:00:01', 'business', 'pay_success',   'pay',     'order',   'ord-1997', '订单1997', 'sess-1010-a', 2, 'web',      'macOS 14',   '1.1.0', 'web',        '58.19.44.100', '武汉', 'CN', 'wifi',  '{}',                   0,    '450.00', 1, 0, '{}', '{}',                    '', '', '', '',   '{"country":"中国","city":"武汉"}',   'Mozilla/5.0', '', NOW(), NOW());

-- ============================================================
-- 3. 会话事实表 sessions_fact
-- ============================================================
INSERT INTO sessions_fact
  (session_id, tenant_id, user_id, device_id, global_user_id,
   start_time, end_time, duration_ms, event_count, page_view_count, action_count,
   entry_page, exit_page, is_bounce, platform, os, app_version, channel,
   ip, ip_city, country, network, total_amount, pay_event_count,
   risk_level, risk_tags, context, created_at, updated_at)
VALUES
  ('sess-1001-a', 1, 1001, 'dev-1001', 'guid-1001', '2025-06-28 10:00:00', '2025-06-28 10:04:00', 240000, 6, 2, 4,    'home',     'order',   false, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi', 299.00, 1, 'low',      '[]',                    '{}', NOW(), NOW()),
  ('sess-1001-b', 1, 1001, 'dev-1001', 'guid-1001', '2025-06-27 10:00:00', '2025-06-27 10:03:00', 180000, 5, 1, 4,    'home',     'order',   false, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '116.23.66.55', '北京', 'CN', 'wifi', 159.00, 1, 'low',      '[]',                    '{}', NOW(), NOW()),
  ('sess-1002-a', 1, 1002, 'dev-1002', 'guid-1002', '2025-06-28 11:00:00', '2025-06-28 11:00:20',  20000, 2, 1, 1,    'home',     'home',    true,  'android', 'Android 14', '2.1.0', 'wechat',     '114.88.22.11', '上海', 'CN', '4g',     0.00,  0, 'low',      '[]',                    '{}', NOW(), NOW()),
  ('sess-1003-a', 1, 1003, 'dev-1003', 'guid-1003', '2025-06-28 03:00:00', '2025-06-28 03:00:06',   6000, 4, 0, 4,    '',         '',        true,  'ios',     'iOS 18',     '3.0.0', 'appstore',   '120.77.45.99', '深圳', 'CN', 'wifi',   0.00,  0, 'high',     '["device_anomaly"]',    '{}', NOW(), NOW()),
  ('sess-1004-a', 1, 1004, 'dev-1004', 'guid-1004', '2025-06-28 14:00:00', '2025-06-28 14:00:30',  30000, 1, 0, 0,    '',         '',        true,  'android', 'Android 14', '1.0.0', 'googleplay','198.51.100.1','NewYork','US','vpn', 0.00,  0, 'critical', '["proxy_detected"]',    '{}', NOW(), NOW()),
  ('sess-1005-a', 1, 1005, 'dev-1005', 'guid-1005', '2025-06-28 19:00:00', '2025-06-28 19:02:00', 120000, 4, 1, 3,    'home',     'product', false, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '121.8.99.22',  '广州', 'CN', 'wifi', 199.00, 0, 'medium',   '["abnormal_flow"]',     '{}', NOW(), NOW()),
  ('sess-1006-a', 1, 1006, 'dev-1006', 'guid-1006', '2025-06-26 10:00:00', '2025-06-26 10:05:00', 300000, 4, 1, 3,    'home',     'order',   false, 'android', 'Android 14', '2.1.0', 'web',        '133.18.44.7',  'Tokyo','JP', 'wifi', 599.00, 1, 'low',      '[]',                    '{}', NOW(), NOW()),
  ('sess-1008-a', 1, 1008, 'dev-1008', 'guid-1008', '2025-06-25 09:00:00', '2025-06-25 09:04:00', 240000, 4, 1, 3,    'home',     'order',   false, 'ios',     'iOS 17',     '1.2.0', 'appstore',   '175.45.12.88', 'Seoul','KR', 'wifi', 880.00, 1, 'medium',   '["location_anomaly"]',  '{}', NOW(), NOW()),
  ('sess-1010-a', 1, 1010, 'dev-1010', 'guid-1010', '2025-06-24 16:00:00', '2025-06-24 16:02:00', 120000, 2, 1, 1,    'home',     'order',   false, 'web',      'macOS 14',   '1.1.0', 'web',        '58.19.44.100', '武汉', 'CN', 'wifi', 450.00, 1, 'low',      '[]',                    '{}', NOW(), NOW());

-- ============================================================
-- 4. 风险事件表 risk_events
-- ============================================================
INSERT INTO risk_events
  (risk_event_id, tenant_id, user_id, device_id, global_user_id,
   risk_type, risk_level, risk_score, rule_id, rule_name,
   rule_context, related_event_ids, session_id, description, evidence,
   status, handler_id, handled_time, handle_remark,
   occur_time, report_time, event_date, created_at, updated_at)
VALUES
  ('risk-001', 1, 1003, 'dev-1003', 'guid-1003', 'abnormal_flow',    'high',     75.0, 1, '高频访问检测',
   '{"threshold":"5/10s"}',     '["e-1003-01","e-1003-02","e-1003-03","e-1003-04"]', 'sess-1003-a',
   '10秒内触发4次app_launch，疑似脚本刷量', '{"interval_sec":"2","count":"4"}',
   'pending',      NULL, NULL, NULL,
   '2025-06-28 03:00:06', '2025-06-28 03:00:07', '2025-06-28', NOW(), NOW()),

  ('risk-002', 1, 1004, 'dev-1004', 'guid-1004', 'proxy_detected',   'critical', 90.0, 2, '代理/VPN检测',
   '{"ip":"198.51.100.1"}',     '["e-1004-01"]', 'sess-1004-a',
   '检测到VPN代理IP，注册地美国，疑似虚拟设备', '{"vpn":true,"country":"US"}',
   'investigating', NULL, NULL, NULL,
   '2025-06-28 14:00:30', '2025-06-28 14:00:31', '2025-06-28', NOW(), NOW()),

  ('risk-003', 1, 1003, 'dev-1003', 'guid-1003', 'fraud_payment',    'high',     80.0, 3, '大额异常支付',
   '{"amount":"15800.5"}',      '["e-1003-pay"]', 'sess-1003-b',
   'VIP用户单笔支付15800.5元，超出历史均值10倍', '{"avg_amount":"1500","ratio":"10x"}',
   'confirmed',     'admin', '2025-06-26 10:00:00', '确认欺诈，已冻结账户',
   '2025-06-26 03:00:00', '2025-06-26 03:00:01', '2025-06-26', NOW(), NOW()),

  ('risk-004', 1, 1005, 'dev-1005', 'guid-1005', 'location_anomaly', 'medium',   30.0, 4, '异地登录检测',
   '{"prev_city":"上海","curr_city":"广州"}', '["e-1005-01"]', 'sess-1005-a',
   '用户登录城市从上海突变为广州', '{"distance":"1200km"}',
   'false_positive', 'admin', '2025-06-24 22:00:00', '用户出差，确认误报',
   '2025-06-24 22:00:00', '2025-06-24 22:00:01', '2025-06-24', NOW(), NOW()),

  ('risk-005', 1, 1008, 'dev-1008', 'guid-1008', 'location_anomaly', 'medium',   45.0, 4, '异地登录检测',
   '{"prev_city":"北京","curr_city":"Seoul"}', '["e-1008-01"]', 'sess-1008-a',
   '用户登录地从北京变为韩国首尔', '{"distance":"950km"}',
   'ignored',       'admin', '2025-06-23 02:00:00', '海外业务，正常',
   '2025-06-23 02:00:00', '2025-06-23 02:00:01', '2025-06-23', NOW(), NOW());

-- ============================================================
-- 数据说明
-- ============================================================
-- 上述数据覆盖以下 BI 场景：
--   1. 事件趋势：events_fact 跨 5 天（6/24~6/28），可画趋势线
--   2. 漏斗分析：app_launch→view_home→view_product→add_to_cart→submit_order→pay_success
--      （用户1001 完整转化，1005 转化中断，1002 跳出）
--   3. 留存分析：1001/1005/1008 在多天有活跃，可算留存矩阵
--   4. 维度分组：platform(ios/android/web)、channel(appstore/wechat/googleplay/web)、
--      country(CN/US/JP/KR)、event_name 都有分布
--   5. 实时大屏：risk_events 含 5 条不同等级/类型/状态的告警
--   6. 会话分析：跳出会话(1002/1003/1004)、转化会话(1001/1006/1008/1010)
--   7. 行为时间轴：按 user_id=1001 可查到完整 6 步漏斗事件序列

-- ============================================================
-- PostgreSQL 业务数据 Demo（ent 实体表）
-- 说明：UBA 平台业务/配置数据。这些表由 ent ORM 管理（PostgreSQL）。
--       配合 doris_demo_data.sql（分析事实表）使用，构成完整的可演示数据集。
--       tenant_id=1（默认租户）。
-- 用法：psql -h <pg_host> -U <user> -d <db> -f postgres_demo_data.sql
-- ============================================================

-- ============================================================
-- 1. UBA 应用表 uba_applications（SDK 接入用的应用）
--    appId/appSecret 是 SDK 上报鉴权凭据
-- ============================================================
DELETE FROM uba_applications WHERE tenant_id = 1 AND app_id IN ('demo_game_001', 'demo_shop_002', 'demo_content_003');

INSERT INTO uba_applications
  (created_at, updated_at, created_by, updated_by, tenant_id,
   name, app_id, app_key, app_secret, type, status, platforms, remark, desensitize, webhook_url, webhook_secret)
VALUES
  (NOW(), NOW(), 1, 1, 1,
   '示例游戏App', 'demo_game_001', 'key_game_001', 'secret_game_001_aB3xK9', 'game', 'ON',
   '["ios","android"]',
   'Unity 游戏接入示例', false, '', ''),
  (NOW(), NOW(), 1, 1, 1,
   '示例电商小程序', 'demo_shop_002', 'key_shop_002', 'secret_shop_002_mN7pQ2', 'ecommerce', 'ON',
   '["web","mini_program"]',
   '微信小程序商城接入示例', true, 'https://example.com/webhook', 'wh_secret_002'),
  (NOW(), NOW(), 1, 1, 1,
   '示例内容平台', 'demo_content_003', 'key_content_003', 'secret_content_003_rT5wL8', 'content', 'ON',
   '["ios","android","web"]',
   '内容资讯平台接入示例', false, '', '');

-- ============================================================
-- 2. 事件 Schema 定义表 uba_event_schemas（事件元数据管理）
--    登记合法事件名 + 属性 schema，用于上报校验
-- ============================================================
DELETE FROM uba_event_schemas WHERE tenant_id = 1 AND event_name IN
  ('app_launch', 'view_home', 'view_product', 'add_to_cart', 'submit_order', 'pay_success', 'click');

INSERT INTO uba_event_schemas
  (created_at, updated_at, created_by, updated_by, tenant_id,
   event_name, display_name, category, description, properties, status)
VALUES
  (NOW(), NOW(), 1, 1, 1,
   'app_launch', '应用启动', 'app', '用户打开应用时触发',
   '[{"name":"scene","type":"string","displayName":"启动场景","required":false},{"name":"duration_ms","type":"int","displayName":"冷启动耗时","required":false}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'view_home', '浏览首页', 'page', '用户进入首页',
   '[{"name":"source","type":"string","displayName":"来源","required":false},{"name":"duration_ms","type":"int","displayName":"停留时长","required":false}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'view_product', '浏览商品', 'business', '用户查看商品详情',
   '[{"name":"product_id","type":"string","displayName":"商品ID","required":true},{"name":"price","type":"double","displayName":"价格","required":false}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'add_to_cart', '加入购物车', 'business', '用户将商品加入购物车',
   '[{"name":"product_id","type":"string","displayName":"商品ID","required":true},{"name":"quantity","type":"int","displayName":"数量","required":false}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'submit_order', '提交订单', 'business', '用户提交订单',
   '[{"name":"order_id","type":"string","displayName":"订单ID","required":true},{"name":"amount","type":"double","displayName":"金额","required":true}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'pay_success', '支付成功', 'business', '用户支付成功（核心转化事件）',
   '[{"name":"order_id","type":"string","displayName":"订单ID","required":true},{"name":"amount","type":"double","displayName":"支付金额","required":true},{"name":"pay_method","type":"string","displayName":"支付方式","required":false}]'::jsonb,
   'ENABLED'),
  (NOW(), NOW(), 1, 1, 1,
   'click', '点击事件', 'interaction', '通用点击事件',
   '[{"name":"element","type":"string","displayName":"元素","required":true},{"name":"page","type":"string","displayName":"页面","required":false}]'::jsonb,
   'ENABLED');

-- ============================================================
-- 3. 风险规则表 uba_risk_rules（风控引擎规则）
--    rule_expression 用 CEL 表达式，rule_config 是 JSON 配置
-- ============================================================
DELETE FROM uba_risk_rules WHERE tenant_id = 1 AND code IN
  ('high_freq_access', 'proxy_vpn_detect', 'large_amount_payment', 'geo_anomaly_login', 'device_fingerprint_change');

INSERT INTO uba_risk_rules
  (created_at, updated_at, created_by, updated_by, tenant_id,
   name, description, risk_type, default_level, "condition", actions,
   enabled, priority, code, rule_expression, rule_config, exec_mode, trigger_count, last_triggered_at)
VALUES
  (NOW(), NOW(), 1, 1, 1,
   '高频访问检测', '同一用户10秒内触发同一事件超过5次',
   'abnormal_flow', 'high',
   '{"window":"10s","threshold":5}'::jsonb,
   '[{"type":"alert","level":"high"}]'::jsonb,
   true, 10, 'high_freq_access',
   'count(events where user_id == ctx.user_id and event_name == ctx.event_name within "10s") > 5',
   '{"metric":"event_count","window":"10s","threshold":5}'::jsonb,
   'REALTIME', 142, '2025-06-28 03:00:06'),

  (NOW(), NOW(), 1, 1, 1,
   '代理/VPN检测', '检测到用户使用VPN或代理IP',
   'proxy_detected', 'critical',
   '{"check_vpn":true}'::jsonb,
   '[{"type":"alert","level":"critical"},{"type":"block"}]'::jsonb,
   true, 5, 'proxy_vpn_detect',
   'request.ip in KNOWN_VPN_RANGE or geo.country != user.register_country',
   '{"check_vpn":true,"ip_db":"maxmind"}'::jsonb,
   'REALTIME', 38, '2025-06-28 14:00:30'),

  (NOW(), NOW(), 1, 1, 1,
   '大额异常支付', '单笔支付金额超过用户历史均值10倍',
   'fraud_payment', 'high',
   '{"ratio_threshold":10}'::jsonb,
   '[{"type":"alert","level":"high"},{"type":"freeze_account"}]'::jsonb,
   true, 20, 'large_amount_payment',
   'event.amount > user.avg_pay_amount * 10 and event.amount > 5000',
   '{"ratio_threshold":10,"min_amount":5000}'::jsonb,
   'REALTIME', 12, '2025-06-26 03:00:00'),

  (NOW(), NOW(), 1, 1, 1,
   '异地登录检测', '用户登录城市与上次登录城市距离超过阈值',
   'location_anomaly', 'medium',
   '{"distance_km":500}'::jsonb,
   '[{"type":"alert","level":"medium"},{"type":"verify"}]'::jsonb,
   true, 50, 'geo_anomaly_login',
   'distance(geo.city, user.last_login_city) > 500',
   '{"distance_km":500}'::jsonb,
   'REALTIME', 256, '2025-06-24 22:00:00'),

  (NOW(), NOW(), 1, 1, 1,
   '设备指纹变更', '同一用户短时间内设备指纹变更',
   'device_change', 'medium',
   '{"window":"1h"}'::jsonb,
   '[{"type":"alert","level":"medium"}]'::jsonb,
   false, 60, 'device_fingerprint_change',
   'event.device_id != user.last_device_id within "1h"',
   '{"window":"1h"}'::jsonb,
   'BATCH', 5, NULL);

-- ============================================================
-- 4. 标签定义表 uba_tag_definitions（用户标签体系）
-- ============================================================
DELETE FROM uba_tag_definitions WHERE tenant_id = 1 AND code IN
  ('vip_level', 'pay_user', 'high_risk_user', 'churn_risk', 'new_user');

INSERT INTO uba_tag_definitions
  (created_at, updated_at, created_by, updated_by, tenant_id,
   name, code, description, category, tag_type,
   is_system, is_dynamic, refresh_interval_seconds, condition_expr, default_value, status)
VALUES
  (NOW(), NOW(), 1, 1, 1,
   'VIP等级', 'vip_level', '用户VIP等级标签（0-5）',
   'TAG_CATEGORY_USER', 'TAG_TYPE_ENUM',
   true, true, 3600,
   'user.vip_level',
   '0', 'ON'),

  (NOW(), NOW(), 1, 1, 1,
   '付费用户', 'pay_user', '是否有付费记录',
   'TAG_CATEGORY_BUSINESS', 'TAG_TYPE_BOOLEAN',
   true, true, 1800,
   'count(events where event_name == "pay_success" and user_id == ctx.user_id) > 0',
   'false', 'ON'),

  (NOW(), NOW(), 1, 1, 1,
   '高风险用户', 'high_risk_user', '风险等级为 high 或 critical 的用户',
   'TAG_CATEGORY_RISK', 'TAG_TYPE_BOOLEAN',
   true, true, 600,
   'user.risk_level in ["high","critical"]',
   'false', 'ON'),

  (NOW(), NOW(), 1, 1, 1,
   '流失风险用户', 'churn_risk', '7天内未活跃的用户',
   'TAG_CATEGORY_BEHAVIOR', 'TAG_TYPE_BOOLEAN',
   false, true, 86400,
   'now() - user.last_active_date > 7d',
   'false', 'ON'),

  (NOW(), NOW(), 1, 1, 1,
   '新用户', 'new_user', '注册7天内的用户',
   'TAG_CATEGORY_USER', 'TAG_TYPE_BOOLEAN',
   true, true, 3600,
   'now() - user.register_time <= 7d',
   'false', 'ON');

-- ============================================================
-- 数据说明
-- ============================================================
-- 配套 doris_demo_data.sql 使用：
--   - uba_applications 提供 SDK 接入凭据（demo_game_001 等），与 events_fact 的 tenant_id 对应
--   - uba_event_schemas 登记 7 个核心事件（漏斗步骤 + click），可在"事件 Schema"页管理
--   - uba_risk_rules 定义 5 条风控规则（高频/代理/大额/异地/设备），与 risk_events 的 rule_id 对应
--   - uba_tag_definitions 定义 5 个用户标签（VIP/付费/高风险/流失/新用户），可在"标签定义"页管理
--
-- 注意：
--   1. tag_definitions 的 category/tag_type 用全大写枚举值（TAG_CATEGORY_USER / TAG_TYPE_ENUM），
--      与前端 ToName 映射对齐
--   2. risk_rules 的 risk_type 用小写蛇形（abnormal_flow），与 risk-event 的 RISK_TYPE 映射对齐
--   3. event_schemas 的 properties 是 jsonb 数组，结构为 [{name,type,displayName,required}]
--   4. 应用表 desensitize 字段：demo_shop_002 开启脱敏，其余关闭（覆盖两种状态）

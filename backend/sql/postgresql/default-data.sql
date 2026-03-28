-- 插入字典类型
INSERT INTO public.sys_dict_types (
    id, type_code, type_name, sort_order, is_enabled, created_at, updated_at
) VALUES
      (1, 'GENDER', '性别', 40, true, now(), now()),
      (2, 'APP_TYPE', '应用类型', 60, true, now(), now()),
      (3, 'APP_PLATFORM', '应用平台', 70, true, now(), now()),
      (4, 'RISK_TYPE', '风险类型', 80, true, now(), now()),
      (5, 'RISK_LEVEL', '风险等级', 90, true, now(), now()),
      (6, 'TAG_SOURCE', '标签来源', 100, true, now(), now()),
      (7, 'ID_TYPE', 'ID类型', 110, true, now(), now())
;
SELECT setval('sys_dict_types_id_seq', (SELECT MAX(id) FROM sys_dict_types));

-- 插入字典条目
INSERT INTO public.sys_dict_entries (
    id, type_id, entry_value, numeric_value, sort_order, is_enabled, created_at, updated_at, tenant_id
) VALUES
      -- 性别
      (12, 1, 'MALE', 1, 1, true, now(), now(), 0),
      (13, 1, 'FEMALE', 2, 2, true, now(), now(), 0),
      (14, 1, 'SECRET', 0, 3, true, now(), now(), 0),

      -- ========== APP_TYPE 应用类型 (type_id=6) ==========
      (20, 2, 'game', 1, 1, true, now(), now(), 0),
      (21, 2, 'ecommerce', 2, 2, true, now(), now(), 0),
      (22, 2, 'content', 3, 3, true, now(), now(), 0),
      (23, 2, 'tool', 4, 4, true, now(), now(), 0),
      (24, 2, 'finance', 5, 5, true, now(), now(), 0),
      (25, 2, 'social', 6, 6, true, now(), now(), 0),
      (26, 2, 'education', 7, 7, true, now(), now(), 0),
      (27, 2, 'other', 99, 99, true, now(), now(), 0),

      -- ========== APP_PLATFORM 应用平台 (type_id=7) ==========
      (30, 3, 'ios', 1, 1, true, now(), now(), 0),
      (31, 3, 'android', 2, 2, true, now(), now(), 0),
      (32, 3, 'web', 3, 3, true, now(), now(), 0),
      (33, 3, 'h5', 4, 4, true, now(), now(), 0),
      (34, 3, 'mini_program', 5, 5, true, now(), now(), 0),
      (35, 3, 'harmony', 6, 6, true, now(), now(), 0),

      -- ========== RISK_TYPE 风险类型 (type_id=8) ==========
      (40, 4, 'login_anomaly', 1, 1, true, now(), now(), 0),
      (41, 4, 'device_anomaly', 2, 2, true, now(), now(), 0),
      (42, 4, 'multiple_account', 3, 3, true, now(), now(), 0),
      (43, 4, 'blacklist', 4, 4, true, now(), now(), 0),
      (44, 4, 'whitelist', 5, 5, true, now(), now(), 0),

      -- ========== RISK_LEVEL 风险等级 (type_id=9) ==========
      (50, 5, 'low', 1, 1, true, now(), now(), 0),
      (51, 5, 'medium', 2, 2, true, now(), now(), 0),
      (52, 5, 'high', 3, 3, true, now(), now(), 0),
      (53, 5, 'critical', 4, 4, true, now(), now(), 0),

      -- ========== TAG_SOURCE 标签来源 (type_id=10) ==========
      (60, 6, 'manual', 1, 1, true, now(), now(), 0),
      (61, 6, 'rule', 2, 2, true, now(), now(), 0),
      (62, 6, 'model', 3, 3, true, now(), now(), 0),
      (63, 6, 'import', 4, 4, true, now(), now(), 0),

      -- ========== ID_TYPE ID类型 (type_id=11) ==========
      (70, 7, 'user_id', 1, 1, true, now(), now(), 0),
      (71, 7, 'device_id', 2, 2, true, now(), now(), 0),
      (72, 7, 'openid', 3, 3, true, now(), now(), 0),
      (73, 7, 'unionid', 4, 4, true, now(), now(), 0),
      (74, 7, 'phone', 5, 5, true, now(), now(), 0),
      (75, 7, 'global_user_id', 6, 6, true, now(), now(), 0)
;
SELECT setval('sys_dict_entries_id_seq', (SELECT MAX(id) FROM sys_dict_entries));

-- 插入字典条目国际化（zh-CN）
INSERT INTO public.sys_dict_entry_i18n (
    entry_id, language_code, entry_label, description, sort_order, tenant_id, created_at, updated_at
) VALUES
      -- --------------------
      -- 中文
      -- --------------------

      -- 性别
      (12, 'zh-CN', '男', '', 1, 0, now(), now()),
      (13, 'zh-CN', '女', '', 2, 0, now(), now()),
      (14, 'zh-CN', '未知', '用户未填写时默认值', 3, 0, now(), now()),

      -- APP_TYPE
      (20, 'zh-CN', '游戏', '', 1, 0, now(), now()),
      (21, 'zh-CN', '电商', '', 2, 0, now(), now()),
      (22, 'zh-CN', '内容', '', 3, 0, now(), now()),
      (23, 'zh-CN', '工具', '', 4, 0, now(), now()),
      (24, 'zh-CN', '金融', '', 5, 0, now(), now()),
      (25, 'zh-CN', '社交', '', 6, 0, now(), now()),
      (26, 'zh-CN', '教育', '', 7, 0, now(), now()),
      (27, 'zh-CN', '其他', '', 99, 0, now(), now()),

      -- APP_PLATFORM
      (30, 'zh-CN', 'iOS', '', 1, 0, now(), now()),
      (31, 'zh-CN', '安卓', '', 2, 0, now(), now()),
      (32, 'zh-CN', '网页', '', 3, 0, now(), now()),
      (33, 'zh-CN', 'H5', '', 4, 0, now(), now()),
      (34, 'zh-CN', '小程序', '', 5, 0, now(), now()),
      (35, 'zh-CN', '鸿蒙', '', 6, 0, now(), now()),

      -- RISK_TYPE
      (40, 'zh-CN', '登录异常', '', 1, 0, now(), now()),
      (41, 'zh-CN', '设备异常', '', 2, 0, now(), now()),
      (42, 'zh-CN', '多账号', '', 3, 0, now(), now()),
      (43, 'zh-CN', '黑名单', '', 4, 0, now(), now()),
      (44, 'zh-CN', '白名单', '', 5, 0, now(), now()),

      -- RISK_LEVEL
      (50, 'zh-CN', '低风险', '', 1, 0, now(), now()),
      (51, 'zh-CN', '中风险', '', 2, 0, now(), now()),
      (52, 'zh-CN', '高风险', '', 3, 0, now(), now()),
      (53, 'zh-CN', '严重风险', '', 4, 0, now(), now()),

      -- TAG_SOURCE
      (60, 'zh-CN', '人工打标', '', 1, 0, now(), now()),
      (61, 'zh-CN', '规则引擎', '', 2, 0, now(), now()),
      (62, 'zh-CN', '算法模型', '', 3, 0, now(), now()),
      (63, 'zh-CN', '批量导入', '', 4, 0, now(), now()),

      -- ID_TYPE
      (70, 'zh-CN', '用户ID', '', 1, 0, now(), now()),
      (71, 'zh-CN', '设备ID', '', 2, 0, now(), now()),
      (72, 'zh-CN', 'OpenID', '', 3, 0, now(), now()),
      (73, 'zh-CN', 'UnionID', '', 4, 0, now(), now()),
      (74, 'zh-CN', '手机号', '', 5, 0, now(), now()),
      (75, 'zh-CN', '全局用户ID', '', 6, 0, now(), now()),

      -- --------------------
      -- 英文
      -- --------------------

      -- Gender
      (12, 'en-US', 'Male', '', 1, 0, now(), now()),
      (13, 'en-US', 'Female', '', 2, 0, now(), now()),
      (14, 'en-US', 'Unknown', 'Default value when user does not specify', 3, 0, now(), now()),

      -- APP_TYPE
      (20, 'en-US', 'Game', '', 1, 0, now(), now()),
      (21, 'en-US', 'E-commerce', '', 2, 0, now(), now()),
      (22, 'en-US', 'Content', '', 3, 0, now(), now()),
      (23, 'en-US', 'Tool', '', 4, 0, now(), now()),
      (24, 'en-US', 'Finance', '', 5, 0, now(), now()),
      (25, 'en-US', 'Social', '', 6, 0, now(), now()),
      (26, 'en-US', 'Education', '', 7, 0, now(), now()),
      (27, 'en-US', 'Other', '', 99, 0, now(), now()),

      -- APP_PLATFORM
      (30, 'en-US', 'iOS', '', 1, 0, now(), now()),
      (31, 'en-US', 'Android', '', 2, 0, now(), now()),
      (32, 'en-US', 'Web', '', 3, 0, now(), now()),
      (33, 'en-US', 'H5', '', 4, 0, now(), now()),
      (34, 'en-US', 'Mini Program', '', 5, 0, now(), now()),
      (35, 'en-US', 'HarmonyOS', '', 6, 0, now(), now()),

      -- RISK_TYPE
      (40, 'en-US', 'Login Anomaly', '', 1, 0, now(), now()),
      (41, 'en-US', 'Device Anomaly', '', 2, 0, now(), now()),
      (42, 'en-US', 'Multiple Account', '', 3, 0, now(), now()),
      (43, 'en-US', 'Blacklist', '', 4, 0, now(), now()),
      (44, 'en-US', 'Whitelist', '', 5, 0, now(), now()),

      -- RISK_LEVEL
      (50, 'en-US', 'Low Risk', '', 1, 0, now(), now()),
      (51, 'en-US', 'Medium Risk', '', 2, 0, now(), now()),
      (52, 'en-US', 'High Risk', '', 3, 0, now(), now()),
      (53, 'en-US', 'Critical Risk', '', 4, 0, now(), now()),

      -- TAG_SOURCE
      (60, 'en-US', 'Manual', '', 1, 0, now(), now()),
      (61, 'en-US', 'Rule Engine', '', 2, 0, now(), now()),
      (62, 'en-US', 'AI Model', '', 3, 0, now(), now()),
      (63, 'en-US', 'Batch Import', '', 4, 0, now(), now()),

      -- ID_TYPE
      (70, 'en-US', 'User ID', '', 1, 0, now(), now()),
      (71, 'en-US', 'Device ID', '', 2, 0, now(), now()),
      (72, 'en-US', 'OpenID', '', 3, 0, now(), now()),
      (73, 'en-US', 'UnionID', '', 4, 0, now(), now()),
      (74, 'en-US', 'Phone', '', 5, 0, now(), now()),
      (75, 'en-US', 'Global User ID', '', 6, 0, now(), now())
;
SELECT setval('sys_dict_entry_i18n_id_seq', (SELECT MAX(id) FROM sys_dict_entry_i18n));

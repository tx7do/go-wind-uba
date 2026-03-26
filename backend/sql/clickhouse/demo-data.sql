INSERT INTO gw_uba.path_features
(id, tenant_id, user_id, session_id, path_hash, first_event, last_event, path_length, first_3_events, last_3_events, is_converted, conversion_event, conversion_time, start_time, end_time, total_duration_ms, step_count)
VALUES
    ('PATH_HASH_001', 0, 10001, 10001001, '8f7d2c1e5a9b3072', 'page_view', 'purchase_success', 6, ['page_view', 'product_browse', 'add_cart'], ['add_cart', 'checkout', 'purchase_success'], 1, 'purchase_success', '2026-03-26 10:15:30.123', '2026-03-26 10:10:00.456', '2026-03-26 10:15:30.123', 330000, 6),
    ('PATH_HASH_002', 0, 10002, 10002001, '3a9d5c7e1b2f4068', 'page_view', 'page_exit', 4, ['page_view', 'product_browse', 'page_exit'], ['product_browse', 'page_view', 'page_exit'], 0, '', '1970-01-01 00:00:00.000', '2026-03-26 11:20:10.789', '2026-03-26 11:22:45.321', 155000, 4),
    ('PATH_HASH_003', 1, 10003, 10003001, '5b1e7d3c9a0f2468', 'login', 'order_pay', 3, ['login', 'order_pay'], ['login', 'order_pay'], 1, 'order_pay', '2026-03-26 14:05:11.222', '2026-03-26 14:04:30.555', '2026-03-26 14:05:11.222', 41000, 3),
    ('PATH_HASH_004', 1, 0, 10004001, '7c9a1e3d5b7f9246', 'ad_click', 'app_close', 8, ['ad_click', 'page_view', 'product_browse'], ['share_click', 'page_view', 'app_close'], 0, '', '1970-01-01 00:00:00.000', '2026-03-26 15:30:00.111', '2026-03-26 15:38:22.333', 502000, 8),
    ('PATH_HASH_005', 2, 10004, 10005001, '1d3f5a7c9e0b2468', 'wechat_login', 'coupon_use', 5, ['wechat_login', 'coupon_view', 'coupon_use'], ['coupon_view', 'coupon_use', 'coupon_use'], 1, 'coupon_use', '2026-03-26 16:40:15.666', '2026-03-26 16:37:20.999', '2026-03-26 16:40:15.666', 175000, 5),
    ('PATH_HASH_006', 2, 10005, 10006001, '9e7c5a3d1b0f2468', 'push_click', 'page_back', 2, ['push_click', 'page_back'], ['push_click', 'page_back'], 0, '', '1970-01-01 00:00:00.000', '2026-03-26 17:10:05.444', '2026-03-26 17:10:15.777', 10000, 2),
    ('PATH_HASH_007', 0, 10001, 10001002, '2b4d6f8a0c1e3579', 'member_center', 'repurchase_success', 5, ['member_center', 'order_list', 'repurchase'], ['order_list', 'repurchase', 'repurchase_success'], 1, 'repurchase_success', '2026-03-26 19:20:30.123', '2026-03-26 19:18:10.456', '2026-03-26 19:20:30.123', 140000, 5),
    ('PATH_HASH_008', 0, 10006, 10007001, '4c6e8a0d2f4b6791', 'search', 'search_purchase', 4, ['search', 'search_result', 'add_cart'], ['search_result', 'add_cart', 'search_purchase'], 1, 'search_purchase', '2026-03-26 20:05:10.333', '2026-03-26 20:03:00.666', '2026-03-26 20:05:10.333', 130000, 4);


INSERT INTO users_dim (
    tenant_id,user_id,register_time,register_channel,first_active_date,last_active_date,
    user_level,vip_level,user_role,total_events,total_sessions,total_pay_amount,last_pay_time,
    prefer_categories,prefer_objects,risk_score,risk_tags,risk_level,last_risk_time,
    geo,platform,device_type,profile,ver,created_at,updated_at
)
VALUES
    (0,10001,'2025-01-10 08:30:00','app_store','2025-01-10','2026-03-26',
     68,3,'member',12850,320,688.00,'2026-03-20 14:22:00',
        ['pay','shop','browse'],['electronics','clothes'],
     10,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'CN','province':'Beijing','city':'Beijing','isp':'ChinaMobile'},'PLATFORM_IOS','mobile',
        {'guild_id':'1001','server':'cn'},1,now(),now()),

    (0,10002,'2025-02-15 10:15:00','web','2025-02-15','2026-03-25',
     32,0,'player',4320,108,0.00,NULL,
        ['browse','video','social'],['entertainment'],
     15,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'CN','province':'Shanghai','city':'Shanghai','isp':'ChinaUnicom'},'PLATFORM_WEB','desktop',
        {'guild_id':'0','server':'cn'},1,now(),now()),

    (0,10003,'2025-03-20 16:40:00','huawei','2025-03-20','2026-03-26',
     99,5,'vip',28600,715,3288.50,'2026-03-25 21:10:00',
        ['pay','game','recharge'],['luxury','digital'],
     5,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'CN','province':'Guangdong','city':'Guangzhou','isp':'ChinaTelecom'},'PLATFORM_ANDROID','mobile',
        {'guild_id':'1003','server':'cn'},1,now(),now()),

    (1,10004,'2025-04-05 09:20:00','google_play','2025-04-05','2026-03-26',
     45,2,'member',7650,198,499.00,'2026-03-10 11:30:00',
        ['shop','pay','checkout'],['beauty','household'],
     8,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'US','province':'California','city':'LosAngeles','isp':'AT&T'},'PLATFORM_ANDROID','mobile',
        {'guild_id':'2001','server':'us'},1,now(),now()),

    (1,10005,'2025-05-12 20:10:00','web','2025-05-12','2026-03-23',
     18,0,'guest',1230,45,0.00,NULL,
        ['video','browse'],['news'],
     70,['abnormal_location'],'RISK_LEVEL_HIGH','2026-03-10 12:00:00',
        {'country':'US','province':'Texas','city':'Dallas','isp':'T-Mobile'},'PLATFORM_WEB','desktop',
        {'guild_id':'0','server':'us'},1,now(),now()),

    (2,10006,'2025-06-18 11:50:00','wechat_mini','2025-06-18','2026-03-26',
     77,4,'vip',18900,472,1860.80,'2026-03-22 19:45:00',
        ['pay','coupon','shop'],['food','fresh'],
     7,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'SG','province':'Singapore','city':'Singapore','isp':'Singtel'},'PLATFORM_MINI_PROGRAM','mobile',
        {'guild_id':'3001','server':'sg'},1,now(),now()),

    (2,10007,'2025-07-22 15:30:00','app_store','2025-07-22','2026-03-24',
     29,0,'player',3870,96,0.00,NULL,
        ['browse','search'],['fashion'],
     12,[],'RISK_LEVEL_NORMAL',NULL,
        {'country':'SG','province':'Singapore','city':'Singapore','isp':'StarHub'},'PLATFORM_IOS','mobile',
        {'guild_id':'0','server':'sg'},1,now(),now()),

    (0,10008,'2025-08-01 07:15:00','huawei','2025-08-01','2026-03-26',
     85,5,'vip',35400,885,5280.20,'2026-03-26 09:12:00',
        ['game','pay'],['digital'],
     3,['high_value'],'RISK_LEVEL_NORMAL',NULL,
        {'country':'CN','province':'Zhejiang','city':'Hangzhou','isp':'ChinaMobile'},'PLATFORM_ANDROID','mobile',
        {'guild_id':'1005','server':'cn'},1,now(),now());


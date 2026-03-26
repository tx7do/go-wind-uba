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


INSERT INTO gw_uba.sessions_fact
(
    id,tenant_id,user_id,device_id,global_user_id,
    start_time,end_time,duration_ms,event_count,page_view_count,action_count,
    entry_page,exit_page,is_bounce,
    platform,os,app_version,ip_city,country,
    total_amount,pay_event_count,
    risk_level,risk_tags,
    context,created_at,updated_at
)
VALUES
    (10001001,0,10001,'device_android_1001','GLOBAL_USER_10001',
     '2026-03-26 10:10:00.456','2026-03-26 10:15:30.123',330000,18,6,12,
     '/home','/checkout',0,
     'PLATFORM_ANDROID','Android 14','v2.5.0','Beijing','CN',
     688.00,1,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'s1','zone':'cn-east','ab_group':'A'},now(),now()),

    (10002001,0,10002,'device_web_1002','GLOBAL_USER_10002',
     '2026-03-26 11:20:10.789','2026-03-26 11:22:45.321',155000,9,3,6,
     '/list','/exit',1,
     'PLATFORM_WEB','Windows 11','web','Shanghai','CN',
     0.00,0,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'s1','zone':'cn-east','ab_group':'B'},now(),now()),

    (10003001,1,10003,'device_ios_1003','GLOBAL_USER_10003',
     '2026-03-26 14:04:30.555','2026-03-26 14:05:11.222',41000,5,2,3,
     '/login','/pay',0,
     'PLATFORM_IOS','iOS 17','v2.5.1','Guangzhou','CN',
     299.00,1,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'us1','zone':'us-west','ab_group':'A'},now(),now()),

    (10004001,1,0,'device_anon_1004','',
     '2026-03-26 15:30:00.111','2026-03-26 15:38:22.333',502000,24,8,16,
     '/ad','/close',0,
     'PLATFORM_ANDROID','Android 13','v2.4.0','Dallas','US',
     0.00,0,
     'RISK_LEVEL_SUSPICIOUS',['abnormal_location'],
        {'server_id':'us1','zone':'us-west','ab_group':'B'},now(),now()),

    (10005001,2,10004,'device_mini_1005','GLOBAL_USER_10004',
     '2026-03-26 16:37:20.999','2026-03-26 16:40:15.666',175000,12,4,8,
     '/wechat_mini','/coupon',0,
     'PLATFORM_MINI_PROGRAM','WeChat','v1.8.0','Singapore','SG',
     99.00,1,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'sg1','zone':'sg','ab_group':'A'},now(),now()),

    (10006001,2,10005,'device_web_1006','GLOBAL_USER_10005',
     '2026-03-26 17:10:05.444','2026-03-26 17:10:15.777',10000,2,1,1,
     '/push','/back',1,
     'PLATFORM_WEB','MacOS','web','Singapore','SG',
     0.00,0,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'sg1','zone':'sg','ab_group':'B'},now(),now()),

    (10001002,0,10001,'device_android_1001','GLOBAL_USER_10001',
     '2026-03-26 19:18:10.456','2026-03-26 19:20:30.123',140000,10,3,7,
     '/member','/repay',0,
     'PLATFORM_ANDROID','Android 14','v2.5.0','Beijing','CN',
     399.00,1,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'s1','zone':'cn-east','ab_group':'A'},now(),now()),

    (10007001,0,10006,'device_web_1007','GLOBAL_USER_10006',
     '2026-03-26 20:03:00.666','2026-03-26 20:05:10.333',130000,8,3,5,
     '/search','/buy',0,
     'PLATFORM_WEB','Windows 11','web','Hangzhou','CN',
     158.00,1,
     'RISK_LEVEL_NORMAL',[],
        {'server_id':'s1','zone':'cn-east','ab_group':'A'},now(),now());


INSERT INTO gw_uba.risk_events
(
    id,tenant_id,user_id,device_id,global_user_id,
    risk_type,risk_level,risk_score,rule_id,rule_name,rule_context,
    related_event_ids,session_id,description,evidence,
    status,handler_id,handled_time,occur_time,report_time,created_at,updated_at
)
VALUES
    (1,0,10001,'device_android_1001','GLOBAL_USER_10001',
     'RISK_TYPE_LOGIN_ANOMALY','RISK_LEVEL_HIGH',92.5,1,'频繁登录失败',{'threshold':'5','window':'300s','current':'8'},
        ['EVT_001','EVT_002','EVT_003'],10001001,
     '10分钟内登录失败8次，超过阈值5次',
        {'ip':'123.123.123.123','location':'Beijing','device':'Android 14'},
     'PENDING','','1970-01-01 00:00:00.000',
     '2026-03-26 10:10:00.123','2026-03-26 10:10:00.456',now(),now()),

    (2,0,10003,'device_ios_1003','GLOBAL_USER_10003',
     'RISK_TYPE_FRAUD_PAYMENT','RISK_LEVEL_CRITICAL',98.8,2,'大额支付检测',{'threshold':'5000','amount':'6888'},
        ['EVT_004','EVT_005'],10003001,
     '单笔支付金额6888元，超过阈值5000元',
        {'ip':'119.119.119.119','location':'Guangzhou','device':'iOS 17'},
     'CONFIRMED','admin_001','2026-03-26 14:06:00.123',
     '2026-03-26 14:05:00.234','2026-03-26 14:05:00.567',now(),now()),

    (3,1,0,'device_anon_1004','',
     'RISK_TYPE_LOCATION_ANOMALY','RISK_LEVEL_SUSPICIOUS',65.2,3,'异地登录检测',{'usual_city':'NewYork','current':'Dallas'},
        ['EVT_006'],10004001,
     '登录城市与常用城市不一致，异地访问',
        {'ip':'150.150.150.150','location':'Dallas','device':'Android 13'},
     'FALSE_POSITIVE','admin_002','2026-03-26 15:40:00.333',
     '2026-03-26 15:30:00.666','2026-03-26 15:30:00.999',now(),now()),

    (4,1,10005,'device_web_1006','GLOBAL_USER_10005',
     'RISK_TYPE_ABNORMAL_FLOW','RISK_LEVEL_CRITICAL',72.0,4,'频繁下单检测',{'window':'24h','threshold':'20','count':'27'},
        ['EVT_007','EVT_008','EVT_009'],10006001,
     '24小时内下单27次，超过阈值20次',
        {'ip':'180.180.180.180','location':'Singapore','device':'Windows 11'},
     'PENDING','','1970-01-01 00:00:00.000',
     '2026-03-26 17:10:00.111','2026-03-26 17:10:00.222',now(),now()),

    (5,2,10004,'device_mini_1005','GLOBAL_USER_10004',
     'RISK_TYPE_DEVICE_CHANGE','RISK_LEVEL_HIGH',89.9,5,'风险设备检测',{'risk_device':'true','simulator':'yes'},
        ['EVT_010'],10005001,
     '使用模拟器/越狱设备登录，设备风险',
        {'ip':'168.168.168.168','location':'Singapore','device':'WeChat MiniProgram'},
     'IGNORED','admin_003','2026-03-26 16:45:00.444',
     '2026-03-26 16:40:00.555','2026-03-26 16:40:00.777',now(),now()),

    (6,0,10008,'device_android_1008','GLOBAL_USER_10008',
     'RISK_TYPE_BRUTE_FORCE','RISK_LEVEL_CRITICAL',95.0,1,'暴力破解尝试',{'threshold':'10','fail_count':'15'},
        ['EVT_011','EVT_012','EVT_013'],10007001,
     '1小时内密码尝试失败15次，疑似暴力破解',
        {'ip':'192.168.1.1','location':'Hangzhou','device':'Android 14'},
     'CONFIRMED','admin_001','2026-03-26 20:10:00.123',
     '2026-03-26 20:05:00.333','2026-03-26 20:05:00.666',now(),now());


INSERT INTO gw_uba.objects_dim
(
    id,tenant_id,object_type,object_name,category_path,
    price,currency,rarity,attributes,
    status,valid_from,valid_to,created_at,updated_at
)
VALUES
    ('sku_10001',0,'product','iPhone 15','electronics/mobile/phone',
     5999.00,'CNY','N',{'color':'white','storage':'256G','cpu':'a16'},
     'online','2025-01-01 00:00:00',NULL,now(),now()),

    ('item_20001',0,'game_item','屠龙刀','game/equipment/weapon',
     888.00,'DIAMOND','SSR',{'attack':'150','durability':'100','type':'sword'},
     'online','2025-02-01 00:00:00',NULL,now(),now()),

    ('page_home',0,'page','首页','page/index/main',
     0.00,'CNY','N',{'layout':'double','need_login':'false'},
     'online','2025-01-01 00:00:00',NULL,now(),now()),

    ('level_1_001',1,'level','第一关','game/level/chapter1',
     0.00,'CNY','N',{'difficulty':'easy','reward':'coin','star':'3'},
     'online','2025-03-01 00:00:00',NULL,now(),now()),

    ('art_30001',1,'article','新手入门攻略','content/article/guide',
     0.00,'CNY','N',{'author':'system','read_time':'5min','topic':'beginner'},
     'online','2025-04-01 00:00:00',NULL,now(),now()),

    ('sku_40001',2,'product','夏季纯棉T恤','clothes/casual/tshirt',
     99.00,'CNY','N',{'color':'black','size':'XL','material':'cotton'},
     'offline','2025-01-01 00:00:00','2026-01-01 00:00:00',now(),now()),

    ('api_user_info',2,'api','用户信息接口','api/user/basic',
     0.00,'CNY','N',{'method':'GET','rate_limit':'100','version':'v1'},
     'online','2025-05-01 00:00:00',NULL,now(),now());

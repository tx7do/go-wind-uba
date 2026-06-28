-- ============================================================
-- UBA 系统 - 离线聚合 ETL
-- 数据库：gw_uba
-- 用途：从 events_fact / risk_events 聚合填充派生表与聚合表。
--       Kafka 仅灌入两张原始事实表，其余派生表需本脚本产出数据。
-- 执行顺序：6（在 02_kafka_tables 之后、05_views 之前或之后均可）
-- 执行方式：定时调度（如每日凌晨），或配合 Doris Job 自动执行。
-- 调度参数：${RUN_DATE} 待回算日期，形如 '2026-06-28'；不传则取昨天。
-- 兼容版本：Apache Doris 2.0+ / 4.x
-- ============================================================

USE gw_uba;

-- 说明：
-- 1) sessions_fact / users_dim 为 UNIQUE KEY 表，重复 INSERT 按主键覆盖，
--    可安全重复执行（幂等）。建议按日分区粒度回算。
-- 2) 聚合表（*_agg_daily / popular_paths_daily）为 AGGREGATE 模型，
--    重复 INSERT 会按聚合键自动 SUM/HLL 合并，天然支持增量。
-- 3) objects_dim / id_mapping / user_tags / path_features 涉及业务逻辑
--    （打标置信度、ID 关联算法、对象维护等），应由 core 服务通过 gRPC 维护，
--    本脚本不从事实表推断（注释见文末）。


-- ============================================================
-- 0. 调度日期变量（手工执行时取消注释并替换；调度系统传入 ${RUN_DATE}）
-- ============================================================
-- SET @run_date = DATE_SUB(CURDATE(), INTERVAL 1 DAY);
-- 以下统一用 ${RUN_DATE} 占位；手工执行请全局替换为目标日期。


-- ============================================================
-- 1. 填充 sessions_fact（从 events_fact 按 session 聚合）
--    会话定义：同一 session_id 的所有事件归为一个会话。
-- ============================================================
INSERT INTO sessions_fact (
    session_id, tenant_id, session_date,
    user_id, device_id, global_user_id,
    start_time, end_time, duration_ms,
    event_count, page_view_count, action_count,
    entry_page, exit_page, is_bounce,
    platform, app_version, ip_city, country,
    total_amount, pay_event_count
)
SELECT
    session_id,
    tenant_id,
    to_date(event_time)                                  AS session_date,
    MAX(user_id)                                         AS user_id,
    MAX(device_id)                                       AS device_id,
    MAX(global_user_id)                                  AS global_user_id,
    MIN(event_time)                                      AS start_time,
    MAX(event_time)                                      AS end_time,
    UNIX_MILLIS(MAX(event_time)) - UNIX_MILLIS(MIN(event_time)) AS duration_ms,
    COUNT(*)                                             AS event_count,
    SUM(IF(event_name = 'page_view', 1, 0))              AS page_view_count,
    SUM(IF(event_action IS NOT NULL AND event_action != '', 1, 0)) AS action_count,
    MIN(referer)                                         AS entry_page,
    MAX(object_name)                                     AS exit_page,
    IF(COUNT(*) = 1, 1, 0)                               AS is_bounce,
    MAX(platform)                                        AS platform,
    MAX(app_version)                                     AS app_version,
    MAX(ip_city)                                         AS ip_city,
    MAX(country)                                         AS country,
    ROUND(SUM(amount), 2)                                AS total_amount,
    SUM(IF(event_name = 'pay', 1, 0))                    AS pay_event_count
FROM events_fact
WHERE to_date(event_time) = ${RUN_DATE}
  AND session_id IS NOT NULL AND session_id != ''
GROUP BY session_id, tenant_id, session_date;


-- ============================================================
-- 2. 填充 users_dim 基础活跃字段（累计值，幂等）
--    关键点（幂等性设计）：
--    a) first_active_date / total_events / total_pay_amount 是「累计值」，
--       不能按当天增量计算，否则首日回算后被后续覆盖丢失历史。
--       解法：只取「当天活跃的 user_id 集合」，再对该集合在 events_fact 上
--             做【无时间限制】的全量聚合 → MIN/COUNT/SUM 即为真累计值，
--             无论哪天回算，结果一致（幂等）。
--    b) users_dim 含画像/标签/风险等由 core 服务维护的字段，本步骤开启
--       PARTIAL UPDATE（仅更新下述列），绝不覆盖 core 写入的画像字段。
--    c) 仅处理当天活跃用户（增量），避免全表扫描历史所有用户。
-- ============================================================

-- 2.1 开启部分列更新（仅作用于本 session 的后续 INSERT）
SET enable_unique_key_partial_update = true;

-- 2.2 对当天活跃用户，全量重算累计活跃指标（覆盖其 users_dim 行的指定列）
INSERT INTO users_dim (
    tenant_id, user_id,
    first_active_date, last_active_date,
    total_events, total_pay_amount, last_pay_time
)
SELECT
    e.tenant_id,
    e.user_id,
    MIN(to_date(e.event_time))                            AS first_active_date,
    MAX(to_date(e.event_time))                            AS last_active_date,
    COUNT(*)                                              AS total_events,
    ROUND(SUM(IF(e.amount > 0, e.amount, 0)), 2)          AS total_pay_amount,
    MAX(IF(e.amount > 0, e.event_time, NULL))             AS last_pay_time
FROM events_fact e
JOIN (
    -- 当天活跃的 (tenant_id, user_id) 集合，作为增量触发范围
    SELECT DISTINCT tenant_id, user_id
    FROM events_fact
    WHERE to_date(event_time) = ${RUN_DATE}
      AND user_id > 0
) act
  ON act.tenant_id = e.tenant_id
 AND act.user_id   = e.user_id
WHERE e.user_id > 0
GROUP BY e.tenant_id, e.user_id;

-- 2.3 关闭部分列更新（恢复默认，避免影响后续语句）
SET enable_unique_key_partial_update = false;


-- ============================================================
-- 3. 填充 sessions_agg_daily（会话日聚合，含 HLL/QUANTILE_STATE）
--    来源：sessions_fact（需先执行第 1 步）。
-- ============================================================
INSERT INTO sessions_agg_daily (
    tenant_id, stat_date, platform,
    session_count, unique_users,
    duration_sum, duration_count,
    bounce_sum, bounce_count,
    total_amount,
    duration_quantile
)
SELECT
    tenant_id,
    session_date                                         AS stat_date,
    IFNULL(platform, 'unknown')                          AS platform,
    COUNT(*)                                             AS session_count,
    HLL_HASH(user_id)                                    AS unique_users,
    SUM(duration_ms)                                     AS duration_sum,
    COUNT(duration_ms)                                   AS duration_count,
    SUM(is_bounce)                                       AS bounce_sum,
    COUNT(is_bounce)                                     AS bounce_count,
    ROUND(SUM(total_amount), 2)                          AS total_amount,
    QUANTILE_UNION(duration_ms)                          AS duration_quantile
FROM sessions_fact
WHERE session_date = ${RUN_DATE}
GROUP BY tenant_id, stat_date, platform;
-- 注：QUANTILE_STATE 列写入用 QUANTILE_UNION(col)，读取用 QUANTILE_PERCENT(col, p) 还原任意分位。
--     单列 duration_quantile 即可支持 P50/P90/P99 等任意分位查询，无需冗余多列。


-- ============================================================
-- 4. 填充 user_tags_agg（用户标签聚合）
--    来源：user_tags（需 core 服务维护打标数据）。
--    若 user_tags 暂无数据，本步产出空结果，不影响其余流程。
-- ============================================================
INSERT INTO user_tags_agg (
    tenant_id, tag_id, tag_value, stat_date, user_count
)
SELECT
    tenant_id,
    tag_id,
    IFNULL(tag_value, '')                                AS tag_value,
    expire_date                                          AS stat_date,
    COUNT(DISTINCT user_id)                              AS user_count
FROM user_tags
WHERE expire_date = ${RUN_DATE}
  AND is_active = 1
GROUP BY tenant_id, tag_id, tag_value, expire_date;


-- ============================================================
-- 5. 填充 popular_paths_daily（热门路径日聚合）
--    来源：path_features（需 core 服务生成路径特征）。
--    若 path_features 暂无数据，本步产出空结果。
-- ============================================================
INSERT INTO popular_paths_daily (
    tenant_id, stat_date, sequence_hash,
    event_sequence, support_count, unique_users,
    duration_sum, duration_count,
    conversion_sum, conversion_count
)
SELECT
    tenant_id,
    event_date                                           AS stat_date,
    path_hash                                            AS sequence_hash,
    first_3_events                                       AS event_sequence,
    COUNT(*)                                             AS support_count,
    HLL_HASH(user_id)                                    AS unique_users,
    SUM(total_duration_ms)                               AS duration_sum,
    COUNT(total_duration_ms)                             AS duration_count,
    SUM(IF(is_converted = 1, 1, 0))                      AS conversion_sum,
    COUNT(*)                                             AS conversion_count
FROM path_features
WHERE event_date = ${RUN_DATE}
GROUP BY tenant_id, event_date, path_hash, first_3_events;


-- ============================================================
-- 6. 校验：回算当日各派生表行数
-- ============================================================
SELECT 'sessions_fact'      AS tbl, COUNT(*) AS cnt FROM sessions_fact      WHERE session_date = ${RUN_DATE}
UNION ALL
SELECT 'sessions_agg_daily' AS tbl, COUNT(*) AS cnt FROM sessions_agg_daily WHERE stat_date = ${RUN_DATE}
UNION ALL
SELECT 'users_dim(updated)' AS tbl, COUNT(*) AS cnt FROM users_dim          WHERE last_active_date = ${RUN_DATE};


-- ============================================================
-- 附录：不由本 ETL 产出的表（数据来源说明）
-- ============================================================
-- objects_dim  : 业务对象维表（商品/道具/关卡），由 core 服务维护，对应
--                ApplicationService / ObjectService 等管理接口写入。
-- id_mapping   : 全局用户ID关联，由 core 服务的 IDMappingService 按算法生成
--                （跨设备/跨账号识别，涉及置信度，不可从事实表简单聚合）。
-- user_tags    : 用户标签关联，由 core 服务的 UserTagService 维护
--                （手动/规则/算法打标，含置信度、来源、有效期）。
-- path_features: 用户行为路径特征，由 core 服务的 EventPathService 生成
--                （会话内事件序列切分、转化标记，需流式/批式计算）。
-- risk_events  : 风险事件，由 Kafka topic uba_risk_events 直接灌入（见 02）。
--
-- 结论：本 ETL 只负责「可从原始事实表纯聚合得出」的派生表；
--       涉及业务算法的维表/特征表应由 core 服务的对应 Service 维护。

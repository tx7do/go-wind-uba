-- ============================================================
-- UBA 系统 - ClickHouse 派生表回算（对应 Doris 侧 06_etl.sql §1/§2）
-- 执行方式：由 `uba-ingest ch-etl` 渲染并执行（见 backend/cmd/uba-ingest），
--          占位符 {{.RunDate}} 为待回算日期（形如 '2026-06-28'）。
--          本地手工执行前先用 `uba-ingest ch-etl --dry-run` 输出渲染后的 SQL。
--          日内新鲜度：`ch-etl --loop --every 1h` 每小时重算"今天"（本文件全为
--          replace 语义，幂等安全），白天查询即可见当天数据。
--
-- 口径说明（与 Doris 侧一致）：
--   1) sessions_fact / users_dim 的写入是【全历史回算】而非块内增量：
--      按 RunDate 当天活跃的会话/用户集合触发，再对该集合在 events_fact 上
--      做无时间限制的全量聚合 → 结果与哪天回算无关（幂等，重跑安全）。
--      这取代了旧的 mv_sessions_fact / mv_users_dim（按插入块聚合，
--      跨块拆行/覆盖、累计值只有块内局部真值）。
--   2) CH 无 Doris 的部分列更新：users_dim 整行由本脚本写入（活跃/付费/
--      会话/平台/国家等可从 events_fact 推断的列），register_time、等级、
--      风险等画像列保持默认值，等待 core 服务通过 users_dim_repo 写入
--      （写入 updated_at 更新，ReplacingMergeTree 取最新版本整行生效）。
--   3) §3 的 OPTIMIZE FINAL 把 ReplacingMergeTree 的后台合并提前做完，
--      让 crud 分页列表（无法带 FINAL）也读到去重后的确定结果。
--
-- 兼容版本：ClickHouse 23.12+（events_fact 使用 optimize_on_insert）。
-- ============================================================

-- ============================================================
-- 1. 填充 sessions_fact（从 events_fact 按 session 全量聚合）
--    会话定义：同一 session_id 的所有事件归为一个会话；
--    触发范围：当天发生过事件的会话。跨天会话在任一天回算时都做全量聚合，
--    最终收敛为完整会话（ReplacingMergeTree 按 (tenant_id, session_id,
--    session_date) 取 updated_at 最新）。
-- ============================================================
INSERT INTO gw_uba.sessions_fact
    (session_id, tenant_id, user_id, device_id, global_user_id,
     start_time, end_time, duration_ms,
     event_count, page_view_count, action_count,
     entry_page, exit_page, is_bounce,
     platform, app_version, ip_city, country,
     total_amount, pay_event_count)
SELECT
    session_id,
    tenant_id,
    max(user_id)                                                    AS user_id,
    max(device_id)                                                  AS device_id,
    max(global_user_id)                                             AS global_user_id,
    min(event_time)                                                 AS start_time,
    max(event_time)                                                 AS end_time,
    dateDiff('millisecond', min(event_time), max(event_time))       AS duration_ms,
    count()                                                         AS event_count,
    countIf(event_name = 'page_view')                               AS page_view_count,
    countIf(event_action IS NOT NULL AND event_action != '')        AS action_count,
    min(referer)                                                    AS entry_page,
    max(object_name)                                                AS exit_page,
    if(count() = 1, 1, 0)                                           AS is_bounce,
    any(platform)                                                   AS platform,
    any(app_version)                                                AS app_version,
    any(ip_city)                                                    AS ip_city,
    any(country)                                                    AS country,
    sum(amount)                                                     AS total_amount,
    countIf(event_category = 'pay')                                 AS pay_event_count
FROM gw_uba.events_fact FINAL
WHERE toDate(event_time) = toDate({{.RunDate}})
  AND session_id != ''
GROUP BY tenant_id, session_id;


-- ============================================================
-- 2. 填充 users_dim 基础活跃字段（累计值，幂等）
--    关键点（与 Doris §2 同款幂等设计）：
--    a) first_active_date / total_events / total_pay_amount 是累计值，
--       只取「当天活跃的 (tenant_id, user_id) 集合」，再对该集合在
--       events_fact 上做【无时间限制】的全量聚合 → 无论哪天回算结果一致。
--    b) 画像列（register/level/risk/profile 等）不写入，保持默认值，
--       等 core 服务写入（其 updated_at 更新，整行覆盖本结果）。
-- ============================================================
INSERT INTO gw_uba.users_dim
    (tenant_id, user_id,
     first_active_date, last_active_date,
     total_events, total_sessions, total_pay_amount, last_pay_time,
     platform, country,
     created_at, updated_at)
SELECT
    e.tenant_id,
    e.user_id,
    min(toDate(e.event_time))                              AS first_active_date,
    max(toDate(e.event_time))                              AS last_active_date,
    count()                                                AS total_events,
    toUInt32(uniqExact(e.session_id))                      AS total_sessions,
    round(sum(if(e.amount > 0, e.amount, 0)), 2)           AS total_pay_amount,
    max(if(e.amount > 0, e.event_time, NULL))              AS last_pay_time,
    any(e.platform)                                        AS platform,
    any(e.country)                                         AS country,
    now(), now()
FROM gw_uba.events_fact AS e FINAL
INNER JOIN
(
    -- 当天活跃的 (tenant_id, user_id) 集合，作为增量触发范围
    SELECT DISTINCT tenant_id, user_id
    FROM gw_uba.events_fact FINAL
    WHERE toDate(event_time) = toDate({{.RunDate}})
      AND user_id > 0
) act ON act.tenant_id = e.tenant_id AND act.user_id = e.user_id
WHERE e.user_id > 0
GROUP BY e.tenant_id, e.user_id;


-- ============================================================
-- 3. 强制合并（OPTIMIZE FINAL）：把 ReplacingMergeTree 的去重合并提前完成
--    分析查询都显式带 FINAL，不依赖本步；本步服务两类读者：
--    a) go-crud 分页列表（生成的 SQL 无法带 FINAL）；
--    b) 降低后续查询的 FINAL 代价（合并过的部分 FINAL 读更快）。
--    events_fact 数据量大时本步耗时较长；每天一次即可接受。
-- ============================================================
OPTIMIZE TABLE gw_uba.events_fact FINAL;
OPTIMIZE TABLE gw_uba.sessions_fact FINAL;
OPTIMIZE TABLE gw_uba.users_dim FINAL;
OPTIMIZE TABLE gw_uba.risk_events FINAL;
OPTIMIZE TABLE gw_uba.id_mapping FINAL;
OPTIMIZE TABLE gw_uba.user_tags FINAL;

-- ============================================================
-- UBA 系统 - 日常查询语句集
-- 数据库：gw_uba
-- 说明：覆盖 UBA 系统常见分析场景，优先复用物化视图/业务视图，
--       底层扫描均带 tenant_id + 时间分区裁剪，避免全表扫描。
-- 兼容版本：Apache Doris 2.0+ / 4.x
-- ============================================================

USE gw_uba;

-- 用法约定：
--   ${TENANT_ID}  租户ID，多租户必填，请替换为实际值
--   ${START}/${END} 时间范围，形如 '2026-06-01' / '2026-06-30'


-- ============================================================
-- 一、流量与活跃分析（基于 mv_events_daily / 业务视图）
-- ============================================================

-- 1.1 日活 DAU / 事件数 PV - 近 30 天趋势
SELECT
    stat_date,
    HLL_CARDINALITY(uv)            AS dau,
    pv                              AS event_count,
    session_count,
    HLL_CARDINALITY(pay_user_count) AS pay_dau
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY)
ORDER BY stat_date;

-- 1.2 等价写法 - 直接查业务视图（推荐给报表层使用）
SELECT stat_date, dau, event_count
FROM v_daily_active_users
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= ${START}
ORDER BY stat_date;

-- 1.3 分平台 DAU / 会话 - 近 7 天
SELECT
    stat_date,
    platform,
    HLL_CARDINALITY(uv) AS uv,
    pv,
    session_count
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
ORDER BY stat_date, platform;

-- 1.4 小时级流量曲线（实时性最高，查 mv_events_hourly）- 某天分时
SELECT
    stat_hour,
    event_name,
    HLL_CARDINALITY(uv) AS uv,
    pv
FROM mv_events_hourly
WHERE tenant_id = ${TENANT_ID}
  AND stat_hour >= '${START} 00:00:00'
  AND stat_hour <  DATE_ADD('${START}', INTERVAL 1 DAY)
ORDER BY stat_hour, pv DESC;

-- 1.5 Top 事件（按 PV / UV 排序）- 指定日期区间
SELECT
    event_name,
    SUM(pv)                                   AS pv,
    HLL_CARDINALITY(HLL_UNION(uv))            AS uv,
    ROUND(SUM(total_amount), 2)               AS total_amount,
    ROUND(SUM(duration_sum) / NULLIF(SUM(duration_count), 0), 2) AS avg_duration_ms
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
GROUP BY event_name
ORDER BY pv DESC
LIMIT 20;


-- ============================================================
-- 二、会话分析（基于 mv_sessions_daily / v_daily_session_analysis）
-- ============================================================

-- 2.1 会话概览（会话数、UV、平均时长、跳出率）- 分平台
SELECT
    stat_date,
    platform,
    session_count,
    HLL_CARDINALITY(unique_users)                                          AS uv,
    ROUND(duration_sum / 1000 / 60, 2)                                     AS total_minutes,
    ROUND(duration_sum / 1000 / 60 / NULLIF(duration_count, 0), 2)         AS avg_session_min,
    ROUND(IF(session_count > 0, bounce_sum / session_count * 100, 0), 2)   AS bounce_rate_pct,
    ROUND(total_amount, 2)                                                 AS total_amount
FROM mv_sessions_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date, platform;

-- 2.2 等价写法 - 业务视图（跳出率已算好）
SELECT stat_date, platform, session_count, uv, total_minutes, bounce_rate
FROM v_daily_session_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date, platform;

-- 2.3 会话时长分位（基于聚合表 QUANTILE_STATE，单列还原任意分位）- P50/P90/P99
SELECT
    platform,
    ROUND(QUANTILE_PERCENT(duration_quantile, 0.5)  / 1000, 2) AS p50_sec,
    ROUND(QUANTILE_PERCENT(duration_quantile, 0.9)  / 1000, 2) AS p90_sec,
    ROUND(QUANTILE_PERCENT(duration_quantile, 0.99) / 1000, 2) AS p99_sec
FROM sessions_agg_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
GROUP BY platform;


-- ============================================================
-- 三、用户留存分析（基于 mv_user_retention）
-- ============================================================

-- 3.1 N 日留存矩阵（首次活跃日 → 第 N 天留存）
SELECT
    first_date           AS cohort_date,
    retention_day,
    HLL_CARDINALITY(retained_users) AS retained_users
FROM mv_user_retention
WHERE tenant_id = ${TENANT_ID}
  AND first_date >= ${START}
  AND retention_day BETWEEN 0 AND 7
ORDER BY first_date, retention_day;

-- 3.2 留存率（第 N 天留存 / 首日新增）- 透视成横向矩阵
SELECT
    cohort_date,
    MAX(IF(retention_day = 0, retained_users, 0)) AS d0,
    MAX(IF(retention_day = 1, retained_users, 0)) AS d1,
    MAX(IF(retention_day = 3, retained_users, 0)) AS d3,
    MAX(IF(retention_day = 7, retained_users, 0)) AS d7
FROM (
    SELECT
        first_date AS cohort_date,
        retention_day,
        HLL_CARDINALITY(retained_users) AS retained_users
    FROM mv_user_retention
    WHERE tenant_id = ${TENANT_ID}
      AND first_date BETWEEN ${START} AND ${END}
      AND retention_day IN (0, 1, 3, 7)
) t
GROUP BY cohort_date
ORDER BY cohort_date;


-- ============================================================
-- 四、用户画像与标签（基于 users_dim / user_tags_agg）
-- ============================================================

-- 4.1 用户活跃度分层（按最后活跃日期）- 高活/中活/低活/流失
SELECT
    CASE
        WHEN last_active_date >= DATE_SUB(CURDATE(), INTERVAL 1 DAY)  THEN '活跃(1天内)'
        WHEN last_active_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)  THEN '近期(7天内)'
        WHEN last_active_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY) THEN '一般(30天内)'
        ELSE '流失(30天+)'
    END AS active_level,
    COUNT(*) AS user_cnt
FROM users_dim
WHERE tenant_id = ${TENANT_ID}
GROUP BY active_level
ORDER BY user_cnt DESC;

-- 4.2 用户价值分布（累计支付金额分层）
SELECT
    CASE
        WHEN total_pay_amount >= 10000 THEN ' whale(>=1万)'
        WHEN total_pay_amount >= 1000  THEN 'high(>=1千)'
        WHEN total_pay_amount >  0     THEN 'low(>0)'
        ELSE 'non_pay'
    END AS pay_level,
    COUNT(*) AS user_cnt
FROM users_dim
WHERE tenant_id = ${TENANT_ID}
GROUP BY pay_level
ORDER BY user_cnt DESC;

-- 4.3 标签人群数量（运营圈选）- 指定日期
SELECT
    tag_id,
    tag_value,
    SUM(user_count) AS user_count
FROM v_user_tag_count
WHERE tenant_id = ${TENANT_ID}
  AND stat_date = ${START}
GROUP BY tag_id, tag_value
ORDER BY user_count DESC
LIMIT 30;

-- 4.4 高风险用户清单（风控黑/灰名单）- 供人工复核
--     risk_level 取值由业务定义（proto 为自由字符串），使用前请核对：
--     SELECT DISTINCT risk_level FROM users_dim WHERE tenant_id=${TENANT_ID};
SELECT
    user_id,
    risk_level,
    risk_score,
    last_risk_time,
    array_join(risk_tags, ',') AS risk_tags,
    last_active_date
FROM users_dim
WHERE tenant_id = ${TENANT_ID}
  AND risk_level IN (${RISK_LEVEL_HIGH}, ${RISK_LEVEL_BLACK})
ORDER BY risk_score DESC
LIMIT 100;


-- ============================================================
-- 五、风险事件分析（基于 mv_risk_daily / risk_events）
-- ============================================================

-- 5.1 风险概览（按等级）- 业务视图
SELECT
    stat_date,
    risk_level,
    total_risk_events,
    risk_user_count
FROM v_daily_risk_overview
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date, risk_level;

-- 5.2 风险类型分布 + 处置状态（确认/误报/待处理）
SELECT
    risk_type,
    status,
    SUM(event_count) AS event_count,
    HLL_CARDINALITY(HLL_UNION(unique_users)) AS unique_users,
    ROUND(SUM(risk_score_sum) / NULLIF(SUM(risk_score_count), 0), 2) AS avg_risk_score
FROM mv_risk_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
GROUP BY risk_type, status
ORDER BY event_count DESC;

-- 5.3 最近高风险事件明细（告警排查）- 走事实表，命中倒排索引
SELECT
    risk_event_id,
    risk_type,
    risk_level,
    risk_score,
    rule_name,
    status,
    user_id,
    device_id,
    occur_time,
    description
FROM risk_events
WHERE tenant_id = ${TENANT_ID}
  AND event_date >= DATE_SUB(CURDATE(), INTERVAL 1 DAY)
  AND risk_level IN (${RISK_LEVEL_HIGH}, ${RISK_LEVEL_CRITICAL})
ORDER BY risk_score DESC, occur_time DESC
LIMIT 100;

-- 5.4 单用户风险事件追溯（用户画像排查）
SELECT
    risk_type, risk_level, risk_score, rule_name,
    status, occur_time, description, handler_id, handle_remark
FROM risk_events
WHERE tenant_id = ${TENANT_ID}
  AND user_id = ${USER_ID}
ORDER BY occur_time DESC
LIMIT 50;


-- ============================================================
-- 六、路径与转化分析（基于 popular_paths_daily / path_features）
-- ============================================================

-- 6.1 热门转化路径 Top N（含转化率）- 业务视图
SELECT
    event_sequence,
    support_count,
    user_count,
    conversion_rate
FROM v_popular_conversion_paths
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY support_count DESC
LIMIT 20;

-- 6.2 路径步数分布（短/中/长路径占比）
SELECT
    path_length,
    COUNT(*) AS path_cnt,
    SUM(IF(is_converted = 1, 1, 0)) AS converted_cnt,
    ROUND(SUM(IF(is_converted = 1, 1, 0)) / COUNT(*) * 100, 2) AS conv_rate_pct
FROM path_features
WHERE tenant_id = ${TENANT_ID}
  AND event_date BETWEEN ${START} AND ${END}
GROUP BY path_length
ORDER BY path_length;


-- ============================================================
-- 七、明细查询与排障（基于事实表，命中倒排索引）
-- ============================================================

-- 7.1 按 trace_id 精确查单条事件（链路排障）
SELECT *
FROM events_fact
WHERE trace_id = '${TRACE_ID}'
  AND tenant_id = ${TENANT_ID}
LIMIT 10;

-- 7.2 按设备/账号查用户行为时间线
SELECT
    event_time, event_name, event_action, object_type, object_name,
    platform, ip_city, duration_ms, amount
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND (device_id = '${DEVICE_ID}' OR account_id = '${ACCOUNT_ID}')
  AND event_time >= '${START} 00:00:00'
ORDER BY event_time
LIMIT 200;

-- 7.3 地域分布（按国家/城市 UV）
SELECT
    country,
    ip_city,
    HLL_CARDINALITY(HLL_UNION(uv)) AS uv,
    SUM(pv)                        AS pv
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
GROUP BY country, ip_city
ORDER BY uv DESC
LIMIT 20;


-- ============================================================
-- 八、运维监控（Doris 元数据语句）
-- ============================================================

-- 8.1 Routine Load 任务运行状态（关注 job_status / consumed_records / errorMsg）
SHOW ROUTINE LOAD FOR gw_uba.job_events_to_fact;
SHOW ROUTINE LOAD FOR gw_uba.job_risk_events_to_fact;

-- 8.2 事实表分区列表（确认动态分区滚动正常，关注分区名/数据量/行数）
SHOW PARTITIONS FROM events_fact     ORDER BY partition_name DESC LIMIT 30;
SHOW PARTITIONS FROM risk_events     ORDER BY partition_name DESC LIMIT 30;
SHOW PARTITIONS FROM sessions_fact   ORDER BY partition_name DESC LIMIT 30;

-- 8.3 物化视图刷新状态（关注 state / last_refresh_start_time / last_refresh_status）
SHOW MATERIALIZED VIEWS FROM gw_uba WHERE name LIKE 'mv_%';

-- 8.4 各事实表行数（按租户；COUNT(*) 命中 Doris UNIQUE 表的 merge-on-write 优化）
SELECT 'events_fact'   AS table_name, COUNT(*) AS rows_cnt FROM events_fact   WHERE tenant_id = ${TENANT_ID}
UNION ALL
SELECT 'risk_events'   AS table_name, COUNT(*) AS rows_cnt FROM risk_events   WHERE tenant_id = ${TENANT_ID}
UNION ALL
SELECT 'sessions_fact' AS table_name, COUNT(*) AS rows_cnt FROM sessions_fact WHERE tenant_id = ${TENANT_ID};


-- ============================================================
-- 九、进阶分析（漏斗 / 序列 / 对象 / ID打通 / 同比环比 / 属性下钻）
-- ============================================================

-- ──────────────────────────────────────────────
-- 9.1 漏斗分析（通用版）：4 步漏斗，事件名用占位符，方便复用
--     用法：把 ${EVENT_1}..${EVENT_4} 替换为实际事件名。
--           游戏示例：game_start / level_finish / item_buy / vip_buy
--           电商示例：page_view / add_to_cart / place_order / pay
--     思路：按用户展开每步首达时间，逐步收缩判断是否进入下一级；
--           以进入漏斗首步的用户为基数（HAVING t1 IS NOT NULL）。
--     说明：严格按事件先后顺序转化（t_n >= t_{n-1}），同级耗时可用 DATEDIFF 拓展。
-- ──────────────────────────────────────────────
WITH step_users AS (
    SELECT
        user_id,
        MIN(IF(event_name = '${EVENT_1}', event_time, NULL)) AS t1,
        MIN(IF(event_name = '${EVENT_2}', event_time, NULL)) AS t2,
        MIN(IF(event_name = '${EVENT_3}', event_time, NULL)) AS t3,
        MIN(IF(event_name = '${EVENT_4}', event_time, NULL)) AS t4
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
      AND event_name IN ('${EVENT_1}', '${EVENT_2}', '${EVENT_3}', '${EVENT_4}')
    GROUP BY user_id
    HAVING t1 IS NOT NULL
)
SELECT
    COUNT(*)                                                         AS step1_users,
    SUM(IF(t2 IS NOT NULL AND t2 >= t1, 1, 0))                       AS step2_users,
    SUM(IF(t3 IS NOT NULL AND t3 >= t2, 1, 0))                       AS step3_users,
    SUM(IF(t4 IS NOT NULL AND t4 >= t3, 1, 0))                       AS step4_users,
    ROUND(SUM(IF(t2 IS NOT NULL AND t2 >= t1, 1, 0))  / COUNT(*) * 100, 2)
        AS step2_rate,
    ROUND(SUM(IF(t3 IS NOT NULL AND t3 >= t2, 1, 0))  / NULLIF(SUM(IF(t2 IS NOT NULL AND t2 >= t1, 1, 0)), 0) * 100, 2)
        AS step3_rate,
    ROUND(SUM(IF(t4 IS NOT NULL AND t4 >= t3, 1, 0))  / NULLIF(SUM(IF(t3 IS NOT NULL AND t3 >= t2, 1, 0)), 0) * 100, 2)
        AS step4_rate,
    ROUND(SUM(IF(t4 IS NOT NULL, 1, 0)) / COUNT(*) * 100, 2)         AS overall_conv_rate
FROM step_users;


-- ──────────────────────────────────────────────
-- 9.3 用户行为序列：还原指定用户的完整行为时间线（含会话切分）
--     用于客服/风控复盘，按 event_time 升序输出。
-- ──────────────────────────────────────────────
SELECT
    session_id,
    session_seq,
    event_time,
    event_name,
    event_action,
    object_type,
    object_name,
    platform,
    ip_city,
    duration_ms,
    amount,
    trace_id
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND user_id = ${USER_ID}
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
ORDER BY event_time, session_seq;


-- ──────────────────────────────────────────────
-- 9.4 单会话内事件序列（精确复盘某次会话，配合 9.3 使用）
-- ──────────────────────────────────────────────
SELECT
    session_seq,
    event_time,
    event_name,
    object_name,
    duration_ms
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND session_id = '${SESSION_ID}'
ORDER BY session_seq;


-- ──────────────────────────────────────────────
-- 9.5 商品/对象分析：Top 对象曝光与互动（按 object_type 分组）
--     推荐走预聚合视图 v_daily_object_analysis（来自 mv_objects_daily）；
--     如需对象价格/分类等维度字段，再 JOIN objects_dim。
-- ──────────────────────────────────────────────
SELECT
    o.object_type,
    o.object_id,
    o.object_name,
    MAX(d.category_path) AS category_path,
    MAX(d.price)         AS price,
    SUM(o.event_cnt)     AS event_cnt,
    HLL_CARDINALITY(HLL_UNION(o.uv)) AS uv,
    SUM(o.click_cnt)     AS click_cnt,
    SUM(o.purchase_cnt)  AS purchase_cnt,
    ROUND(SUM(o.click_cnt)    / NULLIF(SUM(o.event_cnt), 0) * 100, 2) AS click_rate,
    ROUND(SUM(o.purchase_cnt) / NULLIF(SUM(o.event_cnt), 0) * 100, 2) AS purchase_rate,
    ROUND(SUM(o.total_amount), 2) AS total_amount
FROM v_daily_object_analysis o
LEFT JOIN objects_dim d
       ON d.tenant_id = o.tenant_id
      AND d.object_id = o.object_id
      AND d.object_type = o.object_type
WHERE o.tenant_id = ${TENANT_ID}
  AND o.stat_date BETWEEN ${START} AND ${END}
GROUP BY o.object_type, o.object_id, o.object_name
ORDER BY event_cnt DESC
LIMIT 30;


-- ──────────────────────────────────────────────
-- 9.6 ID 打通：通过 id_mapping 把匿名设备用户归并到登录主体
--     场景：已知一个 device_id，查出所有关联的 global_user_id 及置信度。
-- ──────────────────────────────────────────────
SELECT
    id_type,
    id_value,
    global_user_id,
    confidence,
    link_source,
    first_seen,
    last_seen
FROM id_mapping
WHERE tenant_id = ${TENANT_ID}
  AND id_type = 'device_id'
  AND id_value = '${DEVICE_ID}'
  AND is_active = 1
ORDER BY confidence DESC, last_seen DESC;


-- ──────────────────────────────────────────────
-- 9.7 ID 打通：登录用户的所有设备/账号身份（反向查询，识别多设备同一人）
-- ──────────────────────────────────────────────
SELECT
    id_type,
    id_value,
    confidence,
    first_seen,
    last_seen
FROM id_mapping
WHERE tenant_id = ${TENANT_ID}
  AND global_user_id = '${GLOBAL_USER_ID}'
  AND is_active = 1
ORDER BY last_seen DESC;


-- ──────────────────────────────────────────────
-- 9.8 同比环比：日活/事件数环比昨日、同比上周同日
--     基于业务视图 v_daily_active_users，用 LAG 窗口函数一次性算出。
-- ──────────────────────────────────────────────
SELECT
    stat_date,
    dau,
    LAG(dau, 1) OVER (ORDER BY stat_date)                       AS dau_prev_day,
    ROUND((dau - LAG(dau, 1) OVER (ORDER BY stat_date))
          / NULLIF(LAG(dau, 1) OVER (ORDER BY stat_date), 0) * 100, 2) AS mom_pct,
    LAG(dau, 7) OVER (ORDER BY stat_date)                       AS dau_same_day_last_week,
    ROUND((dau - LAG(dau, 7) OVER (ORDER BY stat_date))
          / NULLIF(LAG(dau, 7) OVER (ORDER BY stat_date), 0) * 100, 2) AS yoy_pct
FROM v_daily_active_users
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY)
ORDER BY stat_date;


-- ──────────────────────────────────────────────
-- 9.9 自定义属性下钻：按 context 里的某个 key 分组统计
--     例：按埋点上报的自定义属性 'campaign_id' 拆分流量来源。
--     注意：SDK 上报的 properties 在 collector 转换后落入 events_fact.context 列
--           （见 report_service.go：behaviorEvent.Context = evt.Properties），
--           故此处用 context['campaign_id'] 取值，而非 properties 列。
-- ──────────────────────────────────────────────
SELECT
    context['campaign_id'] AS campaign_id,
    COUNT(*)                                              AS pv,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id))                    AS uv,
    ROUND(SUM(amount), 2)                                  AS total_amount
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
  AND context['campaign_id'] IS NOT NULL
  AND context['campaign_id'] != ''
GROUP BY context['campaign_id']
ORDER BY uv DESC
LIMIT 20;


-- ──────────────────────────────────────────────
-- 9.10 数值指标下钻：按 metrics['xxx'] 统计均值/分位（如游戏关卡得分）
--      metrics 为 MAP<STRING,DOUBLE>，常用于数值型埋点。
-- ──────────────────────────────────────────────
SELECT
    object_name AS level_name,
    COUNT(*)                                                          AS play_cnt,
    ROUND(AVG(metrics['score']), 2)                                    AS avg_score,
    ROUND(MIN(metrics['score']), 2)                                    AS min_score,
    ROUND(MAX(metrics['score']), 2)                                    AS max_score,
    ROUND(PERCENTILE_APPROX(metrics['score'], 0.5), 2)                 AS p50,
    ROUND(PERCENTILE_APPROX(metrics['score'], 0.9), 2)                 AS p90
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'level_finish'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
  AND metrics['score'] IS NOT NULL
GROUP BY object_name
ORDER BY play_cnt DESC
LIMIT 30;


-- ──────────────────────────────────────────────
-- 9.11 自定义属性展开：把 context MAP 展开成多行（探索性分析，无需预知 key）
--      SDK 上报的 properties 实际落入 context 列（见 9.9 说明），此处对 context 展开。
--      使用 Doris 的 explode/map_entries 把键值对炸开，便于统计各 key 分布。
-- ──────────────────────────────────────────────
SELECT
    p.kv_entry.1 AS pkey,
    p.kv_entry.2 AS pval,
    COUNT(*)     AS cnt
FROM events_fact,
LATERAL VIEW explode(map_entries(context)) p AS kv_entry
WHERE tenant_id = ${TENANT_ID}
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
  AND map_size(context) > 0
GROUP BY p.kv_entry.1, p.kv_entry.2
ORDER BY cnt DESC
LIMIT 50;


-- ──────────────────────────────────────────────
-- 9.12 新增用户来源渠道（基于 users_dim 注册渠道 + 首次活跃日期）
-- ──────────────────────────────────────────────
SELECT
    register_channel,
    first_active_date,
    COUNT(*) AS new_user_cnt
FROM users_dim
WHERE tenant_id = ${TENANT_ID}
  AND first_active_date BETWEEN ${START} AND ${END}
GROUP BY register_channel, first_active_date
ORDER BY first_active_date, new_user_cnt DESC;


-- ──────────────────────────────────────────────
-- 9.13 重合分析：同时命中多个标签的用户群（运营圈选交集）
--     例：同时是「高价值」且「流失风险」标签的用户。
-- ──────────────────────────────────────────────
SELECT a.user_id
FROM user_tags a
JOIN user_tags b
  ON a.tenant_id = b.tenant_id
 AND a.user_id   = b.user_id
WHERE a.tenant_id = ${TENANT_ID}
  AND a.is_active = 1 AND b.is_active = 1
  AND a.expire_date >= CURDATE() AND b.expire_date >= CURDATE()
  AND a.tag_id = ${TAG_ID_HIGH_VALUE}
  AND b.tag_id = ${TAG_ID_CHURN_RISK};


-- ──────────────────────────────────────────────
-- 9.14 设备指纹聚类：同一设备多账号（疑似账号共享/养号识别）
--     走预聚合视图 v_daily_device_summary，distinct_users 高即疑似养号。
-- ──────────────────────────────────────────────
SELECT
    stat_date,
    device_id,
    platform,
    distinct_users,
    session_count,
    event_cnt,
    first_seen,
    last_seen
FROM v_daily_device_summary
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
  AND distinct_users >= 2
ORDER BY distinct_users DESC, stat_date DESC
LIMIT 50;


-- ──────────────────────────────────────────────
-- 9.15 风险规则触发排行（风控规则调优）
--     视图 v_risk_rule_ranking 按 rule_id × status 分组，status 取值由业务定义。
-- 9.15a 规则总触发排行（跨 status 汇总，发现高频规则）
-- ──────────────────────────────────────────────
SELECT
    stat_date,
    rule_id,
    rule_name,
    risk_type,
    SUM(status_count)                    AS trigger_count,
    HLL_CARDINALITY(HLL_UNION(affected_users)) AS affected_users,
    ROUND(AVG(avg_risk_score), 2)        AS avg_risk_score
FROM v_risk_rule_ranking
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
GROUP BY stat_date, rule_id, rule_name, risk_type
ORDER BY trigger_count DESC
LIMIT 30;

-- 9.15b 规则处置状态分布（按 status 明细，使用方按实际状态值核对）
--       status 取值核对：SELECT DISTINCT status FROM risk_events WHERE tenant_id=${TENANT_ID};
SELECT
    stat_date,
    rule_id,
    rule_name,
    status,
    status_count,
    affected_users,
    avg_risk_score
FROM v_risk_rule_ranking
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY rule_id, status;


-- ──────────────────────────────────────────────
-- 9.16 支付分析：GMV、付费人数、ARPPU、ARPU
--     走业务视图 v_daily_payment_analysis，单日粒度。
-- ──────────────────────────────────────────────
SELECT
    stat_date,
    uv,
    total_amount AS gmv,
    pay_users,
    arppu,
    arpu,
    ROUND(IF(uv > 0, pay_users / uv * 100, 0), 2) AS pay_rate_pct
FROM v_daily_payment_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date;


-- ──────────────────────────────────────────────
-- 9.17 渠道版本分析：各投放渠道流量与金额对比
--     走业务视图 v_daily_channel_version_analysis。
-- ──────────────────────────────────────────────
SELECT
    stat_date,
    channel,
    app_version,
    platform,
    HLL_CARDINALITY(HLL_UNION(uv)) AS uv,
    SUM(pv)                         AS pv,
    SUM(session_count)              AS session_count,
    ROUND(SUM(total_amount), 2)     AS total_amount
FROM v_daily_channel_version_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
  AND channel IS NOT NULL
GROUP BY stat_date, channel, app_version, platform
ORDER BY uv DESC
LIMIT 30;


-- ──────────────────────────────────────────────
-- 9.18 用户活跃汇总：首末次活跃、活跃天数、累计事件/会话
--     走业务视图 v_user_active_summary，配合 users_dim 可补全画像字段。
-- ──────────────────────────────────────────────
SELECT
    a.user_id,
    a.first_active_time,
    a.last_active_time,
    a.active_days,
    a.total_events,
    a.total_sessions,
    u.user_level,
    u.vip_level
FROM v_user_active_summary a
LEFT JOIN users_dim u
       ON u.tenant_id = a.tenant_id
      AND u.user_id   = a.user_id
WHERE a.tenant_id = ${TENANT_ID}
ORDER BY a.total_events DESC
LIMIT 50;


-- ============================================================
-- 十一、游戏场景查询组
-- 埋点约定（事件名，使用前请核对实际埋点）：
--   game_start    进入游戏      level_start    关卡开始
--   level_finish  关卡完成      level_fail     关卡失败
--   item_buy      道具购买      coin_consume   金币消耗
--   vip_buy       开通/续费VIP  tutorial_step  新手引导步骤
-- 对象约定：object_type='level'(关卡) / 'item'(道具) / 'hero'(英雄)
--           object_id = 关卡ID/道具ID/英雄ID
-- 指标约定：metrics['score'] 分数, context['result'] 胜负(win/lose)
-- 注意：SDK 上报的自定义属性(properties)在 collector 转换后落入 events_fact.context 列，
--       故自定义属性取值统一用 context['key']，而非 properties 列。
-- ============================================================

-- 11.1 关卡完成漏斗：开始 → 完成 → 满星 → 分享（占位符可替换）
WITH s AS (
    SELECT user_id,
           MIN(IF(event_name = 'level_start',  event_time, NULL)) AS t1,
           MIN(IF(event_name = 'level_finish', event_time, NULL)) AS t2,
           MIN(IF(event_name = 'level_finish' AND context['stars']='3', event_time, NULL)) AS t3,
           MIN(IF(event_name = 'level_share',  event_time, NULL)) AS t4
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
    GROUP BY user_id
    HAVING t1 IS NOT NULL
)
SELECT COUNT(*) AS start_users,
       SUM(IF(t2 IS NOT NULL AND t2 >= t1, 1, 0)) AS finish_users,
       SUM(IF(t3 IS NOT NULL AND t3 >= t2, 1, 0)) AS perfect_users,
       SUM(IF(t4 IS NOT NULL AND t4 >= t2, 1, 0)) AS share_users
FROM s;

-- 11.2 关卡难度分析：各关卡通过率、失败率、平均重试次数
SELECT
    object_id                                  AS level_id,
    MAX(object_name)                           AS level_name,
    SUM(IF(event_name = 'level_start',  1, 0)) AS attempt_cnt,
    SUM(IF(event_name = 'level_finish', 1, 0)) AS finish_cnt,
    SUM(IF(event_name = 'level_fail',   1, 0)) AS fail_cnt,
    ROUND(SUM(IF(event_name = 'level_finish', 1, 0))
          / NULLIF(SUM(IF(event_name IN ('level_finish','level_fail'), 1, 0)), 0) * 100, 2) AS pass_rate_pct,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id))         AS player_cnt
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND object_type = 'level'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY object_id
ORDER BY attempt_cnt DESC
LIMIT 50;

-- 11.3 关卡分数分布（均值/分位/最高分）
SELECT
    object_id AS level_id,
    COUNT(*) AS finish_cnt,
    ROUND(AVG(metrics['score']), 1)                          AS avg_score,
    ROUND(PERCENTILE_APPROX(metrics['score'], 0.5), 1)       AS p50,
    ROUND(PERCENTILE_APPROX(metrics['score'], 0.9), 1)       AS p90,
    ROUND(MAX(metrics['score']), 1)                          AS max_score,
    SUM(IF(context['stars'] = '3', 1, 0))                 AS star3_cnt
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'level_finish'
  AND object_type = 'level'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY object_id
ORDER BY finish_cnt DESC
LIMIT 50;

-- 11.4 道具销量 Top N（基于 v_daily_object_analysis，含购买率）
SELECT
    object_type,
    object_id,
    object_name,
    SUM(purchase_cnt) AS buy_cnt,
    SUM(event_cnt)    AS interact_cnt,
    HLL_CARDINALITY(HLL_UNION(uv)) AS buyer_uv,
    ROUND(SUM(total_amount), 2) AS revenue
FROM v_daily_object_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
  AND object_type = 'item'
GROUP BY object_type, object_id, object_name
ORDER BY revenue DESC
LIMIT 30;

-- 11.5 付费分析：付费人数、ARPPU、付费率、大R占比（流水贡献度）
SELECT
    stat_date,
    pay_users,
    amount AS revenue,
    arppu,
    ROUND(IF(uv > 0, pay_users / uv * 100, 0), 2) AS pay_rate_pct
FROM v_daily_payment_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date;

-- 11.6 游戏时长分析：人均在线时长、时长分布（基于 sessions_fact）
SELECT
    platform,
    COUNT(*) AS session_cnt,
    ROUND(AVG(duration_ms) / 1000 / 60, 2)                 AS avg_session_min,
    ROUND(PERCENTILE_APPROX(duration_ms, 0.5) / 1000 / 60, 2) AS p50_min,
    ROUND(PERCENTILE_APPROX(duration_ms, 0.9) / 1000 / 60, 2) AS p90_min
FROM sessions_fact
WHERE tenant_id = ${TENANT_ID}
  AND session_date BETWEEN ${START} AND ${END}
GROUP BY platform;

-- 11.7 新手引导漏斗转化（教程各步骤流失）
SELECT
    context['tutorial_step'] AS step,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id)) AS reach_users,
    COUNT(*) AS event_cnt
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'tutorial_step'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY context['tutorial_step']
ORDER BY CAST(context['tutorial_step'] AS INT);

-- 11.8 首日留存（次留）- 游戏冷启动核心指标
SELECT
    first_date,
    MAX(IF(retention_day = 0, retained_users, 0)) AS d0,
    MAX(IF(retention_day = 1, retained_users, 0)) AS d1,
    ROUND(MAX(IF(retention_day = 1, retained_users, 0))
          / NULLIF(MAX(IF(retention_day = 0, retained_users, 0)), 0) * 100, 2) AS d1_retention_pct
FROM mv_user_retention
WHERE tenant_id = ${TENANT_ID}
  AND first_date BETWEEN ${START} AND ${END}
GROUP BY first_date
ORDER BY first_date;


-- ============================================================
-- 十二、电商场景查询组
-- 埋点约定（事件名，使用前请核对实际埋点）：
--   page_view     商品浏览     search        搜索
--   add_to_cart   加入购物车   place_order   提交订单
--   pay           支付成功     refund        退款
--   add_fav       收藏         share         分享商品
-- 对象约定：object_type='product'(商品) / 'shop'(店铺) / 'category'(类目)
--           object_id = 商品ID/店铺ID，context 含 sku_id/brand
-- 指标约定：amount 金额, quantity 数量, context['order_id'] 订单号
-- ============================================================

-- 12.1 购买漏斗：浏览 → 加购 → 下单 → 支付（电商核心转化）
WITH s AS (
    SELECT user_id,
           MIN(IF(event_name = 'page_view',   event_time, NULL)) AS t1,
           MIN(IF(event_name = 'add_to_cart', event_time, NULL)) AS t2,
           MIN(IF(event_name = 'place_order', event_time, NULL)) AS t3,
           MIN(IF(event_name = 'pay',         event_time, NULL)) AS t4
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
    GROUP BY user_id
    HAVING t1 IS NOT NULL
)
SELECT
    COUNT(*) AS view_users,
    SUM(IF(t2 IS NOT NULL AND t2 >= t1, 1, 0)) AS cart_users,
    SUM(IF(t3 IS NOT NULL AND t3 >= t2, 1, 0)) AS order_users,
    SUM(IF(t4 IS NOT NULL AND t4 >= t3, 1, 0)) AS pay_users,
    ROUND(SUM(IF(t4 IS NOT NULL AND t4 >= t3, 1, 0)) / COUNT(*) * 100, 2) AS overall_conv_pct
FROM s;

-- 12.2 商品销量与转化 Top N（浏览→加购→购买联动）
SELECT
    o.object_id,
    o.object_name,
    SUM(o.event_cnt)                                        AS view_cnt,
    SUM(o.click_cnt)                                        AS interact_cnt,
    SUM(o.purchase_cnt)                                     AS buy_cnt,
    HLL_CARDINALITY(HLL_UNION(o.uv))                        AS uv,
    ROUND(SUM(o.total_amount), 2)                           AS gmv,
    ROUND(SUM(o.purchase_cnt) / NULLIF(SUM(o.event_cnt), 0) * 100, 2) AS view_to_buy_pct
FROM v_daily_object_analysis o
WHERE o.tenant_id = ${TENANT_ID}
  AND o.stat_date BETWEEN ${START} AND ${END}
  AND o.object_type = 'product'
GROUP BY o.object_id, o.object_name
ORDER BY gmv DESC
LIMIT 30;

-- 12.3 GMV 与客单价（基于支付分析视图）
SELECT
    stat_date,
    amount AS gmv,
    pay_users,
    ROUND(amount / NULLIF(pay_users, 0), 2) AS avg_order_value,
    arppu,
    arpu
FROM v_daily_payment_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
ORDER BY stat_date;

-- 12.4 渠道 ROI 对比（各渠道 GMV 与付费用户）
SELECT
    channel,
    HLL_CARDINALITY(HLL_UNION(uv)) AS uv,
    SUM(pv)                         AS pv,
    ROUND(SUM(total_amount), 2)     AS gmv,
    ROUND(SUM(total_amount) / NULLIF(HLL_CARDINALITY(HLL_UNION(uv)), 0), 2) AS arpu
FROM v_daily_channel_version_analysis
WHERE tenant_id = ${TENANT_ID}
  AND stat_date BETWEEN ${START} AND ${END}
  AND channel IS NOT NULL
GROUP BY channel
ORDER BY gmv DESC
LIMIT 20;

-- 12.5 复购分析：周期内购买次数分布（1次/2-3次/4次以上）
WITH pay_users AS (
    SELECT user_id, COUNT(*) AS pay_times, SUM(amount) AS total_amount
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_name = 'pay'
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
    GROUP BY user_id
)
SELECT
    CASE
        WHEN pay_times = 1             THEN '1_time'
        WHEN pay_times BETWEEN 2 AND 3 THEN '2_3_times'
        WHEN pay_times >= 4            THEN '4_plus'
    END AS repurchase_level,
    COUNT(*) AS user_cnt,
    ROUND(SUM(total_amount), 2) AS gmv
FROM pay_users
GROUP BY repurchase_level
ORDER BY user_cnt DESC;

-- 12.6 退款监控（分支付/退款两条独立聚合，再按日比对，避免事实表自连接）
-- 12.6a 支付侧：每日 GMV、支付订单数
SELECT
    to_date(event_time) AS stat_date,
    ROUND(SUM(amount), 2) AS pay_amount,
    COUNT(DISTINCT context['order_id']) AS pay_orders
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'pay'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY stat_date
ORDER BY stat_date;

-- 12.6b 退款侧：每日退款金额、退款订单数（与 12.6a 结果按 stat_date 对照算退款率）
SELECT
    to_date(event_time) AS stat_date,
    ROUND(SUM(amount), 2) AS refund_amount,
    COUNT(DISTINCT context['order_id']) AS refund_orders
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'refund'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY stat_date
ORDER BY stat_date;

-- 12.7 加购流失分析：各商品的加购用户数 vs 下单用户数（分别聚合再对比，避免自连接）
-- 12.7a 加购用户（按商品）
SELECT
    object_id,
    MAX(object_name) AS product_name,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id)) AS cart_users
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'add_to_cart'
  AND object_type = 'product'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY object_id
ORDER BY cart_users DESC
LIMIT 30;

-- 12.7b 下单用户（按商品，与 12.7a 同维度对照，cart_users - order_users 即流失用户估算）
SELECT
    object_id,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id)) AS order_users
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'place_order'
  AND object_type = 'product'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY object_id
ORDER BY order_users DESC
LIMIT 30;

-- 12.8 搜索热词与点击率（搜索→点击商品）
SELECT
    context['keyword'] AS keyword,
    COUNT(*) AS search_cnt,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id)) AS searcher_uv,
    SUM(IF(event_action = 'click', 1, 0)) AS click_cnt,
    ROUND(SUM(IF(event_action = 'click', 1, 0)) / COUNT(*) * 100, 2) AS ctr_pct
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = 'search'
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY context['keyword']
ORDER BY search_cnt DESC
LIMIT 30;


-- ============================================================
-- 十三、事件分布与异常检测（运营日报核心）
-- ============================================================

-- 13.1 各事件 PV/UV 同比环比（日报核心，用 LAG 算环比涨跌）
SELECT
    event_name,
    stat_date,
    pv,
    HLL_CARDINALITY(uv) AS uv,
    LAG(pv, 1) OVER (PARTITION BY event_name ORDER BY stat_date) AS pv_prev_day,
    ROUND((pv - LAG(pv, 1) OVER (PARTITION BY event_name ORDER BY stat_date))
          / NULLIF(LAG(pv, 1) OVER (PARTITION BY event_name ORDER BY stat_date), 0) * 100, 2) AS pv_mom_pct
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
ORDER BY stat_date DESC, pv DESC;

-- 13.2 事件波动 Top N（昨日环比涨跌幅度最大的事件，突增/暴跌预警）
WITH today AS (
    SELECT event_name, pv, HLL_CARDINALITY(uv) AS uv
    FROM mv_events_daily
    WHERE tenant_id = ${TENANT_ID} AND stat_date = CURDATE()
),
yest AS (
    SELECT event_name, pv AS pv_prev
    FROM mv_events_daily
    WHERE tenant_id = ${TENANT_ID} AND stat_date = DATE_SUB(CURDATE(), INTERVAL 1 DAY)
)
SELECT
    t.event_name,
    t.pv,
    y.pv_prev,
    ROUND((t.pv - y.pv_prev) / NULLIF(y.pv_prev, 0) * 100, 2) AS change_pct
FROM today t
LEFT JOIN yest y ON t.event_name = y.event_name
WHERE y.pv_prev > 0
ORDER BY ABS((t.pv - y.pv_prev) / NULLIF(y.pv_prev, 0)) DESC
LIMIT 20;

-- 13.3 单事件 7 日趋势 + 7 日均值（基线对比，判断是否异常）
SELECT
    stat_date,
    pv,
    HLL_CARDINALITY(uv) AS uv,
    ROUND(AVG(pv) OVER (ORDER BY stat_date ROWS BETWEEN 6 PRECEDING AND 1 PRECEDING), 0) AS pv_baseline_7d
FROM mv_events_daily
WHERE tenant_id = ${TENANT_ID}
  AND event_name = '${EVENT_NAME}'
  AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 14 DAY)
ORDER BY stat_date;

-- 13.4 事件粒度异常（当日 PV 显著低于 7 日均值的 50%，疑似埋点丢失/故障）
WITH recent AS (
    SELECT event_name,
           SUM(IF(stat_date = CURDATE(), pv, 0))                            AS today_pv,
           ROUND(AVG(IF(stat_date < CURDATE(), pv, NULL)), 0)               AS avg_pv_6d
    FROM mv_events_daily
    WHERE tenant_id = ${TENANT_ID}
      AND stat_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
    GROUP BY event_name
)
SELECT event_name, today_pv, avg_pv_6d,
       ROUND(today_pv / NULLIF(avg_pv_6d, 0) * 100, 2) AS pct_of_baseline
FROM recent
WHERE avg_pv_6d > 0
  AND today_pv < avg_pv_6d * 0.5    -- 低于基线50%判定为异常
ORDER BY pct_of_baseline ASC;


-- ============================================================
-- 十四、事件时长分布分析（单事件耗时，如页面停留/视频播放）
-- events_fact.duration_ms 记录事件持续时长，单位毫秒。
-- ============================================================

-- 14.1 单事件时长分布（均值/分位/最大值）
SELECT
    event_name,
    COUNT(*) AS cnt,
    ROUND(AVG(duration_ms) / 1000, 2)                                 AS avg_sec,
    ROUND(PERCENTILE_APPROX(duration_ms, 0.5) / 1000, 2)              AS p50_sec,
    ROUND(PERCENTILE_APPROX(duration_ms, 0.9) / 1000, 2)              AS p90_sec,
    ROUND(MAX(duration_ms) / 1000, 2)                                 AS max_sec
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = '${EVENT_NAME}'
  AND duration_ms > 0
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY event_name;

-- 14.2 时长分桶分布（0-10s / 10-60s / 1-5min / 5min+ 的占比）
SELECT
    CASE
        WHEN duration_ms < 10000       THEN '0_10s'
        WHEN duration_ms < 60000       THEN '10_60s'
        WHEN duration_ms < 300000      THEN '1_5min'
        ELSE '5min_plus'
    END AS duration_bucket,
    COUNT(*) AS cnt,
    ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER (), 2) AS pct
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = '${EVENT_NAME}'
  AND duration_ms > 0
  AND event_time >= '${START} 00:00:00'
  AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY duration_bucket
ORDER BY duration_bucket;


-- ============================================================
-- 十五、新老用户对比分析
-- 判定：以 users_dim.first_active_date 为准，N 天内活跃=新用户，否则=老用户。
-- 无 users_dim 时可退化为基于 events_fact 首次出现时间判定。
-- ============================================================

-- 15.1 新老用户构成（活跃用户中新/老占比）
SELECT
    IF(u.first_active_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY), 'new_7d', 'old') AS user_type,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(e.user_id)) AS uv,
    COUNT(*) AS event_cnt
FROM events_fact e
JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.tenant_id = ${TENANT_ID}
  AND e.event_time >= '${START} 00:00:00'
  AND e.event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY user_type;

-- 15.2 新老用户行为差异（各事件在新/老用户中的触发占比）
SELECT
    e.event_name,
    ROUND(SUM(IF(u.first_active_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY), 1, 0))
          / COUNT(*) * 100, 2) AS new_user_pct,
    COUNT(*) AS total_cnt
FROM events_fact e
JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.tenant_id = ${TENANT_ID}
  AND e.event_time >= '${START} 00:00:00'
  AND e.event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY e.event_name
ORDER BY new_user_pct DESC
LIMIT 20;

-- 15.3 新老用户付费对比（新用户付费率 vs 老用户付费率）
SELECT
    IF(u.first_active_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY), 'new_7d', 'old') AS user_type,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(e.user_id))                          AS active_uv,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(IF(e.amount > 0, e.user_id, NULL)))   AS pay_uv,
    ROUND(SUM(e.amount), 2)                                        AS gmv,
    ROUND(HLL_CARDINALITY(HLL_UNION(HLL_HASH(IF(e.amount > 0, e.user_id, NULL)))
          / NULLIF(HLL_CARDINALITY(HLL_UNION(HLL_HASH(e.user_id)), 0) * 100, 2) AS pay_rate_pct
FROM events_fact e
JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.tenant_id = ${TENANT_ID}
  AND e.event_time >= '${START} 00:00:00'
  AND e.event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY user_type;


-- ============================================================
-- 十六、行为归因分析（首触/末触归因）
-- 场景：转化事件（如支付）发生前，用户从哪个渠道/页面首次/最后触达。
-- 字段：events_fact.channel 渠道、referer 来源页、amount 金额。
-- ============================================================

-- 16.1 末次触达归因：转化用户的最后来源渠道分布
WITH converters AS (
    SELECT DISTINCT user_id
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_name = '${CONVERSION_EVENT}'   -- 如 'pay'
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
)
SELECT
    e.channel,
    HLL_CARDINALITY(HLL_UNION(HLL_HASH(e.user_id)) AS converter_uv
FROM events_fact e
JOIN converters c ON e.user_id = c.user_id
WHERE e.tenant_id = ${TENANT_ID}
  AND e.event_time >= '${START} 00:00:00'
  AND e.event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
GROUP BY e.channel
ORDER BY converter_uv DESC
LIMIT 20;

-- 16.2 首次触达归因：转化用户的首次来源（更早的渠道贡献）
SELECT
    first_touch.channel,
    COUNT(*) AS converter_cnt
FROM (
    SELECT user_id,
           channel,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY event_time) AS rn
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
      AND user_id IN (
          SELECT DISTINCT user_id FROM events_fact
          WHERE tenant_id = ${TENANT_ID} AND event_name = '${CONVERSION_EVENT}'
      )
) first_touch
WHERE rn = 1
GROUP BY channel
ORDER BY converter_cnt DESC
LIMIT 20;

-- 16.3 来源页面归因：转化前最后访问的页面分布（末次触达页面）
SELECT
    last_page.referer AS last_source_page,
    COUNT(*) AS converter_cnt
FROM (
    SELECT user_id, referer,
           ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY event_time DESC) AS rn
    FROM events_fact
    WHERE tenant_id = ${TENANT_ID}
      AND event_time >= '${START} 00:00:00'
      AND event_time <  DATE_ADD('${END}', INTERVAL 1 DAY)
      AND user_id IN (
          SELECT DISTINCT user_id FROM events_fact
          WHERE tenant_id = ${TENANT_ID} AND event_name = '${CONVERSION_EVENT}'
      )
) last_page
WHERE rn = 1
GROUP BY last_source_page
ORDER BY converter_cnt DESC
LIMIT 20;


-- ============================================================
-- 十七、行为圈选 SQL（运营/产品直接用）
-- 语法约定：用 LEFT SEMI JOIN / NOT EXISTS 表达"做过/没做过"。
-- 占位符：${EVENT_DO} 想要的行为, ${EVENT_NOT} 排除的行为。
-- ============================================================

-- 17.1 圈选：近 7 天做过某事件 ${EVENT_DO} 的用户
SELECT DISTINCT user_id
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = '${EVENT_DO}'
  AND event_time >= DATE_SUB(CURDATE(), INTERVAL 7 DAY);

-- 17.2 圈选：做过 ${EVENT_DO} 但没做过 ${EVENT_NOT} 的用户（高潜未转化人群）
SELECT DISTINCT a.user_id
FROM events_fact a
WHERE a.tenant_id = ${TENANT_ID}
  AND a.event_name = '${EVENT_DO}'
  AND a.event_time >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
  AND NOT EXISTS (
      SELECT 1 FROM events_fact b
      WHERE b.tenant_id = a.tenant_id
        AND b.user_id = a.user_id
        AND b.event_name = '${EVENT_NOT}'
        AND b.event_time >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
  )
LIMIT 5000;

-- 17.3 圈选：做过 ${EVENT_DO} 且达到 ${MIN_TIMES} 次的高频用户
SELECT user_id
FROM events_fact
WHERE tenant_id = ${TENANT_ID}
  AND event_name = '${EVENT_DO}'
  AND event_time >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
GROUP BY user_id
HAVING COUNT(*) >= ${MIN_TIMES}
ORDER BY COUNT(*) DESC
LIMIT 5000;

-- 17.4 组合圈选：7天内浏览 ${EVENT_DO} + 当日支付的精准转化人群
SELECT a.user_id
FROM events_fact a
JOIN events_fact b
  ON b.tenant_id = a.tenant_id AND b.user_id = a.user_id
WHERE a.tenant_id = ${TENANT_ID}
  AND a.event_name = '${EVENT_DO}'
  AND a.event_time >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
  AND b.event_name = '${CONVERSION_EVENT}'
  AND b.event_time >= CURDATE()
GROUP BY a.user_id
LIMIT 5000;


-- ============================================================
-- 十八、附录：常用 Doris 函数速查
-- ============================================================
-- HLL 去重：       HLL_HASH(col) -> HLL_UNION(hll) -> HLL_CARDINALITY(hll)
-- 分位（聚合表）：  QUANTILE_STATE 列需用 QUANTILE_PERCENT(col, p) 还原
-- 分位（明细表）：  PERCENTILE_APPROX(col, p) 近似分位，p∈[0,1]
-- 窗口函数：        LAG(col, n) / LEAD(col, n) / ROW_NUMBER() OVER(PARTITION BY.. ORDER BY..)
--                    AVG() OVER(ROWS BETWEEN n PRECEDING AND ...) 移动均值
-- 时间分区裁剪：    事实表按 event_time/event_date 分区，WHERE 必带该列
-- 倒排索引：        events_fact 已建 user_id/device_id/event_name 等 INVERTED INDEX，
--                  等值/IN 查询自动命中，无需特殊语法
-- Map 字段取值：    context['key'] / element_at(context, 'key') / map_size(map)
--                  注意：SDK 上报的 properties 实际落入 events_fact.context 列
--                  （见 report_service.go：behaviorEvent.Context = evt.Properties），
--                  故自定义属性取值用 context，而非 properties 列。
-- Map 展开：        map_entries(map) 配合 LATERAL VIEW explode(...) 炸开成多行
-- 数组操作：        array_join(arr, ',') / array_contains(arr, 'x') / array_size(arr)
-- 时间函数：        DATE_SUB(d, INTERVAL n DAY) / DATE_ADD(d, INTERVAL n DAY) / date_trunc('hour', ts)
-- 子查询 EXISTS：   NOT EXISTS (SELECT 1 FROM ... WHERE ...) 表达"排除/未做过"语义

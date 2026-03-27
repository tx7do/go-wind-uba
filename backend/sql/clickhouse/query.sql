-- 1. 查询事件明细
SELECT *
FROM gw_uba.events_fact
WHERE tenant_id = 1
ORDER BY event_time DESC
LIMIT 100;

-- 2. 查询会话明细
SELECT *
FROM gw_uba.sessions_fact
WHERE tenant_id = 1
ORDER BY start_time DESC
LIMIT 100;

-- 3. 查询用户画像
SELECT *
FROM gw_uba.users_dim
WHERE tenant_id = 1
ORDER BY last_active_date DESC
LIMIT 100;

-- 4. 查询用户行为路径
SELECT *
FROM gw_uba.path_features
WHERE tenant_id = 1
ORDER BY start_time DESC
LIMIT 100;

-- 5. 查询用户标签
SELECT *
FROM gw_uba.user_tags
WHERE tenant_id = 1
  AND is_active = 1
ORDER BY updated_at DESC
LIMIT 100;


-- 6. 每日 PV/UV 统计
SELECT *
FROM gw_uba.events_agg_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY stat_date DESC;

-- 7. 每日活跃用户 DAU
SELECT *
FROM gw_uba.user_activity_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY stat_date DESC;

-- 8. 付费数据统计
SELECT *
FROM gw_uba.pay_agg_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 30
ORDER BY stat_date DESC;

-- 9. 用户留存
SELECT *
FROM gw_uba.user_retention_daily_view
WHERE tenant_id = 1
ORDER BY register_date DESC, retention_days ASC
LIMIT 100;

-- 10. 热门行为路径
SELECT *
FROM gw_uba.popular_paths_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY support_count DESC;

-- 11. 漏斗转化
SELECT *
FROM gw_uba.funnel_steps_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY funnel_id, step_index;

-- 12. 页面访问热度
SELECT *
FROM gw_uba.page_visit_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY pv DESC;

-- 13. 风险用户统计
SELECT *
FROM gw_uba.risk_stats_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY total_event_count DESC;

-- 14. 用户标签统计（圈人）
SELECT *
FROM gw_uba.user_tags_agg_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY user_count DESC;


-- 15. 按平台统计日活
SELECT stat_date,
       platform,
       sum(active_users) AS dau
FROM gw_uba.user_activity_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY stat_date, platform
ORDER BY stat_date, platform;

-- 16. 按国家 / 地区统计用户
SELECT country,
       uniqExact(user_id) AS user_count
FROM gw_uba.users_dim
WHERE tenant_id = 1
GROUP BY country
ORDER BY user_count DESC;

-- 17. 高价值付费用户
SELECT *
FROM gw_uba.users_dim
WHERE tenant_id = 1
  AND total_pay_amount > 0
ORDER BY total_pay_amount DESC
LIMIT 100;

-- 18. 最近 7 日转化路径
SELECT event_sequence,
       sum(support_count)   AS cnt,
       avg(conversion_rate) AS cvr
FROM gw_uba.popular_paths_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
  AND total_conversion_sum > 0
GROUP BY event_sequence
ORDER BY cnt DESC
LIMIT 20;


-- 总事件量校验
SELECT count(*) total_events, min(event_time), max(event_time)
FROM gw_uba.events_fact
WHERE tenant_id = 1;

-- 会话总数
SELECT count(*) total_sessions
FROM gw_uba.sessions_fact
WHERE tenant_id = 1;

-- 独立玩家基数
SELECT uniqExact(user_id) total_player
FROM gw_uba.users_dim
WHERE tenant_id = 1;

-- 日活+付费人数+事件总览 近7天
SELECT stat_date,
       sum(active_users) AS dau,
       sum(pay_users)    AS pay_dau,
       sum(total_events) AS total_behavior
FROM gw_uba.user_activity_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY stat_date
ORDER BY stat_date;

-- 收入核心指标（ARPU / ARPPU / 付费率）
SELECT stat_date,
       grand_total_amount,
       total_pay_user_count,
       if(total_pay_user_count > 0, grand_total_amount / total_pay_user_count, 0) AS arppu,
       if(dau > 0, grand_total_amount / dau, 0)                                   AS arpu,
       if(dau > 0, total_pay_user_count / dau, 0)                                 AS pay_rate
FROM (
         SELECT stat_date,
                sum(grand_total_amount)   grand_total_amount,
                sum(total_pay_user_count) total_pay_user_count,
                sum(active_users)         dau
         FROM gw_uba.pay_agg_daily_view t1
                  LEFT JOIN gw_uba.user_activity_daily_view t2
                            ON t1.tenant_id = t2.tenant_id AND t1.stat_date = t2.stat_date
         WHERE t1.tenant_id = 1
           AND t1.stat_date >= today() - 15
         GROUP BY stat_date
         ) tmp
ORDER BY stat_date;


-- 新玩家留存细分（次日 / 7 日标准游戏留存）
SELECT register_date,
       retention_days,
       sum(register_users) new_user_cnt,
       sum(retained_users) retain_cnt,
       avg(retention_rate) retention_avg
FROM gw_uba.user_retention_daily_view
WHERE tenant_id = 1
  AND retention_days IN (1, 7)
  AND register_date >= today() - 30
GROUP BY register_date, retention_days
ORDER BY register_date, retention_days;

-- 新手引导漏斗（游戏刚需：注册→创角→新手→付费）
SELECT stat_date,
       funnel_id,
       step_index,
       step_name,
       enter_users,
       complete_users,
       conversion_rate
FROM gw_uba.funnel_steps_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
ORDER BY stat_date, step_index;

-- 会话质量：平均在线时长 + 跳出率
SELECT stat_date,
       sum(unique_users) player_cnt,
       avg(avg_duration) avg_online_ms,
       avg(bounce_rate)  bounce_ratio
FROM gw_uba.sessions_agg_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY stat_date
ORDER BY stat_date;

-- 高危作弊 / 异常账号监控（风控日报）
SELECT stat_date,
       risk_type,
       risk_level,
       total_event_count,
       unique_users,
       confirm_rate,
       avg_risk_score
FROM gw_uba.risk_stats_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
  AND risk_level IN ('high', 'critical')
ORDER BY total_event_count DESC;

-- 高价值玩家榜单（大额充值 + 高风险交叉筛查）
SELECT user_id,
       total_pay_amount,
       risk_level,
       last_active_date
FROM gw_uba.users_dim
WHERE tenant_id = 1
  AND total_pay_amount > 0
ORDER BY total_pay_amount DESC
LIMIT 50;

-- 页面 / 游戏节点热度（卡点流失分析）
SELECT page_id,
       sum(pv)           total_pv,
       sum(uv)           uv,
       avg(avg_duration) avg_stay_ms
FROM gw_uba.page_visit_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY page_id
ORDER BY total_pv DESC;

-- 玩家行为路径 TOP（只看有转化付费路径）
SELECT event_sequence,
       sum(support_count)   flow_cnt,
       avg(conversion_rate) cvr
FROM gw_uba.popular_paths_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY event_sequence
HAVING sum(total_conversion_sum) > 0
ORDER BY flow_cnt DESC
LIMIT 30;

-- 小时级实时趋势（盯大盘波动）
SELECT stat_hour,
       event_name,
       sum(pv) AS pv,
       sum(uv) AS uv
FROM gw_uba.events_agg_hourly_view
WHERE tenant_id = 1
  AND stat_hour >= now() - INTERVAL 24 HOUR
GROUP BY stat_hour, event_name
ORDER BY stat_hour;


-- ==============================================
-- UBA大数据可视化大屏
-- ==============================================

-- 大屏：整体大盘总览
SELECT stat_date,
       sum(active_users)       AS dau,
       sum(pay_users)          AS pay_dau,
       sum(total_events)       AS event_cnt,
       sum(grand_total_amount) AS revenue
FROM gw_uba.user_activity_daily_view
         LEFT JOIN gw_uba.pay_agg_daily_view
                   USING (tenant_id, stat_date)
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY stat_date
ORDER BY stat_date;

-- 大屏：收入核心看板
SELECT stat_date,
       round(sum(grand_total_amount), 2)                             AS daily_revenue,
       sum(total_pay_user_count)                                     AS pay_user_cnt,
       sum(active_users)                                             AS dau,
       round(sum(grand_total_amount) / sum(total_pay_user_count), 2) AS arppu,
       round(sum(grand_total_amount) / sum(active_users), 2)         AS arpu,
       round(sum(total_pay_user_count) / sum(active_users), 3)       AS pay_rate
FROM gw_uba.pay_agg_daily_view
         LEFT JOIN gw_uba.user_activity_daily_view
                   USING (tenant_id, stat_date)
WHERE tenant_id = 1
  AND stat_date >= today() - 15
GROUP BY stat_date
ORDER BY stat_date;

-- 大屏：新玩家留存趋势
SELECT register_date,
       retention_days,
       sum(register_users)           AS new_users,
       sum(retained_users)           AS retained_users,
       round(avg(retention_rate), 3) AS retention_rate
FROM gw_uba.user_retention_daily_view
WHERE tenant_id = 1
  AND retention_days IN (1, 7)
  AND register_date >= today() - 30
GROUP BY register_date, retention_days
ORDER BY register_date, retention_days;

-- 大屏：玩家在线质量
SELECT stat_date,
       sum(session_count)                 AS total_sessions,
       round(avg(avg_duration) / 1000, 1) AS avg_online_sec,
       round(avg(bounce_rate), 2)         AS bounce_rate
FROM gw_uba.sessions_agg_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY stat_date
ORDER BY stat_date;

-- 大屏：游戏场景/页面热度TOP20
SELECT page_id,
       sum(pv)                            AS pv,
       sum(uv)                            AS uv,
       round(avg(avg_duration) / 1000, 1) AS stay_seconds
FROM gw_uba.page_visit_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY page_id
ORDER BY pv DESC
LIMIT 20;

-- 大屏：游戏场景/页面热度TOP20
SELECT page_id,
       sum(pv)                            AS pv,
       sum(uv)                            AS uv,
       round(avg(avg_duration) / 1000, 1) AS stay_seconds
FROM gw_uba.page_visit_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY page_id
ORDER BY pv DESC
LIMIT 20;

-- 大屏：玩家行为路径TOP
SELECT event_sequence,
       sum(support_count)             AS path_count,
       round(avg(conversion_rate), 3) AS cvr
FROM gw_uba.popular_paths_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
GROUP BY event_sequence
HAVING sum(total_conversion_sum) > 0
ORDER BY path_count DESC
LIMIT 20;

-- 大屏：实时小时级波动
SELECT stat_hour,
       sum(pv) AS event_count,
       sum(uv) AS user_count
FROM gw_uba.events_agg_hourly_view
WHERE tenant_id = 1
  AND stat_hour >= now() - INTERVAL 24 HOUR
GROUP BY stat_hour
ORDER BY stat_hour;

-- 大屏：风控异常监控
SELECT stat_date,
       risk_level,
       sum(total_event_count) AS risk_events,
       sum(unique_users)      AS risk_users
FROM gw_uba.risk_stats_daily_view
WHERE tenant_id = 1
  AND stat_date >= today() - 7
  AND risk_level IN ('high', 'critical')
GROUP BY stat_date, risk_level
ORDER BY stat_date;

package clickhouse

import (
	"context"
	"fmt"
	"time"

	"go-wind-uba/app/core/service/internal/data"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// flagVariantStatRow 实验结果扫描行。
type flagVariantStatRow struct {
	Variant        string `db:"variant"`
	ExposedUsers   int64  `db:"exposed_users"`
	ConvertedUsers int64  `db:"converted_users"`
}

// FlagExperimentResult 聚合 A/B 实验结果：以 $feature_flag 曝光事件圈定各变体的
// 登录用户集，统计每个变体里触发过目标事件的去重用户数。
// appFilter 过滤 events_fact.tenant_id（0 表示不过滤）。
// 与 doris.AnalyticsRepo.FlagExperimentResult 保持同签名同语义。
func (r *AnalyticsRepo) FlagExperimentResult(
	ctx context.Context,
	flagKey, goalEvent string,
	appFilter int64,
	startMs, endMs int64,
) ([]data.FlagVariantStat, error) {
	goal := goalEvent
	if goal == "" {
		goal = "$feature_flag" // 无目标事件时只出曝光（goal 等于曝光事件 → converted == exposed）
	}

	tenantCond := ""
	args := []any{flagKey, time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if appFilter != 0 {
		tenantCond = "AND tenant_id = ?"
		args = append(args, appFilter)
	}

	goalTenantCond := ""
	if appFilter != 0 {
		goalTenantCond = "AND g.tenant_id = ?"
		args = append(args, appFilter)
	}
	args = append(args, goal, time.UnixMilli(startMs), time.UnixMilli(endMs))

	q := fmt.Sprintf(`
WITH exposed AS (
  SELECT user_id, MAX(context['variant']) AS variant
  FROM events_fact FINAL
  WHERE event_name = '$feature_flag'
    AND context['flag_key'] = ?
    AND user_id > 0
    AND event_time >= ? AND event_time < ?
    %s
  GROUP BY user_id
)
SELECT e.variant AS variant,
       count() AS exposed_users,
       uniqExact(g.user_id) AS converted_users
FROM exposed e
LEFT JOIN events_fact g
  ON g.user_id = e.user_id
 AND g.event_name = ?
 AND g.event_time >= ? AND g.event_time < ?
 %s
GROUP BY e.variant`, tenantCond, goalTenantCond)

	var rows []flagVariantStatRow
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("flag experiment result query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to query flag experiment result")
	}

	stats := make([]data.FlagVariantStat, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, data.FlagVariantStat{
			Variant:        row.Variant,
			ExposedUsers:   row.ExposedUsers,
			ConvertedUsers: row.ConvertedUsers,
		})
	}

	return stats, nil
}

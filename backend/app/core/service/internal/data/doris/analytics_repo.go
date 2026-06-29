package doris

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	dorisCrud "github.com/tx7do/go-crud/doris"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// AnalyticsRepo 基于 Doris（MySQL 协议）的 BI 聚合查询仓库。
// 所有查询走原生 SQL（GROUP BY），数据源为 events_fact / sessions_fact。
type AnalyticsRepo struct {
	db  *dorisCrud.Client
	log *log.Helper
}

func NewAnalyticsRepo(
	ctx *bootstrap.Context,
	db *dorisCrud.Client,
) *AnalyticsRepo {
	return &AnalyticsRepo{
		log: ctx.NewLoggerHelper("analytics/doris/repo/core-service"),
		db:  db,
	}
}

// ============================================================================
// 事件趋势
// ============================================================================

func (r *AnalyticsRepo) EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	gran := req.GetGranularity()
	bucketExpr, interval := granularityExpr(gran)

	var where []string
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	where = append(where, "event_time >= ?", "event_time < ?")
	if v := req.GetEventName(); v != "" {
		where = append(where, "event_name = ?")
		args = append(args, v)
	}
	if v := req.GetPlatform(); v != "" {
		where = append(where, "platform = ?")
		args = append(args, v)
	}
	if v := req.GetAppId(); v != 0 {
		where = append(where, "tenant_id = ?")
		args = append(args, v)
	}

	q := fmt.Sprintf(
		"SELECT %s AS bucket, COUNT(*) AS cnt FROM events_fact WHERE %s GROUP BY bucket ORDER BY bucket",
		bucketExpr, strings.Join(where, " AND "),
	)

	type row struct {
		Bucket time.Time `db:"bucket"`
		Cnt    int64     `db:"cnt"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("EventTrend query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("event trend query failed")
	}

	points := make([]*ubaV1.TimeSeriesPoint, 0, len(rows))
	var total int64
	for _, rw := range rows {
		ts := rw.Bucket.UnixMilli()
		points = append(points, &ubaV1.TimeSeriesPoint{Timestamp: ts, Value: float64(rw.Cnt)})
		total += rw.Cnt
	}

	// 补全空桶，保证前端折线连续
	points = fillMissingBuckets(points, startMs, endMs, interval)

	return &ubaV1.EventTrendResponse{
		Points:      points,
		Granularity: effectiveGranularity(gran, startMs, endMs),
		Total:       total,
	}, nil
}

// ============================================================================
// 漏斗分析
// ============================================================================

func (r *AnalyticsRepo) Funnel(ctx context.Context, req *ubaV1.FunnelRequest) (*ubaV1.FunnelResponse, error) {
	steps := req.GetSteps()
	if len(steps) < 2 {
		return nil, ubaV1.ErrorBadRequest("funnel requires at least 2 steps")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	// 统计口径：每个步骤 = 在时间范围内完成该事件的去重用户数（不做严格顺序穿透，
	// 这是 Doris 上的近实时实现；严格漏斗需事件级顺序匹配，留作后续优化）。
	resp := &ubaV1.FunnelResponse{
		Steps: make([]*ubaV1.FunnelStep, 0, len(steps)),
	}
	var prevCount int64
	for i, name := range steps {
		q := `SELECT COUNT(DISTINCT user_id) AS cnt FROM events_fact
		      WHERE event_time >= ? AND event_time < ? AND event_name = ?`
		args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs), name}
		if v := req.GetAppId(); v != 0 {
			q += " AND tenant_id = ?"
			args = append(args, v)
		}
		var cnt int64
		if err := r.db.GetContext(ctx, &cnt, q+" LIMIT 1", args...); err != nil {
			r.log.Errorf("Funnel step %d query failed: %v", i, err)
			return nil, ubaV1.ErrorInternalServerError("funnel query failed")
		}
		step := &ubaV1.FunnelStep{
			StepIndex: uint32(i + 1),
			EventName: name,
			Count:     cnt,
		}
		if i == 0 {
			step.ConversionRate = 1
			step.OverallRate = 1
		} else if prevCount > 0 {
			step.ConversionRate = float64(cnt) / float64(prevCount)
			if resp.Steps[0].Count > 0 {
				step.OverallRate = float64(cnt) / float64(resp.Steps[0].Count)
			}
		}
		resp.Steps = append(resp.Steps, step)
		prevCount = cnt
	}

	resp.EnteredUsers = resp.Steps[0].Count
	if len(resp.Steps) > 0 {
		last := resp.Steps[len(resp.Steps)-1]
		resp.CompletedUsers = last.Count
		if resp.EnteredUsers > 0 {
			resp.OverallConversion = float64(last.Count) / float64(resp.EnteredUsers)
		}
	}
	return resp, nil
}

// ============================================================================
// 留存分析（同期群矩阵）
// ============================================================================

func (r *AnalyticsRepo) Retention(ctx context.Context, req *ubaV1.RetentionRequest) (*ubaV1.RetentionResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	maxOffset := int(req.GetMaxOffsetDays())
	if maxOffset <= 0 {
		maxOffset = 7
	}

	// 1) 取各 cohort 日的"新用户/首活用户"集合规模（以首日出现为准）
	cohortQ := `SELECT FROM_UNIXTIME(event_ts/1000, '%Y-%m-%d') AS d, COUNT(DISTINCT user_id) AS sz
	            FROM events_fact
	            WHERE event_ts >= ? AND event_ts < ?
	            GROUP BY d ORDER BY d`
	var cohortRows []struct {
		D string `db:"d"`
		S int64  `db:"sz"`
	}
	if err := r.db.SelectContext(ctx, &cohortRows, cohortQ, startMs, endMs); err != nil {
		r.log.Errorf("Retention cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("retention query failed")
	}

	// offsetDays 横轴
	offsets := make([]uint32, 0, maxOffset+1)
	for i := 0; i <= maxOffset; i++ {
		offsets = append(offsets, uint32(i))
	}

	cohorts := make([]*ubaV1.RetentionCohort, 0, len(cohortRows))
	for _, cr := range cohortRows {
		cohortDate, err := time.ParseInLocation("2006-01-02", cr.D, time.Local)
		if err != nil {
			continue
		}
		c := &ubaV1.RetentionCohort{
			CohortDate: cohortDate.UnixMilli(),
			Size:       cr.S,
			Cells:      make([]*ubaV1.RetentionCell, 0, maxOffset+1),
		}
		for _, off := range offsets {
			dayStart := cohortDate.AddDate(0, 0, int(off))
			dayEnd := dayStart.AddDate(0, 0, 1)
			q := `SELECT COUNT(DISTINCT user_id) AS cnt FROM events_fact
			      WHERE event_ts >= ? AND event_ts < ?`
			args := []any{dayStart.UnixMilli(), dayEnd.UnixMilli()}
			if ev := req.GetEventName(); req.GetRetentionType() == "EVENT" && ev != "" {
				q += " AND event_name = ?"
				args = append(args, ev)
			}
			var cnt int64
			if err := r.db.GetContext(ctx, &cnt, q+" LIMIT 1", args...); err != nil {
				r.log.Errorf("Retention cell query failed: %v", err)
				continue
			}
			cell := &ubaV1.RetentionCell{OffsetDays: off, Count: cnt}
			if c.Size > 0 {
				cell.Rate = float64(cnt) / float64(c.Size)
			}
			c.Cells = append(c.Cells, cell)
		}
		cohorts = append(cohorts, c)
	}

	return &ubaV1.RetentionResponse{Cohorts: cohorts, OffsetDays: offsets}, nil
}

// ============================================================================
// 维度分组聚合
// ============================================================================

func (r *AnalyticsRepo) GroupBy(ctx context.Context, req *ubaV1.GroupByRequest) (*ubaV1.GroupByResponse, error) {
	col, ok := allowedDimension(req.GetDimension())
	if !ok {
		return nil, ubaV1.ErrorBadRequest(fmt.Sprintf("unsupported dimension: %s", req.GetDimension()))
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())
	topN := int(req.GetTopN())
	if topN <= 0 {
		topN = 20
	}

	metricExpr, err := metricExpr(req.GetMetric(), col)
	if err != nil {
		return nil, ubaV1.ErrorBadRequest(err.Error())
	}

	q := fmt.Sprintf(
		"SELECT %s AS label, %s AS value FROM events_fact WHERE event_time >= ? AND event_time < ?",
		col, metricExpr,
	)
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetEventName(); v != "" {
		q += " AND event_name = ?"
		args = append(args, v)
	}
	if v := req.GetAppId(); v != 0 {
		q += " AND tenant_id = ?"
		args = append(args, v)
	}
	q += fmt.Sprintf(" GROUP BY label ORDER BY value DESC LIMIT %d", topN)

	type row struct {
		Label string  `db:"label"`
		Value float64 `db:"value"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("GroupBy query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("group-by query failed")
	}

	var total float64
	for _, rw := range rows {
		total += rw.Value
	}
	buckets := make([]*ubaV1.GroupByBucket, 0, len(rows))
	for _, rw := range rows {
		b := &ubaV1.GroupByBucket{Label: rw.Label, Value: rw.Value}
		if total > 0 {
			b.Percentage = rw.Value / total
		}
		buckets = append(buckets, b)
	}
	return &ubaV1.GroupByResponse{Buckets: buckets, Dimension: req.GetDimension(), Total: total}, nil
}

// ============================================================================
// 活跃用户（DAU/WAU/MAU）
// ============================================================================

func (r *AnalyticsRepo) ActiveUsers(ctx context.Context, req *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	gran := req.GetGranularity()

	// 预聚合表 mv_events_daily 为日粒度：仅 DAY/WEEK/MONTH（含 UNSPECIFIED 默认按天）
	// 能输出真值 WAU/MAU；HOUR 粒度无小时级状态，回退为等于 DAU。
	if gran == ubaV1.AnalyticsGranularity_HOUR {
		return r.activeUsersFromEventsFact(ctx, req, startMs, endMs)
	}

	// 数据源：mv_events_daily 已按 (tenant_id, stat_date, event_category, event_name, ...) 维度
	// 存好 HLL_UNION(HLL_HASH(user_id)) 的状态 uv。WAU/MAU 通过对滚动窗口内各天 uv 状态做
	// HLL_UNION 再取 HLL_CARDINALITY，得到准确的跨天去重（HLL 近似，误差 <1%）。
	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	q := fmt.Sprintf(`
SELECT d.stat_date,
       HLL_CARDINALITY(HLL_UNION(d.uv)) AS dau,
       (
           SELECT HLL_CARDINALITY(HLL_UNION(uv))
           FROM mv_events_daily
           WHERE %sstat_date BETWEEN DATE_SUB(d.stat_date, INTERVAL 6 DAY) AND d.stat_date
       ) AS wau,
       (
           SELECT HLL_CARDINALITY(HLL_UNION(uv))
           FROM mv_events_daily
           WHERE %sstat_date BETWEEN DATE_SUB(d.stat_date, INTERVAL 29 DAY) AND d.stat_date
       ) AS mau
FROM mv_events_daily d
WHERE %sstat_date >= DATE(?) AND stat_date < DATE(?)
GROUP BY d.stat_date
ORDER BY d.stat_date`,
		tenantCond, tenantCond, tenantCond)

	type row struct {
		StatDate time.Time `db:"stat_date"`
		Dau      int64     `db:"dau"`
		Wau      int64     `db:"wau"`
		Mau      int64     `db:"mau"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("ActiveUsers query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("active users query failed")
	}

	points := make([]*ubaV1.ActiveUsersPoint, 0, len(rows))
	for _, rw := range rows {
		points = append(points, &ubaV1.ActiveUsersPoint{
			Timestamp: rw.StatDate.UnixMilli(),
			Dau:       rw.Dau,
			Wau:       rw.Wau,
			Mau:       rw.Mau,
		})
	}

	resp := &ubaV1.ActiveUsersResponse{Points: points}
	if len(points) > 0 {
		resp.LatestDau = points[len(points)-1].Dau
	}
	return resp, nil
}

// activeUsersFromEventsFact 在 HOUR 粒度下回退到扫描 events_fact：
// 预聚合表仅日级，无法支持小时级滚动窗口，故 WAU/MAU 退化为等于 DAU（仅给出下界）。
func (r *AnalyticsRepo) activeUsersFromEventsFact(ctx context.Context, req *ubaV1.ActiveUsersRequest, startMs, endMs int64) (*ubaV1.ActiveUsersResponse, error) {
	bucketExpr, _ := granularityExpr(ubaV1.AnalyticsGranularity_HOUR)
	q := fmt.Sprintf(
		"SELECT %s AS bucket, COUNT(DISTINCT user_id) AS dau FROM events_fact WHERE event_time >= ? AND event_time < ? GROUP BY bucket ORDER BY bucket",
		bucketExpr,
	)
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		q = strings.Replace(q, "WHERE event_time", "WHERE tenant_id = ? AND event_time", 1)
		args = append([]any{v}, args...)
	}

	type row struct {
		Bucket time.Time `db:"bucket"`
		Dau    int64     `db:"dau"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("ActiveUsers query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("active users query failed")
	}

	points := make([]*ubaV1.ActiveUsersPoint, 0, len(rows))
	for _, rw := range rows {
		// 小时级无滚动窗口状态，WAU/MAU 退化为等于 DAU。
		points = append(points, &ubaV1.ActiveUsersPoint{
			Timestamp: rw.Bucket.UnixMilli(),
			Dau:       rw.Dau,
			Wau:       rw.Dau,
			Mau:       rw.Dau,
		})
	}

	resp := &ubaV1.ActiveUsersResponse{Points: points}
	if len(points) > 0 {
		resp.LatestDau = points[len(points)-1].Dau
	}
	return resp, nil
}

// ============================================================================
// 工具函数
// ============================================================================

func normTimeRange(tr *ubaV1.TimeRange) (int64, int64) {
	start := tr.GetStartMs()
	end := tr.GetEndMs()
	if end <= 0 {
		end = time.Now().UnixMilli()
	}
	if start <= 0 || start > end {
		// 默认最近 7 天
		start = end - int64(7*24*time.Hour/time.Millisecond)
	}
	return start, end
}

// granularityExpr 返回 Doris 的时间分桶表达式与对应的时间间隔。
// Doris 支持 DATE_FORMAT + FLOOR，这里用 DATE_FORMAT 兼容性最好。
func granularityExpr(g ubaV1.AnalyticsGranularity) (string, time.Duration) {
	switch g {
	case ubaV1.AnalyticsGranularity_HOUR:
		return "DATE_FORMAT(event_time, '%Y-%m-%d %H:00:00')", time.Hour
	case ubaV1.AnalyticsGranularity_WEEK:
		return "DATE_FORMAT(event_time, '%x-W%v')", 7 * 24 * time.Hour
	case ubaV1.AnalyticsGranularity_MONTH:
		return "DATE_FORMAT(event_time, '%Y-%m-01')", 30 * 24 * time.Hour
	case ubaV1.AnalyticsGranularity_DAY:
		return "DATE_FORMAT(event_time, '%Y-%m-%d')", 24 * time.Hour
	case ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED:
		fallthrough
	default:
		// UNSPECIFIED：跨度 > 3 天按天，否则按小时
		return "DATE_FORMAT(event_time, '%Y-%m-%d')", 24 * time.Hour
	}
}

func effectiveGranularity(g ubaV1.AnalyticsGranularity, startMs, endMs int64) ubaV1.AnalyticsGranularity {
	if g != ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED {
		return g
	}
	if endMs-startMs > int64(3*24*time.Hour/time.Millisecond) {
		return ubaV1.AnalyticsGranularity_DAY
	}
	return ubaV1.AnalyticsGranularity_HOUR
}

// fillMissingBuckets 按间隔补全缺失的空桶，值置 0，保证折线连续。
func fillMissingBuckets(points []*ubaV1.TimeSeriesPoint, startMs, endMs int64, interval time.Duration) []*ubaV1.TimeSeriesPoint {
	if interval <= 0 || len(points) == 0 {
		return points
	}
	filled := make([]*ubaV1.TimeSeriesPoint, 0, len(points)*2)
	cursor := alignMs(startMs, interval)
	idx := 0
	for cursor <= endMs && idx < len(points) {
		pt := points[idx]
		if pt.Timestamp <= cursor {
			filled = append(filled, pt)
			if pt.Timestamp == cursor {
				idx++
			}
		} else {
			filled = append(filled, &ubaV1.TimeSeriesPoint{Timestamp: cursor, Value: 0})
		}
		cursor += int64(interval / time.Millisecond)
	}
	for ; idx < len(points); idx++ {
		filled = append(filled, points[idx])
	}
	return filled
}

func alignMs(ms int64, interval time.Duration) int64 {
	intervalMs := int64(interval / time.Millisecond)
	if intervalMs <= 0 {
		return ms
	}
	return (ms / intervalMs) * intervalMs
}

// allowedDimension 白名单化维度字段，防止 SQL 注入。
func allowedDimension(dim string) (string, bool) {
	m := map[string]string{
		"platform":    "platform",
		"channel":     "channel",
		"country":     "country",
		"app_version": "app_version",
		"event_name":  "event_name",
		"event_category": "event_category",
		"os":          "os",
		"network":     "network",
	}
	v, ok := m[dim]
	return v, ok
}

func metricExpr(metric, col string) (string, error) {
	switch metric {
	case "", "COUNT":
		return "COUNT(*)", nil
	case "UNIQUE_USER":
		return "COUNT(DISTINCT user_id)", nil
	case "SUM_AMOUNT":
		return "CAST(COALESCE(SUM(CAST(amount AS DOUBLE)), 0) AS DOUBLE)", nil
	default:
		return "", fmt.Errorf("unsupported metric: %s", metric)
	}
}

// 兼容 sql.ErrNoRows
var _ = sql.ErrNoRows

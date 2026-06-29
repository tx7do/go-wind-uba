package doris

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
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
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("event trend query failed: %v", err))
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
			return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("funnel query failed: %v", err))
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
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("retention query failed: %v", err))
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

	// user_level/vip_level 在 users_dim，非 events_fact：需 JOIN。
	joinClause := ""
	dimCol := col
	if joinUsersDim(req.GetDimension()) {
		joinClause = " JOIN users_dim u ON u.tenant_id = events_fact.tenant_id AND u.user_id = events_fact.user_id"
		dimCol = "u." + col
	}

	q := fmt.Sprintf(
		"SELECT %s AS label, %s AS value FROM events_fact %s WHERE event_time >= ? AND event_time < ?",
		dimCol, metricExpr, joinClause,
	)
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetEventName(); v != "" {
		q += " AND event_name = ?"
		args = append(args, v)
	}
	if v := req.GetAppId(); v != 0 {
		q += " AND events_fact.tenant_id = ?"
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
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("group-by query failed: %v", err))
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

	// HOUR 粒度无滚动窗口，回退为等于 DAU。
	if gran == ubaV1.AnalyticsGranularity_HOUR {
		return r.activeUsersFromEventsFact(ctx, req, startMs, endMs)
	}

	// 直接查 events_fact（不依赖物化视图 mv_events_daily，避免 UNIQUE KEY 表上不自动刷新的问题）。
	// 分两步：① DAU 按天分桶；② WAU/MAU 取最新一天的滚动去重值。在 Go 层合并。
	tenantCond := ""
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
	}

	// ① DAU 按天
	dauArgs := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		dauArgs = append([]any{v}, dauArgs...)
	}
	dauQ := fmt.Sprintf(`
SELECT to_date(event_time) AS stat_date, COUNT(DISTINCT user_id) AS dau
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ? AND user_id > 0
GROUP BY stat_date ORDER BY stat_date`, tenantCond)

	type dauRow struct {
		StatDate time.Time `db:"stat_date"`
		Dau      int64     `db:"dau"`
	}
	var dauRows []dauRow
	if err := r.db.SelectContext(ctx, &dauRows, dauQ, dauArgs...); err != nil {
		r.log.Errorf("ActiveUsers DAU query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("active users query failed: %v", err))
	}

	// ② WAU/MAU：取最近一天的滚动 7/30 天去重（dashboard 只需最新值做 KPI）。
	wauArgs := []any{time.UnixMilli(endMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		wauArgs = append([]any{v}, wauArgs...)
	}
	wauQ := fmt.Sprintf(`
SELECT
  (SELECT COUNT(DISTINCT user_id) FROM events_fact WHERE %sevent_time >= DATE_SUB(?, INTERVAL 6 DAY) AND event_time < ? AND user_id > 0) AS wau,
  (SELECT COUNT(DISTINCT user_id) FROM events_fact WHERE %sevent_time >= DATE_SUB(?, INTERVAL 29 DAY) AND event_time < ? AND user_id > 0) AS mau`, tenantCond, tenantCond)
	var wm struct {
		Wau int64 `db:"wau"`
		Mau int64 `db:"mau"`
	}
	if err := r.db.GetContext(ctx, &wm, wauQ, wauArgs...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("ActiveUsers WAU/MAU query failed: %v", err)
		// WAU/MAU 失败不阻断，降级为 DAU
		wm.Wau = 0
		wm.Mau = 0
	}

	points := make([]*ubaV1.ActiveUsersPoint, 0, len(dauRows))
	for _, rw := range dauRows {
		points = append(points, &ubaV1.ActiveUsersPoint{
			Timestamp: rw.StatDate.UnixMilli(),
			Dau:       rw.Dau,
			Wau:       wm.Wau, // 用最新滚动值填充（MVP：曲线展示 DAU 趋势，WAU/MAU 取最新）
			Mau:       wm.Mau,
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
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("active users query failed: %v", err))
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
// 归因分析（首触/末触渠道归因）
// 参考模板：backend/sql/doris/query.sql §16
// ============================================================================

func (r *AnalyticsRepo) Attribution(ctx context.Context, req *ubaV1.AttributionRequest) (*ubaV1.AttributionResponse, error) {
	if req.GetConversionEvent() == "" {
		return nil, ubaV1.ErrorBadRequest("conversion_event is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	// 归因维度：channel（默认）/ referer
	dim := "channel"
	if d := req.GetDimension(); d == "referer" {
		dim = "referer"
	}
	// 归因模型：last_touch（默认，末次触达）/ first_touch（首次触达）
	orderDir := "DESC" // 末次触达：取转化前最后一次
	if req.GetModel() == "first_touch" {
		orderDir = "ASC" // 首次触达：取最早一次
	}

	tenantCond := ""
	args := []any{req.GetConversionEvent(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 转化用户集合（做过转化事件）→ 每个转化用户的指定触点（首/末）→ 按维度聚合去重用户数。
	// rn=1 取每用户在窗口内的第一条（ASC=首触 / DESC=末触）。
	q := fmt.Sprintf(`
WITH converters AS (
    SELECT DISTINCT user_id FROM events_fact
    WHERE %sevent_name = ? AND event_time >= ? AND event_time < ?
),
touchpoint AS (
    SELECT e.user_id, e.%s AS dim_val,
           ROW_NUMBER() OVER (PARTITION BY e.user_id ORDER BY e.event_time %s) AS rn
    FROM events_fact e
    JOIN converters c ON c.user_id = e.user_id
    WHERE e.%sevent_time >= ? AND e.event_time < ?
)
SELECT dim_val, COUNT(DISTINCT user_id) AS converter_uv
FROM touchpoint
WHERE rn = 1 AND dim_val IS NOT NULL AND dim_val <> ''
GROUP BY dim_val
ORDER BY converter_uv DESC
LIMIT 20`,
		tenantCond, dim, orderDir, tenantCond)
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))

	type row struct {
		DimVal     string `db:"dim_val"`
		ConverterUv int64  `db:"converter_uv"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Attribution query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("attribution query failed: %v", err))
	}

	var total int64
	buckets := make([]*ubaV1.AttributionBucket, 0, len(rows))
	for _, rw := range rows {
		total += rw.ConverterUv
		buckets = append(buckets, &ubaV1.AttributionBucket{
			Label:       rw.DimVal,
			ConverterUv: rw.ConverterUv,
		})
	}
	for _, b := range buckets {
		if total > 0 {
			b.Percentage = float64(b.ConverterUv) / float64(total)
		}
	}

	model := req.GetModel()
	if model == "" {
		model = "last_touch"
	}
	return &ubaV1.AttributionResponse{
		Buckets:         buckets,
		Model:           model,
		Dimension:       dim,
		TotalConverters: total,
	}, nil
}

// ============================================================================
// 分布分析（事件时长分桶 + 分位数）
// 参考模板：backend/sql/doris/query.sql §14
// ============================================================================

func (r *AnalyticsRepo) Distribution(ctx context.Context, req *ubaV1.DistributionRequest) (*ubaV1.DistributionResponse, error) {
	if req.GetEventName() == "" {
		return nil, ubaV1.ErrorBadRequest("event_name is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	tenantCond := ""
	args := []any{req.GetEventName(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 分桶分布：0-10s / 10-60s / 1-5min / 5min+
	bucketQ := fmt.Sprintf(`
SELECT
    CASE
        WHEN duration_ms < 10000  THEN '0_10s'
        WHEN duration_ms < 60000  THEN '10_60s'
        WHEN duration_ms < 300000 THEN '1_5min'
        ELSE '5min_plus'
    END AS duration_bucket,
    COUNT(*) AS cnt
FROM events_fact
WHERE %sevent_name = ? AND duration_ms > 0 AND event_time >= ? AND event_time < ?
GROUP BY CASE
        WHEN duration_ms < 10000  THEN '0_10s'
        WHEN duration_ms < 60000  THEN '10_60s'
        WHEN duration_ms < 300000 THEN '1_5min'
        ELSE '5min_plus'
    END
ORDER BY duration_bucket`, tenantCond)

	type bucketRow struct {
		Bucket string `db:"duration_bucket"`
		Cnt    int64  `db:"cnt"`
	}
	var bRows []bucketRow
	if err := r.db.SelectContext(ctx, &bRows, bucketQ, args...); err != nil {
		r.log.Errorf("Distribution bucket query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("distribution query failed: %v", err))
	}

	var bucketTotal int64
	for _, br := range bRows {
		bucketTotal += br.Cnt
	}
	buckets := make([]*ubaV1.DistributionBucket, 0, len(bRows))
	for _, br := range bRows {
		var pct float64
		if bucketTotal > 0 {
			pct = float64(br.Cnt) / float64(bucketTotal)
		}
		buckets = append(buckets, &ubaV1.DistributionBucket{
			Bucket:     br.Bucket,
			Count:      br.Cnt,
			Percentage: pct,
		})
	}

	// 分位数摘要：均值 / P50 / P90 / 最大值
	// CAST(duration_ms AS DOUBLE) 确保 PERCENTILE_APPROX 兼容 BIGINT 列。
	summaryQ := fmt.Sprintf(`
SELECT
    COUNT(*) AS cnt,
    IFNULL(ROUND(AVG(CAST(duration_ms AS DOUBLE)) / 1000, 2), 0)                      AS avg_sec,
    IFNULL(ROUND(PERCENTILE_APPROX(CAST(duration_ms AS DOUBLE), 0.5) / 1000, 2), 0)   AS p50_sec,
    IFNULL(ROUND(PERCENTILE_APPROX(CAST(duration_ms AS DOUBLE), 0.9) / 1000, 2), 0)   AS p90_sec,
    IFNULL(ROUND(MAX(duration_ms) / 1000, 2), 0)                                       AS max_sec
FROM events_fact
WHERE %sevent_name = ? AND duration_ms > 0 AND event_time >= ? AND event_time < ?`, tenantCond)

	var s struct {
		Cnt    int64   `db:"cnt"`
		AvgSec float64 `db:"avg_sec"`
		P50Sec float64 `db:"p50_sec"`
		P90Sec float64 `db:"p90_sec"`
		MaxSec float64 `db:"max_sec"`
	}
	if err := r.db.GetContext(ctx, &s, summaryQ, args...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("Distribution summary query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("distribution summary query failed: %v", err))
	}

	return &ubaV1.DistributionResponse{
		Buckets: buckets,
		Summary: &ubaV1.DistributionSummary{
			AvgSec: s.AvgSec,
			P50Sec: s.P50Sec,
			P90Sec: s.P90Sec,
			MaxSec: s.MaxSec,
			Count:  s.Cnt,
		},
	}, nil
}

// ============================================================================
// 行为序列（指定用户的行为时间线）
// 参考模板：backend/sql/doris/query.sql §9.3
// ============================================================================

func (r *AnalyticsRepo) BehaviorSequence(ctx context.Context, req *ubaV1.BehaviorSequenceRequest) (*ubaV1.BehaviorSequenceResponse, error) {
	if req.GetUserId() == 0 {
		return nil, ubaV1.ErrorBadRequest("user_id is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())
	limit := int64(req.GetLimit())
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	where := []string{"user_id = ?", "event_time >= ?", "event_time < ?"}
	args := []any{req.GetUserId(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		where = append([]string{"tenant_id = ?"}, where...)
		args = append([]any{v}, args...)
	}
	if en := req.GetEventName(); en != "" {
		where = append(where, "event_name = ?")
		args = append(args, en)
	}

	// LIMIT 不用占位符（Doris 某些版本不支持 LIMIT ?）
	q := fmt.Sprintf(`
SELECT event_time, event_name, session_id, session_seq, referer, platform, channel
FROM events_fact
WHERE %s
ORDER BY event_time ASC
LIMIT %d`, strings.Join(where, " AND "), limit)

	type evRow struct {
		EventTime  *time.Time `db:"event_time"`
		EventName  *string    `db:"event_name"`
		SessionID  *string    `db:"session_id"`
		SessionSeq *uint32    `db:"session_seq"`
		Referer    *string    `db:"referer"`
		Platform   *string    `db:"platform"`
		Channel    *string    `db:"channel"`
	}
	var rows []evRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("BehaviorSequence query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("behavior sequence query failed: %v", err))
	}

	events := make([]*ubaV1.SequenceEvent, 0, len(rows))
	for _, rw := range rows {
		ev := &ubaV1.SequenceEvent{EventName: derefStr(rw.EventName)}
		if rw.EventTime != nil {
			ev.Timestamp = rw.EventTime.UnixMilli()
		}
		if rw.SessionID != nil {
			ev.SessionId = rw.SessionID
		}
		if rw.SessionSeq != nil {
			ev.SessionSeq = rw.SessionSeq
		}
		if rw.Referer != nil {
			ev.Referer = rw.Referer
		}
		if rw.Platform != nil {
			ev.Platform = rw.Platform
		}
		if rw.Channel != nil {
			ev.Channel = rw.Channel
		}
		events = append(events, ev)
	}

	return &ubaV1.BehaviorSequenceResponse{
		UserId: req.GetUserId(),
		Events: events,
	}, nil
}

// ============================================================================
// 用户分群/圈选（做过/未做过某事件的人群筛选）
// 参考模板：backend/sql/doris/query.sql §17
// MVP 实现：取 include[0]（做过）+ 可选 exclude[0]（未做过）+ min_times 次数阈值。
// ============================================================================

func (r *AnalyticsRepo) Segmentation(ctx context.Context, req *ubaV1.SegmentationRequest) (*ubaV1.SegmentationResponse, error) {
	if len(req.GetInclude()) == 0 {
		return nil, ubaV1.ErrorBadRequest("at least one include condition is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())
	limit := int64(req.GetLimit())
	if limit <= 0 || limit > 50000 {
		limit = 5000
	}

	inc := req.GetInclude()[0]
	incTimes := int64(inc.GetMinTimes())
	if incTimes <= 0 {
		incTimes = 1
	}

	tenantCond := ""
	args := []any{inc.GetEventName(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 基础：做过 include 事件且达到 min_times 次的用户。
	q := fmt.Sprintf(`
SELECT user_id
FROM events_fact
WHERE %sevent_name = ? AND event_time >= ? AND event_time < ?
GROUP BY user_id
HAVING COUNT(*) >= ?`, tenantCond)
	args = append(args, incTimes)

	// 可选排除：排除做过 exclude[0] 事件的用户。
	if excList := req.GetExclude(); len(excList) > 0 && excList[0].GetEventName() != "" {
		exc := excList[0]
		q = fmt.Sprintf(`
SELECT a.user_id FROM (%s) a
WHERE NOT EXISTS (
    SELECT 1 FROM events_fact b
    WHERE b.%suser_id = a.user_id AND b.event_name = ? AND b.event_time >= ? AND b.event_time < ?
)`, q, tenantCond)
		args = append(args, exc.GetEventName(), time.UnixMilli(startMs), time.UnixMilli(endMs))
	}

	q = q + fmt.Sprintf(" LIMIT %d", limit)

	var userIDs []uint32
	if err := r.db.SelectContext(ctx, &userIDs, q, args...); err != nil {
		r.log.Errorf("Segmentation query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("segmentation query failed: %v", err))
	}

	return &ubaV1.SegmentationResponse{
		UserIds: userIDs,
		Total:   int64(len(userIDs)),
	}, nil
}

// ============================================================================
// 点击热力图（按页面网格分桶聚合点击坐标 + 元素点击 TOP）
// ============================================================================

func (r *AnalyticsRepo) Click(ctx context.Context, req *ubaV1.ClickRequest) (*ubaV1.ClickResponse, error) {
	if req.GetPageUrl() == "" {
		return nil, ubaV1.ErrorBadRequest("page_url is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())
	gridSize := int64(req.GetGridSize())
	if gridSize <= 0 {
		gridSize = 20
	}

	tenantCond := ""
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
	}

	// 网格分桶：FLOOR(click_x / gridSize) * gridSize 对齐到网格左上角。
	// gridSize 直接拼接（经校验的整数，无注入风险），避免 time.Time 与 int64 混参导致驱动类型推断错误。
	gridArgs := []any{}
	if v := req.GetAppId(); v != 0 {
		gridArgs = append(gridArgs, v)
	}
	gridArgs = append(gridArgs, req.GetPageUrl(), time.UnixMilli(startMs), time.UnixMilli(endMs))

	gridSQL := fmt.Sprintf(`
SELECT FLOOR(click_x / %d) * %d AS grid_x,
       FLOOR(click_y / %d) * %d AS grid_y,
       COUNT(*) AS cnt
FROM events_fact
WHERE %sevent_name = 'click' AND page_url = ? AND click_x > 0 AND click_y > 0
  AND event_time >= ? AND event_time < ?
GROUP BY grid_x, grid_y
ORDER BY cnt DESC
LIMIT 2000`, gridSize, gridSize, gridSize, gridSize, tenantCond)

	type gridRow struct {
		GridX int64 `db:"grid_x"`
		GridY int64 `db:"grid_y"`
		Cnt   int64 `db:"cnt"`
	}
	var gRows []gridRow
	if err := r.db.SelectContext(ctx, &gRows, gridSQL, gridArgs...); err != nil {
		r.log.Errorf("Click grid query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("click query failed: %v", err))
	}

	var maxCnt int64
	for _, gr := range gRows {
		if gr.Cnt > maxCnt {
			maxCnt = gr.Cnt
		}
	}
	points := make([]*ubaV1.ClickHeatPoint, 0, len(gRows))
	var totalClicks int64
	for _, gr := range gRows {
		totalClicks += gr.Cnt
		var intensity float64
		if maxCnt > 0 {
			intensity = float64(gr.Cnt) / float64(maxCnt)
		}
		points = append(points, &ubaV1.ClickHeatPoint{
			X:        uint32(gr.GridX),
			Y:        uint32(gr.GridY),
			Count:    gr.Cnt,
			Intensity: intensity,
		})
	}

	// 元素点击 TOP（按 element_xpath 聚合）
	topArgs := []any{}
	if v := req.GetAppId(); v != 0 {
		topArgs = append(topArgs, v)
	}
	topArgs = append(topArgs, req.GetPageUrl(), time.UnixMilli(startMs), time.UnixMilli(endMs))
	topSQL := fmt.Sprintf(`
SELECT element_xpath, COUNT(*) AS cnt
FROM events_fact
WHERE %sevent_name = 'click' AND page_url = ? AND element_xpath != ''
  AND event_time >= ? AND event_time < ?
GROUP BY element_xpath
ORDER BY cnt DESC
LIMIT 20`, tenantCond)

	type elemRow struct {
		ElementXpath string `db:"element_xpath"`
		Cnt          int64  `db:"cnt"`
	}
	var eRows []elemRow
	if err := r.db.SelectContext(ctx, &eRows, topSQL, topArgs...); err != nil {
		r.log.Errorf("Click element query failed: %v", err)
		// 元素 TOP 失败不影响热力图主结果
		eRows = nil
	}

	topElements := make([]*ubaV1.ClickElementBucket, 0, len(eRows))
	for _, er := range eRows {
		var pct float64
		if totalClicks > 0 {
			pct = float64(er.Cnt) / float64(totalClicks)
		}
		topElements = append(topElements, &ubaV1.ClickElementBucket{
			ElementXpath: er.ElementXpath,
			Count:        er.Cnt,
			Percentage:   pct,
		})
	}

	return &ubaV1.ClickResponse{
		Points:      points,
		TopElements: topElements,
		TotalClicks: totalClicks,
		GridSize:    uint32(gridSize),
	}, nil
}

// ============================================================================
// 用户生命周期（新/活跃/留存/流失/回流 阶段分布）
// 参考模板：backend/sql/doris/query.sql §4.1
// ============================================================================

func (r *AnalyticsRepo) Lifecycle(ctx context.Context, req *ubaV1.LifecycleRequest) (*ubaV1.LifecycleResponse, error) {
	_, endMs := normTimeRange(req.GetTimeRange())
	now := time.UnixMilli(endMs)
	newUserDays := int64(req.GetNewUserDays())
	if newUserDays <= 0 {
		newUserDays = 7
	}
	churnDays := int64(req.GetChurnDays())
	if churnDays <= 0 {
		churnDays = 30
	}

	tenantCond := ""
	args := []any{now}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 阶段分桶（以 endMs 视为"今天"）：
	//  - new_user: register_time 在 newUserDays 天内
	//  - active:   last_active 距今 ≤1 天
	//  - retained: 距今 1~churnDays 天
	//  - churned:  距今 > churnDays 天
	//  - reactivated: 曾流失（last_active 曾超 churnDays）但近期又活跃——MVP 近似：
	//    last_active 在 churnDays 之后但距今 ≤ churnDays 的窗口内（即"刚回来"）。
	// 优先级：new_user > active > churned > reactivated > retained。
	q := fmt.Sprintf(`
SELECT stage, COUNT(*) AS user_cnt FROM (
  SELECT
    CASE
      WHEN register_time >= DATE_SUB(?, INTERVAL ? DAY) THEN 'new_user'
      WHEN last_active_date >= DATE_SUB(?, INTERVAL 1 DAY) THEN 'active'
      WHEN last_active_date < DATE_SUB(?, INTERVAL ? DAY) THEN
        CASE WHEN last_active_date >= DATE_SUB(?, INTERVAL ? DAY) THEN 'reactivated' ELSE 'churned' END
      ELSE 'retained'
    END AS stage
  FROM users_dim
  WHERE %suser_id IS NOT NULL
) t GROUP BY stage`, tenantCond)
	args = append(args,
		newUserDays,
		now,          // active DATE_SUB(now, 1 day)
		now, churnDays, // churned DATE_SUB(now, churnDays)
		now, churnDays, // reactivated DATE_SUB(now, churnDays) —— 近 churnDays 内回流
	)

	type stageRow struct {
		Stage   string `db:"stage"`
		UserCnt int64  `db:"user_cnt"`
	}
	var rows []stageRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Lifecycle query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("lifecycle query failed: %v", err))
	}

	labels := map[string]string{
		"new_user":    "新用户",
		"active":      "活跃用户",
		"retained":    "留存用户",
		"churned":     "流失用户",
		"reactivated": "回流用户",
	}
	var total int64
	stageMap := map[string]int64{}
	for _, rw := range rows {
		stageMap[rw.Stage] = rw.UserCnt
		total += rw.UserCnt
	}
	order := []string{"new_user", "active", "retained", "reactivated", "churned"}
	stages := make([]*ubaV1.LifecycleStage, 0, len(order))
	for _, s := range order {
		cnt := stageMap[s]
		var pct float64
		if total > 0 {
			pct = float64(cnt) / float64(total)
		}
		stages = append(stages, &ubaV1.LifecycleStage{
			Stage:      s,
			StageLabel: labels[s],
			UserCount:  cnt,
			Percentage: pct,
		})
	}

	return &ubaV1.LifecycleResponse{Stages: stages, TotalUsers: total}, nil
}

// ============================================================================
// 流失与回流（静默天数判定流失 + 回流触发事件）
// ============================================================================

func (r *AnalyticsRepo) Churn(ctx context.Context, req *ubaV1.ChurnRequest) (*ubaV1.ChurnResponse, error) {
	_, endMs := normTimeRange(req.GetTimeRange())
	now := time.UnixMilli(endMs)
	churnDays := int64(req.GetChurnDays())
	if churnDays <= 0 {
		churnDays = 30
	}
	reactivationDays := int64(req.GetReactivationDays())
	if reactivationDays <= 0 {
		reactivationDays = 7
	}

	tenantCond := ""
	args := []any{now, churnDays, now}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 流失分桶：last_active_date < now - churnDays 的用户，按流失时长再细分。
	churnQ := fmt.Sprintf(`
SELECT
  CASE
    WHEN last_active_date < DATE_SUB(?, INTERVAL ? DAY) AND last_active_date >= DATE_SUB(?, INTERVAL 60 DAY) THEN '30_60d'
    WHEN last_active_date >= DATE_SUB(?, INTERVAL 90 DAY) THEN '60_90d'
    ELSE '90_plus'
  END AS bucket,
  COUNT(*) AS user_cnt
FROM users_dim
WHERE %slast_active_date < DATE_SUB(?, INTERVAL ? DAY) AND user_id IS NOT NULL
GROUP BY bucket ORDER BY bucket`, tenantCond)
	// SQL ? 顺序：CASE(4个: now,churnDays,now,now) + WHERE(2个: now,churnDays) = 6
	// args 已有 [now, churnDays, now]（CASE 前3个），追加 [now, now, churnDays]（CASE第4个 + WHERE 2个）
	churnArgs := append([]any{}, args...)
	churnArgs = append(churnArgs, now, now, churnDays)

	type bucketRow struct {
		Bucket  string `db:"bucket"`
		UserCnt int64  `db:"user_cnt"`
	}
	var bRows []bucketRow
	if err := r.db.SelectContext(ctx, &bRows, churnQ, churnArgs...); err != nil {
		r.log.Errorf("Churn bucket query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("churn query failed: %v", err))
	}
	var churnedUsers int64
	buckets := make([]*ubaV1.ChurnBucket, 0, len(bRows))
	for _, br := range bRows {
		churnedUsers += br.UserCnt
		buckets = append(buckets, &ubaV1.ChurnBucket{Bucket: br.Bucket, UserCount: br.UserCnt})
	}

	// 回流：流失用户（last_active 曾 < now - churnDays）中，last_active 在近 reactivationDays 内重新活跃。
	// 近似：last_active_date 在 [now - reactivationDays, now] 内，且 first_active_date < now - churnDays
	// （注册较早，排除本来就是新用户）。
	reactQ := fmt.Sprintf(`
SELECT COUNT(*) AS cnt FROM users_dim
WHERE %slast_active_date >= DATE_SUB(?, INTERVAL ? DAY)
  AND last_active_date < DATE_SUB(?, INTERVAL ? DAY)
  AND first_active_date < DATE_SUB(?, INTERVAL ? DAY)
  AND user_id IS NOT NULL`, tenantCond)
	reactArgs := []any{}
	if v := req.GetAppId(); v != 0 {
		reactArgs = append(reactArgs, v)
	}
	reactArgs = append(reactArgs, now, reactivationDays, now, churnDays, now, churnDays)
	var reactivated int64
	if err := r.db.GetContext(ctx, &reactivated, fmt.Sprintf(`
SELECT COUNT(*) FROM users_dim
WHERE %slast_active_date >= DATE_SUB(?, INTERVAL ? DAY)
  AND first_active_date < DATE_SUB(?, INTERVAL ? DAY)
  AND user_id IS NOT NULL`, tenantCond), reactArgs...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("Churn reactivation query failed: %v", err)
		reactivated = 0
	}
	_ = reactQ

	var reactivationRate float64
	if churnedUsers > 0 {
		reactivationRate = float64(reactivated) / float64(churnedUsers)
	}

	// 回流触发事件 TOP：近 reactivationDays 内、回流用户触发最多的 event_name。
	// MVP：用回流判定窗口的活跃用户（last_active 在该窗口）在 events_fact 的事件分布。
	triggerArgs := []any{}
	if v := req.GetAppId(); v != 0 {
		triggerArgs = append(triggerArgs, v)
	}
	triggerArgs = append(triggerArgs, now, reactivationDays, now, churnDays)
	triggerQ := fmt.Sprintf(`
SELECT e.event_name, COUNT(*) AS cnt
FROM events_fact e
INNER JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.%sevent_time >= DATE_SUB(?, INTERVAL ? DAY)
  AND u.first_active_date < DATE_SUB(?, INTERVAL ? DAY)
  AND e.user_id > 0
GROUP BY e.event_name ORDER BY cnt DESC LIMIT 20`, strings.Replace(tenantCond, "tenant_id", "e.tenant_id", 1))
	type triggerRow struct {
		EventName string `db:"event_name"`
		Cnt       int64  `db:"cnt"`
	}
	var tRows []triggerRow
	if err := r.db.SelectContext(ctx, &tRows, triggerQ, triggerArgs...); err != nil {
		r.log.Errorf("Churn trigger query failed: %v", err)
		tRows = nil
	}
	var triggerTotal int64
	for _, tr := range tRows {
		triggerTotal += tr.Cnt
	}
	triggers := make([]*ubaV1.ReactivationTrigger, 0, len(tRows))
	for _, tr := range tRows {
		var pct float64
		if triggerTotal > 0 {
			pct = float64(tr.Cnt) / float64(triggerTotal)
		}
		triggers = append(triggers, &ubaV1.ReactivationTrigger{
			EventName:  tr.EventName,
			Count:      tr.Cnt,
			Percentage: pct,
		})
	}

	return &ubaV1.ChurnResponse{
		ChurnBuckets:     buckets,
		ChurnedUsers:     churnedUsers,
		ReactivatedUsers: reactivated,
		ReactivationRate: reactivationRate,
		Triggers:         triggers,
	}, nil
}

// ============================================================================
// 间隔时间分析（两事件之间的时间间隔分布）
// 窗口函数 LEAD 配对同用户的 from→to 事件。
// ============================================================================

func (r *AnalyticsRepo) Interval(ctx context.Context, req *ubaV1.IntervalRequest) (*ubaV1.IntervalResponse, error) {
	if req.GetEventFrom() == "" || req.GetEventTo() == "" {
		return nil, ubaV1.ErrorBadRequest("event_from and event_to are required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	tenantCond := ""
	args := []any{req.GetEventFrom(), req.GetEventTo(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 把同用户的 from/to 事件按时间排列，用 LEAD 找到每个 from 之后紧邻的事件，
	// 若该紧邻事件是 to，则算间隔（小时）。这是 MVP 的"最近一次配对"语义。
	q := fmt.Sprintf(`
SELECT
  CASE
    WHEN diff_hours < 1.0/60 THEN 'instant'
    WHEN diff_hours < 1      THEN 'lt_1h'
    WHEN diff_hours < 24     THEN '1_24h'
    WHEN diff_hours < 168    THEN '1_7d'
    ELSE '7d_plus'
  END AS bucket,
  COUNT(*) AS cnt
FROM (
  SELECT
    user_id,
    event_time AS from_time,
    LEAD(event_name) OVER (PARTITION BY user_id ORDER BY event_time) AS next_name,
    LEAD(event_time) OVER (PARTITION BY user_id ORDER BY event_time) AS next_time,
    TIMESTAMPDIFF(MINUTE, event_time, LEAD(event_time) OVER (PARTITION BY user_id ORDER BY event_time)) / 60.0 AS diff_hours
  FROM events_fact
  WHERE %s(event_name = ? OR event_name = ?) AND event_time >= ? AND event_time < ? AND user_id > 0
) paired
WHERE next_name = ? AND diff_hours >= 0
GROUP BY bucket ORDER BY bucket`, tenantCond)
	args = append(args, req.GetEventTo())

	type bucketRow struct {
		Bucket string `db:"bucket"`
		Cnt    int64  `db:"cnt"`
	}
	var rows []bucketRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Interval query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("interval query failed: %v", err))
	}

	var total int64
	buckets := make([]*ubaV1.IntervalBucket, 0, len(rows))
	for _, br := range rows {
		total += br.Cnt
		buckets = append(buckets, &ubaV1.IntervalBucket{Bucket: br.Bucket, Count: br.Cnt})
	}
	for _, b := range buckets {
		if total > 0 {
			b.Percentage = float64(b.Count) / float64(total)
		}
	}

	// 分位数摘要
	summaryArgs := append([]any{}, args...) // 复用主查询参数
	summaryQ := fmt.Sprintf(`
SELECT
  COUNT(*) AS cnt,
  ROUND(AVG(diff_hours), 2) AS avg_hours,
  ROUND(PERCENTILE_APPROX(diff_hours, 0.5), 2) AS p50_hours,
  ROUND(PERCENTILE_APPROX(diff_hours, 0.9), 2) AS p90_hours
FROM (
  SELECT
    TIMESTAMPDIFF(MINUTE, event_time, LEAD(event_time) OVER (PARTITION BY user_id ORDER BY event_time)) / 60.0 AS diff_hours,
    LEAD(event_name) OVER (PARTITION BY user_id ORDER BY event_time) AS next_name
  FROM events_fact
  WHERE %s(event_name = ? OR event_name = ?) AND event_time >= ? AND event_time < ? AND user_id > 0
) p2 WHERE next_name = ? AND diff_hours >= 0`, tenantCond)
	var s struct {
		Cnt      int64   `db:"cnt"`
		AvgHours float64 `db:"avg_hours"`
		P50Hours float64 `db:"p50_hours"`
		P90Hours float64 `db:"p90_hours"`
	}
	if err := r.db.GetContext(ctx, &s, summaryQ, summaryArgs...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("Interval summary query failed: %v", err)
	}

	return &ubaV1.IntervalResponse{
		Buckets:  buckets,
		P50Hours: s.P50Hours,
		P90Hours: s.P90Hours,
		AvgHours: s.AvgHours,
		Count:    s.Cnt,
	}, nil
}

// ============================================================================
// 矩阵/象限分析（双轴：使用人数 UV × 使用频次，按中位数分四象限）
// ============================================================================

func (r *AnalyticsRepo) Matrix(ctx context.Context, req *ubaV1.MatrixRequest) (*ubaV1.MatrixResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	dim := "event_name"
	if d := req.GetDimension(); d == "event_category" || d == "object_type" || d == "platform" {
		dim = d
	}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// X=使用人数(COUNT DISTINCT user_id)，Y=使用频次(COUNT *)
	q := fmt.Sprintf(`
SELECT %s AS label,
       COUNT(DISTINCT user_id) AS uv,
       COUNT(*) AS freq
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ? AND user_id > 0
GROUP BY %s
ORDER BY uv DESC
LIMIT 100`, dim, tenantCond, dim)

	type ptRow struct {
		Label string  `db:"label"`
		UV    int64   `db:"uv"`
		Freq  int64   `db:"freq"`
	}
	var rows []ptRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Matrix query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("matrix query failed: %v", err))
	}
	if len(rows) == 0 {
		return &ubaV1.MatrixResponse{Points: []*ubaV1.MatrixPoint{}, Dimension: dim}, nil
	}

	// 计算中位数阈值（X=uv, Y=freq）
	sortedUV := make([]int64, len(rows))
	sortedFreq := make([]int64, len(rows))
	for i, rw := range rows {
		sortedUV[i] = rw.UV
		sortedFreq[i] = rw.Freq
	}
	sort.Slice(sortedUV, func(i, j int) bool { return sortedUV[i] < sortedUV[j] })
	sort.Slice(sortedFreq, func(i, j int) bool { return sortedFreq[i] < sortedFreq[j] })
	xThreshold := float64(sortedUV[len(sortedUV)/2])
	yThreshold := float64(sortedFreq[len(sortedFreq)/2])

	points := make([]*ubaV1.MatrixPoint, 0, len(rows))
	for _, rw := range rows {
		x := float64(rw.UV)
		y := float64(rw.Freq)
		var quadrant string
		highX := x >= xThreshold
		highY := y >= yThreshold
		switch {
		case highX && highY:
			quadrant = "core" // 核心：高人高频
		case highX && !highY:
			quadrant = "star" // 明星：高人低频
		case !highX && highY:
			quadrant = "niche" // 小众：低人高频
		default:
			quadrant = "edge" // 边缘：低人低频
		}
		points = append(points, &ubaV1.MatrixPoint{
			Label:     rw.Label,
			X:         x,
			Y:         y,
			Quadrant:  quadrant,
		})
	}

	return &ubaV1.MatrixResponse{
		Points:      points,
		XThreshold:  xThreshold,
		YThreshold:  yThreshold,
		Dimension:   dim,
	}, nil
}

// ============================================================================
// 付费/营收分析（ARPU/ARPPU/付费率/GMV 趋势）
// Doris 数据源：mv_events_daily（已有 uv/pv/pay_user_count/total_amount）。
// ============================================================================

func (r *AnalyticsRepo) Revenue(ctx context.Context, req *ubaV1.RevenueRequest) (*ubaV1.RevenueResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 直接查 events_fact（不依赖 mv_events_daily，避免物化视图无数据/列名不匹配）。
	q := fmt.Sprintf(`
SELECT to_date(event_time) AS d,
       ROUND(SUM(amount), 2) AS gmv,
       COUNT(DISTINCT IF(amount > 0, user_id, NULL)) AS pay_users,
       COUNT_IF(amount > 0) AS pay_orders,
       COUNT(DISTINCT user_id) AS active_users
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ?
GROUP BY d ORDER BY d`, tenantCond)

	type row struct {
		D            time.Time `db:"d"`
		Gmv          float64   `db:"gmv"`
		PayUsers     uint64    `db:"pay_users"`
		PayOrders    int64     `db:"pay_orders"`
		ActiveUsers  uint64    `db:"active_users"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Revenue query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("revenue query failed: %v", err))
	}

	var totalGmv float64
	var totalPayUsers uint64
	var totalPayOrders int64
	points := make([]*ubaV1.RevenuePoint, 0, len(rows))
	for _, rw := range rows {
		totalGmv += rw.Gmv
		totalPayUsers += rw.PayUsers
		totalPayOrders += rw.PayOrders
		var arpu, arppu, payRate float64
		if rw.ActiveUsers > 0 {
			arpu = rw.Gmv / float64(rw.ActiveUsers)
			payRate = float64(rw.PayUsers) / float64(rw.ActiveUsers)
		}
		if rw.PayUsers > 0 {
			arppu = rw.Gmv / float64(rw.PayUsers)
		}
		points = append(points, &ubaV1.RevenuePoint{
			Timestamp: rw.D.UnixMilli(),
			Gmv:       rw.Gmv,
			PayUsers:  int64(rw.PayUsers),
			PayOrders: rw.PayOrders,
			Arpu:      arpu,
			Arppu:     arppu,
			PayRate:   payRate,
		})
	}
	var avgOrderValue float64
	if totalPayOrders > 0 {
		avgOrderValue = totalGmv / float64(totalPayOrders)
	}

	return &ubaV1.RevenueResponse{
		Points:         points,
		TotalGmv:       totalGmv,
		TotalPayUsers:  int64(totalPayUsers),
		TotalPayOrders: totalPayOrders,
		AvgOrderValue:  avgOrderValue,
	}, nil
}

// ============================================================================
// 会话分析（跳出率/时长分位 P50/P90/会话深度）
// Doris 数据源：sessions_agg_daily（duration_quantile 用 QUANTILE_PERCENT 还原）+ events_fact（深度）。
// ============================================================================

func (r *AnalyticsRepo) SessionAnalysis(ctx context.Context, req *ubaV1.SessionAnalysisRequest) (*ubaV1.SessionAnalysisResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	// 动态拼 WHERE 子句，参数顺序：[tenant?] [platform?] start, end
	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	if p := req.GetPlatform(); p != "" {
		whereParts = append(whereParts, "platform = ?")
		args = append(args, p)
	}
	whereCond := ""
	if len(whereParts) > 0 {
		whereCond = strings.Join(whereParts, " AND ") + " AND "
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))

	// 会话聚合：SUM(session_count)，HLL 去重用户，时长分位用 QUANTILE_PERCENT。
	q := fmt.Sprintf(`
SELECT
  SUM(session_count) AS session_count,
  HLL_CARDINALITY(HLL_UNION(unique_users)) AS unique_users,
  ROUND(SUM(duration_sum) / NULLIF(SUM(duration_count), 0) / 1000.0, 2) AS avg_duration_sec,
  ROUND(QUANTILE_PERCENT(QUANTILE_UNION(duration_quantile), 0.5) / 1000.0, 2) AS p50_sec,
  ROUND(QUANTILE_PERCENT(QUANTILE_UNION(duration_quantile), 0.9) / 1000.0, 2) AS p90_sec,
  ROUND(SUM(bounce_sum) / NULLIF(SUM(bounce_count), 0), 4) AS bounce_rate
FROM sessions_agg_daily
WHERE %sstat_date >= DATE(?) AND stat_date < DATE(?)`, whereCond)

	var s struct {
		SessionCount int64   `db:"session_count"`
		UniqueUsers  uint64  `db:"unique_users"`
		AvgDurSec    float64 `db:"avg_duration_sec"`
		P50Sec       float64 `db:"p50_sec"`
		P90Sec       float64 `db:"p90_sec"`
		BounceRate   float64 `db:"bounce_rate"`
	}
	if err := r.db.GetContext(ctx, &s, q, args...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("SessionAnalysis query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("session analysis query failed: %v", err))
	}

	// 会话深度：人均事件数（events_fact 总事件数 / 会话数），扫事实表。
	depthTenantCond := ""
	depthArgs := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		depthTenantCond = "tenant_id = ? AND "
		depthArgs = append([]any{v}, depthArgs...)
	}
	depthQ := fmt.Sprintf(`SELECT COUNT(*) FROM events_fact WHERE %sevent_time >= ? AND event_time < ?`, depthTenantCond)
	var totalEvents int64
	if err := r.db.GetContext(ctx, &totalEvents, depthQ, depthArgs...); err != nil && err != sql.ErrNoRows {
		totalEvents = 0
	}
	var avgDepth float64
	if s.SessionCount > 0 {
		avgDepth = float64(totalEvents) / float64(s.SessionCount)
	}

	return &ubaV1.SessionAnalysisResponse{
		SessionCount:   s.SessionCount,
		UniqueUsers:    int64(s.UniqueUsers),
		AvgDurationSec: s.AvgDurSec,
		P50DurationSec: s.P50Sec,
		P90DurationSec: s.P90Sec,
		BounceRate:     s.BounceRate,
		AvgDepth:       avgDepth,
	}, nil
}

// ============================================================================
// 同比环比/异常检测（事件 PV/UV 环比 + 7日基线异常）
// Doris 数据源：mv_events_daily + LAG 窗口算环比、7日均值基线。
// 参考模板：doris/query.sql §13。
// ============================================================================

func (r *AnalyticsRepo) Anomaly(ctx context.Context, req *ubaV1.AnomalyRequest) (*ubaV1.AnomalyResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	// 动态 WHERE，参数顺序：[tenant?] [event_name?] start, end
	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	if en := req.GetEventName(); en != "" {
		whereParts = append(whereParts, "event_name = ?")
		args = append(args, en)
	}
	whereCond := ""
	if len(whereParts) > 0 {
		whereCond = strings.Join(whereParts, " AND ") + " AND "
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))

	// 直接查 events_fact（不依赖 mv_events_daily），窗口函数算环比/基线。
	q := fmt.Sprintf(`
SELECT event_name, d, pv, uv, baseline, wow_change FROM (
  SELECT
    event_name,
    d,
    pv,
    uv,
    AVG(pv) OVER (PARTITION BY event_name ORDER BY d ROWS BETWEEN 7 PRECEDING AND 1 PRECEDING) AS baseline,
    IF(LAG(pv) OVER (PARTITION BY event_name ORDER BY d) > 0,
       (pv - LAG(pv) OVER (PARTITION BY event_name ORDER BY d)) / LAG(pv) OVER (PARTITION BY event_name ORDER BY d),
       0) AS wow_change
  FROM (
    SELECT event_name, to_date(event_time) AS d,
           COUNT(*) AS pv, COUNT(DISTINCT user_id) AS uv
    FROM events_fact
    WHERE %sevent_time >= ? AND event_time < ?
    GROUP BY event_name, d
  ) daily
) ranked
WHERE baseline > 0
ORDER BY event_name, d`, whereCond)

	type row struct {
		EventName string    `db:"event_name"`
		D         time.Time `db:"d"`
		Pv        int64     `db:"pv"`
		Uv        uint64    `db:"uv"`
		Baseline  float64   `db:"baseline"`
		WowChange float64   `db:"wow_change"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Anomaly query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("anomaly query failed: %v", err))
	}

	points := make([]*ubaV1.AnomalyPoint, 0, len(rows))
	anomalySet := map[string]bool{}
	for _, rw := range rows {
		isAnomaly := rw.Baseline > 0 && float64(rw.Pv) < rw.Baseline*0.5
		if isAnomaly {
			anomalySet[rw.EventName] = true
		}
		points = append(points, &ubaV1.AnomalyPoint{
			EventName: rw.EventName,
			StatDate:  rw.D.UnixMilli(),
			Pv:        rw.Pv,
			Uv:        int64(rw.Uv),
			Baseline:  rw.Baseline,
			WowChange: rw.WowChange,
			IsAnomaly: isAnomaly,
		})
	}

	return &ubaV1.AnomalyResponse{
		Points:       points,
		AnomalyCount: int64(len(anomalySet)),
	}, nil
}

// ============================================================================
// 新老用户对比（构成占比 + 事件/付费差异）
// Doris 数据源：users_dim（first_active_date 判新老）JOIN events_fact。
// 参考模板：doris/query.sql §15。
// ============================================================================

func (r *AnalyticsRepo) NewVsOld(ctx context.Context, req *ubaV1.NewVsOldRequest) (*ubaV1.NewVsOldResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	newUserDays := int64(req.GetNewUserDays())
	if newUserDays <= 0 {
		newUserDays = 7
	}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 用 first_active_date 判新老：>= now - newUserDays 天为新用户。
	q := fmt.Sprintf(`
SELECT
  IF(u.first_active_date >= DATE_SUB(CURDATE(), INTERVAL ? DAY), 'new', 'old') AS user_type,
  COUNT(DISTINCT e.user_id) AS user_count,
  COUNT(*) AS event_count,
  COUNT(DISTINCT IF(e.amount > 0, e.user_id, NULL)) AS pay_users
FROM events_fact e
JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.%sevent_time >= ? AND event_time < ? AND e.user_id > 0
GROUP BY user_type`, tenantCond)

	type row struct {
		UserType   string `db:"user_type"`
		UserCount  int64  `db:"user_count"`
		EventCount int64  `db:"event_count"`
		PayUsers   int64  `db:"pay_users"`
	}
	var rows []row
	qArgs := append([]any{newUserDays}, args...)
	if err := r.db.SelectContext(ctx, &rows, q, qArgs...); err != nil {
		r.log.Errorf("NewVsOld query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("new vs old query failed: %v", err))
	}

	segMap := map[string]*ubaV1.NewVsOldSegment{}
	for _, rw := range rows {
		var payRate float64
		if rw.UserCount > 0 {
			payRate = float64(rw.PayUsers) / float64(rw.UserCount)
		}
		segMap[rw.UserType] = &ubaV1.NewVsOldSegment{
			UserType:   rw.UserType,
			UserCount:  rw.UserCount,
			EventCount: rw.EventCount,
			PayUsers:   rw.PayUsers,
			PayRate:    payRate,
		}
	}
	order := []string{"new", "old"}
	segments := make([]*ubaV1.NewVsOldSegment, 0, 2)
	for _, t := range order {
		if seg, ok := segMap[t]; ok {
			segments = append(segments, seg)
		}
	}
	return &ubaV1.NewVsOldResponse{Segments: segments}, nil
}

// ============================================================================
// 热门转化路径（群体路径 TOP + 转化率）
// Doris 数据源：popular_paths_daily（event_sequence ARRAY + support_count + conversion）。
// 参考模板：doris/query.sql §6.1。
// ============================================================================

func (r *AnalyticsRepo) PathSankey(ctx context.Context, req *ubaV1.PathSankeyRequest) (*ubaV1.PathSankeyResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	topN := int64(req.GetTopN())
	if topN <= 0 || topN > 200 {
		topN = 20
	}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	q := fmt.Sprintf(`
SELECT array_join(event_sequence, ' → ') AS event_sequence,
       SUM(support_count) AS support_count,
       HLL_CARDINALITY(HLL_UNION(unique_users)) AS unique_users,
       IF(SUM(conversion_count) > 0, SUM(conversion_sum) / SUM(conversion_count), 0) AS conversion_rate
FROM popular_paths_daily
WHERE %sstat_date >= DATE(?) AND stat_date < DATE(?)
GROUP BY event_sequence
ORDER BY support_count DESC
LIMIT %d`, tenantCond, topN)

	type row struct {
		EventSequence  string  `db:"event_sequence"`
		SupportCount   int64   `db:"support_count"`
		UniqueUsers    uint64  `db:"unique_users"`
		ConversionRate float64 `db:"conversion_rate"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("PathSankey query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("path sankey query failed: %v", err))
	}

	paths := make([]*ubaV1.PathBucket, 0, len(rows))
	for _, rw := range rows {
		paths = append(paths, &ubaV1.PathBucket{
			EventSequence:  rw.EventSequence,
			SupportCount:   rw.SupportCount,
			UniqueUsers:    int64(rw.UniqueUsers),
			ConversionRate: rw.ConversionRate,
		})
	}
	return &ubaV1.PathSankeyResponse{Paths: paths}, nil
}

// ============================================================================
// 关卡/数值平衡分析（通过率/失败率/卡关率/分数分布/满星率）
// Doris 数据源：events_fact（object_type='level'，event_name=level_start/finish/fail）。
// 参考模板：doris/query.sql §11.2。
// ============================================================================

func (r *AnalyticsRepo) LevelAnalysis(ctx context.Context, req *ubaV1.LevelAnalysisRequest) (*ubaV1.LevelAnalysisResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	whereParts = append(whereParts, "object_type = 'level'")
	if lid := req.GetLevelId(); lid != "" {
		whereParts = append(whereParts, "object_id = ?")
		args = append(args, lid)
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))
	whereCond := strings.Join(whereParts, " AND ") + " AND "

	// 关卡维度聚合：统计各关的尝试/完成/失败/分数/满星/玩家数。
	q := fmt.Sprintf(`
SELECT
  object_id AS level_id,
  MAX(object_name) AS level_name,
  SUM(IF(event_name = 'level_start',  1, 0)) AS attempt_count,
  SUM(IF(event_name = 'level_finish', 1, 0)) AS finish_count,
  SUM(IF(event_name = 'level_fail',   1, 0)) AS fail_count,
  ROUND(AVG(IF(event_name = 'level_finish', metrics['score'], NULL)), 1) AS avg_score,
  SUM(IF(event_name = 'level_finish' AND context['stars'] = '3', 1, 0)) AS star3_count,
  HLL_CARDINALITY(HLL_UNION(HLL_HASH(user_id))) AS player_count
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ?
GROUP BY object_id
ORDER BY player_count DESC
LIMIT 100`, whereCond)

	type row struct {
		LevelId      string  `db:"level_id"`
		LevelName    string  `db:"level_name"`
		AttemptCnt   int64   `db:"attempt_count"`
		FinishCnt    int64   `db:"finish_count"`
		FailCnt      int64   `db:"fail_count"`
		AvgScore     float64 `db:"avg_score"`
		Star3Count   int64   `db:"star3_count"`
		PlayerCount  uint64  `db:"player_count"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("LevelAnalysis query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("level analysis query failed: %v", err))
	}

	levels := make([]*ubaV1.LevelStat, 0, len(rows))
	for _, rw := range rows {
		var passRate, star3Rate float64
		total := rw.FinishCnt + rw.FailCnt
		if total > 0 {
			passRate = float64(rw.FinishCnt) / float64(total)
		}
		if rw.FinishCnt > 0 {
			star3Rate = float64(rw.Star3Count) / float64(rw.FinishCnt)
		}
		levels = append(levels, &ubaV1.LevelStat{
			LevelId:      rw.LevelId,
			LevelName:    rw.LevelName,
			AttemptCount: rw.AttemptCnt,
			FinishCount:  rw.FinishCnt,
			FailCount:    rw.FailCnt,
			PassRate:     passRate,
			StuckRate:    1 - passRate,
			AvgScore:     rw.AvgScore,
			Star3Rate:    star3Rate,
			PlayerCount:  int64(rw.PlayerCount),
		})
	}
	return &ubaV1.LevelAnalysisResponse{Levels: levels}, nil
}

// ============================================================================
// 鲸鱼用户/付费分层（按累计充值自动分层，二八定律分析）
// Doris 数据源：users_dim（total_pay_amount）。
// 参考模板：doris/query.sql §4.2。
// ============================================================================

func (r *AnalyticsRepo) WhaleTier(ctx context.Context, req *ubaV1.WhaleTierRequest) (*ubaV1.WhaleTierResponse, error) {
	tenantCond := ""
	args := []any{}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append(args, v)
	}

	// 按 total_pay_amount 分层：whale(>=1万) / dolphin(>=1千) / minnow(>0) / non_pay(0)
	q := fmt.Sprintf(`
SELECT
  CASE
    WHEN total_pay_amount >= 10000 THEN 'whale'
    WHEN total_pay_amount >= 1000  THEN 'dolphin'
    WHEN total_pay_amount > 0      THEN 'minnow'
    ELSE 'non_pay'
  END AS tier,
  COUNT(*) AS user_count,
  ROUND(SUM(total_pay_amount), 2) AS total_amount
FROM users_dim
WHERE %suser_id IS NOT NULL
GROUP BY tier`, tenantCond)

	type row struct {
		Tier        string  `db:"tier"`
		UserCount   int64   `db:"user_count"`
		TotalAmount float64 `db:"total_amount"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("WhaleTier query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("whale tier query failed: %v", err))
	}

	labels := map[string]string{
		"whale": "大课长", "dolphin": "中课长", "minnow": "小课长", "non_pay": "免费玩家",
	}
	var totalUsers int64
	var totalRevenue float64
	tierMap := map[string]*row{}
	for i := range rows {
		totalUsers += rows[i].UserCount
		totalRevenue += rows[i].TotalAmount
		tierMap[rows[i].Tier] = &rows[i]
	}
	order := []string{"whale", "dolphin", "minnow", "non_pay"}
	segments := make([]*ubaV1.PayTierSegment, 0, len(order))
	for _, t := range order {
		rw, ok := tierMap[t]
		if !ok {
			continue
		}
		var pct, revShare, arppu float64
		if totalUsers > 0 {
			pct = float64(rw.UserCount) / float64(totalUsers)
		}
		if totalRevenue > 0 {
			revShare = rw.TotalAmount / totalRevenue
		}
		if rw.UserCount > 0 {
			arppu = rw.TotalAmount / float64(rw.UserCount)
		}
		segments = append(segments, &ubaV1.PayTierSegment{
			Tier:         t,
			TierLabel:    labels[t],
			UserCount:    rw.UserCount,
			Percentage:   pct,
			TotalAmount:  rw.TotalAmount,
			RevenueShare: revShare,
			Arppu:        arppu,
		})
	}
	return &ubaV1.WhaleTierResponse{
		Segments:     segments,
		TotalUsers:   totalUsers,
		TotalRevenue: totalRevenue,
	}, nil
}

// ============================================================================
// 历史生命周期价值 LTV（用户在第 N 天的累计付费价值，支持按渠道分组配合归因）
// Doris 数据源：users_dim（register_time）JOIN events_fact（amount）。
// 算法：按注册同期群，累计付费金额 / 同期群人数 = LTV(N)。
// ============================================================================

func (r *AnalyticsRepo) LTV(ctx context.Context, req *ubaV1.LTVRequest) (*ubaV1.LTVResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	// 观察天数（默认 30，最大 90）
	maxDays := []uint32{0, 1, 3, 7, 14, 30, 60, 90}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 按维度分组（默认无分组，整体 LTV；可选 channel 配合归因）。
	// 同期群：register_time 在时间范围内的用户。
	// LTV(N) = 该群截至第 N 天的累计付费总额 / 该群人数。
	// 用 JOIN：每个付费事件关联用户的 register_time，算 event_day = DATEDIFF(event_time, register_time)。
	dimSelect := "'' AS label"
	if dim := req.GetDimension(); dim == "channel" {
		dimSelect = "u.register_channel AS label"
	}

	// 先取同期群规模（按维度分组的人数）
	cohortQ := fmt.Sprintf(`
SELECT %s, COUNT(*) AS cohort_size
FROM users_dim u
WHERE u.%sregister_time >= ? AND register_time < ?
GROUP BY label`, dimSelect, tenantCond)
	type cohortRow struct {
		Label      string `db:"label"`
		CohortSize int64  `db:"cohort_size"`
	}
	var cohortRows []cohortRow
	if err := r.db.SelectContext(ctx, &cohortRows, cohortQ, args...); err != nil {
		r.log.Errorf("LTV cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("ltv query failed: %v", err))
	}
	cohortMap := map[string]int64{}
	for _, cr := range cohortRows {
		cohortMap[cr.Label] = cr.CohortSize
	}

	// 取同期群的付费事件，算 event_day 并按 (label, day_bucket) 累计。
	// 注：DATEDIFF(event_time, register_time) 为事件距注册天数。
	payArgs := append([]any{}, args...)
	payQ := fmt.Sprintf(`
SELECT %s,
  CASE
    WHEN DATEDIFF(e.event_time, u.register_time) <= 0  THEN 0
    WHEN DATEDIFF(e.event_time, u.register_time) <= 1  THEN 1
    WHEN DATEDIFF(e.event_time, u.register_time) <= 3  THEN 3
    WHEN DATEDIFF(e.event_time, u.register_time) <= 7  THEN 7
    WHEN DATEDIFF(e.event_time, u.register_time) <= 14 THEN 14
    WHEN DATEDIFF(e.event_time, u.register_time) <= 30 THEN 30
    WHEN DATEDIFF(e.event_time, u.register_time) <= 60 THEN 60
    ELSE 90
  END AS day_n,
  ROUND(SUM(e.amount), 2) AS total_amount
FROM events_fact e
JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE u.%sregister_time >= ? AND u.register_time < ? AND e.amount > 0 AND e.user_id > 0
GROUP BY label, day_n`, dimSelect, tenantCond)
	type payRow struct {
		Label       string  `db:"label"`
		DayN        int64   `db:"day_n"`
		TotalAmount float64 `db:"total_amount"`
	}
	var payRows []payRow
	if err := r.db.SelectContext(ctx, &payRows, payQ, payArgs...); err != nil {
		r.log.Errorf("LTV pay query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("ltv query failed: %v", err))
	}

	// 累计：每个 label 按 day_n 升序累加金额（累计到第 N 天）。
	bucketSum := map[string]map[int64]float64{}
	for _, pr := range payRows {
		if bucketSum[pr.Label] == nil {
			bucketSum[pr.Label] = map[int64]float64{}
		}
		bucketSum[pr.Label][pr.DayN] = pr.TotalAmount
	}

	points := make([]*ubaV1.LTVPoint, 0)
	for label, size := range cohortMap {
		// 按观察天数升序累计
		var cumulative float64
		dayBuckets := bucketSum[label]
		for _, dn := range maxDays {
			if amt, ok := dayBuckets[int64(dn)]; ok {
				cumulative += amt
			}
			var ltv float64
			if size > 0 {
				ltv = cumulative / float64(size)
			}
			points = append(points, &ubaV1.LTVPoint{
				Label:       label,
				DayN:        dn,
				Ltv:         ltv,
				CohortSize:  size,
				TotalAmount: cumulative,
			})
		}
	}

	return &ubaV1.LTVResponse{Points: points, MaxDays: 90}, nil
}

// ============================================================================
// 滚服留存（按区服分组，复用留存算法）
// 数据源：events_fact（新加 server_id 列）。
// ============================================================================

func (r *AnalyticsRepo) ServerRetention(ctx context.Context, req *ubaV1.ServerRetentionRequest) (*ubaV1.ServerRetentionResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	maxOffset := int64(req.GetMaxOffsetDays())
	if maxOffset <= 0 {
		maxOffset = 7
	}
	offsetDays := make([]uint32, 0, maxOffset+1)
	for d := int64(0); d <= maxOffset; d++ {
		offsetDays = append(offsetDays, uint32(d))
	}

	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	whereParts = append(whereParts, "server_id != ''")
	if sid := req.GetServerId(); sid != "" {
		whereParts = append(whereParts, "server_id = ?")
		args = append(args, sid)
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))
	whereCond := strings.Join(whereParts, " AND ") + " AND "

	// 同期群规模：按 server_id 分组，统计每个用户在该 server 的首事件日（首日新增）。
	// users_dim 无 server_id，故以 events_fact 按用户首事件近似首日新增。
	cohortQ := fmt.Sprintf(`
SELECT server_id, COUNT(*) AS cohort_size FROM (
  SELECT user_id, server_id, MIN(to_date(event_time)) AS first_day
  FROM events_fact
  WHERE %sevent_time >= ? AND event_time < ? AND server_id != '' AND user_id > 0
  GROUP BY user_id, server_id
) t GROUP BY server_id ORDER BY cohort_size DESC LIMIT 50`, whereCond)

	type cohortRow struct {
		ServerID   string `db:"server_id"`
		CohortSize int64  `db:"cohort_size"`
	}
	var cohortRows []cohortRow
	if err := r.db.SelectContext(ctx, &cohortRows, cohortQ, args...); err != nil {
		r.log.Errorf("ServerRetention cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("server retention query failed: %v", err))
	}

	// 各 server 的各偏移天留存用户数：JOIN 同期群首日，DATEDIFF 算偏移。
	rows := make([]*ubaV1.ServerRetentionRow, 0, len(cohortRows))
	for _, cr := range cohortRows {
		rates := map[string]float64{"0": 1.0}
		// 逐偏移天查询该 server 的留存用户（去重）
		for _, d := range offsetDays {
			if d == 0 {
				continue
			}
			retArgs := []any{cr.ServerID, time.UnixMilli(startMs), time.UnixMilli(endMs)}
			retQ := `
SELECT COUNT(DISTINCT e.user_id) AS retained
FROM events_fact e
JOIN (
  SELECT user_id, MIN(to_date(event_time)) AS first_day
  FROM events_fact
  WHERE server_id = ? AND event_time >= ? AND event_time < ? AND user_id > 0
  GROUP BY user_id
) f ON f.user_id = e.user_id
WHERE e.server_id = ? AND DATEDIFF(to_date(e.event_time), f.first_day) = ?`
			if v := req.GetAppId(); v != 0 {
				retQ = strings.Replace(retQ, "WHERE e.server_id", "WHERE e.tenant_id = ? AND e.server_id", 1)
				retArgs = append([]any{v}, retArgs...)
			}
			retArgs = append(retArgs, cr.ServerID, int64(d))
			var retained int64
			if err := r.db.GetContext(ctx, &retained, retQ, retArgs...); err != nil && err != sql.ErrNoRows {
				retained = 0
			}
			if cr.CohortSize > 0 {
				rates[fmt.Sprintf("%d", d)] = float64(retained) / float64(cr.CohortSize)
			}
		}
		rows = append(rows, &ubaV1.ServerRetentionRow{
			ServerId:       cr.ServerID,
			CohortSize:     cr.CohortSize,
			RetentionRates: rates,
		})
	}
	return &ubaV1.ServerRetentionResponse{Rows: rows, OffsetDays: offsetDays}, nil
}

// ============================================================================
// 同时在线 PCU/ACU（基于 sessions_fact 会话区间推算）
// ACU = Σ(duration) / 时长分钟；PCU 估算 = 活跃会话峰值（近似）。
// ============================================================================

func (r *AnalyticsRepo) OnlineStats(ctx context.Context, req *ubaV1.OnlineStatsRequest) (*ubaV1.OnlineStatsResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	// sessions_fact 无 server_id 列，不支持区服过滤（仅 tenant + 时间）。
	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))
	whereCond := ""
	if len(whereParts) > 0 {
		whereCond = strings.Join(whereParts, " AND ") + " AND "
	}

	q := fmt.Sprintf(`
SELECT
  COUNT(*) AS total_sessions,
  IFNULL(SUM(duration_ms), 0) AS total_duration_ms
FROM sessions_fact
WHERE %sstart_time >= ? AND start_time < ?`, whereCond)
	var s struct {
		TotalSessions   int64 `db:"total_sessions"`
		TotalDurationMs int64 `db:"total_duration_ms"`
	}
	if err := r.db.GetContext(ctx, &s, q, args...); err != nil && err != sql.ErrNoRows {
		r.log.Errorf("OnlineStats query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("online stats query failed: %v", err))
	}

	spanMs := endMs - startMs
	durationMinutes := spanMs / int64(time.Minute/time.Millisecond)
	if durationMinutes <= 0 {
		durationMinutes = 1
	}
	totalOnlineMinutes := s.TotalDurationMs / 60000
	acu := totalOnlineMinutes / durationMinutes
	// PCU 近似：平均每分钟并发会话数 * 峰值系数（无分钟桶时用 ACU 的 2-3 倍估算上限）。
	// 精确 PCU 需分钟级会话重叠统计，MVP 用活跃会话估算：取统计窗口内并发会话的合理上界。
	// 这里用「窗口内会话总数 / (时长/平均会话时长)」的倒数为并发估算，并取 max(acu, 并发估算)。
	avgSessionMin := int64(0)
	if s.TotalSessions > 0 {
		avgSessionMin = totalOnlineMinutes / s.TotalSessions
	}
	var pcu int64 = acu
	if avgSessionMin > 0 {
		concurrency := s.TotalSessions * avgSessionMin / durationMinutes
		if concurrency > pcu {
			pcu = concurrency
		}
	}

	return &ubaV1.OnlineStatsResponse{
		Pcu:             pcu,
		Acu:             acu,
		DurationMinutes: durationMinutes,
		TotalSessions:   s.TotalSessions,
	}, nil
}

// ============================================================================
// 经济系统/代币流向（产出 Source / 消耗 Sink 平衡）
// 数据源：events_fact（amount 正=产出 负=消耗 + object_type='item'）。
// ============================================================================

func (r *AnalyticsRepo) Economy(ctx context.Context, req *ubaV1.EconomyRequest) (*ubaV1.EconomyResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	whereParts = append(whereParts, "object_type = 'item'")
	if c := req.GetCurrency(); c != "" {
		// events_fact 无 currency 列，按 object_name 近似过滤（代币名）
		whereParts = append(whereParts, "object_name = ?")
		args = append(args, c)
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))
	whereCond := strings.Join(whereParts, " AND ") + " AND "

	// events_fact 无 currency 列，按 object_name（代币/道具名）分组；amount 正负分产出/消耗。
	q := fmt.Sprintf(`
SELECT
  IFNULL(NULLIF(object_name, ''), object_id) AS currency,
  ROUND(SUM(IF(amount > 0, amount, 0)), 2) AS source,
  ROUND(SUM(IF(amount < 0, -amount, 0)), 2) AS sink
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ? AND amount != 0
GROUP BY currency
ORDER BY source DESC`, whereCond)

	type row struct {
		Currency string  `db:"currency"`
		Source   float64 `db:"source"`
		Sink     float64 `db:"sink"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Economy query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError(fmt.Sprintf("economy query failed: %v", err))
	}

	currencies := make([]*ubaV1.CurrencyBalance, 0, len(rows))
	for _, rw := range rows {
		currencies = append(currencies, &ubaV1.CurrencyBalance{
			Currency: rw.Currency,
			Source:   rw.Source,
			Sink:     rw.Sink,
			Net:      rw.Source - rw.Sink,
		})
	}
	return &ubaV1.EconomyResponse{Currencies: currencies}, nil
}

// ============================================================================
// 工具函数
// ============================================================================

// derefStr 安全解引用字符串指针，nil 返回空串。
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

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
		"platform":       "platform",
		"channel":        "channel",
		"country":        "country",
		"app_version":    "app_version",
		"event_name":     "event_name",
		"event_category": "event_category",
		"os":             "os",
		"network":        "network",
		// 游戏维度：events_fact 无此列，需 JOIN users_dim（见 GroupBy 的 joinUsersDim）。
		"user_level": "user_level",
		"vip_level":  "vip_level",
	}
	v, ok := m[dim]
	return v, ok
}

// joinUsersDim 返回该维度是否需要 JOIN users_dim（user_level/vip_level 在维度表，非事实表）。
func joinUsersDim(dim string) bool {
	return dim == "user_level" || dim == "vip_level"
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

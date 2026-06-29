package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// AnalyticsRepo 基于 ClickHouse 的 BI 聚合查询仓库。
// 与 doris.AnalyticsRepo 对应，使用 ClickHouse 原生函数（toStartOfHour / toDate / toStartOfWeek / toStartOfMonth）。
type AnalyticsRepo struct {
	db  *clickhouseCrud.Client
	log *log.Helper
}

func NewAnalyticsRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *AnalyticsRepo {
	return &AnalyticsRepo{
		log: ctx.NewLoggerHelper("analytics/clickhouse/repo/core-service"),
		db:  db,
	}
}

// 时间分桶（ClickHouse 原生函数）
func chGranularityExpr(g ubaV1.AnalyticsGranularity) string {
	switch g {
	case ubaV1.AnalyticsGranularity_HOUR:
		return "toStartOfHour(event_time)"
	case ubaV1.AnalyticsGranularity_WEEK:
		return "toStartOfWeek(event_time)"
	case ubaV1.AnalyticsGranularity_MONTH:
		return "toStartOfMonth(event_time)"
	case ubaV1.AnalyticsGranularity_DAY:
		return "toDate(event_time)"
	default:
		return "toDate(event_time)"
	}
}

func (r *AnalyticsRepo) EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	bucket := chGranularityExpr(req.GetGranularity())

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
		"SELECT %s AS bucket, count() AS cnt FROM events_fact WHERE %s GROUP BY bucket ORDER BY bucket",
		bucket, strings.Join(where, " AND "),
	)

	type row struct {
		Bucket time.Time `db:"bucket" ch:"bucket"`
		Cnt    int64     `db:"cnt" ch:"cnt"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("EventTrend query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("event trend query failed")
	}

	points := make([]*ubaV1.TimeSeriesPoint, 0, len(rows))
	var total int64
	for _, rw := range rows {
		points = append(points, &ubaV1.TimeSeriesPoint{Timestamp: rw.Bucket.UnixMilli(), Value: float64(rw.Cnt)})
		total += rw.Cnt
	}

	return &ubaV1.EventTrendResponse{
		Points:      points,
		Granularity: effectiveGranularity(req.GetGranularity(), startMs, endMs),
		Total:       total,
	}, nil
}

func (r *AnalyticsRepo) Funnel(ctx context.Context, req *ubaV1.FunnelRequest) (*ubaV1.FunnelResponse, error) {
	steps := req.GetSteps()
	if len(steps) < 2 {
		return nil, ubaV1.ErrorBadRequest("funnel requires at least 2 steps")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	resp := &ubaV1.FunnelResponse{Steps: make([]*ubaV1.FunnelStep, 0, len(steps))}
	var prevCount int64
	for i, name := range steps {
		q := `SELECT count(DISTINCT user_id) AS cnt FROM events_fact
		      WHERE event_time >= ? AND event_time < ? AND event_name = ?`
		args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs), name}
		if v := req.GetAppId(); v != 0 {
			q += " AND tenant_id = ?"
			args = append(args, v)
		}
		q += " LIMIT 1"
		var cnt int64
		if err := r.db.QueryRow(ctx, &cnt, q, args...); err != nil {
			r.log.Errorf("Funnel step %d query failed: %v", i, err)
			return nil, ubaV1.ErrorInternalServerError("funnel query failed")
		}
		step := &ubaV1.FunnelStep{StepIndex: uint32(i + 1), EventName: name, Count: cnt}
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

func (r *AnalyticsRepo) Retention(ctx context.Context, req *ubaV1.RetentionRequest) (*ubaV1.RetentionResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	maxOffset := int(req.GetMaxOffsetDays())
	if maxOffset <= 0 {
		maxOffset = 7
	}

	cohortQ := `SELECT toDate(event_ts / 1000) AS d, count(DISTINCT user_id) AS sz
	            FROM events_fact
	            WHERE event_ts >= ? AND event_ts < ?
	            GROUP BY d ORDER BY d`
	type cohortRow struct {
		D time.Time `db:"d" ch:"d"`
		S int64     `db:"sz" ch:"sz"`
	}
	var crows []cohortRow
	if err := r.db.Select(ctx, &crows, cohortQ, startMs, endMs); err != nil {
		r.log.Errorf("Retention cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("retention query failed")
	}

	offsets := make([]uint32, 0, maxOffset+1)
	for i := 0; i <= maxOffset; i++ {
		offsets = append(offsets, uint32(i))
	}

	cohorts := make([]*ubaV1.RetentionCohort, 0, len(crows))
	for _, cr := range crows {
		cohortDate := cr.D
		c := &ubaV1.RetentionCohort{
			CohortDate: cohortDate.UnixMilli(),
			Size:       cr.S,
			Cells:      make([]*ubaV1.RetentionCell, 0, maxOffset+1),
		}
		for _, off := range offsets {
			dayStart := cohortDate.AddDate(0, 0, int(off))
			dayEnd := dayStart.AddDate(0, 0, 1)
			q := `SELECT count(DISTINCT user_id) AS cnt FROM events_fact
			      WHERE event_ts >= ? AND event_ts < ?`
			args := []any{dayStart.UnixMilli(), dayEnd.UnixMilli()}
			if ev := req.GetEventName(); req.GetRetentionType() == "EVENT" && ev != "" {
				q += " AND event_name = ?"
				args = append(args, ev)
			}
			q += " LIMIT 1"
			var cnt int64
			if err := r.db.QueryRow(ctx, &cnt, q, args...); err != nil {
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

	metric, err := metricExpr(req.GetMetric())
	if err != nil {
		return nil, ubaV1.ErrorBadRequest(err.Error())
	}

	// user_level/vip_level 在 users_dim，非 events_fact：需 JOIN。
	joinClause := ""
	dimCol := col
	if joinUsersDim(req.GetDimension()) {
		joinClause = " INNER JOIN users_dim u ON u.tenant_id = events_fact.tenant_id AND u.user_id = events_fact.user_id"
		dimCol = "u." + col
	}

	q := fmt.Sprintf(
		"SELECT %s AS label, %s AS value FROM events_fact %s WHERE event_time >= ? AND event_time < ?",
		dimCol, metric, joinClause,
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
		Label string  `db:"label" ch:"label"`
		Value float64 `db:"value" ch:"value"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
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

func (r *AnalyticsRepo) ActiveUsers(ctx context.Context, req *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	gran := req.GetGranularity()

	// 预聚合表 user_activity_daily 为日粒度：仅 DAY/WEEK/MONTH（含 UNSPECIFIED 默认按天）
	// 能输出真值 WAU/MAU；HOUR 粒度无小时级状态，回退为等于 DAU。
	if gran == ubaV1.AnalyticsGranularity_HOUR {
		return r.activeUsersFromEventsFact(ctx, req, startMs, endMs)
	}

	// 数据源：基础表 user_activity_daily（不是 view——view 已把 active_users 状态 merge 成
	// UInt64，丢失跨天可合并性）。active_users 为 AggregateFunction(uniqCombined, UInt32) 状态，
	// uniqCombinedMerge 可在滚动窗口内合并各天状态，得到准确去重（近似，误差 <1%）。
	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	q := fmt.Sprintf(`
SELECT d.stat_date AS bucket,
       uniqCombinedMerge(d.active_users) AS dau,
       (
           SELECT uniqCombinedMerge(active_users)
           FROM user_activity_daily
           WHERE %sstat_date BETWEEN d.stat_date - INTERVAL 6 DAY AND d.stat_date
       ) AS wau,
       (
           SELECT uniqCombinedMerge(active_users)
           FROM user_activity_daily
           WHERE %sstat_date BETWEEN d.stat_date - INTERVAL 29 DAY AND d.stat_date
       ) AS mau
FROM user_activity_daily d
WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)
GROUP BY d.stat_date
ORDER BY d.stat_date`,
		tenantCond, tenantCond, tenantCond)

	type row struct {
		Bucket time.Time `db:"bucket" ch:"bucket"`
		Dau    int64     `db:"dau" ch:"dau"`
		Wau    int64     `db:"wau" ch:"wau"`
		Mau    int64     `db:"mau" ch:"mau"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("ActiveUsers query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("active users query failed")
	}

	points := make([]*ubaV1.ActiveUsersPoint, 0, len(rows))
	for _, rw := range rows {
		points = append(points, &ubaV1.ActiveUsersPoint{
			Timestamp: rw.Bucket.UnixMilli(),
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
	bucket := chGranularityExpr(ubaV1.AnalyticsGranularity_HOUR)
	q := fmt.Sprintf(
		"SELECT %s AS bucket, count(DISTINCT user_id) AS dau FROM events_fact WHERE event_time >= ? AND event_time < ? GROUP BY bucket ORDER BY bucket",
		bucket,
	)
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		q = strings.Replace(q, "WHERE event_time", "WHERE tenant_id = ? AND event_time", 1)
		args = append([]any{v}, args...)
	}

	type row struct {
		Bucket time.Time `db:"bucket" ch:"bucket"`
		Dau    int64     `db:"dau" ch:"dau"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
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
// 归因分析（首触/末触渠道归因）
// 与 doris.AnalyticsRepo.Attribution 对应，使用 ClickHouse 原生函数。
// ============================================================================

func (r *AnalyticsRepo) Attribution(ctx context.Context, req *ubaV1.AttributionRequest) (*ubaV1.AttributionResponse, error) {
	if req.GetConversionEvent() == "" {
		return nil, ubaV1.ErrorBadRequest("conversion_event is required")
	}
	startMs, endMs := normTimeRange(req.GetTimeRange())

	dim := "channel"
	if d := req.GetDimension(); d == "referer" {
		dim = "referer"
	}
	orderDir := "DESC"
	if req.GetModel() == "first_touch" {
		orderDir = "ASC"
	}

	tenantCond := ""
	args := []any{req.GetConversionEvent(), time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	q := fmt.Sprintf(`
WITH converters AS (
    SELECT DISTINCT user_id FROM events_fact
    WHERE %sevent_name = ? AND event_time >= ? AND event_time < ?
),
touchpoint AS (
    SELECT e.user_id AS user_id, e.%s AS dim_val,
           row_number() OVER (PARTITION BY e.user_id ORDER BY e.event_time %s) AS rn
    FROM events_fact e
    INNER JOIN converters c ON c.user_id = e.user_id
    WHERE e.%sevent_time >= ? AND e.event_time < ?
)
SELECT dim_val, count(DISTINCT user_id) AS converter_uv
FROM touchpoint
WHERE rn = 1 AND dim_val != ''
GROUP BY dim_val
ORDER BY converter_uv DESC
LIMIT 20`,
		tenantCond, dim, orderDir, tenantCond)
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))

	type row struct {
		DimVal      string `db:"dim_val" ch:"dim_val"`
		ConverterUv int64  `db:"converter_uv" ch:"converter_uv"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Attribution query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("attribution query failed")
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
// 与 doris.AnalyticsRepo.Distribution 对应；ClickHouse 用 quantile()。
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

	bucketQ := fmt.Sprintf(`
SELECT
    CASE
        WHEN duration_ms < 10000  THEN '0_10s'
        WHEN duration_ms < 60000  THEN '10_60s'
        WHEN duration_ms < 300000 THEN '1_5min'
        ELSE '5min_plus'
    END AS duration_bucket,
    count() AS cnt
FROM events_fact
WHERE %sevent_name = ? AND duration_ms > 0 AND event_time >= ? AND event_time < ?
GROUP BY duration_bucket
ORDER BY duration_bucket`, tenantCond)

	type bucketRow struct {
		Bucket string `db:"duration_bucket" ch:"duration_bucket"`
		Cnt    int64  `db:"cnt" ch:"cnt"`
	}
	var bRows []bucketRow
	if err := r.db.Select(ctx, &bRows, bucketQ, args...); err != nil {
		r.log.Errorf("Distribution bucket query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("distribution query failed")
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

	// ClickHouse 分位数：quantile(0.5)(col) / quantile(0.9)(col)
	summaryQ := fmt.Sprintf(`
SELECT
    count()                                          AS cnt,
    round(avg(duration_ms) / 1000, 2)                AS avg_sec,
    round(quantile(0.5)(duration_ms) / 1000, 2)      AS p50_sec,
    round(quantile(0.9)(duration_ms) / 1000, 2)      AS p90_sec,
    round(max(duration_ms) / 1000, 2)                AS max_sec
FROM events_fact
WHERE %sevent_name = ? AND duration_ms > 0 AND event_time >= ? AND event_time < ?`, tenantCond)

	var s struct {
		Cnt    int64   `db:"cnt" ch:"cnt"`
		AvgSec float64 `db:"avg_sec" ch:"avg_sec"`
		P50Sec float64 `db:"p50_sec" ch:"p50_sec"`
		P90Sec float64 `db:"p90_sec" ch:"p90_sec"`
		MaxSec float64 `db:"max_sec" ch:"max_sec"`
	}
	if err := r.db.QueryRow(ctx, &s, summaryQ, args...); err != nil {
		r.log.Errorf("Distribution summary query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("distribution summary query failed")
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
// 与 doris.AnalyticsRepo.BehaviorSequence 对应。
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
	args = append(args, limit)

	q := fmt.Sprintf(`
SELECT event_time, event_name, session_id, session_seq, referer, platform, channel
FROM events_fact
WHERE %s
ORDER BY event_time ASC
LIMIT ?`, strings.Join(where, " AND "))

	type evRow struct {
		EventTime  *time.Time `db:"event_time" ch:"event_time"`
		EventName  *string    `db:"event_name" ch:"event_name"`
		SessionID  *string    `db:"session_id" ch:"session_id"`
		SessionSeq *uint32    `db:"session_seq" ch:"session_seq"`
		Referer    *string    `db:"referer" ch:"referer"`
		Platform   *string    `db:"platform" ch:"platform"`
		Channel    *string    `db:"channel" ch:"channel"`
	}
	var rows []evRow
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("BehaviorSequence query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("behavior sequence query failed")
	}

	events := make([]*ubaV1.SequenceEvent, 0, len(rows))
	for _, rw := range rows {
		ev := &ubaV1.SequenceEvent{EventName: chDerefStr(rw.EventName)}
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
// 与 doris.AnalyticsRepo.Segmentation 对应；ClickHouse 用 NOT IN 表达"未做过"。
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
	innerQ := fmt.Sprintf(`
SELECT user_id
FROM events_fact
WHERE %sevent_name = ? AND event_time >= ? AND event_time < ?
GROUP BY user_id
HAVING count() >= ?`, tenantCond)
	args = append(args, incTimes)

	// 可选排除：排除做过 exclude[0] 事件的用户（用 NOT IN 子查询）。
	var q string
	if excList := req.GetExclude(); len(excList) > 0 && excList[0].GetEventName() != "" {
		exc := excList[0]
		q = fmt.Sprintf(`
SELECT a.user_id
FROM (%s) AS a
WHERE a.user_id NOT IN (
    SELECT user_id FROM events_fact
    WHERE %sevent_name = ? AND event_time >= ? AND event_time < ?
)
LIMIT ?`, innerQ, tenantCond)
		args = append(args, exc.GetEventName(), time.UnixMilli(startMs), time.UnixMilli(endMs), limit)
	} else {
		q = innerQ + " LIMIT ?"
		args = append(args, limit)
	}

	var userIDs []uint32
	if err := r.db.Select(ctx, &userIDs, q, args...); err != nil {
		r.log.Errorf("Segmentation query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("segmentation query failed")
	}

	return &ubaV1.SegmentationResponse{
		UserIds: userIDs,
		Total:   int64(len(userIDs)),
	}, nil
}

// ============================================================================
// 点击热力图（按页面网格分桶聚合点击坐标 + 元素点击 TOP）
// 与 doris.AnalyticsRepo.Click 对应；ClickHouse 用 intDiv 对齐网格。
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

	// 网格分桶：intDiv(click_x, gridSize) * gridSize 对齐到网格左上角。
	gridArgs := []any{}
	if v := req.GetAppId(); v != 0 {
		gridArgs = append(gridArgs, v)
	}
	gridArgs = append(gridArgs, req.GetPageUrl(), time.UnixMilli(startMs), time.UnixMilli(endMs), gridSize, gridSize, gridSize, gridSize)

	gridSQL := fmt.Sprintf(`
SELECT (intDiv(click_x, ?) * ?) AS grid_x,
       (intDiv(click_y, ?) * ?) AS grid_y,
       count() AS cnt
FROM events_fact
WHERE %sevent_name = 'click' AND page_url = ? AND click_x > 0 AND click_y > 0
  AND event_time >= ? AND event_time < ?
GROUP BY grid_x, grid_y
ORDER BY cnt DESC
LIMIT 2000`, tenantCond)

	type gridRow struct {
		GridX int64 `db:"grid_x" ch:"grid_x"`
		GridY int64 `db:"grid_y" ch:"grid_y"`
		Cnt   int64 `db:"cnt" ch:"cnt"`
	}
	var gRows []gridRow
	if err := r.db.Select(ctx, &gRows, gridSQL, gridArgs...); err != nil {
		r.log.Errorf("Click grid query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("click query failed")
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
			X:         uint32(gr.GridX),
			Y:         uint32(gr.GridY),
			Count:     gr.Cnt,
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
SELECT element_xpath, count() AS cnt
FROM events_fact
WHERE %sevent_name = 'click' AND page_url = ? AND element_xpath != ''
  AND event_time >= ? AND event_time < ?
GROUP BY element_xpath
ORDER BY cnt DESC
LIMIT 20`, tenantCond)

	type elemRow struct {
		ElementXpath string `db:"element_xpath" ch:"element_xpath"`
		Cnt          int64  `db:"cnt" ch:"cnt"`
	}
	var eRows []elemRow
	if err := r.db.Select(ctx, &eRows, topSQL, topArgs...); err != nil {
		r.log.Errorf("Click element query failed: %v", err)
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
// 与 doris.AnalyticsRepo.Lifecycle 对应；ClickHouse 用 dateDiff/toIntervalDay。
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
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
	}

	// ClickHouse: 用 dateDiff('day', col, ?) 算距今天数。last_active_date 为 Date。
	q := fmt.Sprintf(`
SELECT stage, count() AS user_cnt FROM (
  SELECT
    multiIf(
      register_time >= ? - INTERVAL %d DAY, 'new_user',
      last_active_date >= toDate(?) - INTERVAL 1 DAY, 'active',
      last_active_date < toDate(?) - INTERVAL %d DAY AND last_active_date >= toDate(?) - INTERVAL %d DAY, 'reactivated',
      last_active_date < toDate(?) - INTERVAL %d DAY, 'churned',
      'retained'
    ) AS stage
  FROM users_dim
  WHERE %suser_id IS NOT NULL
) GROUP BY stage`, newUserDays, churnDays, churnDays, churnDays, tenantCond)
	args := []any{now, now, now, now, now}
	if v := req.GetAppId(); v != 0 {
		args = append([]any{v}, args...)
	}

	type stageRow struct {
		Stage   string `db:"stage" ch:"stage"`
		UserCnt int64  `db:"user_cnt" ch:"user_cnt"`
	}
	var rows []stageRow
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Lifecycle query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("lifecycle query failed")
	}

	labels := map[string]string{
		"new_user": "新用户", "active": "活跃用户", "retained": "留存用户",
		"churned": "流失用户", "reactivated": "回流用户",
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
			Stage: s, StageLabel: labels[s], UserCount: cnt, Percentage: pct,
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
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
	}

	churnQ := fmt.Sprintf(`
SELECT
  multiIf(
    last_active_date >= toDate(?) - INTERVAL 60 DAY, '30_60d',
    last_active_date >= toDate(?) - INTERVAL 90 DAY, '60_90d',
    '90_plus'
  ) AS bucket,
  count() AS user_cnt
FROM users_dim
WHERE %slast_active_date < toDate(?) - INTERVAL %d DAY AND user_id IS NOT NULL
GROUP BY bucket ORDER BY bucket`, tenantCond, churnDays)
	churnArgs := []any{now, now, now}
	if v := req.GetAppId(); v != 0 {
		churnArgs = append([]any{v}, churnArgs...)
	}
	type bucketRow struct {
		Bucket  string `db:"bucket" ch:"bucket"`
		UserCnt int64  `db:"user_cnt" ch:"user_cnt"`
	}
	var bRows []bucketRow
	if err := r.db.Select(ctx, &bRows, churnQ, churnArgs...); err != nil {
		r.log.Errorf("Churn bucket query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("churn query failed")
	}
	var churnedUsers int64
	buckets := make([]*ubaV1.ChurnBucket, 0, len(bRows))
	for _, br := range bRows {
		churnedUsers += br.UserCnt
		buckets = append(buckets, &ubaV1.ChurnBucket{Bucket: br.Bucket, UserCount: br.UserCnt})
	}

	// 回流：last_active 在近 reactivationDays 内、且 first_active 较早（非新用户）。
	reactArgs := []any{now, reactivationDays, now, churnDays}
	if v := req.GetAppId(); v != 0 {
		reactArgs = append([]any{v}, reactArgs...)
	}
	var reactivated int64
	reactQ := fmt.Sprintf(`
SELECT count() FROM users_dim
WHERE %slast_active_date >= toDate(?) - INTERVAL %d DAY
  AND first_active_date < toDate(?) - INTERVAL %d DAY
  AND user_id IS NOT NULL`, tenantCond, reactivationDays, churnDays)
	_ = r.db.QueryRow(ctx, &reactivated, reactQ, reactArgs...)

	var reactivationRate float64
	if churnedUsers > 0 {
		reactivationRate = float64(reactivated) / float64(churnedUsers)
	}

	// 回流触发事件 TOP
	triggerArgs := []any{time.UnixMilli(endMs).Add(-time.Duration(reactivationDays) * 24 * time.Hour), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		triggerArgs = append([]any{v}, triggerArgs...)
	}
	eTenant := strings.Replace(tenantCond, "tenant_id", "e.tenant_id", 1)
	triggerQ := fmt.Sprintf(`
SELECT e.event_name AS event_name, count() AS cnt
FROM events_fact e
INNER JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.%sevent_time >= ? AND event_time < ?
  AND u.first_active_date < toDate(?) - INTERVAL %d DAY
  AND e.user_id > 0
GROUP BY e.event_name ORDER BY cnt DESC LIMIT 20`, eTenant, churnDays)
	triggerArgs = append(triggerArgs, now)
	type triggerRow struct {
		EventName string `db:"event_name" ch:"event_name"`
		Cnt       int64  `db:"cnt" ch:"cnt"`
	}
	var tRows []triggerRow
	if err := r.db.Select(ctx, &tRows, triggerQ, triggerArgs...); err != nil {
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
			EventName: tr.EventName, Count: tr.Cnt, Percentage: pct,
		})
	}

	return &ubaV1.ChurnResponse{
		ChurnBuckets: buckets, ChurnedUsers: churnedUsers,
		ReactivatedUsers: reactivated, ReactivationRate: reactivationRate,
		Triggers: triggers,
	}, nil
}

// ============================================================================
// 间隔时间分析（lead() 窗口配对，dateDiff 算间隔）
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

	// ClickHouse: lead(col) over(partition by user_id order by event_time)
	q := fmt.Sprintf(`
SELECT
  multiIf(
    diff_hours < 1.0/60, 'instant',
    diff_hours < 1, 'lt_1h',
    diff_hours < 24, '1_24h',
    diff_hours < 168, '1_7d',
    '7d_plus'
  ) AS bucket,
  count() AS cnt
FROM (
  SELECT
    user_id, event_time,
    lead(event_name) OVER (PARTITION BY user_id ORDER BY event_time) AS next_name,
    dateDiff('second', event_time, lead(event_time) OVER (PARTITION BY user_id ORDER BY event_time)) / 3600.0 AS diff_hours
  FROM events_fact
  WHERE %s(event_name = ? OR event_name = ?) AND event_time >= ? AND event_time < ? AND user_id > 0
) WHERE next_name = ? AND diff_hours >= 0
GROUP BY bucket ORDER BY bucket`, tenantCond)
	args = append(args, req.GetEventTo())

	type bucketRow struct {
		Bucket string `db:"bucket" ch:"bucket"`
		Cnt    int64  `db:"cnt" ch:"cnt"`
	}
	var rows []bucketRow
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Interval query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("interval query failed")
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

	summaryQ := fmt.Sprintf(`
SELECT
  count() AS cnt,
  round(avg(diff_hours), 2) AS avg_hours,
  round(quantile(0.5)(diff_hours), 2) AS p50_hours,
  round(quantile(0.9)(diff_hours), 2) AS p90_hours
FROM (
  SELECT
    dateDiff('second', event_time, lead(event_time) OVER (PARTITION BY user_id ORDER BY event_time)) / 3600.0 AS diff_hours,
    lead(event_name) OVER (PARTITION BY user_id ORDER BY event_time) AS next_name
  FROM events_fact
  WHERE %s(event_name = ? OR event_name = ?) AND event_time >= ? AND event_time < ? AND user_id > 0
) WHERE next_name = ? AND diff_hours >= 0`, tenantCond)
	var s struct {
		Cnt      int64   `db:"cnt" ch:"cnt"`
		AvgHours float64 `db:"avg_hours" ch:"avg_hours"`
		P50Hours float64 `db:"p50_hours" ch:"p50_hours"`
		P90Hours float64 `db:"p90_hours" ch:"p90_hours"`
	}
	_ = r.db.QueryRow(ctx, &s, summaryQ, args...)

	return &ubaV1.IntervalResponse{
		Buckets: buckets, P50Hours: s.P50Hours, P90Hours: s.P90Hours,
		AvgHours: s.AvgHours, Count: s.Cnt,
	}, nil
}

// ============================================================================
// 矩阵/象限分析（双轴：UV × 频次，中位数分四象限）
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

	q := fmt.Sprintf(`
SELECT %s AS label,
       count(DISTINCT user_id) AS uv,
       count() AS freq
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ? AND user_id > 0
GROUP BY %s
ORDER BY uv DESC
LIMIT 100`, dim, tenantCond, dim)

	type ptRow struct {
		Label string `db:"label" ch:"label"`
		UV    int64  `db:"uv" ch:"uv"`
		Freq  int64  `db:"freq" ch:"freq"`
	}
	var rows []ptRow
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Matrix query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("matrix query failed")
	}
	if len(rows) == 0 {
		return &ubaV1.MatrixResponse{Points: []*ubaV1.MatrixPoint{}, Dimension: dim}, nil
	}

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
			quadrant = "core"
		case highX && !highY:
			quadrant = "star"
		case !highX && highY:
			quadrant = "niche"
		default:
			quadrant = "edge"
		}
		points = append(points, &ubaV1.MatrixPoint{
			Label: rw.Label, X: x, Y: y, Quadrant: quadrant,
		})
	}

	return &ubaV1.MatrixResponse{
		Points: points, XThreshold: xThreshold, YThreshold: yThreshold, Dimension: dim,
	}, nil
}

// ============================================================================
// 付费/营收分析（ARPU/ARPPU/付费率/GMV 趋势）
// ClickHouse 数据源：pay_agg_daily_view（已聚合 total_pay_user_count/grand_total_amount 等）。
// ============================================================================

func (r *AnalyticsRepo) Revenue(ctx context.Context, req *ubaV1.RevenueRequest) (*ubaV1.RevenueResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	// 付费数据：pay_agg_daily_view（已聚合 grand_total_amount/total_pay_user_count/total_pay_order_count）
	payQ := fmt.Sprintf(`
SELECT stat_date AS d,
       sum(grand_total_amount) AS gmv,
       sum(total_pay_user_count) AS pay_users,
       sum(total_pay_order_count) AS pay_orders
FROM pay_agg_daily_view
WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)
GROUP BY d ORDER BY d`, tenantCond)
	type payRow struct {
		D         time.Time `db:"d" ch:"d"`
		Gmv       float64   `db:"gmv" ch:"gmv"`
		PayUsers  int64     `db:"pay_users" ch:"pay_users"`
		PayOrders int64     `db:"pay_orders" ch:"pay_orders"`
	}
	var payRows []payRow
	if err := r.db.Select(ctx, &payRows, payQ, args...); err != nil {
		r.log.Errorf("Revenue pay query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("revenue query failed")
	}

	// 活跃用户：events_agg_daily_view（pay_agg 无 active 字段），按日去重 UV
	activeQ := fmt.Sprintf(`
SELECT stat_date AS d, uniqCombinedMerge(uv) AS active_users
FROM events_agg_daily_view
WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)
GROUP BY d`, tenantCond)
	type activeRow struct {
		D           time.Time `db:"d" ch:"d"`
		ActiveUsers int64     `db:"active_users" ch:"active_users"`
	}
	var activeRows []activeRow
	if err := r.db.Select(ctx, &activeRows, activeQ, args...); err != nil {
		r.log.Errorf("Revenue active query failed: %v", err)
		activeRows = nil
	}
	activeMap := map[int64]int64{}
	for _, ar := range activeRows {
		activeMap[ar.D.UnixMilli()] = ar.ActiveUsers
	}

	var totalGmv float64
	var totalPayUsers, totalPayOrders int64
	points := make([]*ubaV1.RevenuePoint, 0, len(payRows))
	for _, rw := range payRows {
		totalGmv += rw.Gmv
		totalPayUsers += rw.PayUsers
		totalPayOrders += rw.PayOrders
		ts := rw.D.UnixMilli()
		activeUsers := activeMap[ts]
		var arpu, arppu, payRate float64
		if activeUsers > 0 {
			arpu = rw.Gmv / float64(activeUsers)
			payRate = float64(rw.PayUsers) / float64(activeUsers)
		}
		if rw.PayUsers > 0 {
			arppu = rw.Gmv / float64(rw.PayUsers)
		}
		points = append(points, &ubaV1.RevenuePoint{
			Timestamp: ts,
			Gmv:       rw.Gmv, PayUsers: rw.PayUsers, PayOrders: rw.PayOrders,
			Arpu: arpu, Arppu: arppu, PayRate: payRate,
		})
	}
	var avgOrderValue float64
	if totalPayOrders > 0 {
		avgOrderValue = totalGmv / float64(totalPayOrders)
	}
	return &ubaV1.RevenueResponse{
		Points: points, TotalGmv: totalGmv,
		TotalPayUsers: totalPayUsers, TotalPayOrders: totalPayOrders,
		AvgOrderValue: avgOrderValue,
	}, nil
}

// ============================================================================
// 会话分析（跳出率/时长分位 P50/P90/会话深度）
// ClickHouse 数据源：sessions_agg_daily_view（已聚合 p50/p90/bounce_rate）。
// ============================================================================

func (r *AnalyticsRepo) SessionAnalysis(ctx context.Context, req *ubaV1.SessionAnalysisRequest) (*ubaV1.SessionAnalysisResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

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

	q := fmt.Sprintf(`
SELECT
  sum(session_count) AS session_count,
  uniqCombinedMerge(unique_users) AS unique_users,
  sum(duration_sum) / sum(duration_count) / 1000.0 AS avg_duration_sec,
  quantileTimingMerge(0.5)(p50_duration) / 1000.0 AS p50_sec,
  quantileTimingMerge(0.9)(p90_duration) / 1000.0 AS p90_sec,
  sum(bounce_sum) / sum(bounce_count) AS bounce_rate
FROM sessions_agg_daily_view
WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)`, whereCond)

	var s struct {
		SessionCount int64   `db:"session_count" ch:"session_count"`
		UniqueUsers  int64   `db:"unique_users" ch:"unique_users"`
		AvgDurSec    float64 `db:"avg_duration_sec" ch:"avg_duration_sec"`
		P50Sec       float64 `db:"p50_sec" ch:"p50_sec"`
		P90Sec       float64 `db:"p90_sec" ch:"p90_sec"`
		BounceRate   float64 `db:"bounce_rate" ch:"bounce_rate"`
	}
	if err := r.db.QueryRow(ctx, &s, q, args...); err != nil {
		r.log.Errorf("SessionAnalysis query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("session analysis query failed")
	}

	// 会话深度：events_fact 总事件 / 会话数
	depthTenantCond := ""
	depthArgs := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		depthTenantCond = "tenant_id = ? AND "
		depthArgs = append([]any{v}, depthArgs...)
	}
	depthQ := fmt.Sprintf(`SELECT count() FROM events_fact WHERE %sevent_time >= ? AND event_time < ?`, depthTenantCond)
	var totalEvents int64
	_ = r.db.QueryRow(ctx, &totalEvents, depthQ, depthArgs...)
	var avgDepth float64
	if s.SessionCount > 0 {
		avgDepth = float64(totalEvents) / float64(s.SessionCount)
	}

	return &ubaV1.SessionAnalysisResponse{
		SessionCount: s.SessionCount, UniqueUsers: s.UniqueUsers,
		AvgDurationSec: s.AvgDurSec, P50DurationSec: s.P50Sec, P90DurationSec: s.P90Sec,
		BounceRate: s.BounceRate, AvgDepth: avgDepth,
	}, nil
}

// ============================================================================
// 同比环比/异常检测（事件 PV/UV 环比 + 7日基线）
// ClickHouse 数据源：events_agg_daily_view + 窗口函数。
// ============================================================================

func (r *AnalyticsRepo) Anomaly(ctx context.Context, req *ubaV1.AnomalyRequest) (*ubaV1.AnomalyResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

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

	q := fmt.Sprintf(`
SELECT event_name, d, pv, uv, baseline, wow_change FROM (
  SELECT
    event_name, d, pv, uv,
    avg(pv) OVER (PARTITION BY event_name ORDER BY d ROWS BETWEEN 7 PRECEDING AND 1 PRECEDING) AS baseline,
    if(lag(pv) OVER (PARTITION BY event_name ORDER BY d) > 0,
       (pv - lag(pv) OVER (PARTITION BY event_name ORDER BY d)) / lag(pv) OVER (PARTITION BY event_name ORDER BY d),
       0) AS wow_change
  FROM (
    SELECT event_name, stat_date AS d,
           sum(pv) AS pv, uniqCombinedMerge(uv) AS uv
    FROM events_agg_daily_view
    WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)
    GROUP BY event_name, d
  ) daily
) ranked
WHERE baseline > 0
ORDER BY event_name, d`, whereCond)

	type row struct {
		EventName string    `db:"event_name" ch:"event_name"`
		D         time.Time `db:"d" ch:"d"`
		Pv        int64     `db:"pv" ch:"pv"`
		Uv        int64     `db:"uv" ch:"uv"`
		Baseline  float64   `db:"baseline" ch:"baseline"`
		WowChange float64   `db:"wow_change" ch:"wow_change"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Anomaly query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("anomaly query failed")
	}
	points := make([]*ubaV1.AnomalyPoint, 0, len(rows))
	anomalySet := map[string]bool{}
	for _, rw := range rows {
		isAnomaly := rw.Baseline > 0 && float64(rw.Pv) < rw.Baseline*0.5
		if isAnomaly {
			anomalySet[rw.EventName] = true
		}
		points = append(points, &ubaV1.AnomalyPoint{
			EventName: rw.EventName, StatDate: rw.D.UnixMilli(),
			Pv: rw.Pv, Uv: rw.Uv, Baseline: rw.Baseline, WowChange: rw.WowChange, IsAnomaly: isAnomaly,
		})
	}
	return &ubaV1.AnomalyResponse{Points: points, AnomalyCount: int64(len(anomalySet))}, nil
}

// ============================================================================
// 新老用户对比（构成占比 + 事件/付费差异）
// ClickHouse 数据源：users_dim + events_fact。
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

	q := fmt.Sprintf(`
SELECT
  multiIf(u.first_active_date >= today() - INTERVAL %d DAY, 'new', 'old') AS user_type,
  count(DISTINCT e.user_id) AS user_count,
  count() AS event_count,
  countDistinctIf(e.user_id, e.amount > 0) AS pay_users
FROM events_fact e
INNER JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE e.%sevent_time >= ? AND e.event_time < ? AND e.user_id > 0
GROUP BY user_type`, newUserDays, tenantCond)

	type row struct {
		UserType   string `db:"user_type" ch:"user_type"`
		UserCount  int64  `db:"user_count" ch:"user_count"`
		EventCount int64  `db:"event_count" ch:"event_count"`
		PayUsers   int64  `db:"pay_users" ch:"pay_users"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("NewVsOld query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("new vs old query failed")
	}
	segMap := map[string]*ubaV1.NewVsOldSegment{}
	for _, rw := range rows {
		var payRate float64
		if rw.UserCount > 0 {
			payRate = float64(rw.PayUsers) / float64(rw.UserCount)
		}
		segMap[rw.UserType] = &ubaV1.NewVsOldSegment{
			UserType: rw.UserType, UserCount: rw.UserCount,
			EventCount: rw.EventCount, PayUsers: rw.PayUsers, PayRate: payRate,
		}
	}
	segments := make([]*ubaV1.NewVsOldSegment, 0, 2)
	for _, t := range []string{"new", "old"} {
		if seg, ok := segMap[t]; ok {
			segments = append(segments, seg)
		}
	}
	return &ubaV1.NewVsOldResponse{Segments: segments}, nil
}

// ============================================================================
// 热门转化路径（群体路径 TOP + 转化率）
// ClickHouse 数据源：popular_paths_daily_view（event_sequence + support_count + conversion）。
// ============================================================================

func (r *AnalyticsRepo) PathSankey(ctx context.Context, req *ubaV1.PathSankeyRequest) (*ubaV1.PathSankeyResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	topN := int64(req.GetTopN())
	if topN <= 0 || topN > 200 {
		topN = 20
	}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs), topN}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	q := fmt.Sprintf(`
SELECT arrayStringConcat(event_sequence, ' → ') AS event_sequence_str,
       sum(support_count) AS support_count,
       uniqCombinedMerge(unique_users) AS unique_users,
       any(conversion_rate) AS conversion_rate
FROM popular_paths_daily_view
WHERE %sstat_date >= toDate(?) AND stat_date < toDate(?)
GROUP BY event_sequence, event_sequence_str
ORDER BY support_count DESC
LIMIT ?`, tenantCond)

	type row struct {
		EventSequence  string  `db:"event_sequence_str" ch:"event_sequence_str"`
		SupportCount   int64   `db:"support_count" ch:"support_count"`
		UniqueUsers    int64   `db:"unique_users" ch:"unique_users"`
		ConversionRate float64 `db:"conversion_rate" ch:"conversion_rate"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("PathSankey query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("path sankey query failed")
	}
	paths := make([]*ubaV1.PathBucket, 0, len(rows))
	for _, rw := range rows {
		paths = append(paths, &ubaV1.PathBucket{
			EventSequence: rw.EventSequence, SupportCount: rw.SupportCount,
			UniqueUsers: rw.UniqueUsers, ConversionRate: rw.ConversionRate,
		})
	}
	return &ubaV1.PathSankeyResponse{Paths: paths}, nil
}

// ============================================================================
// 关卡/数值平衡分析（通过率/失败率/卡关率/分数分布/满星率）
// ClickHouse 数据源：events_fact（object_type='level'）。
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

	// CH 取 context map 用 context['stars']，metrics 用 metrics['score']。
	q := fmt.Sprintf(`
SELECT
  object_id AS level_id,
  any(object_name) AS level_name,
  countIf(event_name = 'level_start') AS attempt_count,
  countIf(event_name = 'level_finish') AS finish_count,
  countIf(event_name = 'level_fail') AS fail_count,
  round(avgIf(metrics['score'], event_name = 'level_finish'), 1) AS avg_score,
  countIf(event_name = 'level_finish' AND context['stars'] = '3') AS star3_count,
  count(DISTINCT user_id) AS player_count
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ?
GROUP BY object_id
ORDER BY player_count DESC
LIMIT 100`, whereCond)

	type row struct {
		LevelId     string  `db:"level_id" ch:"level_id"`
		LevelName   string  `db:"level_name" ch:"level_name"`
		AttemptCnt  int64   `db:"attempt_count" ch:"attempt_count"`
		FinishCnt   int64   `db:"finish_count" ch:"finish_count"`
		FailCnt     int64   `db:"fail_count" ch:"fail_count"`
		AvgScore    float64 `db:"avg_score" ch:"avg_score"`
		Star3Count  int64   `db:"star3_count" ch:"star3_count"`
		PlayerCount int64   `db:"player_count" ch:"player_count"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("LevelAnalysis query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("level analysis query failed")
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
			LevelId: rw.LevelId, LevelName: rw.LevelName,
			AttemptCount: rw.AttemptCnt, FinishCount: rw.FinishCnt, FailCount: rw.FailCnt,
			PassRate: passRate, StuckRate: 1 - passRate, AvgScore: rw.AvgScore,
			Star3Rate: star3Rate, PlayerCount: rw.PlayerCount,
		})
	}
	return &ubaV1.LevelAnalysisResponse{Levels: levels}, nil
}

// ============================================================================
// 鲸鱼用户/付费分层（按累计充值自动分层）
// ClickHouse 数据源：users_dim（total_pay_amount）。
// ============================================================================

func (r *AnalyticsRepo) WhaleTier(ctx context.Context, req *ubaV1.WhaleTierRequest) (*ubaV1.WhaleTierResponse, error) {
	tenantCond := ""
	args := []any{}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append(args, v)
	}

	q := fmt.Sprintf(`
SELECT
  multiIf(
    total_pay_amount >= 10000, 'whale',
    total_pay_amount >= 1000, 'dolphin',
    total_pay_amount > 0, 'minnow',
    'non_pay'
  ) AS tier,
  count() AS user_count,
  round(sum(toFloat64OrZero(toString(total_pay_amount))), 2) AS total_amount
FROM users_dim
WHERE %suser_id IS NOT NULL
GROUP BY tier`, tenantCond)

	type row struct {
		Tier        string  `db:"tier" ch:"tier"`
		UserCount   int64   `db:"user_count" ch:"user_count"`
		TotalAmount float64 `db:"total_amount" ch:"total_amount"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("WhaleTier query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("whale tier query failed")
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
			Tier: t, TierLabel: labels[t], UserCount: rw.UserCount,
			Percentage: pct, TotalAmount: rw.TotalAmount,
			RevenueShare: revShare, Arppu: arppu,
		})
	}
	return &ubaV1.WhaleTierResponse{
		Segments: segments, TotalUsers: totalUsers, TotalRevenue: totalRevenue,
	}, nil
}

// ============================================================================
// 历史生命周期价值 LTV（用户在第 N 天的累计付费价值，支持按渠道分组）
// ClickHouse 数据源：users_dim JOIN events_fact；dateDiff('day', register, event)。
// ============================================================================

func (r *AnalyticsRepo) LTV(ctx context.Context, req *ubaV1.LTVRequest) (*ubaV1.LTVResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())
	maxDays := []uint32{0, 1, 3, 7, 14, 30, 60, 90}

	tenantCond := ""
	args := []any{time.UnixMilli(startMs), time.UnixMilli(endMs)}
	if v := req.GetAppId(); v != 0 {
		tenantCond = "tenant_id = ? AND "
		args = append([]any{v}, args...)
	}

	dimSelect := "'' AS label"
	if dim := req.GetDimension(); dim == "channel" {
		dimSelect = "register_channel AS label"
	}

	// 同期群规模
	cohortQ := fmt.Sprintf(`
SELECT %s AS label, count() AS cohort_size
FROM users_dim
WHERE %sregister_time >= ? AND register_time < ?
GROUP BY label`, dimSelect, tenantCond)
	type cohortRow struct {
		Label      string `db:"label" ch:"label"`
		CohortSize int64  `db:"cohort_size" ch:"cohort_size"`
	}
	var cohortRows []cohortRow
	if err := r.db.Select(ctx, &cohortRows, cohortQ, args...); err != nil {
		r.log.Errorf("LTV cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("ltv query failed")
	}
	cohortMap := map[string]int64{}
	for _, cr := range cohortRows {
		cohortMap[cr.Label] = cr.CohortSize
	}

	// 付费事件按 (label, day_bucket) 聚合
	payArgs := append([]any{}, args...)
	payQ := fmt.Sprintf(`
SELECT %s AS label,
  multiIf(
    dateDiff('day', u.register_time, e.event_time) <= 0, 0,
    dateDiff('day', u.register_time, e.event_time) <= 1, 1,
    dateDiff('day', u.register_time, e.event_time) <= 3, 3,
    dateDiff('day', u.register_time, e.event_time) <= 7, 7,
    dateDiff('day', u.register_time, e.event_time) <= 14, 14,
    dateDiff('day', u.register_time, e.event_time) <= 30, 30,
    dateDiff('day', u.register_time, e.event_time) <= 60, 60,
    90
  ) AS day_n,
  round(sum(toFloat64OrZero(toString(e.amount))), 2) AS total_amount
FROM events_fact e
INNER JOIN users_dim u ON u.tenant_id = e.tenant_id AND u.user_id = e.user_id
WHERE u.%sregister_time >= ? AND u.register_time < ? AND e.amount > 0 AND e.user_id > 0
GROUP BY label, day_n`, dimSelect, tenantCond)
	type payRow struct {
		Label       string  `db:"label" ch:"label"`
		DayN        int64   `db:"day_n" ch:"day_n"`
		TotalAmount float64 `db:"total_amount" ch:"total_amount"`
	}
	var payRows []payRow
	if err := r.db.Select(ctx, &payRows, payQ, payArgs...); err != nil {
		r.log.Errorf("LTV pay query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("ltv query failed")
	}

	bucketSum := map[string]map[int64]float64{}
	for _, pr := range payRows {
		if bucketSum[pr.Label] == nil {
			bucketSum[pr.Label] = map[int64]float64{}
		}
		bucketSum[pr.Label][pr.DayN] = pr.TotalAmount
	}

	points := make([]*ubaV1.LTVPoint, 0)
	for label, size := range cohortMap {
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
				Label: label, DayN: dn, Ltv: ltv,
				CohortSize: size, TotalAmount: cumulative,
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

	// 同期群：每个用户在该 server 的首事件日（首日新增），按 server 分组。
	cohortQ := fmt.Sprintf(`
SELECT server_id, count() AS cohort_size FROM (
  SELECT user_id, server_id, min(toDate(event_time)) AS first_day
  FROM events_fact
  WHERE %sevent_time >= ? AND event_time < ? AND server_id != '' AND user_id > 0
  GROUP BY user_id, server_id
) GROUP BY server_id ORDER BY cohort_size DESC LIMIT 50`, whereCond)
	type cohortRow struct {
		ServerID   string `db:"server_id" ch:"server_id"`
		CohortSize int64  `db:"cohort_size" ch:"cohort_size"`
	}
	var cohortRows []cohortRow
	if err := r.db.Select(ctx, &cohortRows, cohortQ, args...); err != nil {
		r.log.Errorf("ServerRetention cohort query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("server retention query failed")
	}

	rows := make([]*ubaV1.ServerRetentionRow, 0, len(cohortRows))
	for _, cr := range cohortRows {
		rates := map[string]float64{"0": 1.0}
		for _, d := range offsetDays {
			if d == 0 {
				continue
			}
			retArgs := []any{cr.ServerID, time.UnixMilli(startMs), time.UnixMilli(endMs)}
			retQ := `
SELECT count(DISTINCT e.user_id) AS retained
FROM events_fact e
INNER JOIN (
  SELECT user_id, min(toDate(event_time)) AS first_day
  FROM events_fact
  WHERE server_id = ? AND event_time >= ? AND event_time < ? AND user_id > 0
  GROUP BY user_id
) f ON f.user_id = e.user_id
WHERE e.server_id = ? AND dateDiff('day', f.first_day, toDate(e.event_time)) = ?`
			if v := req.GetAppId(); v != 0 {
				retQ = strings.Replace(retQ, "WHERE e.server_id", "WHERE e.tenant_id = ? AND e.server_id", 1)
				retArgs = append([]any{v}, retArgs...)
			}
			retArgs = append(retArgs, cr.ServerID, int64(d))
			var retained int64
			_ = r.db.QueryRow(ctx, &retained, retQ, retArgs...)
			if cr.CohortSize > 0 {
				rates[fmt.Sprintf("%d", d)] = float64(retained) / float64(cr.CohortSize)
			}
		}
		rows = append(rows, &ubaV1.ServerRetentionRow{
			ServerId: cr.ServerID, CohortSize: cr.CohortSize, RetentionRates: rates,
		})
	}
	return &ubaV1.ServerRetentionResponse{Rows: rows, OffsetDays: offsetDays}, nil
}

// ============================================================================
// 同时在线 PCU/ACU（基于 sessions_fact 会话区间推算）
// ============================================================================

func (r *AnalyticsRepo) OnlineStats(ctx context.Context, req *ubaV1.OnlineStatsRequest) (*ubaV1.OnlineStatsResponse, error) {
	startMs, endMs := normTimeRange(req.GetTimeRange())

	var whereParts []string
	var args []any
	if v := req.GetAppId(); v != 0 {
		whereParts = append(whereParts, "tenant_id = ?")
		args = append(args, v)
	}
	if sid := req.GetServerId(); sid != "" {
		whereParts = append(whereParts, "server_id = ?")
		args = append(args, sid)
	}
	args = append(args, time.UnixMilli(startMs), time.UnixMilli(endMs))
	whereCond := ""
	if len(whereParts) > 0 {
		whereCond = strings.Join(whereParts, " AND ") + " AND "
	}

	q := fmt.Sprintf(`
SELECT
  count() AS total_sessions,
  ifNull(sum(duration_ms), 0) AS total_duration_ms
FROM sessions_fact
WHERE %sstart_time >= ? AND start_time < ?`, whereCond)
	var s struct {
		TotalSessions   int64 `db:"total_sessions" ch:"total_sessions"`
		TotalDurationMs int64 `db:"total_duration_ms" ch:"total_duration_ms"`
	}
	if err := r.db.QueryRow(ctx, &s, q, args...); err != nil {
		r.log.Errorf("OnlineStats query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("online stats query failed")
	}

	spanMs := endMs - startMs
	durationMinutes := spanMs / int64(time.Minute/time.Millisecond)
	if durationMinutes <= 0 {
		durationMinutes = 1
	}
	totalOnlineMinutes := s.TotalDurationMs / 60000
	acu := totalOnlineMinutes / durationMinutes
	var pcu int64 = acu
	avgSessionMin := int64(0)
	if s.TotalSessions > 0 {
		avgSessionMin = totalOnlineMinutes / s.TotalSessions
	}
	if avgSessionMin > 0 {
		concurrency := s.TotalSessions * avgSessionMin / durationMinutes
		if concurrency > pcu {
			pcu = concurrency
		}
	}

	return &ubaV1.OnlineStatsResponse{
		Pcu: pcu, Acu: acu, DurationMinutes: durationMinutes, TotalSessions: s.TotalSessions,
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
  if(object_name = '', object_id, object_name) AS currency,
  round(sumIf(toFloat64OrZero(toString(amount)), amount > 0), 2) AS source,
  round(sumIf(toFloat64OrZero(toString(amount)) * -1, amount < 0), 2) AS sink
FROM events_fact
WHERE %sevent_time >= ? AND event_time < ? AND amount != 0
GROUP BY currency
ORDER BY source DESC`, whereCond)

	type row struct {
		Currency string  `db:"currency" ch:"currency"`
		Source   float64 `db:"source" ch:"source"`
		Sink     float64 `db:"sink" ch:"sink"`
	}
	var rows []row
	if err := r.db.Select(ctx, &rows, q, args...); err != nil {
		r.log.Errorf("Economy query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("economy query failed")
	}
	currencies := make([]*ubaV1.CurrencyBalance, 0, len(rows))
	for _, rw := range rows {
		currencies = append(currencies, &ubaV1.CurrencyBalance{
			Currency: rw.Currency, Source: rw.Source, Sink: rw.Sink, Net: rw.Source - rw.Sink,
		})
	}
	return &ubaV1.EconomyResponse{Currencies: currencies}, nil
}

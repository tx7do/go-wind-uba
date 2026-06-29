package clickhouse

import (
	"context"
	"fmt"
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

	q := fmt.Sprintf(
		"SELECT %s AS label, %s AS value FROM events_fact WHERE event_time >= ? AND event_time < ?",
		col, metric,
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

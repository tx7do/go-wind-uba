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
GROUP BY duration_bucket
ORDER BY duration_bucket`, tenantCond)

	type bucketRow struct {
		Bucket string `db:"duration_bucket"`
		Cnt    int64  `db:"cnt"`
	}
	var bRows []bucketRow
	if err := r.db.SelectContext(ctx, &bRows, bucketQ, args...); err != nil {
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

	// 分位数摘要：均值 / P50 / P90 / 最大值
	summaryQ := fmt.Sprintf(`
SELECT
    COUNT(*) AS cnt,
    ROUND(AVG(duration_ms) / 1000, 2)                      AS avg_sec,
    ROUND(APPROX_PERCENTILE(duration_ms, 0.5) / 1000, 2)   AS p50_sec,
    ROUND(APPROX_PERCENTILE(duration_ms, 0.9) / 1000, 2)   AS p90_sec,
    ROUND(MAX(duration_ms) / 1000, 2)                      AS max_sec
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
	args = append(args, limit)

	q := fmt.Sprintf(`
SELECT event_time, event_name, session_id, session_seq, referer, platform, channel
FROM events_fact
WHERE %s
ORDER BY event_time ASC
LIMIT ?`, strings.Join(where, " AND "))

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
		return nil, ubaV1.ErrorInternalServerError("behavior sequence query failed")
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

	q = q + " LIMIT ?"
	args = append(args, limit)

	var userIDs []uint32
	if err := r.db.SelectContext(ctx, &userIDs, q, args...); err != nil {
		r.log.Errorf("Segmentation query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("segmentation query failed")
	}

	return &ubaV1.SegmentationResponse{
		UserIds: userIDs,
		Total:   int64(len(userIDs)),
	}, nil
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

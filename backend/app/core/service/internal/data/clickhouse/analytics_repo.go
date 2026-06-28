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
	bucket := chGranularityExpr(req.GetGranularity())

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

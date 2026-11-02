package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"

	"go-wind-uba/app/core/service/internal/data/ent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

func TestCompareThreshold(t *testing.T) {
	cases := []struct {
		comparison string
		value      float64
		threshold  float64
		want       bool
	}{
		{"GT", 5, 3, true},
		{"GT", 3, 3, false}, // 严格大于：等于不算
		{"GT", 1, 3, false},
		{"GTE", 3, 3, true},
		{"GTE", 2, 3, false},
		{"LT", 2, 3, true},
		{"LT", 3, 3, false}, // 严格小于：等于不算
		{"LTE", 3, 3, true},
		{"LTE", 4, 3, false},
		{"", 5, 3, true},  // 未知算子回退 GT
		{"", 3, 3, false}, // 回退 GT：等于不算
		{"XXX", 5, 3, true},
	}
	for _, c := range cases {
		if got := compareThreshold(c.value, c.comparison, c.threshold); got != c.want {
			t.Fatalf("compareThreshold(%v, %q, %v) = %v, want %v", c.value, c.comparison, c.threshold, got, c.want)
		}
	}
}

func TestComparisonText(t *testing.T) {
	assert.Equal(t, ">=", comparisonText("GTE"))
	assert.Equal(t, "<", comparisonText("LT"))
	assert.Equal(t, "<=", comparisonText("LTE"))
	assert.Equal(t, ">", comparisonText("GT"))
	assert.Equal(t, ">", comparisonText(""), "未知算子回退 GT 文案")
}

func TestDerefHelpers(t *testing.T) {
	s := "x"
	assert.Equal(t, "x", derefStr(&s))
	assert.Equal(t, "", derefStr(nil))

	v := uint32(7)
	assert.Equal(t, uint32(7), derefU32(&v))
	assert.Equal(t, uint32(0), derefU32(nil))
}

// fakeTrendAnalytics 记录入参并返回预设 total，用于验证 evaluateRule 的查询构造。
type fakeTrendAnalytics struct {
	resp *ubaV1.EventTrendResponse
	err  error

	gotReq *ubaV1.EventTrendRequest
	calls  int
}

func (f *fakeTrendAnalytics) EventTrend(_ context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error) {
	f.calls++
	f.gotReq = req
	return f.resp, f.err
}

func newEvaluatorWithTrend(tr trendAnalytics) *AlertEvaluator {
	return &AlertEvaluator{trend: tr, chTrend: tr}
}

func TestAlertEvaluator_EvaluateRule_WindowAndQuery(t *testing.T) {
	ft := &fakeTrendAnalytics{resp: &ubaV1.EventTrendResponse{Total: 42}}
	e := newEvaluatorWithTrend(ft)

	now := time.Now()
	rule := &ent.AlertRule{
		WindowMinutes: 60,
		Comparison:    "GTE",
		Threshold:     30,
		AppID:         trans.Ptr(uint32(9)),
		EventName:     trans.Ptr("pay_success"),
	}

	value, hit, err := e.evaluateRule(context.Background(), rule, now)
	require.NoError(t, err)
	assert.Equal(t, float64(42), value)
	assert.True(t, hit, "42 >= 30 应触发")

	// 查询构造：窗口回看 60 分钟、小时粒度、带 app_id/event_name。
	require.NotNil(t, ft.gotReq)
	req := ft.gotReq
	assert.Equal(t, now.Add(-60*time.Minute).UnixMilli(), req.GetTimeRange().GetStartMs())
	assert.Equal(t, now.UnixMilli(), req.GetTimeRange().GetEndMs())
	assert.Equal(t, ubaV1.AnalyticsGranularity_HOUR, req.GetGranularity())
	require.NotNil(t, req.AppId)
	assert.Equal(t, uint32(9), *req.AppId)
	assert.Equal(t, "pay_success", req.GetEventName())
}

func TestAlertEvaluator_EvaluateRule_DefaultWindowFallback(t *testing.T) {
	ft := &fakeTrendAnalytics{resp: &ubaV1.EventTrendResponse{Total: 1}}
	e := newEvaluatorWithTrend(ft)

	now := time.Now()
	// WindowMinutes=0：窗口回退 1 小时；total=1 < 5 命中 LT。
	rule := &ent.AlertRule{WindowMinutes: 0, Comparison: "LT", Threshold: 5}

	value, hit, err := e.evaluateRule(context.Background(), rule, now)
	require.NoError(t, err)
	assert.Equal(t, float64(1), value)
	assert.True(t, hit, "LT(1,5) 应触发")
	assert.Equal(t, now.Add(-time.Hour).UnixMilli(), ft.gotReq.GetTimeRange().GetStartMs(),
		"非法窗口应回退 1 小时")
}

func TestAlertEvaluator_EvaluateRule_DayGranularityForLongWindow(t *testing.T) {
	ft := &fakeTrendAnalytics{resp: &ubaV1.EventTrendResponse{Total: 0}}
	e := newEvaluatorWithTrend(ft)

	now := time.Now()
	dayRule := &ent.AlertRule{WindowMinutes: 24 * 60, Comparison: "GT", Threshold: 0}
	_, _, err := e.evaluateRule(context.Background(), dayRule, now)
	require.NoError(t, err)
	assert.Equal(t, ubaV1.AnalyticsGranularity_DAY, ft.gotReq.GetGranularity(),
		">=24h 窗口应用日粒度")
	assert.Equal(t, now.Add(-24*time.Hour).UnixMilli(), ft.gotReq.GetTimeRange().GetStartMs())
}

func TestAlertEvaluator_EvaluateRule_TrendErrorPropagates(t *testing.T) {
	e := newEvaluatorWithTrend(&fakeTrendAnalytics{err: context.DeadlineExceeded})

	_, _, err := e.evaluateRule(context.Background(), &ent.AlertRule{WindowMinutes: 60}, time.Now())
	require.Error(t, err, "OLAP 查询失败必须上抛，不得静默判为未触发")
}

package clickhouse

import (
	"fmt"
	"time"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

func normTimeRange(tr *ubaV1.TimeRange) (int64, int64) {
	start := tr.GetStartMs()
	end := tr.GetEndMs()
	if end <= 0 {
		end = time.Now().UnixMilli()
	}
	if start <= 0 || start > end {
		start = end - int64(7*24*time.Hour/time.Millisecond)
	}
	return start, end
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
	}
	v, ok := m[dim]
	return v, ok
}

func metricExpr(metric string) (string, error) {
	switch metric {
	case "", "COUNT":
		return "count()", nil
	case "UNIQUE_USER":
		return "count(DISTINCT user_id)", nil
	case "SUM_AMOUNT":
		return "sum(toFloat64OrZero(toString(amount)))", nil
	default:
		return "", fmt.Errorf("unsupported metric: %s", metric)
	}
}

// chDerefStr 安全解引用字符串指针，nil 返回空串。
func chDerefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

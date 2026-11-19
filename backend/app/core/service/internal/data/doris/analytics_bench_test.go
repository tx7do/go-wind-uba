package doris

import (
	"fmt"
	"testing"
	"time"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// ============================================================================
// 查询构建核心路径的 benchmark。
// 所有共享大对象都在循环外构建一次，循环内只传指针，避免把对象拷贝计进开销。
// ============================================================================

// makeDimFilter 生成一个维度过滤条件。
func makeDimFilter(field string, op ubaV1.PropertyFilter_Operator, values ...string) *ubaV1.PropertyFilter {
	return &ubaV1.PropertyFilter{Scope: ubaV1.PropertyFilter_DIMENSION, Field: field, Op: op, Values: values}
}

// makePropFilter 生成一个事件属性 Map 键过滤条件。
func makePropFilter(field string, op ubaV1.PropertyFilter_Operator, values ...string) *ubaV1.PropertyFilter {
	return &ubaV1.PropertyFilter{Scope: ubaV1.PropertyFilter_EVENT_PROPERTY, Field: field, Op: op, Values: values}
}

// BenchmarkBuildFilterWhere 覆盖三种典型规模：
// 单条件（最常见）、8 条维度条件（复杂筛选面板）、32 条属性条件（分群圈选上限场景）。
// 说明：FilterGroup 目前只有扁平 Filters 列表（AND 组合），proto 无嵌套 OR 组，无法对嵌套结构建模。
func BenchmarkBuildFilterWhere(b *testing.B) {
	cases := []struct {
		name  string
		group *ubaV1.FilterGroup
	}{
		{
			name: "1filter",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{
				makeDimFilter("platform", ubaV1.PropertyFilter_EQ, "ios"),
			}},
		},
		{
			name: "8dimensionFilters",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{
				makeDimFilter("platform", ubaV1.PropertyFilter_EQ, "ios"),
				makeDimFilter("channel", ubaV1.PropertyFilter_NEQ, "test"),
				makeDimFilter("country", ubaV1.PropertyFilter_IN, "CN", "US", "JP"),
				makeDimFilter("app_version", ubaV1.PropertyFilter_CONTAINS, "2."),
				makeDimFilter("os", ubaV1.PropertyFilter_EQ, "windows"),
				makeDimFilter("network", ubaV1.PropertyFilter_GTE, "wifi"),
				makeDimFilter("event_category", ubaV1.PropertyFilter_LT, "page"),
				makeDimFilter("event_name", ubaV1.PropertyFilter_LTE, "pay_click"),
			}},
		},
		{
			name: "32propertyFilters",
			group: func() *ubaV1.FilterGroup {
				filters := make([]*ubaV1.PropertyFilter, 0, 32)
				for i := 0; i < 32; i++ {
					filters = append(filters, makePropFilter(fmt.Sprintf("attr_%d", i), ubaV1.PropertyFilter_GT, fmt.Sprintf("%d", i)))
				}
				return &ubaV1.FilterGroup{Filters: filters}
			}(),
		},
	}
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clauses, _, _, err := buildFilterWhere(bc.group)
				if err != nil {
					b.Fatal(err)
				}
				if len(clauses) == 0 {
					b.Fatal("clauses should not be empty")
				}
			}
		})
	}
}

// BenchmarkMetricExpr 指标表达式查表/报错两条路径。
func BenchmarkMetricExpr(b *testing.B) {
	valid := []string{"", "COUNT", "UNIQUE_USER", "SUM_AMOUNT", "AVG_AMOUNT", "PER_USER"}
	b.Run("valid", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			expr, err := metricExpr(valid[i%len(valid)], "")
			if err != nil || expr == "" {
				b.Fatalf("metricExpr(%q) unexpected: %q, %v", valid[i%len(valid)], expr, err)
			}
		}
	})
	b.Run("invalid", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := metricExpr("NOT_A_METRIC", ""); err == nil {
				b.Fatal("expected error for invalid metric")
			}
		}
	})
}

// BenchmarkValidateAdhocQuery 自助查询静态校验：合法语句与非法语句各一。
func BenchmarkValidateAdhocQuery(b *testing.B) {
	valid := "SELECT event_name, COUNT(*) FROM events_fact WHERE platform = 'ios' GROUP BY event_name LIMIT 100"
	invalid := "SELECT 1; DROP TABLE events_fact"
	b.Run("valid", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateAdhocQuery(valid); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("invalid", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := ValidateAdhocQuery(invalid); err == nil {
				b.Fatal("expected error for multi statement")
			}
		}
	})
}

// BenchmarkFillMissingBuckets 90 天小时桶补零（桶数上限场景，2160 个桶、一半缺口）。
func BenchmarkFillMissingBuckets(b *testing.B) {
	const hourMs = int64(time.Hour / time.Millisecond)
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := start + 90*24*hourMs
	// 一半小时桶有数据（每 2 小时一个点），模拟真实稀疏序列。
	pts := make([]*ubaV1.TimeSeriesPoint, 0, 90*24/2)
	for i := 0; i < 90*24; i += 2 {
		pts = append(pts, &ubaV1.TimeSeriesPoint{Timestamp: start + int64(i)*hourMs, Value: float64(i)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filled := fillMissingBuckets(pts, start, end, time.Hour)
		if len(filled) == 0 {
			b.Fatal("filled should not be empty")
		}
	}
}

// BenchmarkTopKeysByValue 大 map（1 万个 key）取 topN，模拟维度基数很高的分布统计。
func BenchmarkTopKeysByValue(b *testing.B) {
	const mapSize = 10000
	big := make(map[string]float64, mapSize)
	for i := 0; i < mapSize; i++ {
		big[fmt.Sprintf("value_%05d", i)] = float64(i%997) + float64(i)/float64(mapSize)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		top := topKeysByValue(big, 10)
		if len(top) != 10 {
			b.Fatalf("top length = %d, want 10", len(top))
		}
	}
}

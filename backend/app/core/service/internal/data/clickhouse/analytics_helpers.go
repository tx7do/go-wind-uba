package clickhouse

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// mapKeyRe 约束 Map 键名仅允许字母/数字/下划线，防止 SQL 注入。
var mapKeyRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

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

func metricExpr(metric string) (string, error) {
	switch metric {
	case "", "COUNT":
		return "count()", nil
	case "UNIQUE_USER":
		return "count(DISTINCT user_id)", nil
	case "SUM_AMOUNT":
		return "sum(toFloat64OrZero(toString(amount)))", nil
	case "AVG_AMOUNT":
		return "avg(toFloat64OrZero(toString(amount)))", nil
	case "PER_USER":
		// 人均次数：事件数 / 去重用户数（nullIf 防除零）
		return "count() * 1.0 / nullIf(count(DISTINCT user_id), 0)", nil
	default:
		return "", fmt.Errorf("unsupported metric: %s", metric)
	}
}

// operatorSQL 把 PropertyFilter.Operator 映射为 SQL 片段。返回值为前置片段，
// 占位符由调用方补齐（如 EQ 返回 "=", CONTAINS 返回 "ILIKE"）。
func operatorSQL(op ubaV1.PropertyFilter_Operator) (sql string, isIn bool, err error) {
	switch op {
	case ubaV1.PropertyFilter_OPERATOR_UNSPECIFIED, ubaV1.PropertyFilter_EQ:
		return "=", false, nil
	case ubaV1.PropertyFilter_NEQ:
		return "<>", false, nil
	case ubaV1.PropertyFilter_CONTAINS:
		// ClickHouse 无 ILIKE，用 lower(positionCaseInsensitive()) 模拟大小写不敏感包含
		return "positionCaseInsensitive({}, {}) > 0", false, nil
	case ubaV1.PropertyFilter_IN:
		return "IN", true, nil
	case ubaV1.PropertyFilter_GT:
		return ">", false, nil
	case ubaV1.PropertyFilter_GTE:
		return ">=", false, nil
	case ubaV1.PropertyFilter_LT:
		return "<", false, nil
	case ubaV1.PropertyFilter_LTE:
		return "<=", false, nil
	default:
		return "", false, fmt.Errorf("unsupported operator: %s", op)
	}
}

// resolveFilterColumn 把 (scope, field) 解析为安全的 SQL 列表达式。
//   - DIMENSION：走 allowedDimension 白名单；JOIN 维度返回带表别名前缀的列。
//   - EVENT_CONTEXT/EVENT_PROPERTY/EVENT_METRIC：Map 键名，校验 mapKeyRe 后拼成 map['key']。
//
// 返回 (列表达式, 是否需要 JOIN users_dim, 错误)。
func resolveFilterColumn(scope ubaV1.PropertyFilter_FieldScope, field string) (string, bool, error) {
	switch scope {
	case ubaV1.PropertyFilter_FIELD_SCOPE_UNSPECIFIED, ubaV1.PropertyFilter_DIMENSION:
		col, ok := allowedDimension(field)
		if !ok {
			return "", false, fmt.Errorf("invalid dimension field: %s", field)
		}
		if joinUsersDim(field) {
			return "u." + col, true, nil
		}
		return col, false, nil
	case ubaV1.PropertyFilter_EVENT_CONTEXT:
		if !mapKeyRe.MatchString(field) {
			return "", false, fmt.Errorf("invalid context key: %s", field)
		}
		return "context['" + field + "']", false, nil
	case ubaV1.PropertyFilter_EVENT_PROPERTY:
		if !mapKeyRe.MatchString(field) {
			return "", false, fmt.Errorf("invalid properties key: %s", field)
		}
		return "properties['" + field + "']", false, nil
	case ubaV1.PropertyFilter_EVENT_METRIC:
		if !mapKeyRe.MatchString(field) {
			return "", false, fmt.Errorf("invalid metrics key: %s", field)
		}
		return "metrics['" + field + "']", false, nil
	default:
		return "", false, fmt.Errorf("unsupported scope: %s", scope)
	}
}

// buildFilterWhere 把 FilterGroup 构造为 WHERE 子句片段与对应参数。
//   - needJoin 返回 true 表示该过滤组需要 JOIN users_dim（DIMENSION 含 user_level/vip_level）。
//   - 调用方需把 needJoin 与其他来源的 JOIN 判断做 OR 合并。
//
// 返回 (where 片段列表, 参数列表, 是否需要 JOIN, 错误)。
func buildFilterWhere(group *ubaV1.FilterGroup) ([]string, []any, bool, error) {
	if group == nil || len(group.GetFilters()) == 0 {
		return nil, nil, false, nil
	}
	var (
		clauses  []string
		args     []any
		needJoin bool
	)
	for _, f := range group.GetFilters() {
		if f == nil {
			continue
		}
		col, join, err := resolveFilterColumn(f.GetScope(), f.GetField())
		if err != nil {
			return nil, nil, false, err
		}
		if join {
			needJoin = true
		}
		opSQL, isIn, err := operatorSQL(f.GetOp())
		if err != nil {
			return nil, nil, false, err
		}
		vals := f.GetValues()
		if isIn {
			// IN (?, ?, ...)：至少 1 个值，否则跳过该条件
			if len(vals) == 0 {
				continue
			}
			placeholders := make([]string, len(vals))
			for i, v := range vals {
				placeholders[i] = "?"
				args = append(args, v)
			}
			clauses = append(clauses, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")))
			continue
		}
		// 单值算子：CONTAINS 用 positionCaseInsensitive 模板（双占位符），其余用标准占位符
		if len(vals) == 0 {
			continue
		}
		if f.GetOp() == ubaV1.PropertyFilter_CONTAINS {
			// 模板：positionCaseInsensitive({col}, {?}) > 0
			clauses = append(clauses, fmt.Sprintf(opSQL, col, "?"))
			args = append(args, vals[0])
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s %s ?", col, opSQL))
		args = append(args, vals[0])
	}
	return clauses, args, needJoin, nil
}

// dimensionColumnExpr 把维度名解析为 SELECT/GROUP BY 用的列表达式，返回 (列表达式, 是否需要 JOIN users_dim)。
// 与 resolveFilterColumn 的 DIMENSION 分支等价，但语义更清晰，供 EventTrend 维度拆分复用。
func dimensionColumnExpr(dim string) (string, bool, error) {
	col, ok := allowedDimension(dim)
	if !ok {
		return "", false, fmt.Errorf("invalid dimension: %s", dim)
	}
	if joinUsersDim(dim) {
		return "u." + col, true, nil
	}
	return col, false, nil
}

// chDerefStr 安全解引用字符串指针，nil 返回空串。
func chDerefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

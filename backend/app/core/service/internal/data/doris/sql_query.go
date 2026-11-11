package doris

import (
	"context"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// adhocQueryTimeout 单条自助查询的服务端超时。
	adhocQueryTimeout = 30 * time.Second

	// adhocQueryMaxRows 行数上限硬顶。
	adhocQueryMaxRows = 1000
)

// readOnlyKeywordWhitelist 允许的语句首关键字（只读白名单）。
var readOnlyKeywordWhitelist = []string{
	"SELECT", "SHOW", "DESC", "DESCRIBE", "EXPLAIN",
}

// forbiddenSQLMarkers 自助查询里不允许出现的片段：多语句、注释都可能改变语句语义。
var forbiddenSQLMarkers = []string{";", "--", "/*", "*/", "#"}

// ValidateAdhocQuery 只读查询静态校验：非空、单语句、无注释、首关键字在白名单。
func ValidateAdhocQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ubaV1.ErrorBadRequest("query is empty")
	}

	// 容忍结尾的单个分号，其余分号（多语句）一律拒绝
	trimmed = strings.TrimSuffix(trimmed, ";")
	if strings.Contains(trimmed, ";") {
		return ubaV1.ErrorBadRequest("multiple statements are not allowed")
	}

	lower := strings.ToLower(trimmed)
	for _, marker := range forbiddenSQLMarkers {
		if strings.Contains(lower, marker) {
			return ubaV1.ErrorBadRequest("comment markers are not allowed: %q", marker)
		}
	}

	first := strings.ToUpper(strings.Fields(trimmed)[0])
	allowed := false
	for _, kw := range readOnlyKeywordWhitelist {
		if first == kw {
			allowed = true
			break
		}
	}
	if !allowed {
		return ubaV1.ErrorBadRequest("only read-only queries (SELECT/SHOW/DESCRIBE/EXPLAIN) are allowed, got: %s", first)
	}

	return nil
}

// ExecuteAdhocQuery 执行自助只读查询：带超时与行数上限，结果转 JSON 安全值。
func (r *AnalyticsRepo) ExecuteAdhocQuery(
	ctx context.Context,
	query string,
	maxRows int32,
) (*ubaV1.ExecuteSqlQueryResponse, error) {
	if err := ValidateAdhocQuery(query); err != nil {
		return nil, err
	}
	if maxRows <= 0 {
		maxRows = 200
	}
	if maxRows > adhocQueryMaxRows {
		maxRows = adhocQueryMaxRows
	}

	runCtx, cancel := context.WithTimeout(ctx, adhocQueryTimeout)
	defer cancel()

	start := time.Now()

	rows, err := r.db.DB().QueryxContext(runCtx, strings.TrimSpace(query))
	if err != nil {
		r.log.Errorf("adhoc query failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("query failed: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		r.log.Errorf("adhoc query columns failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("query failed")
	}

	// 多读一行用于判定截断
	resultRows := make([]*ubaV1.SqlResultRow, 0, maxRows)
	truncated := false
	for rows.Next() {
		if int32(len(resultRows)) >= maxRows {
			truncated = true
			break
		}

		raw := make([]any, len(columns))
		for i := range raw {
			raw[i] = new(any)
		}
		if err := rows.Scan(raw...); err != nil {
			r.log.Errorf("adhoc query scan failed: %v", err)
			return nil, ubaV1.ErrorInternalServerError("query scan failed")
		}

		values := make([]*structpb.Value, 0, len(columns))
		for _, cell := range raw {
			v, convErr := toStructValue(*(cell.(*any)))
			if convErr != nil {
				return nil, ubaV1.ErrorInternalServerError("query result conversion failed")
			}
			values = append(values, v)
		}
		resultRows = append(resultRows, &ubaV1.SqlResultRow{Values: values})
	}
	if err := rows.Err(); err != nil {
		r.log.Errorf("adhoc query iterate failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("query failed")
	}

	return &ubaV1.ExecuteSqlQueryResponse{
		Columns:    columns,
		Rows:       resultRows,
		RowCount:   uint64(len(resultRows)),
		Truncated:  truncated,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// toStructValue 把驱动返回的任意标量转为 JSON 安全的 protobuf Value。
func toStructValue(v any) (*structpb.Value, error) {
	switch val := v.(type) {
	case nil:
		return structpb.NewNullValue(), nil
	case []byte:
		return structpb.NewStringValue(string(val)), nil
	case time.Time:
		return structpb.NewStringValue(val.Format(time.RFC3339Nano)), nil
	default:
		return structpb.NewValue(val)
	}
}

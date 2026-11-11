package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/trans"
	authnEngine "github.com/tx7do/kratos-authn/engine"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"

	auditV1 "go-wind-uba/api/gen/go/audit/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// adhocAnalytics 自助 SQL 的 OLAP 查询能力（Doris/ClickHouse 双实现共用签名）。
type adhocAnalytics interface {
	ExecuteAdhocQuery(ctx context.Context, query string, maxRows int32) (*ubaV1.ExecuteSqlQueryResponse, error)
}

// SqlQueryService 自助 SQL 查询：对 OLAP 分析库的受控只读通道。
// 安全边界（见 doris/clickhouse 的 ValidateAdhocQuery / ExecuteAdhocQuery）：
// 仅允许单条 SELECT/SHOW/DESCRIBE/EXPLAIN、拒绝注释与多语句、
// 强制行数上限（默认 200 / 硬顶 1000）与服务端 30s 超时。
// 每次执行写入数据访问审计日志（数据源/SQL/行数/耗时/成败）。
type SqlQueryService struct {
	ubaV1.UnimplementedSqlQueryServiceServer

	log       *log.Helper
	doris     *dorisData.AnalyticsRepo
	ch        *clickhouseData.AnalyticsRepo
	auditRepo *data.DataAccessAuditLogRepo
}

func NewSqlQueryService(
	ctx *bootstrap.Context,
	doris *dorisData.AnalyticsRepo,
	clickhouse *clickhouseData.AnalyticsRepo,
	auditRepo *data.DataAccessAuditLogRepo,
) *SqlQueryService {
	return &SqlQueryService{
		log:       ctx.NewLoggerHelper("sql-query/service/core-service"),
		doris:     doris,
		ch:        clickhouse,
		auditRepo: auditRepo,
	}
}

// analyticsImpl 按编译期引擎开关路由（与 alertEvaluator/reportEngine 同一约定）。
func (s *SqlQueryService) analyticsImpl() adhocAnalytics {
	if data.UseClickHouse && s.ch != nil {
		return s.ch
	}
	return s.doris
}

// dataSource 当前引擎名，写进数据访问审计日志。
func (s *SqlQueryService) dataSource() string {
	if data.UseClickHouse && s.ch != nil {
		return "clickhouse"
	}
	return "doris"
}

// ExecuteQuery 执行只读查询。
func (s *SqlQueryService) ExecuteQuery(ctx context.Context, req *ubaV1.ExecuteSqlQueryRequest) (*ubaV1.ExecuteSqlQueryResponse, error) {
	if req == nil || req.GetQuery() == "" {
		return nil, ubaV1.ErrorBadRequest("query is required")
	}

	resp, err := s.analyticsImpl().ExecuteAdhocQuery(ctx, req.GetQuery(), req.GetMaxRows())

	s.writeAudit(ctx, req.GetQuery(), resp, err)
	if err != nil {
		return nil, err
	}

	s.log.Infof("adhoc query executed: rows=%d truncated=%v duration=%dms query=%q",
		resp.GetRowCount(), resp.GetTruncated(), resp.GetDurationMs(), req.GetQuery())

	return resp, nil
}

// writeAudit 查询落数据访问审计日志；审计失败不影响查询结果返回。
func (s *SqlQueryService) writeAudit(ctx context.Context, query string, resp *ubaV1.ExecuteSqlQueryResponse, execErr error) {
	if s.auditRepo == nil {
		return
	}

	entry := &auditV1.DataAccessAuditLog{
		DataSource: trans.Ptr(s.dataSource()),
		TableName:  trans.Ptr("adhoc"),
		SqlText:    trans.Ptr(query),
		SqlDigest:  trans.Ptr(digestSQL(query)),
		Success:    trans.Ptr(execErr == nil),
	}

	if resp != nil {
		entry.AffectedRows = trans.Ptr(uint32(resp.GetRowCount()))
		entry.LatencyMs = trans.Ptr(uint32(resp.GetDurationMs()))
	}

	if claims, ok := authnEngine.AuthClaimsFromContext(ctx); ok {
		if uid, err := claims.GetInt64("uid"); err == nil {
			entry.UserId = &uid
		}
		if name, err := claims.GetString(authnEngine.ClaimFieldSubject); err == nil && name != "" {
			entry.Username = &name
		}
	}

	if err := s.auditRepo.Create(ctx, &auditV1.CreateDataAccessAuditLogRequest{Data: entry}); err != nil {
		s.log.Errorf("write data access audit log failed: %v", err)
	}
}

// digestSQL 轻量 SQL 摘要：去空白后截断，便于聚合同一查询。
func digestSQL(query string) string {
	runes := []rune(query)
	const max = 64
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}

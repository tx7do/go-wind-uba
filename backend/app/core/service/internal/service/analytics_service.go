package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	"go-wind-uba/app/core/service/internal/data/doris"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// AnalyticsService 实现 uba.service.v1.AnalyticsServiceServer，提供 BI 聚合查询。
// 与 RiskEventService 一致，按 data.UseClickHouse 在 Doris / ClickHouse 之间路由。
type AnalyticsService struct {
	ubaV1.UnimplementedAnalyticsServiceServer

	log *log.Helper

	dorisRepo *doris.AnalyticsRepo
	ckRepo    *clickhouse.AnalyticsRepo
}

func NewAnalyticsService(
	ctx *bootstrap.Context,
	dorisRepo *doris.AnalyticsRepo,
	ckRepo *clickhouse.AnalyticsRepo,
) *AnalyticsService {
	return &AnalyticsService{
		log:       ctx.NewLoggerHelper("analytics/service/core-service"),
		dorisRepo: dorisRepo,
		ckRepo:    ckRepo,
	}
}

func (s *AnalyticsService) repo() interface {
	EventTrend(context.Context, *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error)
	Funnel(context.Context, *ubaV1.FunnelRequest) (*ubaV1.FunnelResponse, error)
	Retention(context.Context, *ubaV1.RetentionRequest) (*ubaV1.RetentionResponse, error)
	GroupBy(context.Context, *ubaV1.GroupByRequest) (*ubaV1.GroupByResponse, error)
	ActiveUsers(context.Context, *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error)
} {
	if data.UseClickHouse {
		return s.ckRepo
	}
	return s.dorisRepo
}

func (s *AnalyticsService) EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error) {
	return s.repo().EventTrend(ctx, req)
}

func (s *AnalyticsService) Funnel(ctx context.Context, req *ubaV1.FunnelRequest) (*ubaV1.FunnelResponse, error) {
	return s.repo().Funnel(ctx, req)
}

func (s *AnalyticsService) Retention(ctx context.Context, req *ubaV1.RetentionRequest) (*ubaV1.RetentionResponse, error) {
	return s.repo().Retention(ctx, req)
}

func (s *AnalyticsService) GroupBy(ctx context.Context, req *ubaV1.GroupByRequest) (*ubaV1.GroupByResponse, error) {
	return s.repo().GroupBy(ctx, req)
}

func (s *AnalyticsService) ActiveUsers(ctx context.Context, req *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error) {
	return s.repo().ActiveUsers(ctx, req)
}

package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// AnalyticsService 实现 admin.service.v1.AnalyticsServiceHTTPServer，
// 作为 HTTP 网关转发至 core 层 gRPC AnalyticsService。自身不含业务逻辑。
type AnalyticsService struct {
	client ubaV1.AnalyticsServiceClient
	log    *log.Helper
}

func NewAnalyticsService(
	ctx *bootstrap.Context,
	client ubaV1.AnalyticsServiceClient,
) *AnalyticsService {
	return &AnalyticsService{
		log:    ctx.NewLoggerHelper("analytics/service/admin-service"),
		client: client,
	}
}

func (s *AnalyticsService) EventTrend(ctx context.Context, req *ubaV1.EventTrendRequest) (*ubaV1.EventTrendResponse, error) {
	return s.client.EventTrend(ctx, req)
}

func (s *AnalyticsService) Funnel(ctx context.Context, req *ubaV1.FunnelRequest) (*ubaV1.FunnelResponse, error) {
	return s.client.Funnel(ctx, req)
}

func (s *AnalyticsService) Retention(ctx context.Context, req *ubaV1.RetentionRequest) (*ubaV1.RetentionResponse, error) {
	return s.client.Retention(ctx, req)
}

func (s *AnalyticsService) GroupBy(ctx context.Context, req *ubaV1.GroupByRequest) (*ubaV1.GroupByResponse, error) {
	return s.client.GroupBy(ctx, req)
}

func (s *AnalyticsService) ActiveUsers(ctx context.Context, req *ubaV1.ActiveUsersRequest) (*ubaV1.ActiveUsersResponse, error) {
	return s.client.ActiveUsers(ctx, req)
}

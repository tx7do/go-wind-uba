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

func (s *AnalyticsService) Attribution(ctx context.Context, req *ubaV1.AttributionRequest) (*ubaV1.AttributionResponse, error) {
	return s.client.Attribution(ctx, req)
}

func (s *AnalyticsService) Distribution(ctx context.Context, req *ubaV1.DistributionRequest) (*ubaV1.DistributionResponse, error) {
	return s.client.Distribution(ctx, req)
}

func (s *AnalyticsService) BehaviorSequence(ctx context.Context, req *ubaV1.BehaviorSequenceRequest) (*ubaV1.BehaviorSequenceResponse, error) {
	return s.client.BehaviorSequence(ctx, req)
}

func (s *AnalyticsService) Segmentation(ctx context.Context, req *ubaV1.SegmentationRequest) (*ubaV1.SegmentationResponse, error) {
	return s.client.Segmentation(ctx, req)
}

func (s *AnalyticsService) Click(ctx context.Context, req *ubaV1.ClickRequest) (*ubaV1.ClickResponse, error) {
	return s.client.Click(ctx, req)
}

func (s *AnalyticsService) Lifecycle(ctx context.Context, req *ubaV1.LifecycleRequest) (*ubaV1.LifecycleResponse, error) {
	return s.client.Lifecycle(ctx, req)
}

func (s *AnalyticsService) Churn(ctx context.Context, req *ubaV1.ChurnRequest) (*ubaV1.ChurnResponse, error) {
	return s.client.Churn(ctx, req)
}

func (s *AnalyticsService) Interval(ctx context.Context, req *ubaV1.IntervalRequest) (*ubaV1.IntervalResponse, error) {
	return s.client.Interval(ctx, req)
}

func (s *AnalyticsService) Matrix(ctx context.Context, req *ubaV1.MatrixRequest) (*ubaV1.MatrixResponse, error) {
	return s.client.Matrix(ctx, req)
}

func (s *AnalyticsService) Revenue(ctx context.Context, req *ubaV1.RevenueRequest) (*ubaV1.RevenueResponse, error) {
	return s.client.Revenue(ctx, req)
}

func (s *AnalyticsService) SessionAnalysis(ctx context.Context, req *ubaV1.SessionAnalysisRequest) (*ubaV1.SessionAnalysisResponse, error) {
	return s.client.SessionAnalysis(ctx, req)
}

func (s *AnalyticsService) Anomaly(ctx context.Context, req *ubaV1.AnomalyRequest) (*ubaV1.AnomalyResponse, error) {
	return s.client.Anomaly(ctx, req)
}

func (s *AnalyticsService) NewVsOld(ctx context.Context, req *ubaV1.NewVsOldRequest) (*ubaV1.NewVsOldResponse, error) {
	return s.client.NewVsOld(ctx, req)
}

func (s *AnalyticsService) PathSankey(ctx context.Context, req *ubaV1.PathSankeyRequest) (*ubaV1.PathSankeyResponse, error) {
	return s.client.PathSankey(ctx, req)
}

func (s *AnalyticsService) LevelAnalysis(ctx context.Context, req *ubaV1.LevelAnalysisRequest) (*ubaV1.LevelAnalysisResponse, error) {
	return s.client.LevelAnalysis(ctx, req)
}

func (s *AnalyticsService) WhaleTier(ctx context.Context, req *ubaV1.WhaleTierRequest) (*ubaV1.WhaleTierResponse, error) {
	return s.client.WhaleTier(ctx, req)
}

func (s *AnalyticsService) LTV(ctx context.Context, req *ubaV1.LTVRequest) (*ubaV1.LTVResponse, error) {
	return s.client.LTV(ctx, req)
}

func (s *AnalyticsService) ServerRetention(ctx context.Context, req *ubaV1.ServerRetentionRequest) (*ubaV1.ServerRetentionResponse, error) {
	return s.client.ServerRetention(ctx, req)
}

func (s *AnalyticsService) OnlineStats(ctx context.Context, req *ubaV1.OnlineStatsRequest) (*ubaV1.OnlineStatsResponse, error) {
	return s.client.OnlineStats(ctx, req)
}

func (s *AnalyticsService) Economy(ctx context.Context, req *ubaV1.EconomyRequest) (*ubaV1.EconomyResponse, error) {
	return s.client.Economy(ctx, req)
}

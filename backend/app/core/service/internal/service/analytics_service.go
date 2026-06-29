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
	Attribution(context.Context, *ubaV1.AttributionRequest) (*ubaV1.AttributionResponse, error)
	Distribution(context.Context, *ubaV1.DistributionRequest) (*ubaV1.DistributionResponse, error)
	BehaviorSequence(context.Context, *ubaV1.BehaviorSequenceRequest) (*ubaV1.BehaviorSequenceResponse, error)
	Segmentation(context.Context, *ubaV1.SegmentationRequest) (*ubaV1.SegmentationResponse, error)
	Click(context.Context, *ubaV1.ClickRequest) (*ubaV1.ClickResponse, error)
	Lifecycle(context.Context, *ubaV1.LifecycleRequest) (*ubaV1.LifecycleResponse, error)
	Churn(context.Context, *ubaV1.ChurnRequest) (*ubaV1.ChurnResponse, error)
	Interval(context.Context, *ubaV1.IntervalRequest) (*ubaV1.IntervalResponse, error)
	Matrix(context.Context, *ubaV1.MatrixRequest) (*ubaV1.MatrixResponse, error)
	Revenue(context.Context, *ubaV1.RevenueRequest) (*ubaV1.RevenueResponse, error)
	SessionAnalysis(context.Context, *ubaV1.SessionAnalysisRequest) (*ubaV1.SessionAnalysisResponse, error)
	Anomaly(context.Context, *ubaV1.AnomalyRequest) (*ubaV1.AnomalyResponse, error)
	NewVsOld(context.Context, *ubaV1.NewVsOldRequest) (*ubaV1.NewVsOldResponse, error)
	PathSankey(context.Context, *ubaV1.PathSankeyRequest) (*ubaV1.PathSankeyResponse, error)
	LevelAnalysis(context.Context, *ubaV1.LevelAnalysisRequest) (*ubaV1.LevelAnalysisResponse, error)
	WhaleTier(context.Context, *ubaV1.WhaleTierRequest) (*ubaV1.WhaleTierResponse, error)
	LTV(context.Context, *ubaV1.LTVRequest) (*ubaV1.LTVResponse, error)
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

func (s *AnalyticsService) Attribution(ctx context.Context, req *ubaV1.AttributionRequest) (*ubaV1.AttributionResponse, error) {
	return s.repo().Attribution(ctx, req)
}

func (s *AnalyticsService) Distribution(ctx context.Context, req *ubaV1.DistributionRequest) (*ubaV1.DistributionResponse, error) {
	return s.repo().Distribution(ctx, req)
}

func (s *AnalyticsService) BehaviorSequence(ctx context.Context, req *ubaV1.BehaviorSequenceRequest) (*ubaV1.BehaviorSequenceResponse, error) {
	return s.repo().BehaviorSequence(ctx, req)
}

func (s *AnalyticsService) Segmentation(ctx context.Context, req *ubaV1.SegmentationRequest) (*ubaV1.SegmentationResponse, error) {
	return s.repo().Segmentation(ctx, req)
}

func (s *AnalyticsService) Click(ctx context.Context, req *ubaV1.ClickRequest) (*ubaV1.ClickResponse, error) {
	return s.repo().Click(ctx, req)
}

func (s *AnalyticsService) Lifecycle(ctx context.Context, req *ubaV1.LifecycleRequest) (*ubaV1.LifecycleResponse, error) {
	return s.repo().Lifecycle(ctx, req)
}

func (s *AnalyticsService) Churn(ctx context.Context, req *ubaV1.ChurnRequest) (*ubaV1.ChurnResponse, error) {
	return s.repo().Churn(ctx, req)
}

func (s *AnalyticsService) Interval(ctx context.Context, req *ubaV1.IntervalRequest) (*ubaV1.IntervalResponse, error) {
	return s.repo().Interval(ctx, req)
}

func (s *AnalyticsService) Matrix(ctx context.Context, req *ubaV1.MatrixRequest) (*ubaV1.MatrixResponse, error) {
	return s.repo().Matrix(ctx, req)
}

func (s *AnalyticsService) Revenue(ctx context.Context, req *ubaV1.RevenueRequest) (*ubaV1.RevenueResponse, error) {
	return s.repo().Revenue(ctx, req)
}

func (s *AnalyticsService) SessionAnalysis(ctx context.Context, req *ubaV1.SessionAnalysisRequest) (*ubaV1.SessionAnalysisResponse, error) {
	return s.repo().SessionAnalysis(ctx, req)
}

func (s *AnalyticsService) Anomaly(ctx context.Context, req *ubaV1.AnomalyRequest) (*ubaV1.AnomalyResponse, error) {
	return s.repo().Anomaly(ctx, req)
}

func (s *AnalyticsService) NewVsOld(ctx context.Context, req *ubaV1.NewVsOldRequest) (*ubaV1.NewVsOldResponse, error) {
	return s.repo().NewVsOld(ctx, req)
}

func (s *AnalyticsService) PathSankey(ctx context.Context, req *ubaV1.PathSankeyRequest) (*ubaV1.PathSankeyResponse, error) {
	return s.repo().PathSankey(ctx, req)
}

func (s *AnalyticsService) LevelAnalysis(ctx context.Context, req *ubaV1.LevelAnalysisRequest) (*ubaV1.LevelAnalysisResponse, error) {
	return s.repo().LevelAnalysis(ctx, req)
}

func (s *AnalyticsService) WhaleTier(ctx context.Context, req *ubaV1.WhaleTierRequest) (*ubaV1.WhaleTierResponse, error) {
	return s.repo().WhaleTier(ctx, req)
}

func (s *AnalyticsService) LTV(ctx context.Context, req *ubaV1.LTVRequest) (*ubaV1.LTVResponse, error) {
	return s.repo().LTV(ctx, req)
}

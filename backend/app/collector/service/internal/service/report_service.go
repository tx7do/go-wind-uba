package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"
	"google.golang.org/protobuf/types/known/emptypb"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type ReportService struct {
	collectorV1.ReportServiceHTTPServer

	kafkaBroker broker.Broker
	log         *log.Helper

	applicationServiceClient ubaV1.ApplicationServiceClient
}

func NewReportService(
	ctx *bootstrap.Context,
	kafkaBroker broker.Broker,
	applicationServiceClient ubaV1.ApplicationServiceClient,
) *ReportService {
	return &ReportService{
		log:                      ctx.NewLoggerHelper("report/service/collector-service"),
		kafkaBroker:              kafkaBroker,
		applicationServiceClient: applicationServiceClient,
	}
}

func (s *ReportService) PostReport(ctx context.Context, req *ubaV1.PostReportRequest) (*ubaV1.PostReportResponse, error) {
	if req == nil || len(req.Events) == 0 {
		return nil, ubaV1.ErrorBadRequest("request data is required")
	}

	requestID := uuid.New().String()

	for _, event := range req.Events {
		if event == nil {
			s.log.Warnf("invalid event data: %v", event)
			continue
		}

		switch event.GetEventType() {
		case ubaV1.ReportEvent_BEHAVIOR:
			// 处理行为事件
			if err := s.handleBehavior(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle behavior event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_PATH:
			// 处理路径事件
			if err := s.handlePath(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle path event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_RISK:
			// 处理风险事件
			if err := s.handleRisk(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle risk event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_SESSION:
			// 处理会话事件
			if err := s.handleSession(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle session event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_FUNNEL:
			// 处理漏斗事件
			if err := s.handleFunnel(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle funnel event: %v", err)
				continue
			}

		default:
			s.log.Warnf("unsupported event type: %v", event.GetEventType())
			continue
		}
	}

	return &ubaV1.PostReportResponse{
		Success:      true,
		Message:      "accepted",
		RequestId:    requestID,
		ServerTime:   time.Now().UnixMilli(),
		TotalCount:   int32(len(req.Events)),
		SuccessCount: int32(len(req.Events)),
	}, nil
}

func (s *ReportService) HealthCheck(_ context.Context, _ *emptypb.Empty) (*collectorV1.HealthCheckResponse, error) {
	return &collectorV1.HealthCheckResponse{
		Status:    collectorV1.HealthCheckResponse_OK,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// handleBehavior 处理行为事件
func (s *ReportService) handleBehavior(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

// handlePath 处理路径事件
func (s *ReportService) handlePath(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

// handleRisk 处理风险事件
func (s *ReportService) handleRisk(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

// handleSession 处理会话事件
func (s *ReportService) handleSession(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

// handleFunnel 处理漏斗事件
func (s *ReportService) handleFunnel(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

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

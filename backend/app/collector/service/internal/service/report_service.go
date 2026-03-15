package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
)

type ReportService struct {
	collectorV1.ReportServiceHTTPServer

	kafkaBroker broker.Broker
	log         *log.Helper
}

func NewReportService(ctx *bootstrap.Context, kafkaBroker broker.Broker) *ReportService {
	return &ReportService{
		log:         ctx.NewLoggerHelper("report/service/collector-service"),
		kafkaBroker: kafkaBroker,
	}
}

func (s *ReportService) PostReport(ctx context.Context, req *collectorV1.PostReportRequest) (*collectorV1.PostReportResponse, error) {
	return &collectorV1.PostReportResponse{
		Code: 0,
		Msg:  "success",
	}, nil
}

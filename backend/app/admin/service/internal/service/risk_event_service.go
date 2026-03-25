package service

import (
	"context"
	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventService struct {
	adminV1.RiskEventServiceHTTPServer

	log                 *log.Helper
	objectServiceClient ubaV1.RiskEventServiceClient
}

func NewRiskEventService(
	ctx *bootstrap.Context,
	objectServiceClient ubaV1.RiskEventServiceClient,
) *RiskEventService {
	svc := &RiskEventService{
		log:                 ctx.NewLoggerHelper("risk-event/service/admin-service"),
		objectServiceClient: objectServiceClient,
	}

	svc.init()

	return svc
}

func (s *RiskEventService) init() {
}

func (s *RiskEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskEventResponse, error) {
	return s.objectServiceClient.List(ctx, req)
}

func (s *RiskEventService) Get(ctx context.Context, req *ubaV1.GetRiskEventRequest) (*ubaV1.RiskEvent, error) {
	return s.objectServiceClient.Get(ctx, req)
}

func (s *RiskEventService) Create(ctx context.Context, req *ubaV1.CreateRiskEventRequest) (*ubaV1.RiskEvent, error) {
	return s.objectServiceClient.Create(ctx, req)
}

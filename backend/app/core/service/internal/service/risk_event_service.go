package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventService struct {
	ubaV1.UnimplementedRiskEventServiceServer

	log *log.Helper
}

func NewRiskEventService(
	ctx *bootstrap.Context,
) *RiskEventService {
	svc := &RiskEventService{
		log: ctx.NewLoggerHelper("risk-event/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *RiskEventService) init() {
}

func (s *RiskEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskEventResponse, error) {
	return nil, nil
}

func (s *RiskEventService) Get(ctx context.Context, req *ubaV1.GetRiskEventRequest) (*ubaV1.RiskEvent, error) {
	return nil, nil
}

func (s *RiskEventService) Create(ctx context.Context, req *ubaV1.CreateRiskEventRequest) (*ubaV1.RiskEvent, error) {
	return nil, nil
}

package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	"go-wind-uba/app/core/service/internal/data/doris"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventService struct {
	ubaV1.UnimplementedRiskEventServiceServer

	log *log.Helper

	riskEventDorisRepo *doris.RiskEventsRepo
	riskEventCkRepo    *clickhouse.RiskEventsRepo
}

func NewRiskEventService(
	ctx *bootstrap.Context,
	riskEventDorisRepo *doris.RiskEventsRepo,
	riskEventCkRepo *clickhouse.RiskEventsRepo,
) *RiskEventService {
	svc := &RiskEventService{
		log:                ctx.NewLoggerHelper("risk-event/service/core-service"),
		riskEventDorisRepo: riskEventDorisRepo,
		riskEventCkRepo:    riskEventCkRepo,
	}

	svc.init()

	return svc
}

func (s *RiskEventService) init() {
}

func (s *RiskEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskEventResponse, error) {
	if data.UseClickHouse {
		return s.riskEventCkRepo.List(ctx, req)
	} else {
		return s.riskEventDorisRepo.List(ctx, req)
	}
}

func (s *RiskEventService) Get(ctx context.Context, req *ubaV1.GetRiskEventRequest) (*ubaV1.RiskEvent, error) {
	return nil, nil
}

func (s *RiskEventService) Create(ctx context.Context, req *ubaV1.RiskEvent) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.riskEventCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.riskEventDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *RiskEventService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateRiskEventRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.riskEventCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.riskEventDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

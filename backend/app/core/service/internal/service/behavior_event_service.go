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

type BehaviorEventService struct {
	ubaV1.UnimplementedBehaviorEventServiceServer

	log *log.Helper

	eventDorisRepo *doris.EventsFactRepo
	eventCkRepo    *clickhouse.EventsFactRepo
}

func NewBehaviorEventService(
	ctx *bootstrap.Context,
	eventDorisRepo *doris.EventsFactRepo,
	eventCkRepo *clickhouse.EventsFactRepo,
) *BehaviorEventService {
	svc := &BehaviorEventService{
		log:            ctx.NewLoggerHelper("behavior-event/service/core-service"),
		eventDorisRepo: eventDorisRepo,
		eventCkRepo:    eventCkRepo,
	}

	svc.init()

	return svc
}

func (s *BehaviorEventService) init() {
}

func (s *BehaviorEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListBehaviorEventResponse, error) {
	if data.UseClickHouse {
		return s.eventCkRepo.List(ctx, req)
	} else {
		return s.eventDorisRepo.List(ctx, req)
	}
}

func (s *BehaviorEventService) Get(ctx context.Context, req *ubaV1.GetBehaviorEventRequest) (*ubaV1.BehaviorEvent, error) {
	return nil, nil
}

func (s *BehaviorEventService) Create(ctx context.Context, req *ubaV1.BehaviorEvent) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.eventCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.eventDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *BehaviorEventService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateBehaviorEventRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.eventCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.eventDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

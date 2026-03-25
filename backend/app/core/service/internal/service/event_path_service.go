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

type EventPathService struct {
	ubaV1.UnimplementedEventPathServiceServer

	log *log.Helper

	pathDorisRepo *doris.PathFeaturesRepo
	pathCkRepo    *clickhouse.PathFeaturesRepo
}

func NewEventPathService(
	ctx *bootstrap.Context,
	pathDorisRepo *doris.PathFeaturesRepo,
	pathCkRepo *clickhouse.PathFeaturesRepo,
) *EventPathService {
	svc := &EventPathService{
		log:           ctx.NewLoggerHelper("event-path/service/core-service"),
		pathDorisRepo: pathDorisRepo,
		pathCkRepo:    pathCkRepo,
	}

	svc.init()

	return svc
}

func (s *EventPathService) init() {
}

func (s *EventPathService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventPathResponse, error) {
	if data.UseClickHouse {
		return s.pathCkRepo.List(ctx, req)
	} else {
		return s.pathDorisRepo.List(ctx, req)
	}
}

func (s *EventPathService) Get(ctx context.Context, req *ubaV1.GetEventPathRequest) (*ubaV1.EventPath, error) {
	return nil, nil
}

func (s *EventPathService) Create(ctx context.Context, req *ubaV1.EventPath) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.pathCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.pathDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *EventPathService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateEventPathRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.pathCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.pathDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

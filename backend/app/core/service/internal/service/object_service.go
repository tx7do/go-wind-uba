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

type ObjectService struct {
	ubaV1.UnimplementedObjectServiceServer

	log *log.Helper

	objectDorisRepo *doris.ObjectsDimRepo
	objectCkRepo    *clickhouse.ObjectsDimRepo
}

func NewObjectService(
	ctx *bootstrap.Context,
	objectDorisRepo *doris.ObjectsDimRepo,
	objectCkRepo *clickhouse.ObjectsDimRepo,
) *ObjectService {
	svc := &ObjectService{
		log:             ctx.NewLoggerHelper("object-dim/service/core-service"),
		objectDorisRepo: objectDorisRepo,
		objectCkRepo:    objectCkRepo,
	}

	svc.init()

	return svc
}

func (s *ObjectService) init() {
}

func (s *ObjectService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListObjectDimResponse, error) {
	if data.UseClickHouse {
		return s.objectCkRepo.List(ctx, req)
	} else {
		return s.objectDorisRepo.List(ctx, req)
	}
}

func (s *ObjectService) Get(ctx context.Context, req *ubaV1.GetObjectDimRequest) (*ubaV1.ObjectDim, error) {
	return nil, nil
}

func (s *ObjectService) Create(ctx context.Context, req *ubaV1.ObjectDim) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.objectCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.objectDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *ObjectService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateObjectDimRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.objectCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.objectDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

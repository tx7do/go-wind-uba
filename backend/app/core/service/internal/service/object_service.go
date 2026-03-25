package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type ObjectService struct {
	ubaV1.UnimplementedObjectServiceServer

	log *log.Helper
}

func NewObjectService(
	ctx *bootstrap.Context,
) *ObjectService {
	svc := &ObjectService{
		log: ctx.NewLoggerHelper("object-dim/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *ObjectService) init() {
}

func (s *ObjectService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListObjectDimResponse, error) {
	return nil, nil
}

func (s *ObjectService) Get(ctx context.Context, req *ubaV1.GetObjectDimRequest) (*ubaV1.ObjectDim, error) {
	return nil, nil
}

func (s *ObjectService) Create(ctx context.Context, req *ubaV1.CreateObjectDimRequest) (*ubaV1.ObjectDim, error) {
	return nil, nil
}

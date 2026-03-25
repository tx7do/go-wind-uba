package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type ObjectService struct {
	adminV1.UnimplementedObjectServiceServer

	log *log.Helper

	objectServiceClient ubaV1.ObjectServiceClient
}

func NewObjectService(
	ctx *bootstrap.Context,
	objectServiceClient ubaV1.ObjectServiceClient,
) *ObjectService {
	svc := &ObjectService{
		log:                 ctx.NewLoggerHelper("object-dim/service/core-service"),
		objectServiceClient: objectServiceClient,
	}

	svc.init()

	return svc
}

func (s *ObjectService) init() {
}

func (s *ObjectService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListObjectDimResponse, error) {
	return s.objectServiceClient.List(ctx, req)
}

func (s *ObjectService) Get(ctx context.Context, req *ubaV1.GetObjectDimRequest) (*ubaV1.ObjectDim, error) {
	return s.objectServiceClient.Get(ctx, req)
}

func (s *ObjectService) Create(ctx context.Context, req *ubaV1.CreateObjectDimRequest) (*ubaV1.ObjectDim, error) {
	return s.objectServiceClient.Create(ctx, req)
}

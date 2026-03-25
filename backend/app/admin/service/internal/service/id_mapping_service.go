package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type IDMappingService struct {
	adminV1.IDMappingServiceHTTPServer

	log *log.Helper

	idMappingServiceClient ubaV1.IDMappingServiceClient
}

func NewIDMappingService(
	ctx *bootstrap.Context,
	idMappingServiceClient ubaV1.IDMappingServiceClient,
) *IDMappingService {
	svc := &IDMappingService{
		log:                    ctx.NewLoggerHelper("id-mapping/service/admin-service"),
		idMappingServiceClient: idMappingServiceClient,
	}

	svc.init()

	return svc
}

func (s *IDMappingService) init() {
}

func (s *IDMappingService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListIDMappingResponse, error) {
	resp, err := s.idMappingServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *IDMappingService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountIDMappingResponse, error) {
	return s.idMappingServiceClient.Count(ctx, req)
}

func (s *IDMappingService) Get(ctx context.Context, req *ubaV1.GetIDMappingRequest) (*ubaV1.IDMapping, error) {
	resp, err := s.idMappingServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *IDMappingService) Create(ctx context.Context, req *ubaV1.CreateIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.idMappingServiceClient.Create(ctx, req)
}

func (s *IDMappingService) Update(ctx context.Context, req *ubaV1.UpdateIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.idMappingServiceClient.Update(ctx, req)
}

func (s *IDMappingService) Delete(ctx context.Context, req *ubaV1.DeleteIDMappingRequest) (*emptypb.Empty, error) {
	return s.idMappingServiceClient.Delete(ctx, req)
}

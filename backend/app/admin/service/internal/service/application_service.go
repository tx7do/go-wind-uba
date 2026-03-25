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

type ApplicationService struct {
	adminV1.ApplicationServiceHTTPServer

	log *log.Helper

	applicationServiceClient ubaV1.ApplicationServiceClient
}

func NewApplicationService(
	ctx *bootstrap.Context,
	applicationServiceClient ubaV1.ApplicationServiceClient,
) *ApplicationService {
	svc := &ApplicationService{
		log:                      ctx.NewLoggerHelper("application/service/admin-service"),
		applicationServiceClient: applicationServiceClient,
	}

	svc.init()

	return svc
}

func (s *ApplicationService) init() {
}

func (s *ApplicationService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListApplicationResponse, error) {
	resp, err := s.applicationServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ApplicationService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountApplicationResponse, error) {
	return s.applicationServiceClient.Count(ctx, req)
}

func (s *ApplicationService) Get(ctx context.Context, req *ubaV1.GetApplicationRequest) (*ubaV1.Application, error) {
	resp, err := s.applicationServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ApplicationService) Create(ctx context.Context, req *ubaV1.CreateApplicationRequest) (*ubaV1.Application, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.applicationServiceClient.Create(ctx, req)
}

func (s *ApplicationService) Update(ctx context.Context, req *ubaV1.UpdateApplicationRequest) (*ubaV1.Application, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.applicationServiceClient.Update(ctx, req)
}

func (s *ApplicationService) Delete(ctx context.Context, req *ubaV1.DeleteApplicationRequest) (*emptypb.Empty, error) {
	return s.applicationServiceClient.Delete(ctx, req)
}

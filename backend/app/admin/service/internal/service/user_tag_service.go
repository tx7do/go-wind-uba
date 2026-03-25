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

type UserTagService struct {
	adminV1.UserTagServiceHTTPServer

	log *log.Helper

	userTagServiceClient ubaV1.UserTagServiceClient
}

func NewUserTagService(
	ctx *bootstrap.Context,
	userTagServiceClient ubaV1.UserTagServiceClient,
) *UserTagService {
	svc := &UserTagService{
		log:                  ctx.NewLoggerHelper("user-tag/service/admin-service"),
		userTagServiceClient: userTagServiceClient,
	}

	svc.init()

	return svc
}

func (s *UserTagService) init() {
}

func (s *UserTagService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserTagResponse, error) {
	resp, err := s.userTagServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserTagService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountUserTagResponse, error) {
	return s.userTagServiceClient.Count(ctx, req)
}

func (s *UserTagService) Get(ctx context.Context, req *ubaV1.GetUserTagRequest) (*ubaV1.UserTag, error) {
	resp, err := s.userTagServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserTagService) Create(ctx context.Context, req *ubaV1.CreateUserTagRequest) (*ubaV1.UserTag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.userTagServiceClient.Create(ctx, req)
}

func (s *UserTagService) Update(ctx context.Context, req *ubaV1.UpdateUserTagRequest) (*ubaV1.UserTag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.userTagServiceClient.Update(ctx, req)
}

func (s *UserTagService) Delete(ctx context.Context, req *ubaV1.DeleteUserTagRequest) (*emptypb.Empty, error) {
	return s.userTagServiceClient.Delete(ctx, req)
}

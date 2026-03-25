package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type UserTagService struct {
	ubaV1.UnimplementedUserTagServiceServer

	log *log.Helper

	userTagRepo *data.UserTagRepo
}

func NewUserTagService(
	ctx *bootstrap.Context,
	userTagRepo *data.UserTagRepo,
) *UserTagService {
	svc := &UserTagService{
		log:         ctx.NewLoggerHelper("user-tag/service/core-service"),
		userTagRepo: userTagRepo,
	}

	svc.init()

	return svc
}

func (s *UserTagService) init() {
}

func (s *UserTagService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserTagResponse, error) {
	resp, err := s.userTagRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserTagService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountUserTagResponse, error) {
	return s.userTagRepo.Count(ctx, req)
}

func (s *UserTagService) Get(ctx context.Context, req *ubaV1.GetUserTagRequest) (*ubaV1.UserTag, error) {
	resp, err := s.userTagRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserTagService) Create(ctx context.Context, req *ubaV1.CreateUserTagRequest) (*ubaV1.UserTag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.userTagRepo.Create(ctx, req)
}

func (s *UserTagService) Update(ctx context.Context, req *ubaV1.UpdateUserTagRequest) (*ubaV1.UserTag, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.userTagRepo.Update(ctx, req)
}

func (s *UserTagService) Delete(ctx context.Context, req *ubaV1.DeleteUserTagRequest) (*emptypb.Empty, error) {
	if err := s.userTagRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type UserBehaviorProfileService struct {
	ubaV1.UnimplementedUserBehaviorProfileServiceServer

	log *log.Helper
}

func NewUserBehaviorProfileService(
	ctx *bootstrap.Context,
) *UserBehaviorProfileService {
	svc := &UserBehaviorProfileService{
		log: ctx.NewLoggerHelper("user-behavior-profile/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *UserBehaviorProfileService) init() {
}

func (s *UserBehaviorProfileService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserBehaviorProfileResponse, error) {
	return nil, nil
}

func (s *UserBehaviorProfileService) Get(ctx context.Context, req *ubaV1.GetUserBehaviorProfileRequest) (*ubaV1.UserBehaviorProfile, error) {
	return nil, nil
}

func (s *UserBehaviorProfileService) Create(ctx context.Context, req *ubaV1.CreateUserBehaviorProfileRequest) (*ubaV1.UserBehaviorProfile, error) {
	return nil, nil
}

package service

import (
	"context"
	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type UserBehaviorProfileService struct {
	adminV1.UserBehaviorProfileServiceHTTPServer

	log *log.Helper

	userBehaviorProfileServiceClient ubaV1.UserBehaviorProfileServiceClient
}

func NewUserBehaviorProfileService(
	ctx *bootstrap.Context,
	userBehaviorProfileServiceClient ubaV1.UserBehaviorProfileServiceClient,
) *UserBehaviorProfileService {
	svc := &UserBehaviorProfileService{
		log:                              ctx.NewLoggerHelper("user-behavior-profile/service/admin-service"),
		userBehaviorProfileServiceClient: userBehaviorProfileServiceClient,
	}

	svc.init()

	return svc
}

func (s *UserBehaviorProfileService) init() {
}

func (s *UserBehaviorProfileService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserBehaviorProfileResponse, error) {
	return s.userBehaviorProfileServiceClient.List(ctx, req)
}

func (s *UserBehaviorProfileService) Get(ctx context.Context, req *ubaV1.GetUserBehaviorProfileRequest) (*ubaV1.UserBehaviorProfile, error) {
	return s.userBehaviorProfileServiceClient.Get(ctx, req)
}

func (s *UserBehaviorProfileService) Create(ctx context.Context, req *ubaV1.CreateUserBehaviorProfileRequest) (*ubaV1.UserBehaviorProfile, error) {
	return s.userBehaviorProfileServiceClient.Create(ctx, req)
}

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

type UserBehaviorProfileService struct {
	ubaV1.UnimplementedUserBehaviorProfileServiceServer

	log *log.Helper

	userDorisRepo *doris.UsersDimRepo
	userCkRepo    *clickhouse.UsersDimRepo
}

func NewUserBehaviorProfileService(
	ctx *bootstrap.Context,
	userDorisRepo *doris.UsersDimRepo,
	userCkRepo *clickhouse.UsersDimRepo,
) *UserBehaviorProfileService {
	svc := &UserBehaviorProfileService{
		log:           ctx.NewLoggerHelper("user-behavior-profile/service/core-service"),
		userDorisRepo: userDorisRepo,
		userCkRepo:    userCkRepo,
	}

	svc.init()

	return svc
}

func (s *UserBehaviorProfileService) init() {
}

func (s *UserBehaviorProfileService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserBehaviorProfileResponse, error) {
	if data.UseClickHouse {
		return s.userCkRepo.List(ctx, req)
	} else {
		return s.userDorisRepo.List(ctx, req)
	}
}

func (s *UserBehaviorProfileService) Get(ctx context.Context, req *ubaV1.GetUserBehaviorProfileRequest) (*ubaV1.UserBehaviorProfile, error) {
	return nil, nil
}

func (s *UserBehaviorProfileService) Create(ctx context.Context, req *ubaV1.UserBehaviorProfile) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.userCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.userDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *UserBehaviorProfileService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateUserBehaviorProfileRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.userCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.userDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

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

type SessionService struct {
	ubaV1.UnimplementedSessionServiceServer

	log *log.Helper

	sessionDorisRepo *doris.SessionsFactRepo
	sessionCkRepo    *clickhouse.SessionsFactRepo
}

func NewSessionService(
	ctx *bootstrap.Context,
	sessionDorisRepo *doris.SessionsFactRepo,
	sessionCkRepo *clickhouse.SessionsFactRepo,
) *SessionService {
	svc := &SessionService{
		log:              ctx.NewLoggerHelper("session/service/core-service"),
		sessionDorisRepo: sessionDorisRepo,
		sessionCkRepo:    sessionCkRepo,
	}

	svc.init()

	return svc
}

func (s *SessionService) init() {
}

func (s *SessionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListSessionResponse, error) {
	if data.UseClickHouse {
		return s.sessionCkRepo.List(ctx, req)
	} else {
		return s.sessionDorisRepo.List(ctx, req)
	}
}

func (s *SessionService) Get(ctx context.Context, req *ubaV1.GetSessionRequest) (*ubaV1.Session, error) {
	return nil, nil
}

func (s *SessionService) Create(ctx context.Context, req *ubaV1.Session) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.sessionCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.sessionDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *SessionService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateSessionRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.sessionCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.sessionDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

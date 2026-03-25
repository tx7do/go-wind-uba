package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type SessionService struct {
	ubaV1.UnimplementedSessionServiceServer

	log *log.Helper
}

func NewSessionService(
	ctx *bootstrap.Context,
) *SessionService {
	svc := &SessionService{
		log: ctx.NewLoggerHelper("session/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *SessionService) init() {
}

func (s *SessionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListSessionResponse, error) {
	return nil, nil
}

func (s *SessionService) Get(ctx context.Context, req *ubaV1.GetSessionRequest) (*ubaV1.Session, error) {
	return nil, nil
}

func (s *SessionService) Create(ctx context.Context, req *ubaV1.CreateSessionRequest) (*ubaV1.Session, error) {
	return nil, nil
}

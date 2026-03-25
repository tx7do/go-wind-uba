package service

import (
	"context"
	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type SessionService struct {
	adminV1.SessionServiceHTTPServer

	log                  *log.Helper
	sessionServiceClient ubaV1.SessionServiceClient
}

func NewSessionService(
	ctx *bootstrap.Context,
	sessionServiceClient ubaV1.SessionServiceClient,
) *SessionService {
	svc := &SessionService{
		log:                  ctx.NewLoggerHelper("session/service/admin-service"),
		sessionServiceClient: sessionServiceClient,
	}

	svc.init()

	return svc
}

func (s *SessionService) init() {
}

func (s *SessionService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListSessionResponse, error) {
	return s.sessionServiceClient.List(ctx, req)
}

func (s *SessionService) Get(ctx context.Context, req *ubaV1.GetSessionRequest) (*ubaV1.Session, error) {
	return s.sessionServiceClient.Get(ctx, req)
}

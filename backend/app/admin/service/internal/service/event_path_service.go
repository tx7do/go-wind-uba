package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type EventPathService struct {
	adminV1.EventPathServiceHTTPServer

	log *log.Helper

	eventPathServiceClient ubaV1.EventPathServiceClient
}

func NewEventPathService(
	ctx *bootstrap.Context,
	eventPathServiceClient ubaV1.EventPathServiceClient,
) *EventPathService {
	svc := &EventPathService{
		log:                    ctx.NewLoggerHelper("event-path/service/admin-service"),
		eventPathServiceClient: eventPathServiceClient,
	}

	svc.init()

	return svc
}

func (s *EventPathService) init() {
}

func (s *EventPathService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventPathResponse, error) {
	return s.eventPathServiceClient.List(ctx, req)
}

func (s *EventPathService) Get(ctx context.Context, req *ubaV1.GetEventPathRequest) (*ubaV1.EventPath, error) {
	return s.eventPathServiceClient.Get(ctx, req)
}

func (s *EventPathService) Create(ctx context.Context, req *ubaV1.CreateEventPathRequest) (*emptypb.Empty, error) {
	return s.eventPathServiceClient.Create(ctx, req.GetData())
}

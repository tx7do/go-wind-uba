package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type EventPathService struct {
	ubaV1.UnimplementedEventPathServiceServer

	log *log.Helper
}

func NewEventPathService(
	ctx *bootstrap.Context,
) *EventPathService {
	svc := &EventPathService{
		log: ctx.NewLoggerHelper("event-path/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *EventPathService) init() {
}

func (s *EventPathService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventPathResponse, error) {
	return nil, nil
}

func (s *EventPathService) Get(ctx context.Context, req *ubaV1.GetEventPathRequest) (*ubaV1.EventPath, error) {
	return nil, nil
}

func (s *EventPathService) Create(ctx context.Context, req *ubaV1.CreateEventPathRequest) (*ubaV1.EventPath, error) {
	return nil, nil
}

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

type EventSchemaService struct {
	adminV1.EventSchemaServiceHTTPServer

	log    *log.Helper
	client ubaV1.EventSchemaServiceClient
}

func NewEventSchemaService(
	ctx *bootstrap.Context,
	client ubaV1.EventSchemaServiceClient,
) *EventSchemaService {
	return &EventSchemaService{
		log:    ctx.NewLoggerHelper("event-schema/service/admin-service"),
		client: client,
	}
}

func (s *EventSchemaService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventSchemaResponse, error) {
	return s.client.List(ctx, req)
}

func (s *EventSchemaService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountEventSchemaResponse, error) {
	return s.client.Count(ctx, req)
}

func (s *EventSchemaService) Get(ctx context.Context, req *ubaV1.GetEventSchemaRequest) (*ubaV1.EventSchema, error) {
	return s.client.Get(ctx, req)
}

func (s *EventSchemaService) Create(ctx context.Context, req *ubaV1.CreateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	return s.client.Create(ctx, req)
}

func (s *EventSchemaService) Update(ctx context.Context, req *ubaV1.UpdateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	return s.client.Update(ctx, req)
}

func (s *EventSchemaService) Delete(ctx context.Context, req *ubaV1.DeleteEventSchemaRequest) (*emptypb.Empty, error) {
	return s.client.Delete(ctx, req)
}

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

type EventSchemaService struct {
	ubaV1.UnimplementedEventSchemaServiceServer

	log *log.Helper

	eventSchemaRepo *data.EventSchemaRepo
}

func NewEventSchemaService(
	ctx *bootstrap.Context,
	eventSchemaRepo *data.EventSchemaRepo,
) *EventSchemaService {
	return &EventSchemaService{
		log:             ctx.NewLoggerHelper("event-schema/service/core-service"),
		eventSchemaRepo: eventSchemaRepo,
	}
}

func (s *EventSchemaService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventSchemaResponse, error) {
	return s.eventSchemaRepo.List(ctx, req)
}

func (s *EventSchemaService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountEventSchemaResponse, error) {
	return s.eventSchemaRepo.Count(ctx, req)
}

func (s *EventSchemaService) Get(ctx context.Context, req *ubaV1.GetEventSchemaRequest) (*ubaV1.EventSchema, error) {
	return s.eventSchemaRepo.Get(ctx, req)
}

func (s *EventSchemaService) Create(ctx context.Context, req *ubaV1.CreateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	return s.eventSchemaRepo.Create(ctx, req)
}

func (s *EventSchemaService) Update(ctx context.Context, req *ubaV1.UpdateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	return s.eventSchemaRepo.Update(ctx, req)
}

func (s *EventSchemaService) Delete(ctx context.Context, req *ubaV1.DeleteEventSchemaRequest) (*emptypb.Empty, error) {
	if err := s.eventSchemaRepo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type BehaviorEventService struct {
	ubaV1.UnimplementedBehaviorEventServiceServer

	log *log.Helper
}

func NewBehaviorEventService(
	ctx *bootstrap.Context,
) *BehaviorEventService {
	svc := &BehaviorEventService{
		log: ctx.NewLoggerHelper("behavior-event/service/core-service"),
	}

	svc.init()

	return svc
}

func (s *BehaviorEventService) init() {
}

func (s *BehaviorEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListBehaviorEventResponse, error) {
	return nil, nil
}

func (s *BehaviorEventService) Get(ctx context.Context, req *ubaV1.GetBehaviorEventRequest) (*ubaV1.BehaviorEvent, error) {
	return nil, nil
}

func (s *BehaviorEventService) Create(ctx context.Context, req *ubaV1.BehaviorEvent) (*ubaV1.BehaviorEvent, error) {
	return nil, nil
}

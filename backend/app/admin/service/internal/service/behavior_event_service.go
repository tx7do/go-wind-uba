package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// BehaviorEventService 实现 admin.service.v1.BehaviorEventServiceHTTPServer，
// 作为 HTTP 网关转发至 core 层 gRPC BehaviorEventService，
// 用于"用户行为时间轴"按 user_id / session_id 分页查询原始行为事件明细。
type BehaviorEventService struct {
	client ubaV1.BehaviorEventServiceClient
	log    *log.Helper
}

func NewBehaviorEventService(
	ctx *bootstrap.Context,
	client ubaV1.BehaviorEventServiceClient,
) *BehaviorEventService {
	return &BehaviorEventService{
		log:    ctx.NewLoggerHelper("behavior-event/service/admin-service"),
		client: client,
	}
}

func (s *BehaviorEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListBehaviorEventResponse, error) {
	return s.client.List(ctx, req)
}

func (s *BehaviorEventService) Get(ctx context.Context, req *ubaV1.GetBehaviorEventRequest) (*ubaV1.BehaviorEvent, error) {
	return s.client.Get(ctx, req)
}

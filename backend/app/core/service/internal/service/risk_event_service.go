package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse"
	"go-wind-uba/app/core/service/internal/data/doris"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventService struct {
	ubaV1.UnimplementedRiskEventServiceServer

	log *log.Helper

	riskEventDorisRepo *doris.RiskEventsRepo
	riskEventCkRepo    *clickhouse.RiskEventsRepo
}

func NewRiskEventService(
	ctx *bootstrap.Context,
	riskEventDorisRepo *doris.RiskEventsRepo,
	riskEventCkRepo *clickhouse.RiskEventsRepo,
) *RiskEventService {
	svc := &RiskEventService{
		log:                ctx.NewLoggerHelper("risk-event/service/core-service"),
		riskEventDorisRepo: riskEventDorisRepo,
		riskEventCkRepo:    riskEventCkRepo,
	}

	svc.init()

	return svc
}

func (s *RiskEventService) init() {
}

func (s *RiskEventService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskEventResponse, error) {
	if data.UseClickHouse {
		return s.riskEventCkRepo.List(ctx, req)
	} else {
		return s.riskEventDorisRepo.List(ctx, req)
	}
}

func (s *RiskEventService) Get(ctx context.Context, req *ubaV1.GetRiskEventRequest) (*ubaV1.RiskEvent, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	// 风险事件主键是 risk_event_id（Snowflake 字符串），proto 暂用 uint64 id 承载，
	// 这里按字符串透传给 repo 精确匹配。
	// TODO(api): GetRiskEventRequest.id 现为 uint64，无法承载字符串 risk_event_id。
	//   建议把 risk_event.proto 中 oneof 的 id 改为 string 并重新生成（admin+core），
	//   届时 HTTP 路由 /admin/v1/risk-events/{id} 才能正确绑定字符串 ID。
	riskEventID := strconv.FormatUint(req.GetId(), 10)

	var dto *ubaV1.RiskEvent
	var err error
	if data.UseClickHouse {
		dto, err = s.riskEventCkRepo.Get(ctx, riskEventID)
	} else {
		dto, err = s.riskEventDorisRepo.Get(ctx, riskEventID)
	}
	if err != nil {
		return nil, err
	}
	if dto == nil {
		return nil, ubaV1.ErrorNotFound("risk event %s not found", riskEventID)
	}
	return dto, nil
}

func (s *RiskEventService) Create(ctx context.Context, req *ubaV1.RiskEvent) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.riskEventCkRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	} else {
		if err := s.riskEventDorisRepo.Create(ctx, req); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *RiskEventService) BatchCreate(ctx context.Context, req *ubaV1.BatchCreateRiskEventRequest) (*emptypb.Empty, error) {
	if data.UseClickHouse {
		if err := s.riskEventCkRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	} else {
		if err := s.riskEventDorisRepo.BatchCreate(ctx, req.GetItems()); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

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

type RiskRuleService struct {
	ubaV1.UnimplementedRiskRuleServiceServer

	log *log.Helper

	riskRuleRepo *data.RiskRuleRepo
}

func NewRiskRuleService(
	ctx *bootstrap.Context,
	riskRuleRepo *data.RiskRuleRepo,
) *RiskRuleService {
	svc := &RiskRuleService{
		log:          ctx.NewLoggerHelper("risk-rule/service/core-service"),
		riskRuleRepo: riskRuleRepo,
	}

	svc.init()

	return svc
}

func (s *RiskRuleService) init() {
}

func (s *RiskRuleService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskRuleResponse, error) {
	resp, err := s.riskRuleRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *RiskRuleService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountRiskRuleResponse, error) {
	return s.riskRuleRepo.Count(ctx, req)
}

func (s *RiskRuleService) Get(ctx context.Context, req *ubaV1.GetRiskRuleRequest) (*ubaV1.RiskRule, error) {
	resp, err := s.riskRuleRepo.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *RiskRuleService) Create(ctx context.Context, req *ubaV1.CreateRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.riskRuleRepo.Create(ctx, req)
}

func (s *RiskRuleService) Update(ctx context.Context, req *ubaV1.UpdateRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.riskRuleRepo.Update(ctx, req)
}

func (s *RiskRuleService) Delete(ctx context.Context, req *ubaV1.DeleteRiskRuleRequest) (*emptypb.Empty, error) {
	if err := s.riskRuleRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

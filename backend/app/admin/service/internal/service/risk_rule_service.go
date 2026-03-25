package service

import (
	"context"
	adminV1 "go-wind-uba/api/gen/go/admin/service/v1"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskRuleService struct {
	adminV1.RiskRuleServiceHTTPServer

	log *log.Helper

	ruleServiceClient ubaV1.RiskRuleServiceClient
}

func NewRiskRuleService(
	ctx *bootstrap.Context,
	ruleServiceClient ubaV1.RiskRuleServiceClient,
) *RiskRuleService {
	svc := &RiskRuleService{
		log:               ctx.NewLoggerHelper("risk-rule/service/admin-service"),
		ruleServiceClient: ruleServiceClient,
	}

	svc.init()

	return svc
}

func (s *RiskRuleService) init() {
}

func (s *RiskRuleService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskRuleResponse, error) {
	resp, err := s.ruleServiceClient.List(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *RiskRuleService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountRiskRuleResponse, error) {
	return s.ruleServiceClient.Count(ctx, req)
}

func (s *RiskRuleService) Get(ctx context.Context, req *ubaV1.GetRiskRuleRequest) (*ubaV1.RiskRule, error) {
	resp, err := s.ruleServiceClient.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *RiskRuleService) Create(ctx context.Context, req *ubaV1.CreateRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.ruleServiceClient.Create(ctx, req)
}

func (s *RiskRuleService) Update(ctx context.Context, req *ubaV1.UpdateRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	return s.ruleServiceClient.Update(ctx, req)
}

func (s *RiskRuleService) Delete(ctx context.Context, req *ubaV1.DeleteRiskRuleRequest) (*emptypb.Empty, error) {
	return s.ruleServiceClient.Delete(ctx, req)
}

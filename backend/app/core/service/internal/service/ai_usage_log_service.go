package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
	"go-wind-uba/app/core/service/internal/data"
)

// AiUsageLogService 用量流水查询与租户配额管理（配额记账事实源的只读视图）。
// 用量口径：租户用户按本租户，平台管理员（tenantId=0）见全量。
// 配额语义：仅平台管理员显式设置的租户受限（fail-open），平台自用恒不限量。
type AiUsageLogService struct {
	aiV1.UnimplementedAiUsageLogServiceServer
	log       *log.Helper
	repo      *data.AiUsageLogRepo
	quotaRepo *data.AiTenantQuotaRepo
}

func NewAiUsageLogService(
	ctx *bootstrap.Context,
	repo *data.AiUsageLogRepo,
	quotaRepo *data.AiTenantQuotaRepo,
) *AiUsageLogService {
	return &AiUsageLogService{
		log:       ctx.NewLoggerHelper("ai_usage_log/service/core-service"),
		repo:      repo,
		quotaRepo: quotaRepo,
	}
}

func (s *AiUsageLogService) List(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.ListAiUsageLogResponse, error) {
	tenantId, _ := s.operatorTenant(ctx)
	return s.repo.List(ctx, req, tenantId)
}

func (s *AiUsageLogService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*aiV1.CountAiUsageLogResponse, error) {
	tenantId, _ := s.operatorTenant(ctx)
	return s.repo.Count(ctx, req, tenantId)
}

// GetUsageSummary 当月用量汇总：tokens/调用次数 + 本租户配额（未配置不限量）。
func (s *AiUsageLogService) GetUsageSummary(ctx context.Context, _ *emptypb.Empty) (*aiV1.UsageSummaryResponse, error) {
	tenantId, _ := s.operatorTenant(ctx)
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	tokens, calls, err := s.repo.MonthStats(ctx, tenantId, monthStart)
	if err != nil {
		return nil, err
	}

	resp := &aiV1.UsageSummaryResponse{
		MonthTokens: uint32(tokens),
		MonthCalls:  uint32(calls),
	}
	if s.quotaRepo != nil {
		if limit, configured, qErr := s.quotaRepo.MonthlyLimit(ctx, tenantId); qErr == nil && configured {
			resp.QuotaConfigured = true
			resp.QuotaLimit = limit
		}
	}
	return resp, nil
}

// GetTenantQuota 查询租户配额。租户用户只能看自己（入参 tenant_id 被覆盖）；
// 平台管理员可查任意租户。
func (s *AiUsageLogService) GetTenantQuota(ctx context.Context, req *aiV1.GetTenantQuotaRequest) (*aiV1.TenantQuotaResponse, error) {
	operatorTenant, isPlatform := s.operatorTenant(ctx)
	tenantId := req.GetTenantId()
	if !isPlatform || operatorTenant != 0 {
		// 租户用户强制看本租户
		tenantId = operatorTenant
	}
	if tenantId == 0 {
		return nil, aiV1.ErrorBadRequest("tenant_id is required")
	}
	if s.quotaRepo == nil {
		return &aiV1.TenantQuotaResponse{TenantId: tenantId}, nil
	}

	limit, configured, err := s.quotaRepo.MonthlyLimit(ctx, tenantId)
	if err != nil {
		return nil, aiV1.ErrorInternalServerError("query tenant quota failed")
	}
	return &aiV1.TenantQuotaResponse{
		TenantId:          tenantId,
		Configured:        configured,
		MonthlyTokenLimit: limit,
	}, nil
}

// SetTenantQuota 设置租户配额（平台管理员专属；limit=0 取消配额）。
func (s *AiUsageLogService) SetTenantQuota(ctx context.Context, req *aiV1.SetTenantQuotaRequest) (*emptypb.Empty, error) {
	if s.quotaRepo == nil {
		return nil, aiV1.ErrorInternalServerError("quota storage not available")
	}

	operatorTenant, _ := s.operatorTenant(ctx)
	if operatorTenant != 0 {
		return nil, aiV1.ErrorForbidden("only platform administrators can manage tenant quotas")
	}
	if req.GetTenantId() == 0 {
		return nil, aiV1.ErrorBadRequest("tenant_id is required")
	}

	if err := s.quotaRepo.Upsert(ctx, req.GetTenantId(), req.GetMonthlyTokenLimit()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// operatorTenant 取调用者租户（viewer 上下文；平台用户 tenantId=0 见全量）。
func (s *AiUsageLogService) operatorTenant(ctx context.Context) (int64, bool) {
	vc, exist := viewer.FromContext(ctx)
	if !exist || vc == nil {
		return 0, false
	}
	return int64(vc.TenantID()), true
}

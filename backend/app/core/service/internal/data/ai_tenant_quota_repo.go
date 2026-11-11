package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/aitenantquota"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
)

// AiTenantQuotaRepo AI 租户月度 token 配额仓储。
// 语义 fail-open：未建行的租户不限量；平台自用（tenant_id=0）恒不限量。
type AiTenantQuotaRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewAiTenantQuotaRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AiTenantQuotaRepo {
	return &AiTenantQuotaRepo{
		log:       ctx.NewLoggerHelper("ai_tenant_quota/repo/core-service"),
		entClient: entClient,
	}
}

// MonthlyLimit 返回租户的月度 token 上限；未配置返回 (0,false)。
// 显式按 tenant_id 查询（不走租户隐私层）：配额是平台管理员设置的跨租户治理数据。
func (r *AiTenantQuotaRepo) MonthlyLimit(ctx context.Context, tenantId int64) (uint64, bool, error) {
	if tenantId == 0 {
		return 0, false, nil
	}
	row, err := r.entClient.Client().AiTenantQuota.Query().
		Where(aitenantquota.TenantIDEQ(tenantId)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, false, nil
		}
		r.log.Errorf("query ai tenant quota failed: %s", err.Error())
		return 0, false, err
	}
	return row.MonthlyTokenLimit, true, nil
}

// Upsert 设置租户配额（limit=0 表示取消——删除配置行，回到不限量）。
func (r *AiTenantQuotaRepo) Upsert(ctx context.Context, tenantId int64, monthlyTokenLimit uint64) error {
	if tenantId == 0 {
		return aiV1.ErrorBadRequest("tenant_id is required")
	}
	if monthlyTokenLimit == 0 {
		if _, err := r.entClient.Client().AiTenantQuota.Delete().
			Where(aitenantquota.TenantIDEQ(tenantId)).
			Exec(ctx); err != nil {
			r.log.Errorf("delete ai tenant quota failed: %s", err.Error())
			return aiV1.ErrorInternalServerError("delete ai tenant quota failed")
		}
		return nil
	}

	exist, err := r.entClient.Client().AiTenantQuota.Query().
		Where(aitenantquota.TenantIDEQ(tenantId)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query ai tenant quota exist failed: %s", err.Error())
		return aiV1.ErrorInternalServerError("query ai tenant quota failed")
	}
	if exist {
		if err = r.entClient.Client().AiTenantQuota.Update().
			Where(aitenantquota.TenantIDEQ(tenantId)).
			SetMonthlyTokenLimit(uint64(monthlyTokenLimit)).
			SetUpdatedAt(time.Now()).
			Exec(ctx); err != nil {
			r.log.Errorf("update ai tenant quota failed: %s", err.Error())
			return aiV1.ErrorInternalServerError("update ai tenant quota failed")
		}
		return nil
	}
	if _, err = r.entClient.Client().AiTenantQuota.Create().
		SetTenantID(tenantId).
		SetMonthlyTokenLimit(uint64(monthlyTokenLimit)).
		SetCreatedAt(time.Now()).
		Save(ctx); err != nil {
		r.log.Errorf("insert ai tenant quota failed: %s", err.Error())
		return aiV1.ErrorInternalServerError("insert ai tenant quota failed")
	}
	return nil
}

// EnsureWithinQuota 校验租户本月 token 用量是否超出配额；超限返回 429。
// 未配置配额 / 平台自用（tenantId=0）/ 配额或用量查询失败 → 放行
// （配额是治理手段，不应因自身故障阻断正常调用）。
func (r *AiTenantQuotaRepo) EnsureWithinQuota(ctx context.Context, usageRepo *AiUsageLogRepo, tenantId int64) error {
	if r == nil || tenantId == 0 {
		return nil
	}
	limit, configured, err := r.MonthlyLimit(ctx, tenantId)
	if err != nil || !configured || limit == 0 {
		return nil
	}

	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
	used, _, err := usageRepo.MonthStats(ctx, tenantId, monthStart)
	if err != nil {
		r.log.Warnf("quota check: month stats failed, allow: tenant=%d: %v", tenantId, err)
		return nil
	}
	if used >= int64(limit) {
		return aiV1.ErrorTooManyRequests("AI 月度 token 配额已用尽（%d/%d），请联系管理员调整", used, limit)
	}
	return nil
}

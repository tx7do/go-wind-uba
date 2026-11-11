package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/webhookdeliverylog"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// WebhookDeliveryLogRepo Webhook投递日志数据仓库
type WebhookDeliveryLogRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.WebhookDeliveryLog, ent.WebhookDeliveryLog]
}

func NewWebhookDeliveryLogRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *WebhookDeliveryLogRepo {
	repo := &WebhookDeliveryLogRepo{
		log:       ctx.NewLoggerHelper("webhookdeliverylog/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.WebhookDeliveryLog, ent.WebhookDeliveryLog](),
	}

	repo.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	repo.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	return repo
}

// Create 记录一次投递结果。
func (r *WebhookDeliveryLogRepo) Create(ctx context.Context, dto *ubaV1.WebhookDeliveryLog) error {
	builder := r.entClient.Client().WebhookDeliveryLog.Create().
		SetWebhookID(dto.GetWebhookId()).
		SetNillableTenantID(&dto.TenantId).
		SetEventType(dto.GetEventType()).
		SetPayload(dto.GetPayload()).
		SetStatusCode(dto.GetStatusCode()).
		SetResponseBody(dto.GetResponseBody()).
		SetRetryCount(dto.GetRetryCount()).
		SetSuccess(dto.GetSuccess()).
		SetErrorMessage(dto.GetErrorMessage()).
		SetSentAt(time.Now())

	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf("insert webhook delivery log failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("insert webhook delivery log failed")
	}
	return nil
}

// ListByWebhook 按 Webhook 配置查询最近的投递日志（最新在前）。
func (r *WebhookDeliveryLogRepo) ListByWebhook(ctx context.Context, webhookID int64, success *bool, limit int) ([]*ubaV1.WebhookDeliveryLog, int64, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	query := r.entClient.Client().WebhookDeliveryLog.Query().
		Where(webhookdeliverylog.WebhookIDEQ(webhookID))

	if success != nil {
		query = query.Where(webhookdeliverylog.SuccessEQ(*success))
	}

	total, err := query.Count(ctx)
	if err != nil {
		r.log.Errorf("query webhook delivery log count failed: %s", err.Error())
		return nil, 0, ubaV1.ErrorInternalServerError("query webhook delivery log count failed")
	}

	entities, err := query.
		Order(ent.Desc(webhookdeliverylog.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		r.log.Errorf("query webhook delivery logs failed: %s", err.Error())
		return nil, 0, ubaV1.ErrorInternalServerError("query webhook delivery logs failed")
	}

	items := make([]*ubaV1.WebhookDeliveryLog, 0, len(entities))
	for _, entity := range entities {
		items = append(items, r.mapper.ToDTO(entity))
	}

	return items, int64(total), nil
}

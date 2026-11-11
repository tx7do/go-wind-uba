package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/webhook"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// WebhookRepo Webhook配置数据仓库
type WebhookRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.Webhook, ent.Webhook]

	repository *entCrud.Repository[
		ent.WebhookQuery, ent.WebhookSelect,
		ent.WebhookCreate, ent.WebhookCreateBulk,
		ent.WebhookUpdate, ent.WebhookUpdateOne,
		ent.WebhookDelete,
		predicate.Webhook,
		ubaV1.Webhook, ent.Webhook,
	]
}

func NewWebhookRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *WebhookRepo {
	repo := &WebhookRepo{
		log:       ctx.NewLoggerHelper("webhook/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.Webhook, ent.Webhook](),
	}

	repo.init()

	return repo
}

func (r *WebhookRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.WebhookQuery, ent.WebhookSelect,
		ent.WebhookCreate, ent.WebhookCreateBulk,
		ent.WebhookUpdate, ent.WebhookUpdateOne,
		ent.WebhookDelete,
		predicate.Webhook,
		ubaV1.Webhook, ent.Webhook,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *WebhookRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountWebhookResponse, error) {
	builder := r.entClient.Client().Webhook.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query webhook count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query webhook count failed")
	}

	return &ubaV1.CountWebhookResponse{
		Count: int64(count),
	}, nil
}

func (r *WebhookRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListWebhookResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Webhook.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListWebhookResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListWebhookResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *WebhookRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().Webhook.Query().
		Where(webhook.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// ListEnabledByTenantApp 列出租户下启用的 Webhook；appID 为空表示不限应用。
// 应用匹配规则：webhook 未绑定应用，或绑定的应用与事件一致。
func (r *WebhookRepo) ListEnabledByTenantApp(ctx context.Context, tenantID int64, appID string) ([]*ent.Webhook, error) {
	entities, err := r.entClient.Client().Webhook.Query().
		Where(
			webhook.EnabledEQ(true),
			webhook.TenantIDEQ(tenantID),
		).
		All(ctx)
	if err != nil {
		r.log.Errorf("list enabled webhooks failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list enabled webhooks failed")
	}

	if appID == "" {
		return entities, nil
	}

	matched := make([]*ent.Webhook, 0, len(entities))
	for _, entity := range entities {
		if entity.AppID == nil || *entity.AppID == "" || *entity.AppID == appID {
			matched = append(matched, entity)
		}
	}
	return matched, nil
}

// UpdateDeliveryState 投递完成后回写状态：成功更新最后触发时间并清零失败计数，失败累加计数。
func (r *WebhookRepo) UpdateDeliveryState(ctx context.Context, id int64, triggeredAt *time.Time, failureCount *int64) error {
	builder := r.entClient.Client().Webhook.UpdateOneID(id).SetUpdatedAt(time.Now())
	if triggeredAt != nil {
		builder.SetLastTriggeredAt(*triggeredAt)
		builder.SetFailureCount(0)
	}
	if failureCount != nil {
		builder.SetFailureCount(uint32(*failureCount))
	}

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("update webhook delivery state failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update webhook delivery state failed")
	}
	return nil
}

func (r *WebhookRepo) Get(ctx context.Context, req *ubaV1.GetWebhookRequest) (*ubaV1.Webhook, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Webhook.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetWebhookRequest_Id:
		whereCond = append(whereCond, webhook.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *WebhookRepo) Create(ctx context.Context, req *ubaV1.CreateWebhookRequest) (*ubaV1.Webhook, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newWebhookCreate(req.Data)

	var entity *ent.Webhook
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert webhook failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert webhook failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *WebhookRepo) newWebhookCreate(w *ubaV1.Webhook) *ent.WebhookCreate {
	builder := r.entClient.Client().Webhook.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableURL(w.Url).
		SetNillableSecret(w.Secret).
		SetNillableEnabled(w.Enabled).
		SetNillableLastTriggeredAt(timeutil.TimestamppbToTime(w.LastTriggeredAt)).
		SetNillableFailureCount(w.FailureCount).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	if w.EventTypes != nil {
		builder.SetEventTypes(w.GetEventTypes())
	}
	//if w.Filter != nil {
	//	builder.SetFilter(w.GetFilter())
	//}

	return builder
}

func (r *WebhookRepo) BatchCreate(ctx context.Context, webhooks []*ubaV1.Webhook) error {
	if len(webhooks) == 0 {
		return nil
	}

	bulk := make([]*ent.WebhookCreate, 0, len(webhooks))
	for _, dto := range webhooks {
		builder := r.newWebhookCreate(dto)
		bulk = append(bulk, builder)
	}

	bulkBuilder := r.entClient.Client().Webhook.CreateBulk(bulk...)

	if err := bulkBuilder.Exec(ctx); err != nil {
		r.log.Errorf("batch insert webhooks failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("batch insert webhooks failed")
	}

	return nil
}

func (r *WebhookRepo) Update(ctx context.Context, req *ubaV1.UpdateWebhookRequest) (*ubaV1.Webhook, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateWebhookRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Webhook.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.Webhook) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableURL(req.Data.Url).
				SetNillableSecret(req.Data.Secret).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableLastTriggeredAt(timeutil.TimestamppbToTime(req.Data.LastTriggeredAt)).
				SetNillableFailureCount(req.Data.FailureCount).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.EventTypes != nil {
				builder.SetEventTypes(req.Data.GetEventTypes())
			}
			//if req.Data.Filter != nil {
			//	builder.SetFilter(req.Data.GetFilter())
			//}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(webhook.FieldID, req.GetId()))
		},
	)

	return dto, err
}

func (r *WebhookRepo) Delete(ctx context.Context, req *ubaV1.DeleteWebhookRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Webhook.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(webhook.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete webhook failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete webhook failed")
	}

	return nil
}

func (r *WebhookRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Webhook.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate webhooks table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

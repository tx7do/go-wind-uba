package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/visualevent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// VisualEventRepo 可视化埋点规则数据仓库
type VisualEventRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	matchTypeConverter *mapper.EnumTypeConverter[ubaV1.VisualMatchType, string]

	mapper *mapper.CopierMapper[ubaV1.VisualEvent, ent.VisualEvent]

	repository *entCrud.Repository[
		ent.VisualEventQuery, ent.VisualEventSelect,
		ent.VisualEventCreate, ent.VisualEventCreateBulk,
		ent.VisualEventUpdate, ent.VisualEventUpdateOne,
		ent.VisualEventDelete,
		predicate.VisualEvent,
		ubaV1.VisualEvent, ent.VisualEvent,
	]
}

func NewVisualEventRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *VisualEventRepo {
	repo := &VisualEventRepo{
		log:                ctx.NewLoggerHelper("visual-event/repo/core-service"),
		entClient:          entClient,
		mapper:             mapper.NewCopierMapper[ubaV1.VisualEvent, ent.VisualEvent](),
		matchTypeConverter: mapper.NewEnumTypeConverter[ubaV1.VisualMatchType, string](ubaV1.VisualMatchType_name, ubaV1.VisualMatchType_value),
	}

	repo.init()

	return repo
}

func (r *VisualEventRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.VisualEventQuery, ent.VisualEventSelect,
		ent.VisualEventCreate, ent.VisualEventCreateBulk,
		ent.VisualEventUpdate, ent.VisualEventUpdateOne,
		ent.VisualEventDelete,
		predicate.VisualEvent,
		ubaV1.VisualEvent, ent.VisualEvent,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.matchTypeConverter.NewConverterPair())
}

func (r *VisualEventRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountVisualEventResponse, error) {
	builder := r.entClient.Client().VisualEvent.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query visual event count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query visual event count failed")
	}

	return &ubaV1.CountVisualEventResponse{
		Count: int64(count),
	}, nil
}

func (r *VisualEventRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListVisualEventResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().VisualEvent.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListVisualEventResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListVisualEventResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabledByTenantApp 列出租户下启用的规则（SDK 下发用）；appID 空/0 不过滤。
func (r *VisualEventRepo) ListEnabledByTenantApp(ctx context.Context, tenantID int64, appID string) ([]*ent.VisualEvent, error) {
	entities, err := r.entClient.Client().VisualEvent.Query().
		Where(
			visualevent.EnabledEQ(true),
			visualevent.TenantIDEQ(tenantID),
		).
		All(ctx)
	if err != nil {
		r.log.Errorf("list enabled visual events failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list enabled visual events failed")
	}

	if appID == "" {
		return entities, nil
	}

	matched := make([]*ent.VisualEvent, 0, len(entities))
	for _, entity := range entities {
		if entity.AppID == nil || *entity.AppID == "" || *entity.AppID == appID {
			matched = append(matched, entity)
		}
	}
	return matched, nil
}

func (r *VisualEventRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().VisualEvent.Query().
		Where(visualevent.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *VisualEventRepo) Get(ctx context.Context, req *ubaV1.GetVisualEventRequest) (*ubaV1.VisualEvent, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().VisualEvent.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetVisualEventRequest_Id:
		whereCond = append(whereCond, visualevent.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *VisualEventRepo) Create(ctx context.Context, req *ubaV1.CreateVisualEventRequest) (*ubaV1.VisualEvent, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().VisualEvent.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableAppID(req.Data.AppId).
		SetNillableEventName(req.Data.EventName).
		SetNillableDisplayName(req.Data.DisplayName).
		SetNillablePageURL(req.Data.PageUrl).
		SetElementXpath(req.Data.GetElementXpath()).
		SetMatchType(derefStr(r.matchTypeConverter.ToEntity(req.Data.MatchType))).
		SetProperties(map[string]string(req.Data.GetProperties())).
		SetNillableEnabled(req.Data.Enabled).
		SetNillableRemark(req.Data.Remark).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	var entity *ent.VisualEvent
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert visual event failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert visual event failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *VisualEventRepo) Update(ctx context.Context, req *ubaV1.UpdateVisualEventRequest) (*ubaV1.VisualEvent, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateVisualEventRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().VisualEvent.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.VisualEvent) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableEventName(req.Data.EventName).
				SetNillableDisplayName(req.Data.DisplayName).
				SetNillablePageURL(req.Data.PageUrl).
				SetNillableRemark(req.Data.Remark).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.ElementXpath != nil {
				builder.SetElementXpath(*req.Data.ElementXpath)
			}
			if req.Data.MatchType != nil {
				builder.SetMatchType(derefStr(r.matchTypeConverter.ToEntity(req.Data.MatchType)))
			}
			if req.Data.Properties != nil {
				builder.SetProperties(map[string]string(req.Data.GetProperties()))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(visualevent.FieldID, req.GetId()))
		},
	)

	return dto, err
}

func (r *VisualEventRepo) Delete(ctx context.Context, req *ubaV1.DeleteVisualEventRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().VisualEvent.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(visualevent.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete visual event failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete visual event failed")
	}

	return nil
}

func (r *VisualEventRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().VisualEvent.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate visual events table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

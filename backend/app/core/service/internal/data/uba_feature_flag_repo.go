package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/featureflag"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// FeatureFlagRepo 功能开关/AB实验数据仓库
type FeatureFlagRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.FeatureFlag, ent.FeatureFlag]

	repository *entCrud.Repository[
		ent.FeatureFlagQuery, ent.FeatureFlagSelect,
		ent.FeatureFlagCreate, ent.FeatureFlagCreateBulk,
		ent.FeatureFlagUpdate, ent.FeatureFlagUpdateOne,
		ent.FeatureFlagDelete,
		predicate.FeatureFlag,
		ubaV1.FeatureFlag, ent.FeatureFlag,
	]
}

func NewFeatureFlagRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *FeatureFlagRepo {
	repo := &FeatureFlagRepo{
		log:       ctx.NewLoggerHelper("feature-flag/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.FeatureFlag, ent.FeatureFlag](),
	}

	repo.init()

	return repo
}

func (r *FeatureFlagRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.FeatureFlagQuery, ent.FeatureFlagSelect,
		ent.FeatureFlagCreate, ent.FeatureFlagCreateBulk,
		ent.FeatureFlagUpdate, ent.FeatureFlagUpdateOne,
		ent.FeatureFlagDelete,
		predicate.FeatureFlag,
		ubaV1.FeatureFlag, ent.FeatureFlag,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(mapper.NewGenericTypeConverterPair(
		[]*ubaV1.FeatureFlagVariant{}, []schema.FeatureVariant{},
		variantsToEntity, variantsToDTO,
	))
}

func variantsToDTO(src []schema.FeatureVariant) []*ubaV1.FeatureFlagVariant {
	if len(src) == 0 {
		return nil
	}
	ret := make([]*ubaV1.FeatureFlagVariant, 0, len(src))
	for _, v := range src {
		name := v.Name
		ret = append(ret, &ubaV1.FeatureFlagVariant{
			Key:    v.Key,
			Name:   &name,
			Weight: v.Weight,
		})
	}
	return ret
}

func variantsToEntity(src []*ubaV1.FeatureFlagVariant) []schema.FeatureVariant {
	if len(src) == 0 {
		return nil
	}
	ret := make([]schema.FeatureVariant, 0, len(src))
	for _, v := range src {
		if v == nil || v.GetKey() == "" {
			continue
		}
		ret = append(ret, schema.FeatureVariant{
			Key:    v.GetKey(),
			Name:   v.GetName(),
			Weight: v.GetWeight(),
		})
	}
	return ret
}

// ToDTO 实体转 DTO（评估/结果组装用）。
func (r *FeatureFlagRepo) ToDTO(entity *ent.FeatureFlag) *ubaV1.FeatureFlag {
	return r.mapper.ToDTO(entity)
}

func (r *FeatureFlagRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountFeatureFlagResponse, error) {
	builder := r.entClient.Client().FeatureFlag.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query feature flag count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query feature flag count failed")
	}

	return &ubaV1.CountFeatureFlagResponse{
		Count: int64(count),
	}, nil
}

func (r *FeatureFlagRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListFeatureFlagResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().FeatureFlag.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListFeatureFlagResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListFeatureFlagResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *FeatureFlagRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().FeatureFlag.Query().
		Where(featureflag.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *FeatureFlagRepo) Get(ctx context.Context, req *ubaV1.GetFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().FeatureFlag.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetFeatureFlagRequest_Id:
		whereCond = append(whereCond, featureflag.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *FeatureFlagRepo) Create(ctx context.Context, req *ubaV1.CreateFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newFeatureFlagCreate(req.Data)

	var entity *ent.FeatureFlag
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert feature flag failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert feature flag failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *FeatureFlagRepo) newFeatureFlagCreate(w *ubaV1.FeatureFlag) *ent.FeatureFlagCreate {
	builder := r.entClient.Client().FeatureFlag.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableKey(w.Key).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetVariants(variantsToEntity(w.Variants)).
		SetRolloutPercentage(w.GetRolloutPercentage()).
		SetNillableGoalEventName(w.GoalEventName).
		SetNillableEnabled(w.Enabled).
		SetNillableStartedAt(timeutil.TimestamppbToTime(w.StartedAt)).
		SetNillableEndedAt(timeutil.TimestamppbToTime(w.EndedAt)).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *FeatureFlagRepo) Update(ctx context.Context, req *ubaV1.UpdateFeatureFlagRequest) (*ubaV1.FeatureFlag, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateFeatureFlagRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().FeatureFlag.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.FeatureFlag) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableRolloutPercentage(req.Data.RolloutPercentage).
				SetNillableGoalEventName(req.Data.GoalEventName).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableStartedAt(timeutil.TimestamppbToTime(req.Data.StartedAt)).
				SetNillableEndedAt(timeutil.TimestamppbToTime(req.Data.EndedAt)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Key != nil {
				builder.SetNillableKey(req.Data.Key)
			}
			if req.Data.Variants != nil {
				builder.SetVariants(variantsToEntity(req.Data.Variants))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(featureflag.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// ListEnabledByTenant 列出租户下启用的开关（评估用）；appID 非 0 时按应用过滤。
func (r *FeatureFlagRepo) ListEnabledByTenant(ctx context.Context, tenantID, appID int64) ([]*ent.FeatureFlag, error) {
	entities, err := r.entClient.Client().FeatureFlag.Query().
		Where(
			featureflag.EnabledEQ(true),
			featureflag.TenantIDEQ(tenantID),
		).
		All(ctx)
	if err != nil {
		r.log.Errorf("list enabled feature flags failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list enabled feature flags failed")
	}

	if appID == 0 {
		return entities, nil
	}

	matched := make([]*ent.FeatureFlag, 0, len(entities))
	for _, entity := range entities {
		if entity.AppID == nil || *entity.AppID == 0 || *entity.AppID == appID {
			matched = append(matched, entity)
		}
	}
	return matched, nil
}

// GetEntity 读取原始实体（评估/结果分析需要 variants 等完整字段）。
func (r *FeatureFlagRepo) GetEntity(ctx context.Context, id int64) (*ent.FeatureFlag, error) {
	entity, err := r.entClient.Client().FeatureFlag.Query().
		Where(featureflag.IDEQ(id)).
		Only(ctx)
	if err != nil {
		r.log.Errorf("get feature flag entity failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("get feature flag entity failed")
	}
	return entity, nil
}

func (r *FeatureFlagRepo) Delete(ctx context.Context, req *ubaV1.DeleteFeatureFlagRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().FeatureFlag.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(featureflag.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete feature flag failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete feature flag failed")
	}

	return nil
}

func (r *FeatureFlagRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().FeatureFlag.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate feature flags table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

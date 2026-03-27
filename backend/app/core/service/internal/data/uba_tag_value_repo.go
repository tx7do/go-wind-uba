package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/tagvalue"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type TagValueRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.TagValue, ent.TagValue]

	repository *entCrud.Repository[
		ent.TagValueQuery, ent.TagValueSelect,
		ent.TagValueCreate, ent.TagValueCreateBulk,
		ent.TagValueUpdate, ent.TagValueUpdateOne,
		ent.TagValueDelete,
		predicate.TagValue,
		ubaV1.TagValue, ent.TagValue,
	]
}

func NewTagValueRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *TagValueRepo {
	repo := &TagValueRepo{
		log:       ctx.NewLoggerHelper("tag-value/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.TagValue, ent.TagValue](),
	}

	repo.init()
	return repo
}

func (r *TagValueRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.TagValueQuery, ent.TagValueSelect,
		ent.TagValueCreate, ent.TagValueCreateBulk,
		ent.TagValueUpdate, ent.TagValueUpdateOne,
		ent.TagValueDelete,
		predicate.TagValue,
		ubaV1.TagValue, ent.TagValue,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Count 统计标签值数量
func (r *TagValueRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().TagValue.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}
	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query count failed: %s", err.Error())
		return 0, ubaV1.ErrorInternalServerError("query count failed")
	}
	return count, nil
}

// IsExist 判断标签值是否存在
func (r *TagValueRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().TagValue.Query().
		Where(tagvalue.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Create 创建标签值
func (r *TagValueRepo) Create(ctx context.Context, req *ubaV1.TagValue) (*ubaV1.TagValue, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().TagValue.Create().
		SetNillableTenantID(req.TenantId).
		SetNillableTagID(req.TagId).
		SetNillableValue(req.Value).
		SetNillableLabel(req.Label).
		SetNillableSortOrder(req.SortOrder).
		SetNillableIcon(req.Icon).
		SetNillableColor(req.Color).
		SetNillableDescription(req.Description).
		SetCreatedAt(time.Now())

	var err error
	var entity *ent.TagValue
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert tag-value failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert tag-value failed")
	}
	return r.mapper.ToDTO(entity), nil
}

package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/tagdefinition"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type TagDefinitionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.TagDefinition, ent.TagDefinition]

	categoryConverter *mapper.EnumTypeConverter[ubaV1.TagCategory, tagdefinition.Category]
	typeConverter     *mapper.EnumTypeConverter[ubaV1.TagType, tagdefinition.TagType]

	repository *entCrud.Repository[
		ent.TagDefinitionQuery, ent.TagDefinitionSelect,
		ent.TagDefinitionCreate, ent.TagDefinitionCreateBulk,
		ent.TagDefinitionUpdate, ent.TagDefinitionUpdateOne,
		ent.TagDefinitionDelete,
		predicate.TagDefinition,
		ubaV1.TagDefinition, ent.TagDefinition,
	]
}

func NewTagDefinitionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *TagDefinitionRepo {
	repo := &TagDefinitionRepo{
		log:       ctx.NewLoggerHelper("tag-definition/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.TagDefinition, ent.TagDefinition](),
		categoryConverter: mapper.NewEnumTypeConverter[ubaV1.TagCategory, tagdefinition.Category](
			ubaV1.TagCategory_name, ubaV1.TagCategory_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[ubaV1.TagType, tagdefinition.TagType](
			ubaV1.TagType_name, ubaV1.TagType_value,
		),
	}

	repo.init()
	return repo
}

func (r *TagDefinitionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.TagDefinitionQuery, ent.TagDefinitionSelect,
		ent.TagDefinitionCreate, ent.TagDefinitionCreateBulk,
		ent.TagDefinitionUpdate, ent.TagDefinitionUpdateOne,
		ent.TagDefinitionDelete,
		predicate.TagDefinition,
		ubaV1.TagDefinition, ent.TagDefinition,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.categoryConverter.NewConverterPair())
	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

// Count 统计标签数量
func (r *TagDefinitionRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountTagDefinitionResponse, error) {
	builder := r.entClient.Client().TagDefinition.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query tag-definition count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query tag-definition count failed")
	}

	return &ubaV1.CountTagDefinitionResponse{
		Count: uint64(count),
	}, nil
}

// List 标签列表
func (r *TagDefinitionRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListTagDefinitionResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().TagDefinition.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListTagDefinitionResponse{Total: 0, Items: nil}, nil
	}
	return &ubaV1.ListTagDefinitionResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// IsExist 判断标签是否存在
func (r *TagDefinitionRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().TagDefinition.Query().
		Where(tagdefinition.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Get 获取标签信息
func (r *TagDefinitionRepo) Get(ctx context.Context, req *ubaV1.GetTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().TagDefinition.Query()
	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetTagDefinitionRequest_Id:
		whereCond = append(whereCond, tagdefinition.IDEQ(req.GetId()))
	}
	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}
	return dto, err
}

// Create 创建标签
func (r *TagDefinitionRepo) Create(ctx context.Context, req *ubaV1.CreateTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().TagDefinition.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableName(req.Data.Name).
		SetNillableCode(req.Data.Code).
		SetNillableCategory(r.categoryConverter.ToEntity(req.Data.Category)).
		SetNillableTagType(r.typeConverter.ToEntity(req.Data.TagType)).
		SetNillableIsSystem(req.Data.IsSystem).
		SetNillableIsDynamic(req.Data.IsDynamic).
		SetNillableRefreshIntervalSeconds(req.Data.RefreshIntervalSeconds).
		SetNillableDescription(req.Data.Description).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Rule != nil {
		builder.SetRule(req.Data.Rule)
	}
	if req.Data.AllowedValues != nil {
		builder.SetAllowedValues(req.Data.AllowedValues)
	}

	var err error
	var entity *ent.TagDefinition
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert tag failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert tag failed")
	}
	return r.mapper.ToDTO(entity), nil
}

// Update 更新标签
func (r *TagDefinitionRepo) Update(ctx context.Context, req *ubaV1.UpdateTagDefinitionRequest) (*ubaV1.TagDefinition, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateTagDefinitionRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}
	builder := r.entClient.Client().TagDefinition.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.TagDefinition) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableCategory(r.categoryConverter.ToEntity(req.Data.Category)).
				SetNillableTagType(r.typeConverter.ToEntity(req.Data.TagType)).
				SetNillableIsSystem(req.Data.IsSystem).
				SetNillableIsDynamic(req.Data.IsDynamic).
				SetNillableRefreshIntervalSeconds(req.Data.RefreshIntervalSeconds).
				SetNillableDescription(req.Data.Description).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Rule != nil {
				builder.SetRule(req.Data.Rule)
			}
			if req.Data.AllowedValues != nil {
				builder.SetAllowedValues(req.Data.AllowedValues)
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(tagdefinition.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// Delete 删除标签
func (r *TagDefinitionRepo) Delete(ctx context.Context, req *ubaV1.DeleteTagDefinitionRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := r.entClient.Client().TagDefinition.DeleteOneID(req.GetId()).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ubaV1.ErrorNotFound("tag not found")
		}
		r.log.Errorf("delete one tag failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

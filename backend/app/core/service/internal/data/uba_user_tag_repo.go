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
	"go-wind-uba/app/core/service/internal/data/ent/usertag"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type UserTagRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.UserTag, ent.UserTag]

	repository *entCrud.Repository[
		ent.UserTagQuery, ent.UserTagSelect,
		ent.UserTagCreate, ent.UserTagCreateBulk,
		ent.UserTagUpdate, ent.UserTagUpdateOne,
		ent.UserTagDelete,
		predicate.UserTag,
		ubaV1.UserTag, ent.UserTag,
	]
}

func NewUserTagRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *UserTagRepo {
	repo := &UserTagRepo{
		log:       ctx.NewLoggerHelper("user-tag/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.UserTag, ent.UserTag](),
	}

	repo.init()
	return repo
}

func (r *UserTagRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.UserTagQuery, ent.UserTagSelect,
		ent.UserTagCreate, ent.UserTagCreateBulk,
		ent.UserTagUpdate, ent.UserTagUpdateOne,
		ent.UserTagDelete,
		predicate.UserTag,
		ubaV1.UserTag, ent.UserTag,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// Count 统计用户标签数量
func (r *UserTagRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountUserTagResponse, error) {
	builder := r.entClient.Client().UserTag.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query user-tag count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query user-tag count failed")
	}

	return &ubaV1.CountUserTagResponse{
		Count: uint64(count),
	}, nil
}

// List 用户标签列表
func (r *UserTagRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserTagResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().UserTag.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListUserTagResponse{Total: 0, Items: nil}, nil
	}
	return &ubaV1.ListUserTagResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// IsExist 判断用户标签是否存在
func (r *UserTagRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().UserTag.Query().
		Where(usertag.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Get 获取用户标签信息
func (r *UserTagRepo) Get(ctx context.Context, req *ubaV1.GetUserTagRequest) (*ubaV1.UserTag, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().UserTag.Query()
	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetUserTagRequest_Id:
		whereCond = append(whereCond, usertag.IDEQ(req.GetId()))
	}
	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}
	return dto, err
}

// Create 创建用户标签
func (r *UserTagRepo) Create(ctx context.Context, req *ubaV1.CreateUserTagRequest) (*ubaV1.UserTag, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().UserTag.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableUserID(req.Data.UserId).
		SetNillableTagID(req.Data.TagId).
		SetNillableValue(req.Data.Value).
		SetNillableValueLabel(req.Data.ValueLabel).
		SetNillableConfidence(req.Data.Confidence).
		SetNillableSourceRuleID(req.Data.SourceRuleId).
		SetNillableSource(req.Data.Source).
		SetNillableEffectiveTime(timeutil.TimestamppbToTime(req.Data.EffectiveTime)).
		SetNillableExpireTime(timeutil.TimestamppbToTime(req.Data.ExpireTime)).
		SetNillableSourceRuleID(req.Data.SourceRuleId).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	var err error
	var entity *ent.UserTag
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert user-tag failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert user-tag failed")
	}
	return r.mapper.ToDTO(entity), nil
}

// Update 更新用户标签
func (r *UserTagRepo) Update(ctx context.Context, req *ubaV1.UpdateUserTagRequest) (*ubaV1.UserTag, error) {
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
			createReq := &ubaV1.CreateUserTagRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}
	builder := r.entClient.Client().UserTag.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.UserTag) {
			builder.
				SetNillableUserID(req.Data.UserId).
				SetNillableTagID(req.Data.TagId).
				SetNillableValue(req.Data.Value).
				SetNillableValueLabel(req.Data.ValueLabel).
				SetNillableConfidence(req.Data.Confidence).
				SetNillableSourceRuleID(req.Data.SourceRuleId).
				SetNillableSource(req.Data.Source).
				SetNillableEffectiveTime(timeutil.TimestamppbToTime(req.Data.EffectiveTime)).
				SetNillableExpireTime(timeutil.TimestamppbToTime(req.Data.ExpireTime)).
				SetNillableSourceRuleID(req.Data.SourceRuleId).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(usertag.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// Delete 删除用户标签
func (r *UserTagRepo) Delete(ctx context.Context, req *ubaV1.DeleteUserTagRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := r.entClient.Client().UserTag.DeleteOneID(req.GetId()).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ubaV1.ErrorNotFound("user-tag not found")
		}
		r.log.Errorf("delete one user-tag failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

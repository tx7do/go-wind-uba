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
	"go-wind-uba/app/core/service/internal/data/ent/idmapping"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type IDMappingRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.IDMapping, ent.IDMapping]

	typeConverter *mapper.EnumTypeConverter[ubaV1.IDType, idmapping.IDType]

	repository *entCrud.Repository[
		ent.IDMappingQuery, ent.IDMappingSelect,
		ent.IDMappingCreate, ent.IDMappingCreateBulk,
		ent.IDMappingUpdate, ent.IDMappingUpdateOne,
		ent.IDMappingDelete,
		predicate.IDMapping,
		ubaV1.IDMapping, ent.IDMapping,
	]
}

func NewIDMappingRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *IDMappingRepo {
	repo := &IDMappingRepo{
		log:       ctx.NewLoggerHelper("id-mapping/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.IDMapping, ent.IDMapping](),

		typeConverter: mapper.NewEnumTypeConverter[ubaV1.IDType, idmapping.IDType](
			ubaV1.IDType_name, ubaV1.IDType_value,
		),
	}

	repo.init()
	return repo
}

func (r *IDMappingRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.IDMappingQuery, ent.IDMappingSelect,
		ent.IDMappingCreate, ent.IDMappingCreateBulk,
		ent.IDMappingUpdate, ent.IDMappingUpdateOne,
		ent.IDMappingDelete,
		predicate.IDMapping,
		ubaV1.IDMapping, ent.IDMapping,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

// Count 统计ID映射数量
func (r *IDMappingRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().IDMapping.Query()
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

// List ID映射列表
func (r *IDMappingRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListIDMappingResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().IDMapping.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListIDMappingResponse{Total: 0, Items: nil}, nil
	}
	return &ubaV1.ListIDMappingResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// IsExist 判断ID映射是否存在
func (r *IDMappingRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().IDMapping.Query().
		Where(idmapping.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Get 获取ID映射信息
func (r *IDMappingRepo) Get(ctx context.Context, req *ubaV1.GetIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().IDMapping.Query()
	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetIDMappingRequest_Id:
		whereCond = append(whereCond, idmapping.IDEQ(req.GetId()))
	}
	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}
	return dto, err
}

// Create 创建ID映射
func (r *IDMappingRepo) Create(ctx context.Context, req *ubaV1.CreateIDMappingRequest) (*ubaV1.IDMapping, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().IDMapping.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableGlobalUserID(req.Data.GlobalUserId).
		SetNillableIDType(r.typeConverter.ToEntity(req.Data.IdType)).
		SetNillableIDValue(req.Data.IdValue).
		SetNillableConfidence(req.Data.Confidence).
		SetNillableLinkSource(req.Data.LinkSource).
		SetNillableFirstSeen(timeutil.TimestamppbToTime(req.Data.FirstSeen)).
		SetNillableLastSeen(timeutil.TimestamppbToTime(req.Data.LastSeen)).
		SetNillableIsActive(req.Data.IsActive).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())
	if req.Data.Properties != nil {
		builder.SetProperties(req.GetData().GetProperties())
	}
	var err error
	var entity *ent.IDMapping
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert idmapping failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert idmapping failed")
	}
	return r.mapper.ToDTO(entity), nil
}

// Update 更新ID映射
func (r *IDMappingRepo) Update(ctx context.Context, req *ubaV1.UpdateIDMappingRequest) error {
	if req == nil || req.Data == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &ubaV1.CreateIDMappingRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			_, err = r.Create(ctx, createReq)
			return err
		}
	}
	builder := r.entClient.Client().IDMapping.UpdateOneID(req.GetId())
	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.IDMapping) {
			builder.
				SetNillableGlobalUserID(req.Data.GlobalUserId).
				SetNillableIDType(r.typeConverter.ToEntity(req.Data.IdType)).
				SetNillableIDValue(req.Data.IdValue).
				SetNillableConfidence(req.Data.Confidence).
				SetNillableLinkSource(req.Data.LinkSource).
				SetNillableFirstSeen(timeutil.TimestamppbToTime(req.Data.FirstSeen)).
				SetNillableLastSeen(timeutil.TimestamppbToTime(req.Data.LastSeen)).
				SetNillableIsActive(req.Data.IsActive).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
			if req.Data.Properties != nil {
				builder.SetProperties(req.GetData().GetProperties())
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(idmapping.FieldID, req.GetId()))
		},
	)
	return err
}

// Delete 删除ID映射
func (r *IDMappingRepo) Delete(ctx context.Context, req *ubaV1.DeleteIDMappingRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := r.entClient.Client().IDMapping.DeleteOneID(req.GetId()).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ubaV1.ErrorNotFound("idmapping not found")
		}
		r.log.Errorf("delete one idmapping failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

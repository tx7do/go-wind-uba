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
	"go-wind-uba/app/core/service/internal/data/ent/riskrule"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskRuleRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.RiskRule, ent.RiskRule]

	typeConverter  *mapper.EnumTypeConverter[ubaV1.RiskType, riskrule.RiskType]
	levelConverter *mapper.EnumTypeConverter[ubaV1.RiskLevel, riskrule.DefaultLevel]

	repository *entCrud.Repository[
		ent.RiskRuleQuery, ent.RiskRuleSelect,
		ent.RiskRuleCreate, ent.RiskRuleCreateBulk,
		ent.RiskRuleUpdate, ent.RiskRuleUpdateOne,
		ent.RiskRuleDelete,
		predicate.RiskRule,
		ubaV1.RiskRule, ent.RiskRule,
	]
}

func NewRiskRuleRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *RiskRuleRepo {
	repo := &RiskRuleRepo{
		log:       ctx.NewLoggerHelper("risk-rule/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.RiskRule, ent.RiskRule](),

		typeConverter: mapper.NewEnumTypeConverter[ubaV1.RiskType, riskrule.RiskType](
			ubaV1.RiskType_name, ubaV1.RiskType_value,
		),
		levelConverter: mapper.NewEnumTypeConverter[ubaV1.RiskLevel, riskrule.DefaultLevel](
			ubaV1.RiskLevel_name, ubaV1.RiskLevel_value,
		),
	}

	repo.init()
	return repo
}

func (r *RiskRuleRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.RiskRuleQuery, ent.RiskRuleSelect,
		ent.RiskRuleCreate, ent.RiskRuleCreateBulk,
		ent.RiskRuleUpdate, ent.RiskRuleUpdateOne,
		ent.RiskRuleDelete,
		predicate.RiskRule,
		ubaV1.RiskRule, ent.RiskRule,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
	r.mapper.AppendConverters(r.levelConverter.NewConverterPair())
}

// Count 统计风险规则数量
func (r *RiskRuleRepo) Count(ctx context.Context, whereCond []func(s *sql.Selector)) (int, error) {
	builder := r.entClient.Client().RiskRule.Query()
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

// List 风险规则列表
func (r *RiskRuleRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskRuleResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().RiskRule.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListRiskRuleResponse{Total: 0, Items: nil}, nil
	}
	return &ubaV1.ListRiskRuleResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// IsExist 判断风险规则是否存在
func (r *RiskRuleRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().RiskRule.Query().
		Where(riskrule.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

// Get 获取风险规则信息
func (r *RiskRuleRepo) Get(ctx context.Context, req *ubaV1.GetRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().RiskRule.Query()
	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetRiskRuleRequest_Id:
		whereCond = append(whereCond, riskrule.IDEQ(req.GetId()))
	}
	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}
	return dto, err
}

// Create 创建风险规则
func (r *RiskRuleRepo) Create(ctx context.Context, req *ubaV1.CreateRiskRuleRequest) (*ubaV1.RiskRule, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().RiskRule.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableName(req.Data.Name).
		SetNillableCode(req.Data.Code).
		SetNillableDescription(req.Data.Description).
		SetNillableRiskType(r.typeConverter.ToEntity(req.Data.RiskType)).
		SetNillableDefaultLevel(r.levelConverter.ToEntity(req.Data.DefaultLevel)).
		SetNillableEnabled(req.Data.Enabled).
		SetNillablePriority(req.Data.Priority).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Condition != nil {
		builder.SetCondition(req.GetData().GetCondition().AsMap())
	}
	if req.Data.Actions != nil {
		builder.SetActions(req.GetData().GetActions())
	}

	var err error
	var entity *ent.RiskRule
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert riskrule failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert riskrule failed")
	}
	return r.mapper.ToDTO(entity), nil
}

// Update 更新风险规则
func (r *RiskRuleRepo) Update(ctx context.Context, req *ubaV1.UpdateRiskRuleRequest) error {
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
			createReq := &ubaV1.CreateRiskRuleRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			_, err = r.Create(ctx, createReq)
			return err
		}
	}
	builder := r.entClient.Client().RiskRule.UpdateOneID(req.GetId())
	_, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.RiskRule) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableDescription(req.Data.Description).
				SetNillableRiskType(r.typeConverter.ToEntity(req.Data.RiskType)).
				SetNillableDefaultLevel(r.levelConverter.ToEntity(req.Data.DefaultLevel)).
				SetNillableEnabled(req.Data.Enabled).
				SetNillablePriority(req.Data.Priority).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Condition != nil {
				builder.SetCondition(req.GetData().GetCondition().AsMap())
			}
			if req.Data.Actions != nil {
				builder.SetActions(req.GetData().GetActions())
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(riskrule.FieldID, req.GetId()))
		},
	)
	return err
}

// Delete 删除风险规则
func (r *RiskRuleRepo) Delete(ctx context.Context, req *ubaV1.DeleteRiskRuleRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := r.entClient.Client().RiskRule.DeleteOneID(req.GetId()).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ubaV1.ErrorNotFound("riskrule not found")
		}
		r.log.Errorf("delete one riskrule failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

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
	"go-wind-uba/app/core/service/internal/data/ent/alertrule"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// AlertRuleRepo 指标阈值告警规则数据仓库
type AlertRuleRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	aggregatorConverter *mapper.EnumTypeConverter[ubaV1.AlertAggregator, string]
	comparisonConverter *mapper.EnumTypeConverter[ubaV1.AlertComparison, string]

	mapper *mapper.CopierMapper[ubaV1.AlertRule, ent.AlertRule]

	repository *entCrud.Repository[
		ent.AlertRuleQuery, ent.AlertRuleSelect,
		ent.AlertRuleCreate, ent.AlertRuleCreateBulk,
		ent.AlertRuleUpdate, ent.AlertRuleUpdateOne,
		ent.AlertRuleDelete,
		predicate.AlertRule,
		ubaV1.AlertRule, ent.AlertRule,
	]
}

func NewAlertRuleRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AlertRuleRepo {
	repo := &AlertRuleRepo{
		log:                 ctx.NewLoggerHelper("alertrule/repo/core-service"),
		entClient:           entClient,
		mapper:              mapper.NewCopierMapper[ubaV1.AlertRule, ent.AlertRule](),
		aggregatorConverter: mapper.NewEnumTypeConverter[ubaV1.AlertAggregator, string](ubaV1.AlertAggregator_name, ubaV1.AlertAggregator_value),
		comparisonConverter: mapper.NewEnumTypeConverter[ubaV1.AlertComparison, string](ubaV1.AlertComparison_name, ubaV1.AlertComparison_value),
	}

	repo.init()

	return repo
}

func (r *AlertRuleRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.AlertRuleQuery, ent.AlertRuleSelect,
		ent.AlertRuleCreate, ent.AlertRuleCreateBulk,
		ent.AlertRuleUpdate, ent.AlertRuleUpdateOne,
		ent.AlertRuleDelete,
		predicate.AlertRule,
		ubaV1.AlertRule, ent.AlertRule,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.aggregatorConverter.NewConverterPair())
	r.mapper.AppendConverters(r.comparisonConverter.NewConverterPair())
	r.mapper.AppendConverters(mapper.NewGenericTypeConverterPair(
		[]ubaV1.AlertChannel{}, []string{},
		channelToEntity, channelToDTO,
	))
}

func channelToEntity(src []ubaV1.AlertChannel) []string {
	if len(src) == 0 {
		return nil
	}
	ret := make([]string, 0, len(src))
	for _, c := range src {
		ret = append(ret, c.String())
	}
	return ret
}

func channelToDTO(src []string) []ubaV1.AlertChannel {
	if len(src) == 0 {
		return nil
	}
	ret := make([]ubaV1.AlertChannel, 0, len(src))
	for _, s := range src {
		if v, ok := ubaV1.AlertChannel_value[s]; ok {
			ret = append(ret, ubaV1.AlertChannel(v))
		}
	}
	return ret
}

func (r *AlertRuleRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountAlertRuleResponse, error) {
	builder := r.entClient.Client().AlertRule.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query alert rule count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query alert rule count failed")
	}

	return &ubaV1.CountAlertRuleResponse{
		Count: int64(count),
	}, nil
}

func (r *AlertRuleRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListAlertRuleResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AlertRule.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListAlertRuleResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListAlertRuleResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *AlertRuleRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().AlertRule.Query().
		Where(alertrule.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *AlertRuleRepo) Get(ctx context.Context, req *ubaV1.GetAlertRuleRequest) (*ubaV1.AlertRule, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AlertRule.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetAlertRuleRequest_Id:
		whereCond = append(whereCond, alertrule.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *AlertRuleRepo) Create(ctx context.Context, req *ubaV1.CreateAlertRuleRequest) (*ubaV1.AlertRule, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newAlertRuleCreate(req.Data)

	var entity *ent.AlertRule
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert alert rule failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert alert rule failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *AlertRuleRepo) newAlertRuleCreate(w *ubaV1.AlertRule) *ent.AlertRuleCreate {
	builder := r.entClient.Client().AlertRule.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetNillableEventName(w.EventName).
		SetAggregator(r.aggregatorString(w.Aggregator)).
		SetComparison(r.comparisonString(w.Comparison)).
		SetThreshold(w.GetThreshold()).
		SetWindowMinutes(w.GetWindowMinutes()).
		SetSilenceMinutes(w.GetSilenceMinutes()).
		SetChannels(channelToEntity(w.Channels)).
		SetNillableEnabled(w.Enabled).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

// aggregatorString 把聚合方式枚举转为存储值，未指定时回退默认。
func (r *AlertRuleRepo) aggregatorString(v *ubaV1.AlertAggregator) string {
	if v == nil {
		return "COUNT"
	}
	if s := r.aggregatorConverter.ToEntity(v); s != nil {
		return *s
	}
	return "COUNT"
}

// comparisonString 把比较算子枚举转为存储值，未指定时回退默认。
func (r *AlertRuleRepo) comparisonString(v *ubaV1.AlertComparison) string {
	if v == nil {
		return "GT"
	}
	if s := r.comparisonConverter.ToEntity(v); s != nil {
		return *s
	}
	return "GT"
}

func (r *AlertRuleRepo) Update(ctx context.Context, req *ubaV1.UpdateAlertRuleRequest) (*ubaV1.AlertRule, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateAlertRuleRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().AlertRule.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.AlertRule) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableEventName(req.Data.EventName).
				SetNillableThreshold(req.Data.Threshold).
				SetNillableWindowMinutes(req.Data.WindowMinutes).
				SetNillableSilenceMinutes(req.Data.SilenceMinutes).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableLastTriggeredAt(timeutil.TimestamppbToTime(req.Data.LastTriggeredAt)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Aggregator != nil {
				builder.SetAggregator(r.aggregatorString(req.Data.Aggregator))
			}
			if req.Data.Comparison != nil {
				builder.SetComparison(r.comparisonString(req.Data.Comparison))
			}
			if req.Data.Channels != nil {
				builder.SetChannels(channelToEntity(req.Data.Channels))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(alertrule.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// UpdateTriggerState 触发后回写最后触发时间（评估器专用，不经 DTO 层）。
func (r *AlertRuleRepo) UpdateTriggerState(ctx context.Context, id int64, triggeredAt time.Time) error {
	err := r.entClient.Client().AlertRule.UpdateOneID(id).
		SetLastTriggeredAt(triggeredAt).
		SetUpdatedAt(triggeredAt).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("update alert rule trigger state failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update alert rule trigger state failed")
	}
	return nil
}

// ListDueRules 列出到期待评估的启用规则：从未触发过，或距上次触发已超过静默时长。
func (r *AlertRuleRepo) ListDueRules(ctx context.Context, now time.Time) ([]*ent.AlertRule, error) {
	rules, err := r.entClient.Client().AlertRule.Query().
		Where(alertrule.EnabledEQ(true)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list due alert rules failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list due alert rules failed")
	}

	due := make([]*ent.AlertRule, 0, len(rules))
	for _, rule := range rules {
		if rule.LastTriggeredAt == nil {
			due = append(due, rule)
			continue
		}
		if now.Sub(*rule.LastTriggeredAt) >= time.Duration(rule.SilenceMinutes)*time.Minute {
			due = append(due, rule)
		}
	}
	return due, nil
}

func (r *AlertRuleRepo) Delete(ctx context.Context, req *ubaV1.DeleteAlertRuleRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AlertRule.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(alertrule.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete alert rule failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete alert rule failed")
	}

	return nil
}

func (r *AlertRuleRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().AlertRule.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate alert rules table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

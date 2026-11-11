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
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/reportsubscription"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// ReportSubscriptionRepo 定时报告订阅数据仓库
type ReportSubscriptionRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	channelConverter *mapper.EnumTypeConverter[ubaV1.ReportChannel, string]

	mapper *mapper.CopierMapper[ubaV1.ReportSubscription, ent.ReportSubscription]

	repository *entCrud.Repository[
		ent.ReportSubscriptionQuery, ent.ReportSubscriptionSelect,
		ent.ReportSubscriptionCreate, ent.ReportSubscriptionCreateBulk,
		ent.ReportSubscriptionUpdate, ent.ReportSubscriptionUpdateOne,
		ent.ReportSubscriptionDelete,
		predicate.ReportSubscription,
		ubaV1.ReportSubscription, ent.ReportSubscription,
	]
}

func NewReportSubscriptionRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ReportSubscriptionRepo {
	repo := &ReportSubscriptionRepo{
		log:              ctx.NewLoggerHelper("report-subscription/repo/core-service"),
		entClient:        entClient,
		mapper:           mapper.NewCopierMapper[ubaV1.ReportSubscription, ent.ReportSubscription](),
		channelConverter: mapper.NewEnumTypeConverter[ubaV1.ReportChannel, string](ubaV1.ReportChannel_name, ubaV1.ReportChannel_value),
	}

	repo.init()

	return repo
}

func (r *ReportSubscriptionRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.ReportSubscriptionQuery, ent.ReportSubscriptionSelect,
		ent.ReportSubscriptionCreate, ent.ReportSubscriptionCreateBulk,
		ent.ReportSubscriptionUpdate, ent.ReportSubscriptionUpdateOne,
		ent.ReportSubscriptionDelete,
		predicate.ReportSubscription,
		ubaV1.ReportSubscription, ent.ReportSubscription,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.channelConverter.NewConverterPair())
}

func reportChannelToEntity(src []ubaV1.ReportChannel) []string {
	if len(src) == 0 {
		return nil
	}
	ret := make([]string, 0, len(src))
	for _, c := range src {
		ret = append(ret, c.String())
	}
	return ret
}

func reportChannelToDTO(src []string) []ubaV1.ReportChannel {
	if len(src) == 0 {
		return nil
	}
	ret := make([]ubaV1.ReportChannel, 0, len(src))
	for _, name := range src {
		if v, ok := ubaV1.ReportChannel_value[name]; ok {
			ret = append(ret, ubaV1.ReportChannel(v))
		}
	}
	return ret
}

func (r *ReportSubscriptionRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountReportSubscriptionResponse, error) {
	builder := r.entClient.Client().ReportSubscription.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query report subscription count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query report subscription count failed")
	}

	return &ubaV1.CountReportSubscriptionResponse{
		Count: int64(count),
	}, nil
}

func (r *ReportSubscriptionRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListReportSubscriptionResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ReportSubscription.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListReportSubscriptionResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListReportSubscriptionResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ReportSubscriptionRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().ReportSubscription.Query().
		Where(reportsubscription.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *ReportSubscriptionRepo) Get(ctx context.Context, req *ubaV1.GetReportSubscriptionRequest) (*ubaV1.ReportSubscription, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ReportSubscription.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetReportSubscriptionRequest_Id:
		whereCond = append(whereCond, reportsubscription.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *ReportSubscriptionRepo) Create(ctx context.Context, req *ubaV1.CreateReportSubscriptionRequest) (*ubaV1.ReportSubscription, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newReportSubscriptionCreate(req.Data)

	var entity *ent.ReportSubscription
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert report subscription failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert report subscription failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *ReportSubscriptionRepo) newReportSubscriptionCreate(w *ubaV1.ReportSubscription) *ent.ReportSubscriptionCreate {
	builder := r.entClient.Client().ReportSubscription.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetNillableModel(w.Model).
		SetNillableParamsJSON(w.ParamsJson).
		SetIntervalMinutes(w.GetIntervalMinutes()).
		SetChannels(reportChannelToEntity(w.Channels)).
		SetNillableEnabled(w.Enabled).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *ReportSubscriptionRepo) Update(ctx context.Context, req *ubaV1.UpdateReportSubscriptionRequest) (*ubaV1.ReportSubscription, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateReportSubscriptionRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().ReportSubscription.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.ReportSubscription) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableModel(req.Data.Model).
				SetNillableParamsJSON(req.Data.ParamsJson).
				SetNillableIntervalMinutes(req.Data.IntervalMinutes).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableLastRunAt(timeutil.TimestamppbToTime(req.Data.LastRunAt)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Channels != nil {
				builder.SetChannels(reportChannelToEntity(req.Data.Channels))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(reportsubscription.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// ListDueSubscriptions 列出到期待执行的启用订阅：从未执行过，或距上次执行已超过间隔。
func (r *ReportSubscriptionRepo) ListDueSubscriptions(ctx context.Context, now time.Time) ([]*ent.ReportSubscription, error) {
	defs, err := r.entClient.Client().ReportSubscription.Query().
		Where(reportsubscription.EnabledEQ(true)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list due report subscriptions failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list due report subscriptions failed")
	}

	due := make([]*ent.ReportSubscription, 0, len(defs))
	for _, def := range defs {
		interval := time.Duration(def.IntervalMinutes) * time.Minute
		if interval < time.Hour {
			interval = time.Hour
		}
		if def.LastRunAt == nil || now.Sub(*def.LastRunAt) >= interval {
			due = append(due, def)
		}
	}
	return due, nil
}

// UpdateRunState 执行完成后回写时间。
func (r *ReportSubscriptionRepo) UpdateRunState(ctx context.Context, id int64, runAt time.Time) error {
	err := r.entClient.Client().ReportSubscription.UpdateOneID(id).
		SetLastRunAt(runAt).
		SetUpdatedAt(runAt).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("update report subscription run state failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update report subscription run state failed")
	}
	return nil
}

func (r *ReportSubscriptionRepo) Delete(ctx context.Context, req *ubaV1.DeleteReportSubscriptionRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().ReportSubscription.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(reportsubscription.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete report subscription failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete report subscription failed")
	}

	return nil
}

func (r *ReportSubscriptionRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().ReportSubscription.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate report subscriptions table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

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
	"go-wind-uba/app/core/service/internal/data/ent/campaign"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// CampaignRepo 营销触达活动数据仓库
type CampaignRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.Campaign, ent.Campaign]

	repository *entCrud.Repository[
		ent.CampaignQuery, ent.CampaignSelect,
		ent.CampaignCreate, ent.CampaignCreateBulk,
		ent.CampaignUpdate, ent.CampaignUpdateOne,
		ent.CampaignDelete,
		predicate.Campaign,
		ubaV1.Campaign, ent.Campaign,
	]
}

func NewCampaignRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CampaignRepo {
	repo := &CampaignRepo{
		log:       ctx.NewLoggerHelper("campaign/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.Campaign, ent.Campaign](),
	}

	repo.init()

	return repo
}

func (r *CampaignRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.CampaignQuery, ent.CampaignSelect,
		ent.CampaignCreate, ent.CampaignCreateBulk,
		ent.CampaignUpdate, ent.CampaignUpdateOne,
		ent.CampaignDelete,
		predicate.Campaign,
		ubaV1.Campaign, ent.Campaign,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// ToDTO 实体转 DTO。
func (r *CampaignRepo) ToDTO(entity *ent.Campaign) *ubaV1.Campaign {
	return r.mapper.ToDTO(entity)
}

func (r *CampaignRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountCampaignResponse, error) {
	builder := r.entClient.Client().Campaign.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query campaign count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query campaign count failed")
	}

	return &ubaV1.CountCampaignResponse{
		Count: int64(count),
	}, nil
}

func (r *CampaignRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListCampaignResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Campaign.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListCampaignResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListCampaignResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *CampaignRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().Campaign.Query().
		Where(campaign.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *CampaignRepo) Get(ctx context.Context, req *ubaV1.GetCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Campaign.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetCampaignRequest_Id:
		whereCond = append(whereCond, campaign.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

// GetEntity 读取原始实体（发送执行器需要完整字段）。
func (r *CampaignRepo) GetEntity(ctx context.Context, id int64) (*ent.Campaign, error) {
	entity, err := r.entClient.Client().Campaign.Query().
		Where(campaign.IDEQ(id)).
		Only(ctx)
	if err != nil {
		r.log.Errorf("get campaign entity failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("get campaign entity failed")
	}
	return entity, nil
}

func (r *CampaignRepo) Create(ctx context.Context, req *ubaV1.CreateCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newCampaignCreate(req.Data)

	var entity *ent.Campaign
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert campaign failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert campaign failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *CampaignRepo) newCampaignCreate(w *ubaV1.Campaign) *ent.CampaignCreate {
	builder := r.entClient.Client().Campaign.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetSegmentID(w.GetSegmentId()).
		SetNillableTitle(w.Title).
		SetNillableContent(w.Content).
		SetNillableTemplateCode(w.TemplateCode).
		SetSendMode(campaignSendModeString(w.SendMode)).
		SetNillableScheduledAt(timeutil.TimestamppbToTime(w.ScheduledAt)).
		SetStatus(campaignStatusString(w.Status)).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *CampaignRepo) Update(ctx context.Context, req *ubaV1.UpdateCampaignRequest) (*ubaV1.Campaign, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateCampaignRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Campaign.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.Campaign) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableTitle(req.Data.Title).
				SetNillableContent(req.Data.Content).
				SetNillableTemplateCode(req.Data.TemplateCode).
				SetNillableScheduledAt(timeutil.TimestamppbToTime(req.Data.ScheduledAt)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.GetSegmentId() != 0 {
				builder.SetSegmentID(req.Data.GetSegmentId())
			}
			if req.Data.SendMode != nil {
				builder.SetSendMode(campaignSendModeString(req.Data.SendMode))
			}
			if req.Data.Status != nil {
				builder.SetStatus(campaignStatusString(req.Data.Status))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(campaign.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// UpdateSendProgress 群发执行器回写进度与结论。
func (r *CampaignRepo) UpdateSendProgress(ctx context.Context, id int64, total, sent, failed int64, status string, now time.Time) error {
	builder := r.entClient.Client().Campaign.UpdateOneID(id).
		SetTotalCount(uint64(total)).
		SetSentCount(uint64(sent)).
		SetFailedCount(uint64(failed)).
		SetStatus(status).
		SetLastSentAt(now).
		SetUpdatedAt(now)

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("update campaign send progress failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update campaign send progress failed")
	}
	return nil
}

// UpdateStatus 仅改状态（取消/开始发送等）。
func (r *CampaignRepo) UpdateStatus(ctx context.Context, id int64, status string, now time.Time) error {
	err := r.entClient.Client().Campaign.UpdateOneID(id).
		SetStatus(status).
		SetUpdatedAt(now).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("update campaign status failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update campaign status failed")
	}
	return nil
}

func campaignSendModeString(v *ubaV1.CampaignSendMode) string {
	if v == nil {
		return "MANUAL"
	}
	if s := v.String(); s != "CAMPAIGN_SEND_MODE_UNSPECIFIED" {
		return trimCampaignPrefix(s)
	}
	return "MANUAL"
}

func campaignStatusString(v *ubaV1.CampaignStatus) string {
	if v == nil {
		return "DRAFT"
	}
	if s := v.String(); s != "CAMPAIGN_STATUS_UNSPECIFIED" {
		return trimCampaignPrefix(s)
	}
	return "DRAFT"
}

// trimCampaignPrefix 去掉枚举名前缀（CAMPAIGN_SEND_MODE_ / CAMPAIGN_STATUS_）。
func trimCampaignPrefix(s string) string {
	for _, prefix := range []string{"CAMPAIGN_SEND_MODE_", "CAMPAIGN_STATUS_"} {
		if idx := indexOf(s, prefix); idx == 0 {
			return s[len(prefix):]
		}
	}
	return s
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (r *CampaignRepo) Delete(ctx context.Context, req *ubaV1.DeleteCampaignRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Campaign.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(campaign.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete campaign failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete campaign failed")
	}

	return nil
}

func (r *CampaignRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Campaign.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate campaigns table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/aiusagelog"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	aiV1 "go-wind-uba/api/gen/go/ai/service/v1"
)

// AiUsageLogRepo AI 调用用量流水仓储：只增不改（留档对账）。
// List/MonthStats 为只读视图：租户管理员只见本租户流水（谓词显式过滤），
// 平台管理员（tenantId=0）见全量。
type AiUsageLogRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[aiV1.AiUsageLog, ent.AiUsageLog]

	repository *entCrud.Repository[
		ent.AiUsageLogQuery, ent.AiUsageLogSelect,
		ent.AiUsageLogCreate, ent.AiUsageLogCreateBulk,
		ent.AiUsageLogUpdate, ent.AiUsageLogUpdateOne,
		ent.AiUsageLogDelete,
		predicate.AiUsageLog,
		aiV1.AiUsageLog, ent.AiUsageLog,
	]
}

func NewAiUsageLogRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AiUsageLogRepo {
	repo := &AiUsageLogRepo{
		log:       ctx.NewLoggerHelper("ai_usage_log/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[aiV1.AiUsageLog, ent.AiUsageLog](),
	}

	repo.repository = entCrud.NewRepository[
		ent.AiUsageLogQuery, ent.AiUsageLogSelect,
		ent.AiUsageLogCreate, ent.AiUsageLogCreateBulk,
		ent.AiUsageLogUpdate, ent.AiUsageLogUpdateOne,
		ent.AiUsageLogDelete,
		predicate.AiUsageLog,
		aiV1.AiUsageLog, ent.AiUsageLog,
	](repo.mapper)

	repo.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	repo.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	return repo
}

// UsageRecord 一次成功模型调用的记账条目（服务层便捷形态，内部转 DTO 落库）。
type UsageRecord struct {
	ProviderID       int64
	ConversationID   int64
	UserID           int64
	TenantID         int64
	ModelName        string
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	DurationMs       int64
}

// Create 记一笔成功调用的用量流水。记账是尽力而为：调用方吞掉错误只打日志，
// 不因记账失败阻断生成结果返回。
func (r *AiUsageLogRepo) Create(ctx context.Context, rec *UsageRecord) error {
	if rec == nil {
		return nil
	}
	return r.CreateDTO(ctx, &aiV1.AiUsageLog{
		ProviderId:       nillableInt64(rec.ProviderID),
		ConversationId:   nillableInt64(rec.ConversationID),
		UserId:           nillableInt64(rec.UserID),
		TenantId:         nillableInt64(rec.TenantID),
		ModelName:        nillableString(rec.ModelName),
		PromptTokens:     nillableUint32(rec.PromptTokens),
		CompletionTokens: nillableUint32(rec.CompletionTokens),
		TotalTokens:      nillableUint32(rec.TotalTokens),
		DurationMs:       nillableUint32(rec.DurationMs),
	})
}

// CreateDTO 落一行流水（只增不改）。
func (r *AiUsageLogRepo) CreateDTO(ctx context.Context, data *aiV1.AiUsageLog) error {
	if data == nil {
		return nil
	}

	builder := r.entClient.Client().AiUsageLog.Create().
		SetNillableProviderID(data.ProviderId).
		SetNillableConversationID(data.ConversationId).
		SetNillableUserID(data.UserId).
		SetNillableTenantID(data.TenantId).
		SetNillableModelName(data.ModelName).
		SetNillablePromptTokens(data.PromptTokens).
		SetNillableCompletionTokens(data.CompletionTokens).
		SetNillableTotalTokens(data.TotalTokens).
		SetNillableDurationMs(data.DurationMs).
		SetCreatedAt(time.Now())

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf("insert ai usage log failed: %s", err.Error())
		return err
	}

	return nil
}

// Count 流水条数（tenantId>0 限本租户）。
func (r *AiUsageLogRepo) Count(ctx context.Context, req *paginationV1.PagingRequest, tenantId int64) (*aiV1.CountAiUsageLogResponse, error) {
	builder := r.entClient.Client().AiUsageLog.Query()
	if tenantId > 0 {
		builder = builder.Where(aiusagelog.TenantIDEQ(tenantId))
	}

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query ai usage log count failed: %s", err.Error())
		return nil, aiV1.ErrorInternalServerError("query ai usage log count failed")
	}

	return &aiV1.CountAiUsageLogResponse{Count: int64(count)}, nil
}

// List 流水分页（tenantId>0 限本租户）。
func (r *AiUsageLogRepo) List(ctx context.Context, req *paginationV1.PagingRequest, tenantId int64) (*aiV1.ListAiUsageLogResponse, error) {
	if req == nil {
		return nil, aiV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().AiUsageLog.Query()
	if tenantId > 0 {
		builder = builder.Where(aiusagelog.TenantIDEQ(tenantId))
	}

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &aiV1.ListAiUsageLogResponse{Total: 0, Items: nil}, nil
	}

	return &aiV1.ListAiUsageLogResponse{Total: ret.Total, Items: ret.Items}, nil
}

// MonthStats 聚合本月（自 monthStart 起）的用量：tokens 总和与调用次数。
// 口径与流水列表一致：租户用户按本租户，平台管理员（tenantId=0）算全量。
func (r *AiUsageLogRepo) MonthStats(ctx context.Context, tenantId int64, monthStart time.Time) (int64, int64, error) {
	var rows []struct {
		Total int64 `sql:"total"`
		Cnt   int64 `sql:"cnt"`
	}
	q := r.entClient.Client().AiUsageLog.Query().Where(aiusagelog.CreatedAtGTE(monthStart))
	if tenantId > 0 {
		q = q.Where(aiusagelog.TenantIDEQ(tenantId))
	}
	err := q.Aggregate(
		ent.As(ent.Sum(aiusagelog.FieldTotalTokens), "total"),
		ent.As(ent.Count(), "cnt"),
	).Scan(ctx, &rows)
	if err != nil {
		r.log.Errorf("month stats failed: %s", err.Error())
		return 0, 0, aiV1.ErrorInternalServerError("month stats failed")
	}
	if len(rows) > 0 {
		return rows[0].Total, rows[0].Cnt, nil
	}
	return 0, 0, nil
}

func nillableUint32(v int64) *uint32 {
	if v == 0 {
		return nil
	}
	x := uint32(v)
	return &x
}

func nillableInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func nillableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

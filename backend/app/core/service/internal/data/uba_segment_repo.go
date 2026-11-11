package data

import (
	"context"
	"encoding/json"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/encoding/protojson"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"
	"go-wind-uba/app/core/service/internal/data/ent/segment"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// SegmentRepo 用户分群数据仓库
type SegmentRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.Segment, ent.Segment]

	repository *entCrud.Repository[
		ent.SegmentQuery, ent.SegmentSelect,
		ent.SegmentCreate, ent.SegmentCreateBulk,
		ent.SegmentUpdate, ent.SegmentUpdateOne,
		ent.SegmentDelete,
		predicate.Segment,
		ubaV1.Segment, ent.Segment,
	]
}

func NewSegmentRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SegmentRepo {
	repo := &SegmentRepo{
		log:       ctx.NewLoggerHelper("segment/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.Segment, ent.Segment](),
	}

	repo.init()

	return repo
}

func (r *SegmentRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.SegmentQuery, ent.SegmentSelect,
		ent.SegmentCreate, ent.SegmentCreateBulk,
		ent.SegmentUpdate, ent.SegmentUpdateOne,
		ent.SegmentDelete,
		predicate.Segment,
		ubaV1.Segment, ent.Segment,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(mapper.NewGenericTypeConverterPair(
		&ubaV1.SegmentationDefinition{}, map[string]any{},
		SegmentDefinitionToEntity, SegmentDefinitionToDTO,
	))
}

// SegmentDefinitionToDTO 把持久化的 JSON 快照还原为圈选定义。
func SegmentDefinitionToDTO(src map[string]any) *ubaV1.SegmentationDefinition {
	if len(src) == 0 {
		return nil
	}

	raw, err := json.Marshal(src)
	if err != nil {
		return nil
	}

	def := &ubaV1.SegmentationDefinition{}
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal(raw, def); err != nil {
		return nil
	}
	return def
}

// SegmentDefinitionToEntity 把圈选定义序列化为可持久化的 JSON 快照。
func SegmentDefinitionToEntity(src *ubaV1.SegmentationDefinition) map[string]any {
	if src == nil {
		return nil
	}

	raw, err := protojson.Marshal(src)
	if err != nil {
		return nil
	}

	ret := map[string]any{}
	if err := json.Unmarshal(raw, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *SegmentRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountSegmentResponse, error) {
	builder := r.entClient.Client().Segment.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query segment count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query segment count failed")
	}

	return &ubaV1.CountSegmentResponse{
		Count: int64(count),
	}, nil
}

func (r *SegmentRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListSegmentResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Segment.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListSegmentResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListSegmentResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *SegmentRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().Segment.Query().
		Where(segment.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *SegmentRepo) Get(ctx context.Context, req *ubaV1.GetSegmentRequest) (*ubaV1.Segment, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Segment.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetSegmentRequest_Id:
		whereCond = append(whereCond, segment.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

// GetEntity 读取原始实体（刷新流程需要租户/应用/定义等完整字段）。
func (r *SegmentRepo) GetEntity(ctx context.Context, id int64) (*ent.Segment, error) {
	entity, err := r.entClient.Client().Segment.Query().
		Where(segment.IDEQ(id)).
		Only(ctx)
	if err != nil {
		r.log.Errorf("get segment entity failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("get segment entity failed")
	}
	return entity, nil
}

func (r *SegmentRepo) Create(ctx context.Context, req *ubaV1.CreateSegmentRequest) (*ubaV1.Segment, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newSegmentCreate(req.Data)

	var entity *ent.Segment
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert segment failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert segment failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *SegmentRepo) newSegmentCreate(w *ubaV1.Segment) *ent.SegmentCreate {
	builder := r.entClient.Client().Segment.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetDefinition(SegmentDefinitionToEntity(w.Definition)).
		SetRefreshIntervalMinutes(w.GetRefreshIntervalMinutes()).
		SetMaxUsers(w.GetMaxUsers()).
		SetUserCount(uint64(w.GetUserCount())).
		SetNillableLastRefreshedAt(timeutil.TimestamppbToTime(w.LastRefreshedAt)).
		SetNillableEnabled(w.Enabled).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *SegmentRepo) Update(ctx context.Context, req *ubaV1.UpdateSegmentRequest) (*ubaV1.Segment, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateSegmentRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Segment.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.Segment) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableRefreshIntervalMinutes(req.Data.RefreshIntervalMinutes).
				SetNillableMaxUsers(req.Data.MaxUsers).
				SetNillableUserCount(u64p(req.Data.UserCount)).
				SetNillableLastRefreshedAt(timeutil.TimestamppbToTime(req.Data.LastRefreshedAt)).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Definition != nil {
				builder.SetDefinition(SegmentDefinitionToEntity(req.Data.Definition))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(segment.FieldID, req.GetId()))
		},
	)

	return dto, err
}

// UpdateRefreshState 刷新完成后回写人数与刷新时间。
func (r *SegmentRepo) UpdateRefreshState(ctx context.Context, id int64, userCount int64, refreshedAt time.Time) error {
	err := r.entClient.Client().Segment.UpdateOneID(id).
		SetUserCount(uint64(userCount)).
		SetLastRefreshedAt(refreshedAt).
		SetUpdatedAt(refreshedAt).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("update segment refresh state failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("update segment refresh state failed")
	}
	return nil
}

// ListDueSegments 列出到期待刷新的启用分群：设了刷新间隔且距上次刷新已到期。
func (r *SegmentRepo) ListDueSegments(ctx context.Context, now time.Time) ([]*ent.Segment, error) {
	entities, err := r.entClient.Client().Segment.Query().
		Where(
			segment.EnabledEQ(true),
			segment.RefreshIntervalMinutesGT(0),
		).
		All(ctx)
	if err != nil {
		r.log.Errorf("list due segments failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("list due segments failed")
	}

	due := make([]*ent.Segment, 0, len(entities))
	for _, entity := range entities {
		if entity.LastRefreshedAt == nil {
			due = append(due, entity)
			continue
		}
		if now.Sub(*entity.LastRefreshedAt) >= time.Duration(entity.RefreshIntervalMinutes)*time.Minute {
			due = append(due, entity)
		}
	}
	return due, nil
}

func (r *SegmentRepo) Delete(ctx context.Context, req *ubaV1.DeleteSegmentRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Segment.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(segment.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete segment failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete segment failed")
	}

	return nil
}

func (r *SegmentRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Segment.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate segments table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

func u64p(v *int64) *uint64 {
	if v == nil {
		return nil
	}
	x := uint64(*v)
	return &x
}

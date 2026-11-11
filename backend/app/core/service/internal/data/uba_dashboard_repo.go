package data

import (
	"context"
	"encoding/json"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/encoding/protojson"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/dashboard"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// DashboardRepo 自定义看板数据仓库
type DashboardRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.Dashboard, ent.Dashboard]

	repository *entCrud.Repository[
		ent.DashboardQuery, ent.DashboardSelect,
		ent.DashboardCreate, ent.DashboardCreateBulk,
		ent.DashboardUpdate, ent.DashboardUpdateOne,
		ent.DashboardDelete,
		predicate.Dashboard,
		ubaV1.Dashboard, ent.Dashboard,
	]
}

func NewDashboardRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *DashboardRepo {
	repo := &DashboardRepo{
		log:       ctx.NewLoggerHelper("dashboard/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.Dashboard, ent.Dashboard](),
	}

	repo.init()

	return repo
}

func (r *DashboardRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.DashboardQuery, ent.DashboardSelect,
		ent.DashboardCreate, ent.DashboardCreateBulk,
		ent.DashboardUpdate, ent.DashboardUpdateOne,
		ent.DashboardDelete,
		predicate.Dashboard,
		ubaV1.Dashboard, ent.Dashboard,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(mapper.NewGenericTypeConverterPair(
		[]*ubaV1.DashboardWidget{}, []map[string]any{},
		widgetsToEntity, widgetsToDTO,
	))
}

// widgetsToDTO 把持久化的组件 JSON 数组还原为组件列表。
func widgetsToDTO(src []map[string]any) []*ubaV1.DashboardWidget {
	if len(src) == 0 {
		return nil
	}

	ret := make([]*ubaV1.DashboardWidget, 0, len(src))
	for _, item := range src {
		raw, err := json.Marshal(item)
		if err != nil {
			continue
		}
		widget := &ubaV1.DashboardWidget{}
		opts := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := opts.Unmarshal(raw, widget); err != nil {
			continue
		}
		ret = append(ret, widget)
	}
	return ret
}

// widgetsToEntity 把组件列表序列化为可持久化的 JSON 数组。
func widgetsToEntity(src []*ubaV1.DashboardWidget) []map[string]any {
	if len(src) == 0 {
		return nil
	}

	ret := make([]map[string]any, 0, len(src))
	for _, widget := range src {
		if widget == nil {
			continue
		}
		raw, err := protojson.Marshal(widget)
		if err != nil {
			continue
		}
		item := map[string]any{}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		ret = append(ret, item)
	}
	return ret
}

func (r *DashboardRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountDashboardResponse, error) {
	builder := r.entClient.Client().Dashboard.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query dashboard count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query dashboard count failed")
	}

	return &ubaV1.CountDashboardResponse{
		Count: int64(count),
	}, nil
}

func (r *DashboardRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListDashboardResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Dashboard.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListDashboardResponse{Total: 0, Items: nil}, nil
	}

	return &ubaV1.ListDashboardResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *DashboardRepo) IsExist(ctx context.Context, id int64) (bool, error) {
	exist, err := r.entClient.Client().Dashboard.Query().
		Where(dashboard.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *DashboardRepo) Get(ctx context.Context, req *ubaV1.GetDashboardRequest) (*ubaV1.Dashboard, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Dashboard.Query()

	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetDashboardRequest_Id:
		whereCond = append(whereCond, dashboard.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *DashboardRepo) Create(ctx context.Context, req *ubaV1.CreateDashboardRequest) (*ubaV1.Dashboard, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.newDashboardCreate(req.Data)

	var entity *ent.Dashboard
	var err error
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf("insert dashboard failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert dashboard failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *DashboardRepo) newDashboardCreate(w *ubaV1.Dashboard) *ent.DashboardCreate {
	builder := r.entClient.Client().Dashboard.Create().
		SetNillableTenantID(w.TenantId).
		SetNillableAppID(w.AppId).
		SetNillableName(w.Name).
		SetNillableDescription(w.Description).
		SetWidgets(widgetsToEntity(w.Widgets)).
		SetNillableEnabled(w.Enabled).
		SetNillableCreatedBy(w.CreatedBy).
		SetCreatedAt(time.Now())

	return builder
}

func (r *DashboardRepo) Update(ctx context.Context, req *ubaV1.UpdateDashboardRequest) (*ubaV1.Dashboard, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}

	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateDashboardRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.entClient.Client().Dashboard.UpdateOneID(req.GetId())
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.Dashboard) {
			builder.
				SetNillableAppID(req.Data.AppId).
				SetNillableName(req.Data.Name).
				SetNillableDescription(req.Data.Description).
				SetNillableEnabled(req.Data.Enabled).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.Widgets != nil {
				builder.SetWidgets(widgetsToEntity(req.Data.Widgets))
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(dashboard.FieldID, req.GetId()))
		},
	)

	return dto, err
}

func (r *DashboardRepo) Delete(ctx context.Context, req *ubaV1.DeleteDashboardRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}

	builder := r.entClient.Client().Dashboard.Delete()

	_, err := r.repository.Delete(ctx, builder, func(s *sql.Selector) {
		s.Where(sql.EQ(dashboard.FieldID, req.GetId()))
	})
	if err != nil {
		r.log.Errorf("delete dashboard failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete dashboard failed")
	}

	return nil
}

func (r *DashboardRepo) Truncate(ctx context.Context) error {
	if _, err := r.entClient.Client().Dashboard.Delete().Exec(ctx); err != nil {
		r.log.Errorf("failed to truncate dashboards table: %s", err.Error())
		return ubaV1.ErrorInternalServerError("truncate failed")
	}
	return nil
}

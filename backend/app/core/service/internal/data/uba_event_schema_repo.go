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
	"go-wind-uba/app/core/service/internal/data/ent/eventschema"
	"go-wind-uba/app/core/service/internal/data/ent/predicate"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type EventSchemaRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper

	mapper *mapper.CopierMapper[ubaV1.EventSchema, ent.EventSchema]

	repository *entCrud.Repository[
		ent.EventSchemaQuery, ent.EventSchemaSelect,
		ent.EventSchemaCreate, ent.EventSchemaCreateBulk,
		ent.EventSchemaUpdate, ent.EventSchemaUpdateOne,
		ent.EventSchemaDelete,
		predicate.EventSchema,
		ubaV1.EventSchema, ent.EventSchema,
	]
}

func NewEventSchemaRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *EventSchemaRepo {
	repo := &EventSchemaRepo{
		log:       ctx.NewLoggerHelper("event-schema/repo/core-service"),
		entClient: entClient,
		mapper:    mapper.NewCopierMapper[ubaV1.EventSchema, ent.EventSchema](),
	}

	repo.init()
	return repo
}

func (r *EventSchemaRepo) init() {
	r.repository = entCrud.NewRepository[
		ent.EventSchemaQuery, ent.EventSchemaSelect,
		ent.EventSchemaCreate, ent.EventSchemaCreateBulk,
		ent.EventSchemaUpdate, ent.EventSchemaUpdateOne,
		ent.EventSchemaDelete,
		predicate.EventSchema,
		ubaV1.EventSchema, ent.EventSchema,
	](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *EventSchemaRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountEventSchemaResponse, error) {
	builder := r.entClient.Client().EventSchema.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf("query event-schema count failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("query event-schema count failed")
	}

	return &ubaV1.CountEventSchemaResponse{Count: uint64(count)}, nil
}

func (r *EventSchemaRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListEventSchemaResponse, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().EventSchema.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &ubaV1.ListEventSchemaResponse{Total: 0, Items: nil}, nil
	}
	return &ubaV1.ListEventSchemaResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *EventSchemaRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.entClient.Client().EventSchema.Query().
		Where(eventschema.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf("query exist failed: %s", err.Error())
		return false, ubaV1.ErrorInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *EventSchemaRepo) Get(ctx context.Context, req *ubaV1.GetEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	builder := r.entClient.Client().EventSchema.Query()
	var whereCond []func(s *sql.Selector)
	switch req.QueryBy.(type) {
	default:
	case *ubaV1.GetEventSchemaRequest_Id:
		whereCond = append(whereCond, eventschema.IDEQ(uint32(req.GetId())))
	}
	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	return dto, err
}

func (r *EventSchemaRepo) Create(ctx context.Context, req *ubaV1.CreateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	status := req.Data.Status.String()
	builder := r.entClient.Client().EventSchema.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetEventName(req.Data.EventName).
		SetDisplayName(req.Data.DisplayName).
		SetNillableCategory(req.Data.Category).
		SetNillableDescription(req.Data.Description).
		SetStatus(status).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Properties != nil {
		builder.SetProperties(req.Data.Properties)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("insert event-schema failed: %s", err.Error())
		return nil, ubaV1.ErrorInternalServerError("insert event-schema failed")
	}
	return r.mapper.ToDTO(entity), nil
}

func (r *EventSchemaRepo) Update(ctx context.Context, req *ubaV1.UpdateEventSchemaRequest) (*ubaV1.EventSchema, error) {
	if req == nil || req.Data == nil {
		return nil, ubaV1.ErrorBadRequest("invalid parameter")
	}
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, uint32(req.GetId()))
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &ubaV1.CreateEventSchemaRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}
	builder := r.entClient.Client().EventSchema.UpdateOneID(uint32(req.GetId()))
	status := ""
	if req.Data.Status != nil {
		status = req.Data.Status.String()
	}
	dto, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *ubaV1.EventSchema) {
			upd := builder.
				SetNillableCategory(req.Data.Category).
				SetNillableDescription(req.Data.Description).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
			if req.Data.EventName != "" {
				upd.SetEventName(req.Data.EventName)
			}
			if req.Data.DisplayName != "" {
				upd.SetDisplayName(req.Data.DisplayName)
			}
			if status != "" {
				upd.SetStatus(status)
			}
			if req.Data.Properties != nil {
				upd.SetProperties(req.Data.Properties)
			}
		},
		func(s *sql.Selector) {
			s.Where(sql.EQ(eventschema.FieldID, uint32(req.GetId())))
		},
	)

	return dto, err
}

func (r *EventSchemaRepo) Delete(ctx context.Context, req *ubaV1.DeleteEventSchemaRequest) error {
	if req == nil {
		return ubaV1.ErrorBadRequest("invalid parameter")
	}
	if err := r.entClient.Client().EventSchema.DeleteOneID(uint32(req.GetId())).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ubaV1.ErrorNotFound("event-schema not found")
		}
		r.log.Errorf("delete event-schema failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete failed")
	}
	return nil
}

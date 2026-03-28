package doris

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	dorisCrud "github.com/tx7do/go-crud/doris"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data/doris/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type EventsFactRepo struct {
	db        *dorisCrud.Client
	log       *log.Helper
	tableName string

	mapper *mapper.CopierMapper[ubaV1.BehaviorEvent, schema.EventsFact]

	repository *dorisCrud.Repository[ubaV1.BehaviorEvent, schema.EventsFact]
}

func NewEventsFactRepo(
	ctx *bootstrap.Context,
	db *dorisCrud.Client,
) *EventsFactRepo {
	repo := &EventsFactRepo{
		log:       ctx.NewLoggerHelper("events-fact/doris/repo/core-service"),
		db:        db,
		tableName: "events_fact",
		mapper:    mapper.NewCopierMapper[ubaV1.BehaviorEvent, schema.EventsFact](),
	}

	repo.init()

	return repo
}

func (r *EventsFactRepo) init() {
	r.repository = dorisCrud.NewRepository[ubaV1.BehaviorEvent, schema.EventsFact](
		r.db,
		r.mapper,
		r.tableName,
		r.log,
	)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *EventsFactRepo) Create(ctx context.Context, dto *ubaV1.BehaviorEvent) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}

	entity := r.mapper.ToEntity(dto)

	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert events fact data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert events fact data")
	}

	return nil
}

func (r *EventsFactRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.BehaviorEvent) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}

	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}

	if _, err := r.db.BatchInsertStruct(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert events fact entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert events fact entities")
	}

	return nil
}

func (r *EventsFactRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListBehaviorEventResponse, error) {
	result, err := r.repository.ListWithPaging(ctx, req)
	if err != nil {
		r.log.Errorf("failed to list events fact data: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to list events fact data")
	}

	return &ubaV1.ListBehaviorEventResponse{
		Items: result.Items,
		Total: result.Total,
	}, nil
}

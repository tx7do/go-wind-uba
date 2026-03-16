package clickhouse

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data/clickhouse/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type EventsFactRepo struct {
	db        *clickhouseCrud.Client
	log       *log.Helper
	tableName string

	mapper             *mapper.CopierMapper[ubaV1.BehaviorEvent, schema.EventsFact]
	categoryConverter  *mapper.EnumTypeConverter[ubaV1.EventCategory, string]
	platformConverter  *mapper.EnumTypeConverter[ubaV1.Platform, string]
	opResultConverter  *mapper.EnumTypeConverter[ubaV1.OpResult, string]
	riskLevelConverter *mapper.EnumTypeConverter[ubaV1.RiskLevel, string]
}

func NewEventsFactRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *EventsFactRepo {
	repo := &EventsFactRepo{
		log:       ctx.NewLoggerHelper("events-fact/ck/repo/core-service"),
		db:        db,
		tableName: "events_fact",
		mapper:    mapper.NewCopierMapper[ubaV1.BehaviorEvent, schema.EventsFact](),
		categoryConverter: mapper.NewEnumTypeConverter[ubaV1.EventCategory, string](
			ubaV1.EventCategory_name, ubaV1.EventCategory_value,
		),
		platformConverter: mapper.NewEnumTypeConverter[ubaV1.Platform, string](
			ubaV1.Platform_name, ubaV1.Platform_value,
		),
		opResultConverter: mapper.NewEnumTypeConverter[ubaV1.OpResult, string](
			ubaV1.OpResult_name, ubaV1.OpResult_value,
		),
		riskLevelConverter: mapper.NewEnumTypeConverter[ubaV1.RiskLevel, string](
			ubaV1.RiskLevel_name, ubaV1.RiskLevel_value,
		),
	}

	repo.init()

	return repo
}

func (r *EventsFactRepo) init() {
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.categoryConverter.NewConverterPair())
	r.mapper.AppendConverters(r.platformConverter.NewConverterPair())
	r.mapper.AppendConverters(r.opResultConverter.NewConverterPair())
	r.mapper.AppendConverters(r.riskLevelConverter.NewConverterPair())
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

	if err := r.db.BatchInsert(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert events fact entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert events fact entities")
	}

	return nil
}

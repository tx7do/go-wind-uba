package doris

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	dorisCrud "github.com/tx7do/go-crud/doris"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data/doris/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventsRepo struct {
	db        *dorisCrud.Client
	log       *log.Helper
	tableName string
	mapper    *mapper.CopierMapper[ubaV1.RiskEvent, schema.RiskEvents]
}

func NewRiskEventsRepo(
	ctx *bootstrap.Context,
	db *dorisCrud.Client,
) *RiskEventsRepo {
	repo := &RiskEventsRepo{
		log:       ctx.NewLoggerHelper("risk-events/doris/repo/core-service"),
		db:        db,
		tableName: "risk_events",
		mapper:    mapper.NewCopierMapper[ubaV1.RiskEvent, schema.RiskEvents](),
	}
	repo.init()
	return repo
}

func (r *RiskEventsRepo) init() {
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *RiskEventsRepo) Create(ctx context.Context, dto *ubaV1.RiskEvent) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert risk events data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert risk events data")
	}
	return nil
}

func (r *RiskEventsRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.RiskEvent) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}
	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}
	if _, err := r.db.BatchInsertStruct(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert risk events entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert risk events entities")
	}
	return nil
}

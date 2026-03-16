package clickhouse

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data/clickhouse/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type RiskEventsRepo struct {
	db                 *clickhouseCrud.Client
	log                *log.Helper
	tableName          string
	mapper             *mapper.CopierMapper[ubaV1.RiskEvent, schema.RiskEvents]
	riskLevelConverter *mapper.EnumTypeConverter[ubaV1.RiskLevel, string]
	repository         *clickhouseCrud.Repository[ubaV1.RiskEvent, schema.RiskEvents]
}

func NewRiskEventsRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *RiskEventsRepo {
	repo := &RiskEventsRepo{
		log:       ctx.NewLoggerHelper("risk-events/ck/repo/core-service"),
		db:        db,
		tableName: "risk_events",
		mapper:    mapper.NewCopierMapper[ubaV1.RiskEvent, schema.RiskEvents](),
		riskLevelConverter: mapper.NewEnumTypeConverter[ubaV1.RiskLevel, string](
			ubaV1.RiskLevel_name, ubaV1.RiskLevel_value,
		),
	}
	repo.init()
	return repo
}

func (r *RiskEventsRepo) init() {
	r.repository = clickhouseCrud.NewRepository[ubaV1.RiskEvent, schema.RiskEvents](
		r.db,
		r.mapper,
		r.tableName,
		r.log,
	)
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.riskLevelConverter.NewConverterPair())
}

func (r *RiskEventsRepo) Create(ctx context.Context, dto *ubaV1.RiskEvent) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert risk event data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert risk event data")
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
	if err := r.db.BatchInsert(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert risk event entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert risk event entities")
	}
	return nil
}

func (r *RiskEventsRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListRiskEventResponse, error) {
	result, err := r.repository.ListWithPaging(ctx, req)
	if err != nil {
		r.log.Errorf("failed to list risk events data: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to list risk events data")
	}
	return &ubaV1.ListRiskEventResponse{
		Items: result.Items,
		Total: result.Total,
	}, nil
}

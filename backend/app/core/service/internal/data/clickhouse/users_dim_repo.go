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

type UsersDimRepo struct {
	db        *clickhouseCrud.Client
	log       *log.Helper
	tableName string
	mapper    *mapper.CopierMapper[ubaV1.UserBehaviorProfile, schema.UsersDim]
}

func NewUsersDimRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *UsersDimRepo {
	repo := &UsersDimRepo{
		log:       ctx.NewLoggerHelper("users-dim/ck/repo/core-service"),
		db:        db,
		tableName: "users_dim",
		mapper:    mapper.NewCopierMapper[ubaV1.UserBehaviorProfile, schema.UsersDim](),
	}
	repo.init()
	return repo
}

func (r *UsersDimRepo) init() {
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *UsersDimRepo) Create(ctx context.Context, dto *ubaV1.UserBehaviorProfile) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert users dim data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert users dim data")
	}
	return nil
}

func (r *UsersDimRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.UserBehaviorProfile) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}
	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}
	if err := r.db.BatchInsert(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert users dim entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert users dim entities")
	}
	return nil
}

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

type ObjectsDimRepo struct {
	db         *clickhouseCrud.Client
	log        *log.Helper
	tableName  string
	mapper     *mapper.CopierMapper[ubaV1.ObjectDim, schema.ObjectsDim]
	repository *clickhouseCrud.Repository[ubaV1.ObjectDim, schema.ObjectsDim]
}

func NewObjectsDimRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *ObjectsDimRepo {
	repo := &ObjectsDimRepo{
		log:       ctx.NewLoggerHelper("objects-dim/ck/repo/core-service"),
		db:        db,
		tableName: "objects_dim",
		mapper:    mapper.NewCopierMapper[ubaV1.ObjectDim, schema.ObjectsDim](),
	}
	repo.init()
	return repo
}

func (r *ObjectsDimRepo) init() {
	r.repository = clickhouseCrud.NewRepository[ubaV1.ObjectDim, schema.ObjectsDim](
		r.db,
		r.mapper,
		r.tableName,
		r.log,
	)
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *ObjectsDimRepo) Create(ctx context.Context, dto *ubaV1.ObjectDim) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert objects dim data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert objects dim data")
	}
	return nil
}

func (r *ObjectsDimRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.ObjectDim) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}
	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}
	if err := r.db.BatchInsert(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert objects dim entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert objects dim entities")
	}
	return nil
}

func (r *ObjectsDimRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListObjectDimResponse, error) {
	result, err := r.repository.ListWithPaging(ctx, req)
	if err != nil {
		r.log.Errorf("failed to list objects dim data: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to list objects dim data")
	}
	return &ubaV1.ListObjectDimResponse{
		Items: result.Items,
		Total: result.Total,
	}, nil
}

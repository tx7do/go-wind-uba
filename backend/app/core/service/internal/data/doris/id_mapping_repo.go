package doris

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	dorisCrud "github.com/tx7do/go-crud/doris"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/doris/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type IDMappingRepo struct {
	db        *dorisCrud.Client
	log       *log.Helper
	tableName string
	mapper    *mapper.CopierMapper[ubaV1.IDMapping, schema.IDMapping]

	repository *dorisCrud.Repository[ubaV1.IDMapping, schema.IDMapping]
}

func NewIDMappingRepo(
	ctx *bootstrap.Context,
	db *dorisCrud.Client,
) *IDMappingRepo {
	repo := &IDMappingRepo{
		log:       ctx.NewLoggerHelper("id-mapping/doris/repo/core-service"),
		db:        db,
		tableName: "id_mapping",
		mapper:    mapper.NewCopierMapper[ubaV1.IDMapping, schema.IDMapping](),
	}
	repo.init()
	return repo
}

func (r *IDMappingRepo) init() {
	r.repository = dorisCrud.NewRepository[ubaV1.IDMapping, schema.IDMapping](
		r.db,
		r.mapper,
		r.tableName,
		r.log,
	)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *IDMappingRepo) Create(ctx context.Context, dto *ubaV1.IDMapping) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert id mapping data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert id mapping data")
	}
	return nil
}

func (r *IDMappingRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.IDMapping) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}
	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}
	if _, err := r.db.BatchInsertStruct(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert id mapping entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert id mapping entities")
	}
	return nil
}

// ListUserDevicePairs 从 events_fact 聚合时间窗内同时出现过的 登录用户↔设备 对，
// 供 ID-Mapping 引擎构建跨端身份关联。
func (r *IDMappingRepo) ListUserDevicePairs(ctx context.Context, startMs, endMs int64, limit int) ([]data.IdentityPair, error) {
	if limit <= 0 || limit > 50000 {
		limit = 20000
	}

	query := `
SELECT tenant_id, user_id, MAX(device_id) AS device_id
FROM events_fact
WHERE user_id > 0
  AND device_id IS NOT NULL AND device_id != ''
  AND event_time >= ? AND event_time < ?
GROUP BY tenant_id, user_id, device_id
LIMIT ?`

	rows, err := r.db.DB().QueryxContext(ctx, query, time.UnixMilli(startMs), time.UnixMilli(endMs), limit)
	if err != nil {
		r.log.Errorf("failed to query user device pairs: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to query user device pairs")
	}
	defer rows.Close()

	pairs := make([]data.IdentityPair, 0, 1024)
	for rows.Next() {
		var (
			tenantID int64
			userID   int64
			deviceID string
		)
		if err := rows.Scan(&tenantID, &userID, &deviceID); err != nil {
			r.log.Errorf("failed to scan user device pair: %v", err)
			continue
		}
		pairs = append(pairs, data.IdentityPair{
			TenantID: tenantID,
			UserID:   userID,
			DeviceID: deviceID,
		})
	}
	if err := rows.Err(); err != nil {
		r.log.Errorf("iterate user device pairs failed: %v", err)
		return nil, ubaV1.ErrorInternalServerError("iterate user device pairs failed")
	}

	return pairs, nil
}

func (r *IDMappingRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListIDMappingResponse, error) {
	result, err := r.repository.ListWithPaging(ctx, req)
	if err != nil {
		r.log.Errorf("failed to list id mapping data: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to list id mapping data")
	}
	return &ubaV1.ListIDMappingResponse{
		Items: result.Items,
		Total: int64(result.Total),
	}, nil
}

package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/clickhouse/schema"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

type UserTagsRepo struct {
	db         *clickhouseCrud.Client
	log        *log.Helper
	tableName  string
	mapper     *mapper.CopierMapper[ubaV1.UserTag, schema.UserTags]
	repository *clickhouseCrud.Repository[ubaV1.UserTag, schema.UserTags]
}

func NewUserTagsRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *UserTagsRepo {
	repo := &UserTagsRepo{
		log:       ctx.NewLoggerHelper("user-tags/ck/repo/core-service"),
		db:        db,
		tableName: "user_tags",
		mapper:    mapper.NewCopierMapper[ubaV1.UserTag, schema.UserTags](),
	}
	repo.init()
	return repo
}

func (r *UserTagsRepo) init() {
	r.repository = clickhouseCrud.NewRepository[ubaV1.UserTag, schema.UserTags](
		r.db,
		r.mapper,
		r.tableName,
		r.log,
	)
	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// forbiddenSQLFragments 动态标签谓词里不允许出现的片段：标签规则虽然由租户管理员
// 维护，执行器也要把"多语句/注释/危险的系统表与函数"挡在门外。
// 与 doris 版对应，黑名单按 ClickHouse 适配（system. 系统库、url/file 表函数）。
var forbiddenSQLFragments = []string{
	";", "--", "/*", "*/", "#",
	"insert ", "update ", "delete ", "drop ", "alter ", "create ",
	"grant ", "revoke ", "truncate ", "set ", "system.", "system ",
	"url(", "file(", "remote(", "mysql(", "postgresql(",
}

// SelectUsersByPredicate 把标签规则表达式当作 users_dim 上的布尔谓词执行，
// 返回命中的 (tenant_id, user_id)。表达式在服务端拼接前做黑名单校验。
// 与 doris.UserTagsRepo 对应。
func (r *UserTagsRepo) SelectUsersByPredicate(ctx context.Context, predicate string, limit int) ([]data.TagUserHit, error) {
	if predicate == "" {
		return nil, ubaV1.ErrorBadRequest("tag rule predicate is empty")
	}
	if limit <= 0 || limit > 50000 {
		limit = 50000
	}

	lower := strings.ToLower(predicate)
	for _, frag := range forbiddenSQLFragments {
		if strings.Contains(lower, frag) {
			return nil, ubaV1.ErrorBadRequest("tag rule predicate contains forbidden fragment: %q", frag)
		}
	}

	query := fmt.Sprintf(
		"SELECT tenant_id, user_id FROM users_dim FINAL WHERE (%s) LIMIT %d",
		predicate, limit,
	)

	type hitRow struct {
		TenantID int64 `ch:"tenant_id"`
		UserID   int64 `ch:"user_id"`
	}
	var rows []hitRow
	if err := r.db.Select(ctx, &rows, query); err != nil {
		r.log.Errorf("failed to select users by tag predicate: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to select users by tag predicate")
	}

	hits := make([]data.TagUserHit, 0, len(rows))
	for _, row := range rows {
		hits = append(hits, data.TagUserHit{TenantID: row.TenantID, UserID: row.UserID})
	}
	return hits, nil
}

func (r *UserTagsRepo) Create(ctx context.Context, dto *ubaV1.UserTag) error {
	if dto == nil {
		return ubaV1.ErrorBadRequest("request data is required")
	}
	entity := r.mapper.ToEntity(dto)
	if err := r.db.Insert(ctx, r.tableName, entity); err != nil {
		r.log.Errorf("failed to insert user tags data: %v", err)
		return ubaV1.ErrorInternalServerError("failed to insert user tags data")
	}
	return nil
}

func (r *UserTagsRepo) BatchCreate(ctx context.Context, dtos []*ubaV1.UserTag) error {
	if len(dtos) == 0 {
		return ubaV1.ErrorBadRequest("request dtos is required")
	}
	var entities []any
	for _, dto := range dtos {
		entity := r.mapper.ToEntity(dto)
		entities = append(entities, entity)
	}
	if err := r.db.BatchInsert(ctx, r.tableName, entities); err != nil {
		r.log.Errorf("failed to batch insert user tags entities: %v", err)
		return ubaV1.ErrorInternalServerError("failed to batch insert user tags entities")
	}
	return nil
}

func (r *UserTagsRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListUserTagResponse, error) {
	result, err := r.repository.ListWithPaging(ctx, req)
	if err != nil {
		r.log.Errorf("failed to list user tags data: %v", err)
		return nil, ubaV1.ErrorInternalServerError("failed to list user tags data")
	}
	return &ubaV1.ListUserTagResponse{
		Items: result.Items,
		Total: int64(result.Total),
	}, nil
}

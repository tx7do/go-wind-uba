package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/segmentuser"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// SegmentUserRepo 用户分群成员数据仓库
type SegmentUserRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewSegmentUserRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SegmentUserRepo {
	return &SegmentUserRepo{
		log:       ctx.NewLoggerHelper("segmentuser/repo/core-service"),
		entClient: entClient,
	}
}

// ReplaceUsers 全量替换分群成员（刷新时调用：先清空再批量写入）。
func (r *SegmentUserRepo) ReplaceUsers(ctx context.Context, segmentID int64, userIDs []int64) error {
	if _, err := r.entClient.Client().SegmentUser.Delete().
		Where(segmentuser.SegmentIDEQ(segmentID)).
		Exec(ctx); err != nil {
		r.log.Errorf("clear segment users failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("clear segment users failed")
	}

	if len(userIDs) == 0 {
		return nil
	}

	now := time.Now()
	bulk := make([]*ent.SegmentUserCreate, 0, len(userIDs))
	for _, userID := range userIDs {
		bulk = append(bulk, r.entClient.Client().SegmentUser.Create().
			SetSegmentID(segmentID).
			SetUserID(userID).
			SetCreatedAt(now))
	}

	if err := r.entClient.Client().SegmentUser.CreateBulk(bulk...).Exec(ctx); err != nil {
		r.log.Errorf("insert segment users failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("insert segment users failed")
	}

	return nil
}

// ListBySegment 分页查询分群成员。
func (r *SegmentUserRepo) ListBySegment(ctx context.Context, segmentID int64, page, pageSize int) ([]*ent.SegmentUser, int, error) {
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}

	query := r.entClient.Client().SegmentUser.Query().
		Where(segmentuser.SegmentIDEQ(segmentID))

	total, err := query.Count(ctx)
	if err != nil {
		r.log.Errorf("query segment user count failed: %s", err.Error())
		return nil, 0, ubaV1.ErrorInternalServerError("query segment user count failed")
	}

	entities, err := query.
		Order(ent.Asc(segmentuser.FieldUserID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		r.log.Errorf("query segment users failed: %s", err.Error())
		return nil, 0, ubaV1.ErrorInternalServerError("query segment users failed")
	}

	return entities, total, nil
}

// DeleteBySegment 删除分群时清理成员。
func (r *SegmentUserRepo) DeleteBySegment(ctx context.Context, segmentID int64) error {
	if _, err := r.entClient.Client().SegmentUser.Delete().
		Where(segmentuser.SegmentIDEQ(segmentID)).
		Exec(ctx); err != nil {
		r.log.Errorf("delete segment users failed: %s", err.Error())
		return ubaV1.ErrorInternalServerError("delete segment users failed")
	}
	return nil
}

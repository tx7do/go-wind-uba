package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"go-wind-uba/app/core/service/internal/data/ent"
	"go-wind-uba/app/core/service/internal/data/ent/replaychunk"
)

// ReplayRepo 会话回放录制分块数据仓库
type ReplayRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewReplayRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ReplayRepo {
	return &ReplayRepo{
		log:       ctx.NewLoggerHelper("replay/repo/core-service"),
		entClient: entClient,
	}
}

// UpsertChunk 写入一个分块：同会话同序号重复上传按最新覆盖（幂等）。
func (r *ReplayRepo) UpsertChunk(
	ctx context.Context,
	tenantID int64,
	appID, sessionID string,
	deviceID *string,
	userID int64,
	pageURL *string,
	chunkSeq int64,
	eventsJSON string,
	eventCount, durationMs int64,
) (int64, error) {
	now := time.Now()

	builder := r.entClient.Client().ReplayChunk.Create().
		SetNillableTenantID(&tenantID).
		SetNillableAppID(&appID).
		SetSessionID(sessionID).
		SetNillableDeviceID(deviceID).
		SetUserID(userID).
		SetNillablePageURL(pageURL).
		SetChunkSeq(uint32(chunkSeq)).
		SetEventsJSON(eventsJSON).
		SetEventCount(uint32(eventCount)).
		SetDurationMs(uint32(durationMs)).
		SetCreatedAt(now)

	err := builder.
		OnConflictColumns(replaychunk.FieldSessionID, replaychunk.FieldChunkSeq).
		Update(func(u *ent.ReplayChunkUpsert) {
			u.SetEventsJSON(eventsJSON).
				SetEventCount(uint32(eventCount)).
				SetDurationMs(uint32(durationMs)).
				SetUpdatedAt(now)
		}).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("upsert replay chunk failed: %s", err.Error())
		return 0, ubaV1.ErrorReplayInternalError("upsert replay chunk failed")
	}

	// 取回该分块 ID（唯一键 session+seq 定位）
	entity, err := r.entClient.Client().ReplayChunk.Query().
		Where(
			replaychunk.SessionIDEQ(sessionID),
			replaychunk.ChunkSeqEQ(uint32(chunkSeq)),
		).
		Only(ctx)
	if err != nil {
		return 0, nil
	}
	return entity.ID, nil
}

// ReplaySessionSummaryRow 会话聚合扫描行。
type ReplaySessionSummaryRow struct {
	SessionID   string     `json:"sessionId"`
	TenantID    *int64     `json:"tenantId,omitempty"`
	AppID       *string    `json:"appId,omitempty"`
	DeviceID    *string    `json:"deviceId,omitempty"`
	UserID      int64      `json:"userId"`
	PageURL     *string    `json:"pageUrl,omitempty"`
	ChunkCount  int        `json:"chunkCount"`
	EventCount  int64      `json:"eventCount"`
	FirstSeenAt *time.Time `json:"firstSeenAt,omitempty"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
}

// ListSessions 按会话聚合（最近优先）；sessionFilter 包含匹配、userFilter 可选。
func (r *ReplayRepo) ListSessions(ctx context.Context, sessionFilter string, userFilter int64) ([]ReplaySessionSummaryRow, error) {
	q := `
SELECT session_id,
       MAX(tenant_id) AS tenant_id,
       MAX(app_id)    AS app_id,
       MAX(device_id) AS device_id,
       MAX(user_id)   AS user_id,
       MIN(COALESCE(page_url, '')) AS page_url,
       COUNT(*)       AS chunk_count,
       SUM(event_count) AS event_count,
       MIN(created_at) AS first_seen_at,
       MAX(created_at) AS last_seen_at
FROM uba_replay_chunks
WHERE 1=1`
	var args []any
	if sessionFilter != "" {
		q += " AND session_id LIKE ?"
		args = append(args, "%"+sessionFilter+"%")
	}
	if userFilter != 0 {
		q += " AND user_id = ?"
		args = append(args, userFilter)
	}
	q += " GROUP BY session_id ORDER BY MAX(created_at) DESC LIMIT 200"

	rows := make([]ReplaySessionSummaryRow, 0, 64)
	db := r.entClient.DB()
	if db == nil {
		return rows, nil
	}
	resultRows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		r.log.Errorf("list replay sessions failed: %s", err.Error())
		return nil, ubaV1.ErrorReplayInternalError("list replay sessions failed")
	}
	defer resultRows.Close()

	for resultRows.Next() {
		var row ReplaySessionSummaryRow
		if scanErr := resultRows.Scan(
			&row.SessionID, &row.TenantID, &row.AppID, &row.DeviceID,
			&row.UserID, &row.PageURL, &row.ChunkCount,
			&row.EventCount, &row.FirstSeenAt, &row.LastSeenAt,
		); scanErr != nil {
			r.log.Errorf("scan replay session summary failed: %s", scanErr.Error())
			continue
		}
		rows = append(rows, row)
	}
	if err := resultRows.Err(); err != nil {
		r.log.Errorf("iterate replay sessions failed: %s", err.Error())
		return nil, ubaV1.ErrorReplayInternalError("iterate replay sessions failed")
	}
	return rows, nil
}

// GetSessionChunks 取会话全部分块（按 chunk_seq 升序）。
func (r *ReplayRepo) GetSessionChunks(ctx context.Context, sessionID string) ([]*ent.ReplayChunk, error) {
	return r.entClient.Client().ReplayChunk.Query().
		Where(replaychunk.SessionIDEQ(sessionID)).
		Order(replaychunk.ByChunkSeq()).
		All(ctx)
}

// DeleteSession 删除整个会话的录制。
func (r *ReplayRepo) DeleteSession(ctx context.Context, sessionID string) (int, error) {
	count, err := r.entClient.Client().ReplayChunk.Delete().
		Where(replaychunk.SessionIDEQ(sessionID)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete replay session failed: %s", err.Error())
		return 0, ubaV1.ErrorInternalServerError("delete replay session failed")
	}
	return count, nil
}

// DeleteOlderThan 保留期清扫：删除早于 cutoff 的全部分块，返回删除行数。
func (r *ReplayRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	count, err := r.entClient.Client().ReplayChunk.Delete().
		Where(replaychunk.CreatedAtLT(cutoff)).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("replay retention sweep failed: %s", err.Error())
		return 0, err
	}
	return int64(count), nil
}

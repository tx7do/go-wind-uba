package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/pkg/task"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	appViewer "go-wind-uba/pkg/entgo/viewer"
)

const (
	// replayChunkMaxBytes 单块 events_json 的服务端上限（约 8MB）。
	replayChunkMaxBytes = 8 << 20
	// replayRetentionEnv 保留天数环境变量。
	replayRetentionEnv = "REPLAY_RETENTION_DAYS"
	// replayRetentionDefault 缺省保留 30 天。
	replayRetentionDefault = 30
)

// ReplayService 会话回放：rrweb 录制分块的上传、会话查询与保留期清扫。
type ReplayService struct {
	ubaV1.UnimplementedReplayServiceServer

	log *log.Helper

	replayRepo *data.ReplayRepo

	// taskEnqueuer 保留期清扫任务入队（asynq 未配置时不上自动清扫）。
	taskEnqueuer TaskEnqueuer
}

func NewReplayService(
	ctx *bootstrap.Context,
	replayRepo *data.ReplayRepo,
) *ReplayService {
	return &ReplayService{
		log:        ctx.NewLoggerHelper("replay/service/core-service"),
		replayRepo: replayRepo,
	}
}

// RegisterTaskEnqueuer 注入 asynq 入队能力。
func (s *ReplayService) RegisterTaskEnqueuer(enqueuer TaskEnqueuer) {
	s.taskEnqueuer = enqueuer
}

// UploadChunk 上传一个录制分块（collector 转发）。
func (s *ReplayService) UploadChunk(ctx context.Context, req *ubaV1.UploadReplayChunkRequest) (*ubaV1.UploadReplayChunkResponse, error) {
	if req == nil || req.GetSessionId() == "" {
		return nil, ubaV1.ErrorReplayBadRequest("session_id is required")
	}
	if len(req.GetEventsJson()) > replayChunkMaxBytes {
		return nil, ubaV1.ErrorReplayPayloadTooLarge("events_json exceeds %d bytes", replayChunkMaxBytes)
	}

	id, err := s.replayRepo.UpsertChunk(
		ctx,
		req.GetTenantId(),
		req.GetAppId(),
		req.GetSessionId(),
		req.DeviceId,
		req.GetUserId(),
		req.PageUrl,
		int64(req.GetChunkSeq()),
		req.GetEventsJson(),
		int64(req.GetEventCount()),
		int64(req.GetDurationMs()),
	)
	if err != nil {
		return nil, err
	}

	return &ubaV1.UploadReplayChunkResponse{Id: id}, nil
}

// ListSessions 会话列表（按 session 聚合，最近优先）。
func (s *ReplayService) ListSessions(ctx context.Context, req *ubaV1.ListReplaySessionsRequest) (*ubaV1.ListReplaySessionsResponse, error) {
	rows, err := s.replayRepo.ListSessions(
		ctx,
		req.GetSessionId(),
		req.GetUserId(),
	)
	if err != nil {
		return nil, err
	}

	items := make([]*ubaV1.ReplaySessionSummary, 0, len(rows))
	for _, row := range rows {
		item := &ubaV1.ReplaySessionSummary{
			SessionId:  row.SessionID,
			TenantId:   row.TenantID,
			AppId:      row.AppID,
			DeviceId:   row.DeviceID,
			UserId:     &row.UserID,
			PageUrl:    row.PageURL,
			ChunkCount: uint32(int64(row.ChunkCount)),
			EventCount: uint64(int64(row.EventCount)),
		}
		if row.FirstSeenAt != nil {
			item.FirstSeenAt = timestamppb.New(*row.FirstSeenAt)
		}
		if row.LastSeenAt != nil {
			item.LastSeenAt = timestamppb.New(*row.LastSeenAt)
		}
		items = append(items, item)
	}

	return &ubaV1.ListReplaySessionsResponse{
		Items: items,
		Total: int64(len(items)),
	}, nil
}

// GetSessionEvents 会话详情：全部分块按 chunk_seq 拼接成完整事件数组。
func (s *ReplayService) GetSessionEvents(ctx context.Context, req *ubaV1.GetReplaySessionRequest) (*ubaV1.GetReplaySessionResponse, error) {
	if req == nil || req.GetSessionId() == "" {
		return nil, ubaV1.ErrorReplayBadRequest("session_id is required")
	}

	chunks, err := s.replayRepo.GetSessionChunks(ctx, req.GetSessionId())
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, ubaV1.ErrorReplaySessionNotFound("replay session [%s] not found", req.GetSessionId())
	}

	var totalEvents int64
	var sb strings.Builder
	sb.WriteByte('[')
	for i, chunk := range chunks {
		body := strings.TrimSpace(chunk.EventsJSON)
		// 去掉块级 [] 边界后拼接
		body = strings.TrimPrefix(body, "[")
		body = strings.TrimSuffix(body, "]")
		if body == "" {
			continue
		}
		if i > 0 && sb.Len() > 1 {
			sb.WriteByte(',')
		}
		sb.WriteString(body)
		totalEvents += int64(chunk.EventCount)
	}
	sb.WriteByte(']')

	return &ubaV1.GetReplaySessionResponse{
		SessionId:  req.GetSessionId(),
		EventsJson: sb.String(),
		ChunkCount: uint32(int64(len(chunks))),
		EventCount: uint64(totalEvents),
	}, nil
}

// DeleteSession 删除整个会话的录制。
func (s *ReplayService) DeleteSession(ctx context.Context, req *ubaV1.DeleteReplaySessionRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetSessionId() == "" {
		return nil, ubaV1.ErrorReplayBadRequest("session_id is required")
	}

	if _, err := s.replayRepo.DeleteSession(ctx, req.GetSessionId()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ──────────── 保留期清扫 ────────────

// retentionDays 保留天数（环境变量 REPLAY_RETENTION_DAYS，缺省 30）。
func (s *ReplayService) retentionDays() int {
	raw := strings.TrimSpace(os.Getenv(replayRetentionEnv))
	if raw == "" {
		return replayRetentionDefault
	}
	if v, err := strconv.Atoi(raw); err == nil && v > 0 {
		return v
	}
	return replayRetentionDefault
}

// AsyncReplayRetentionSweep asynq 清扫消费者：删除超过保留期的录制分块。
func (s *ReplayService) AsyncReplayRetentionSweep(_ string, _ *task.ReplayRetentionSweepTaskData) error {
	ctx := appViewer.NewSystemViewerContext(context.Background())
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays())
	count, err := s.replayRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if count > 0 {
		s.log.Infof("replay retention sweep removed %d chunks older than %s", count, cutoff.Format(time.RFC3339))
	}
	return nil
}

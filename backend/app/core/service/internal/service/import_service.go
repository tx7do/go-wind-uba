package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-wind-uba/app/core/service/internal/data"
	clickhouseData "go-wind-uba/app/core/service/internal/data/clickhouse"
	dorisData "go-wind-uba/app/core/service/internal/data/doris"
	appViewer "go-wind-uba/pkg/entgo/viewer"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// importBatchSize 每批写入 OLAP 的事件数。
	importBatchSize = 500
	// importMaxPayloadBytes events_json 的服务端上限（50MB）。
	importMaxPayloadBytes = 50 << 20
)

// eventsFactWriter 导入服务所需的事件写入能力（Doris/ClickHouse 双实现共用签名）。
type eventsFactWriter interface {
	Create(ctx context.Context, event *ubaV1.BehaviorEvent) error
}

// ImportService 历史数据导入：JSON 事件批量写入 OLAP events_fact。
// 走 ent 系统上下文（绕过租户隐私层），分批同步写入（单 goroutine 顺序保证）。
type ImportService struct {
	ubaV1.UnimplementedImportServiceServer

	log *log.Helper

	importRepo *data.ImportJobRepo
	doris      *dorisData.EventsFactRepo
	ch         *clickhouseData.EventsFactRepo

	mu        sync.Mutex
	cancelled map[int64]bool
}

func NewImportService(
	ctx *bootstrap.Context,
	importRepo *data.ImportJobRepo,
	dorisRepo *dorisData.EventsFactRepo,
	chRepo *clickhouseData.EventsFactRepo,
) *ImportService {
	return &ImportService{
		log:        ctx.NewLoggerHelper("import/service/core-service"),
		importRepo: importRepo,
		doris:      dorisRepo,
		ch:         chRepo,
		cancelled:  make(map[int64]bool),
	}
}

// eventsFactImpl 按编译期引擎开关路由（与 alertEvaluator/reportEngine 同一约定）。
func (s *ImportService) eventsFactImpl() eventsFactWriter {
	if data.UseClickHouse && s.ch != nil {
		return s.ch
	}
	return s.doris
}

func (s *ImportService) List(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.ListImportJobResponse, error) {
	return s.importRepo.List(ctx, req)
}

func (s *ImportService) Count(ctx context.Context, req *paginationV1.PagingRequest) (*ubaV1.CountImportJobResponse, error) {
	return s.importRepo.Count(ctx, req)
}

func (s *ImportService) Get(ctx context.Context, req *ubaV1.GetImportJobRequest) (*ubaV1.ImportJob, error) {
	return s.importRepo.Get(ctx, req)
}

// Create 创建导入任务并同步执行。
func (s *ImportService) Create(ctx context.Context, req *ubaV1.CreateImportJobRequest) (*ubaV1.ImportJob, error) {
	if req == nil || req.GetEventsJson() == "" {
		return nil, ubaV1.ErrorImportBadRequest("events_json is required")
	}
	if len(req.GetEventsJson()) > importMaxPayloadBytes {
		return nil, ubaV1.ErrorImportPayloadTooLarge("events_json exceeds %d bytes", importMaxPayloadBytes)
	}

	// 预解析校验 JSON 合法性
	var rawEvents []map[string]any
	if err := json.Unmarshal([]byte(req.GetEventsJson()), &rawEvents); err != nil {
		return nil, ubaV1.ErrorImportBadRequest("invalid events_json: %v", err)
	}
	if len(rawEvents) == 0 {
		return nil, ubaV1.ErrorImportBadRequest("events_json contains no events")
	}

	operatorID := operatorIDFromCtx(ctx)
	tenantID := tenantIDFromCtx(ctx)

	name := req.GetName()
	if name == "" {
		name = fmt.Sprintf("import_%d", time.Now().Unix())
	}
	appID := req.GetAppId()

	entity, err := s.importRepo.Create(ctx, name, &appID, req.GetEventsJson(), int64(len(rawEvents)), tenantID, operatorID)
	if err != nil {
		return nil, err
	}

	// 同步执行导入
	go s.executeImport(appViewer.NewSystemViewerContext(context.Background()), entity.ID)

	return s.importRepo.ToDTO(entity), nil
}

// Cancel 取消排队中/执行中的导入。
func (s *ImportService) Cancel(ctx context.Context, req *ubaV1.CancelImportJobRequest) (*ubaV1.ImportJob, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorImportBadRequest("id is required")
	}

	s.mu.Lock()
	s.cancelled[req.GetId()] = true
	s.mu.Unlock()

	if err := s.importRepo.UpdateStatus(ctx, req.GetId(), "CANCELLED"); err != nil {
		return nil, err
	}

	dto, err := s.importRepo.Get(ctx, &ubaV1.GetImportJobRequest{Id: req.GetId()})
	return dto, err
}

func (s *ImportService) Delete(ctx context.Context, req *ubaV1.DeleteImportJobRequest) (*emptypb.Empty, error) {
	if req == nil || req.GetId() == 0 {
		return nil, ubaV1.ErrorImportBadRequest("id is required")
	}

	if err := s.importRepo.Delete(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// executeImport 解析 events_json → 分批构造行为事件 → 写入 Doris events_fact。
func (s *ImportService) executeImport(ctx context.Context, jobID int64) {
	s.mu.Lock()
	if s.cancelled[jobID] {
		delete(s.cancelled, jobID)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	entity, err := s.importRepo.GetEntityWithEvents(ctx, jobID)
	if err != nil {
		s.log.Errorf("import job [%d] read entity failed: %s", jobID, err.Error())
		return
	}
	if entity.Status == "CANCELLED" || entity.Status == "COMPLETED" {
		return
	}

	startedAt := time.Now()
	_ = s.importRepo.UpdateStatus(ctx, jobID, "RUNNING")

	var rawEvents []map[string]any
	if err = json.Unmarshal([]byte(entity.EventsJSON), &rawEvents); err != nil {
		s.log.Errorf("import job [%d] parse events_json failed: %s", jobID, err.Error())
		_ = s.importRepo.UpdateProgress(ctx, jobID, 0, 0, "FAILED",
			[]string{"JSON parse error: " + err.Error()}, startedAt, time.Now())
		return
	}

	total := int64(len(rawEvents))
	var imported, failed int64
	var errMsgs []string

	tenantID := int64(0)
	if entity.TenantID != nil {
		tenantID = *entity.TenantID
	}
	appID := derefStr(entity.AppID)

	// 分批处理
	for offset := 0; offset < len(rawEvents); offset += importBatchSize {
		// 取消检查
		s.mu.Lock()
		cancelled := s.cancelled[jobID]
		s.mu.Unlock()
		if cancelled {
			break
		}

		end := offset + importBatchSize
		if end > len(rawEvents) {
			end = len(rawEvents)
		}
		batch := rawEvents[offset:end]

		batchImported, batchFailed, batchErrs := s.importBatch(ctx, batch, tenantID, appID)
		imported += batchImported
		failed += batchFailed
		errMsgs = append(errMsgs, batchErrs...)
	}

	now := time.Now()
	status := "COMPLETED"
	if failed > 0 && imported == 0 {
		status = "FAILED"
	} else if failed > 0 {
		status = "PARTIAL"
	}

	if err = s.importRepo.UpdateProgress(ctx, jobID, imported, failed, status, errMsgs, startedAt, now); err != nil {
		s.log.Errorf("import job [%d] final update failed: %s", jobID, err.Error())
	}

	// 导入完成后清空原始数据释放存储
	s.importRepo.ClearEventsJSON(ctx, jobID)

	s.log.Infof("import job [%d] done: total=%d imported=%d failed=%d", jobID, total, imported, failed)
}

// importBatch 批量写入 Doris。
func (s *ImportService) importBatch(ctx context.Context, batch []map[string]any, tenantID int64, appID string) (imported, failed int64, errMsgs []string) {
	for _, raw := range batch {
		if err := s.importSingleEvent(ctx, raw, tenantID, appID); err != nil {
			failed++
			eventID, _ := raw["eventId"].(string)
			errMsgs = append(errMsgs, fmt.Sprintf("event [%s]: %v", eventID, err))
			if len(errMsgs) > 20 {
				errMsgs = errMsgs[:20]
			}
			continue
		}
		imported++
	}
	return
}

// importSingleEvent 把单条历史事件转换为 BehaviorEvent 并写入 Doris。
func (s *ImportService) importSingleEvent(ctx context.Context, raw map[string]any, tenantID int64, appID string) error {
	eventID, _ := raw["eventId"].(string)
	eventName, _ := raw["eventName"].(string)
	if eventName == "" {
		return fmt.Errorf("eventName is required")
	}

	eventTimeStr, _ := raw["eventTime"].(string)
	var eventTime *time.Time
	if eventTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, eventTimeStr); err == nil {
			eventTime = &t
		}
	}

	behaviorData, _ := raw["behavior"].(map[string]any)
	propsData, _ := raw["properties"].(map[string]string)

	// 构造最小可用的 BehaviorEvent 并通过 Doris repo 写入
	event := &ubaV1.BehaviorEvent{
		EventId:       eventID,
		EventName:     eventName,
		TenantId:      tenantID,
		UserId:        extractUint32Val(raw, "userId"),
		DeviceId:      extractStr(raw, "deviceId"),
		SessionId:     extractStr(raw, "sessionId"),
		Platform:      trans.Ptr(extractStr(raw, "platform")),
		EventCategory: extractStrPtr(raw, "eventCategory"),
		Amount:        extractStr(raw, "amount"),
		Context:       propsData,
	}

	if eventTime != nil {
		event.EventTime = timeutil.TimeToTimestamppb(eventTime)
	}

	if behaviorData != nil {
		if v, ok := behaviorData["pageUrl"].(string); ok {
			event.PageUrl = &v
		}
		if v, ok := behaviorData["durationMs"].(float64); ok {
			d := int64(v)
			event.DurationMs = uint32(d)
		}
	}

	if appID != "" {
		event.AppVersion = &appID
	}

	return s.eventsFactImpl().Create(ctx, event)
}

// ──────────── 工具函数 ────────────

func extractStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func extractStrPtr(m map[string]any, key string) *string {
	if v, ok := m[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

func extractUint32Val(m map[string]any, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// operatorIDFromCtx / tenantIDFromCtx 从上下文提取操作者（简化版，供导入服务用）。
func operatorIDFromCtx(ctx context.Context) int64 { return 0 }
func tenantIDFromCtx(ctx context.Context) int64   { return 0 }

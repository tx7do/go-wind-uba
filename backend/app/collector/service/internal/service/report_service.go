package service

import (
	"context"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/go-utils/timeutil"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-transport/broker"
	"google.golang.org/protobuf/types/known/emptypb"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"go-wind-uba/pkg/topic"
)

type ReportService struct {
	collectorV1.ReportServiceHTTPServer

	kafkaBroker broker.Broker
	log         *log.Helper

	applicationServiceClient ubaV1.ApplicationServiceClient
	appAuth                  *AppAuthenticator
}

func NewReportService(
	ctx *bootstrap.Context,
	kafkaBroker broker.Broker,
	applicationServiceClient ubaV1.ApplicationServiceClient,
	appAuth *AppAuthenticator,
) *ReportService {
	return &ReportService{
		log:                      ctx.NewLoggerHelper("report/service/collector-service"),
		kafkaBroker:              kafkaBroker,
		applicationServiceClient: applicationServiceClient,
		appAuth:                  appAuth,
	}
}

func (s *ReportService) HealthCheck(_ context.Context, _ *emptypb.Empty) (*collectorV1.HealthCheckResponse, error) {
	return &collectorV1.HealthCheckResponse{
		Status:    collectorV1.HealthCheckResponse_OK,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (s *ReportService) PostReport(ctx context.Context, req *ubaV1.PostReportRequest) (*ubaV1.PostReportResponse, error) {
	if req == nil || len(req.Events) == 0 {
		return nil, ubaV1.ErrorBadRequest("request data is required")
	}

	now := time.Now()
	requestID := uuid.New().String()

	// 应用级鉴权：校验请求体内的 app_id / app_secret，并取得应用所属的权威租户 ID。
	app, err := s.appAuth.Authenticate(ctx, req.AppId, req.AppSecret)
	if err != nil {
		return nil, err
	}

	errorsByType, validEvents := s.validateEvents(req.Events)

	// 用应用所属的权威 tenant_id 覆盖每个事件，杜绝客户端伪造跨租户上报。
	for _, event := range validEvents {
		event.TenantId = app.TenantID
	}

	// 处理事件：跟踪真实发布结果，失败计入 errorsByType，避免向客户端虚报成功。
	var successCount int32
	// recordError 把处理失败归入 errorsByType（复用校验阶段的分组结构）。
	recordError := func(eventType ubaV1.ReportEvent_EventType, eventID, code, msg string) {
		key := eventType.String()
		errorsByType[key] = append(errorsByType[key], &ubaV1.ErrorDetail{
			Code:    code,
			Message: msg,
			EventId: eventID,
		})
	}

	for _, event := range validEvents {
		event.ServerTime = timeutil.TimeToTimestamppb(&now)

		var handleErr error
		switch event.GetEventType() {
		case ubaV1.ReportEvent_BEHAVIOR:
			handleErr = s.handleBehavior(ctx, event, req.ClientInfo)
		case ubaV1.ReportEvent_RISK:
			handleErr = s.handleRisk(ctx, event, req.ClientInfo)
		default:
			s.log.Warnf("unsupported event type: %v", event.GetEventType())
			recordError(event.GetEventType(), event.EventId, "UNSUPPORTED_EVENT",
				"unsupported event_type: "+event.GetEventType().String())
			continue
		}

		if handleErr != nil {
			s.log.Errorf("failed to handle %s event [%s]: %v",
				event.GetEventType().String(), event.EventId, handleErr)
			recordError(event.GetEventType(), event.EventId, "HANDLE_FAILED", handleErr.Error())
			continue
		}
		successCount++
	}

	// 把 errorsByType（校验失败 + 处理失败）转为响应结构。
	var resultErrors []*ubaV1.TypeErrorDetail
	for eventType, errs := range errorsByType {
		resultErrors = append(resultErrors, &ubaV1.TypeErrorDetail{
			Type:   eventType,
			Errors: errs,
		})
	}

	return &ubaV1.PostReportResponse{
		Success: successCount > 0,
		Message: "accepted",

		ErrorsByType: resultErrors,
		RequestId:    requestID,
		ServerTime:   time.Now().UnixMilli(),

		TotalCount:   int32(len(req.Events)),
		SuccessCount: successCount,
		FailedCount:  int32(len(req.Events)) - successCount,
	}, nil
}

// validateEvents 校验事件列表，返回按事件类型分组的错误 map 与通过校验的事件列表。
func (s *ReportService) validateEvents(
	events []*ubaV1.ReportEvent,
) (map[string][]*ubaV1.ErrorDetail, []*ubaV1.ReportEvent) {

	errorsByType := make(map[string][]*ubaV1.ErrorDetail)
	validEvents := make([]*ubaV1.ReportEvent, 0, len(events))

	for _, event := range events {
		// 防御 nil 元素：proto repeated 字段允许 nil，直接校验会解引用 panic。
		if event == nil {
			continue
		}
		if err := s.validateEvent(event); err != nil {
			errDetail := &ubaV1.ErrorDetail{
				Code:    "INVALID_EVENT",
				Message: err.Error(),
				EventId: event.EventId,
			}
			errorsByType[event.EventType.String()] = append(
				errorsByType[event.EventType.String()], errDetail)
			continue
		}
		validEvents = append(validEvents, event)
	}

	return errorsByType, validEvents
}

// validateEvent 校验单个事件
func (s *ReportService) validateEvent(event *ubaV1.ReportEvent) error {
	if event.EventId == "" {
		return ubaV1.ErrorBadRequest("event_id is required")
	}
	if event.EventName == "" {
		return ubaV1.ErrorBadRequest("event_name is required")
	}
	if event.EventTime == nil {
		return ubaV1.ErrorBadRequest("event_time is required")
	}
	// tenant_id 不在此校验：它由 PostReport 用应用所属的权威 tenant_id 统一覆盖，
	// 客户端无需上报（上报也会被覆盖），故不强制要求。

	// 校验 oneof payload
	switch event.EventType {
	case ubaV1.ReportEvent_BEHAVIOR:
		if event.GetBehavior() == nil {
			return ubaV1.ErrorBadRequest("behavior payload required for BEHAVIOR event")
		}
	case ubaV1.ReportEvent_RISK:
		if event.GetRisk() == nil {
			return ubaV1.ErrorBadRequest("risk payload required for RISK event")
		}
	default:
		return ubaV1.ErrorBadRequest("unsupported event_type: %s", event.EventType)
	}

	return nil
}

// handleBehavior 处理行为事件
func (s *ReportService) handleBehavior(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	behaviorEvent := evt.GetBehavior()
	if behaviorEvent == nil {
		return ubaV1.ErrorBadRequest("behavior event data is required")
	}

	// event_id
	if evt.EventId == "" {
		evt.EventId = uuid.New().String()
	}
	behaviorEvent.EventId = evt.EventId

	// event_time
	if evt.EventTime == nil {
		now := time.Now()
		evt.EventTime = timeutil.TimeToTimestamppb(&now)
	}
	behaviorEvent.EventTime = evt.EventTime

	// event_name
	if evt.EventName == "" {
		evt.EventName = "behavior_event"
	}
	behaviorEvent.EventName = evt.EventName

	// context/properties
	if evt.Properties == nil {
		evt.Properties = map[string]string{}
	}
	behaviorEvent.Context = evt.GetProperties()

	// user_id
	if evt.UserId == nil {
		behaviorEvent.UserId = 0
	} else {
		behaviorEvent.UserId = *evt.UserId
	}
	// device_id
	behaviorEvent.DeviceId = evt.GetDeviceId()
	// session_id
	behaviorEvent.SessionId = evt.GetSessionId()
	// trace_id
	behaviorEvent.TraceId = evt.GetTraceId()

	// platform, event_category
	behaviorEvent.Platform = trans.Ptr(evt.GetPlatform())
	behaviorEvent.EventCategory = trans.Ptr(evt.GetEventCategory())

	// geo, user_agent, referer
	if ci != nil {
		if city := ci.GetCity(); city != "" {
			behaviorEvent.IpCity = trans.Ptr(trimAndLimit(city, 64))
		}
		if country := ci.GetCountry(); country != "" {
			behaviorEvent.Country = trans.Ptr(trimAndLimit(country, 64))
		}
		if ua := ci.GetUserAgent(); ua != "" {
			behaviorEvent.UserAgent = trans.Ptr(trimAndLimit(ua, 256))
		}
		if ref := ci.GetReferer(); ref != "" {
			behaviorEvent.Referer = trans.Ptr(trimAndLimit(ref, 256))
		}
	}
	// geo 字段补全
	if evt.Properties != nil {
		if geo, ok := evt.Properties["geo"]; ok {
			behaviorEvent.Geo = trans.Ptr(trimAndLimit(geo, 128))
		}
	}

	behaviorEvent.TenantId = evt.TenantId
	behaviorEvent.ServerTime = evt.GetServerTime()
	behaviorEvent.Ip = trans.Ptr(evt.GetIp())

	// 业务扩展字段透传：优先保留 behavior oneof 内已填的值，
	// 否则回退到 ReportEvent 顶层字段（兼容两种 SDK 上报方式）。
	//
	// 注意：仅对「具备存在性语义」的字段做回退——
	//   - string 类型：空串("") 视为未设置，可安全回退；
	//   - optional (*string)：nil 视为未设置，可安全回退；
	//   - map 类型：len==0 视为未设置，可安全回退。
	// 数值标量（SessionSeq/DurationMs/Quantity/Score）在 proto3 下无法区分
	// 「未设置」与「显式 0」，做零值回退会误杀合法的 0（如 Score=0 最低分），
	// 因此这些字段只信任 oneof 内的值，不做顶层回退。
	if behaviorEvent.EventAction == "" {
		behaviorEvent.EventAction = evt.GetEventAction()
	}
	if behaviorEvent.ObjectType == "" {
		behaviorEvent.ObjectType = evt.GetObjectType()
	}
	if behaviorEvent.ObjectId == "" {
		behaviorEvent.ObjectId = evt.GetObjectId()
	}
	if behaviorEvent.ObjectName == "" {
		behaviorEvent.ObjectName = evt.GetObjectName()
	}
	if behaviorEvent.Amount == "" {
		behaviorEvent.Amount = evt.GetAmount()
	}
	if behaviorEvent.ErrorCode == "" {
		behaviorEvent.ErrorCode = evt.GetErrorCode()
	}
	if behaviorEvent.Os == nil && evt.Os != nil {
		behaviorEvent.Os = trans.Ptr(evt.GetOs())
	}
	if behaviorEvent.AppVersion == nil && evt.AppVersion != nil {
		behaviorEvent.AppVersion = trans.Ptr(evt.GetAppVersion())
	}
	if behaviorEvent.Channel == nil && evt.Channel != nil {
		behaviorEvent.Channel = trans.Ptr(evt.GetChannel())
	}
	if behaviorEvent.Network == nil && evt.Network != nil {
		behaviorEvent.Network = trans.Ptr(evt.GetNetwork())
	}
	if behaviorEvent.OpResult == nil && evt.OpResult != nil {
		behaviorEvent.OpResult = trans.Ptr(evt.GetOpResult())
	}
	if len(behaviorEvent.Metrics) == 0 && len(evt.GetMetrics()) > 0 {
		behaviorEvent.Metrics = evt.GetMetrics()
	}

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventRaw, broker.NewMessage(behaviorEvent)); err != nil {
		s.log.Errorf("failed to publish behavior event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process behavior event")
	}

	return nil
}

// trimAndLimit 去除首尾空格并限制最大长度
func trimAndLimit(s string, max int) string {
	t := strings.TrimSpace(s)
	if len(t) > max {
		t = t[:max]
	}
	return t
}

// handleRisk 处理风险事件
func (s *ReportService) handleRisk(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	riskEvent := evt.GetRisk()
	if riskEvent == nil {
		return ubaV1.ErrorBadRequest("risk event data is required")
	}

	// risk_event_id（兜底生成，risk_events 表的 UNIQUE KEY 首列且 NOT NULL）
	if riskEvent.RiskEventId == "" {
		riskEvent.RiskEventId = uuid.New().String()
	}

	// occur_time（风险发生时间，risk_events 表 NOT NULL 且为分区键 event_date 的来源）
	// 客户端未单独上报时，以事件时间为准；事件时间缺失则用当前时间兜底。
	if riskEvent.OccurTime == nil {
		if evt.EventTime != nil {
			riskEvent.OccurTime = evt.EventTime
		} else {
			now := time.Now()
			riskEvent.OccurTime = timeutil.TimeToTimestamppb(&now)
		}
	}

	// report_time（上报时间，缺失则用当前时间）
	if riskEvent.ReportTime == nil {
		now := time.Now()
		riskEvent.ReportTime = timeutil.TimeToTimestamppb(&now)
	}

	// 主体字段：优先保留 risk oneof 内的值，否则回退 ReportEvent 顶层字段，
	// 与 handleBehavior 策略保持一致，避免无条件覆盖丢弃客户端自带的值。
	//
	// tenant_id 由 PostReport 用应用权威值覆盖（必为非 0），可直接采用；
	// user_id 仅在客户端显式上报（evt.UserId != nil）时才赋值，避免把匿名用户
	// 误写成「存在的用户 id 0」；device_id/session_id 为字符串，空串视为未设置。
	riskEvent.TenantId = trans.Ptr(evt.GetTenantId())
	if riskEvent.UserId == nil && evt.UserId != nil {
		riskEvent.UserId = trans.Ptr(*evt.UserId)
	}
	if riskEvent.DeviceId == nil && evt.GetDeviceId() != "" {
		riskEvent.DeviceId = trans.Ptr(evt.GetDeviceId())
	}
	if riskEvent.SessionId == nil && evt.GetSessionId() != "" {
		riskEvent.SessionId = trans.Ptr(evt.GetSessionId())
	}

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventRisk, broker.NewMessage(riskEvent)); err != nil {
		s.log.Errorf("failed to publish risk event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process risk event")
	}

	return nil
}

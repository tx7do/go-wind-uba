package service

import (
	"context"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/go-utils/mapper"
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

	platformConverter *mapper.EnumTypeConverter[ubaV1.Platform, string]
	categoryConverter *mapper.EnumTypeConverter[ubaV1.EventCategory, string]
}

func NewReportService(
	ctx *bootstrap.Context,
	kafkaBroker broker.Broker,
	applicationServiceClient ubaV1.ApplicationServiceClient,
) *ReportService {
	return &ReportService{
		log:                      ctx.NewLoggerHelper("report/service/collector-service"),
		kafkaBroker:              kafkaBroker,
		applicationServiceClient: applicationServiceClient,
		platformConverter: mapper.NewEnumTypeConverter[ubaV1.Platform, string](
			ubaV1.Platform_name, ubaV1.Platform_value,
		),
		categoryConverter: mapper.NewEnumTypeConverter[ubaV1.EventCategory, string](
			ubaV1.EventCategory_name, ubaV1.EventCategory_value,
		),
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

	// TODO 认证鉴权

	errorsByType, validEvents := s.validateEvents(req.Events)

	for _, event := range validEvents {
		if event == nil {
			s.log.Warnf("invalid event data: %v", event)
			continue
		}

		event.ServerTime = timeutil.TimeToTimestamppb(&now)

		switch event.GetEventType() {
		case ubaV1.ReportEvent_BEHAVIOR:
			// 处理行为事件
			if err := s.handleBehavior(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle behavior event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_RISK:
			// 处理风险事件
			if err := s.handleRisk(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle risk event: %v", err)
				continue
			}

		default:
			s.log.Warnf("unsupported event type: %v", event.GetEventType())
			continue
		}
	}

	return &ubaV1.PostReportResponse{
		Success: true,
		Message: "accepted",

		ErrorsByType: errorsByType,
		RequestId:    requestID,
		ServerTime:   time.Now().UnixMilli(),

		TotalCount:   int32(len(req.Events)),
		SuccessCount: int32(len(validEvents)),
		FailedCount:  int32(len(req.Events) - len(validEvents)),
	}, nil
}

// validateEvents 校验事件列表
func (s *ReportService) validateEvents(
	events []*ubaV1.ReportEvent,
) ([]*ubaV1.TypeErrorDetail, []*ubaV1.ReportEvent) {

	errorsByType := make(map[string][]*ubaV1.ErrorDetail)
	validEvents := make([]*ubaV1.ReportEvent, 0, len(events))

	for _, event := range events {
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

	// 转换为响应格式
	var result []*ubaV1.TypeErrorDetail
	for eventType, errors := range errorsByType {
		result = append(result, &ubaV1.TypeErrorDetail{
			Type:   eventType,
			Errors: errors,
		})
	}

	return result, validEvents
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
	if event.TenantId == 0 {
		return ubaV1.ErrorBadRequest("tenant_id is required")
	}

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
	behaviorEvent.Platform = s.platformConverter.ToDTO(&evt.Platform)
	behaviorEvent.EventCategory = s.categoryConverter.ToDTO(&evt.EventCategory)

	// geo, user_agent, referer
	if ci != nil {
		if city := ci.GetCity(); city != "" {
			behaviorEvent.IpCity = trimAndLimit(city, 64)
		}
		if country := ci.GetCountry(); country != "" {
			behaviorEvent.Country = trimAndLimit(country, 64)
		}
		if ua := ci.GetUserAgent(); ua != "" {
			behaviorEvent.UserAgent = trimAndLimit(ua, 256)
		}
		if ref := ci.GetReferer(); ref != "" {
			behaviorEvent.Referer = trimAndLimit(ref, 256)
		}
	}
	// geo 字段补全
	if evt.Properties != nil {
		if geo, ok := evt.Properties["geo"]; ok {
			behaviorEvent.Geo = trimAndLimit(geo, 128)
		}
	}

	behaviorEvent.TenantId = evt.TenantId
	behaviorEvent.ServerTime = evt.GetServerTime()
	behaviorEvent.Ip = evt.GetIp()

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

	riskEvent.TenantId = trans.Ptr(evt.GetTenantId())
	riskEvent.UserId = trans.Ptr(evt.GetUserId())
	riskEvent.DeviceId = trans.Ptr(evt.GetDeviceId())
	riskEvent.SessionId = trans.Ptr(evt.GetSessionId())

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventRisk, broker.NewMessage(riskEvent)); err != nil {
		s.log.Errorf("failed to publish risk event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process risk event")
	}

	return nil
}

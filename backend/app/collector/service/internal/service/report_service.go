package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/timeutil"
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

		case ubaV1.ReportEvent_PATH:
			// 处理路径事件
			if err := s.handlePath(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle path event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_RISK:
			// 处理风险事件
			if err := s.handleRisk(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle risk event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_SESSION:
			// 处理会话事件
			if err := s.handleSession(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle session event: %v", err)
				continue
			}

		case ubaV1.ReportEvent_FUNNEL:
			// 处理漏斗事件
			if err := s.handleFunnel(ctx, event, req.ClientInfo); err != nil {
				s.log.Errorf("failed to handle funnel event: %v", err)
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
	case ubaV1.ReportEvent_PATH:
		if event.GetPath() == nil {
			return ubaV1.ErrorBadRequest("path payload required for PATH event")
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

	if ci != nil {
		if ci.GetCity() != "" {
			behaviorEvent.IpCity = ci.GetCity()
		}
		if ci.GetCountry() != "" {
			behaviorEvent.Country = ci.GetCountry()
		}
		if ci.GetUserAgent() != "" {
			behaviorEvent.UserAgent = ci.GetUserAgent()
		}
		if ci.GetReferer() != "" {
			behaviorEvent.Referer = ci.GetReferer()
		}
	}

	behaviorEvent.EventId = evt.EventId
	behaviorEvent.EventName = evt.EventName
	behaviorEvent.EventTime = evt.EventTime
	behaviorEvent.EventCategory = s.categoryConverter.ToDTO(&evt.EventCategory)
	behaviorEvent.TenantId = evt.TenantId
	behaviorEvent.UserId = evt.GetUserId()
	behaviorEvent.DeviceId = evt.GetDeviceId()
	behaviorEvent.TraceId = evt.GetTraceId()
	behaviorEvent.ServerTime = evt.GetServerTime()
	behaviorEvent.Context = evt.GetProperties()
	behaviorEvent.Ip = evt.GetIp()
	behaviorEvent.SessionId = evt.GetSessionId()
	behaviorEvent.Platform = s.platformConverter.ToDTO(&evt.Platform)

	bt, _ := json.Marshal(behaviorEvent)
	s.log.Debugf("received behavior event: %s", string(bt))

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventRaw, broker.NewMessage(behaviorEvent)); err != nil {
		s.log.Errorf("failed to publish behavior event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process behavior event")
	}

	return nil
}

// handlePath 处理路径事件
func (s *ReportService) handlePath(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	pathEvent := evt.GetPath()
	if evt.GetPath() == nil {
		return ubaV1.ErrorBadRequest("path event data is required")
	}

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventPath, broker.NewMessage(pathEvent)); err != nil {
		s.log.Errorf("failed to publish path event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process path event")
	}

	return nil
}

// handleRisk 处理风险事件
func (s *ReportService) handleRisk(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	riskEvent := evt.GetRisk()
	if evt.GetRisk() == nil {
		return ubaV1.ErrorBadRequest("risk event data is required")
	}

	if err := s.kafkaBroker.Publish(ctx, topic.UbaEventRisk, broker.NewMessage(riskEvent)); err != nil {
		s.log.Errorf("failed to publish risk event to kafka: %v", err)
		return ubaV1.ErrorInternalServerError("failed to process risk event")
	}

	return nil
}

// handleSession 处理会话事件
func (s *ReportService) handleSession(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

// handleFunnel 处理漏斗事件
func (s *ReportService) handleFunnel(ctx context.Context, evt *ubaV1.ReportEvent, ci *ubaV1.ClientInfo) error {
	return nil
}

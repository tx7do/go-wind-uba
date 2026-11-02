package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	broker "github.com/tx7do/kratos-transport/broker"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// fakeBroker 捕获 Publish 调用，便于断言投递的主题与消息体。
type fakeBroker struct {
	broker.Broker

	fail   bool
	topics []string
	bodies []any
}

func (f *fakeBroker) Publish(_ context.Context, topicName string, msg *broker.Message, _ ...broker.PublishOption) error {
	if f.fail {
		return errors.New("broker unavailable")
	}
	f.topics = append(f.topics, topicName)
	f.bodies = append(f.bodies, msg.Body)
	return nil
}

func newTestReportService(t *testing.T, fb broker.Broker, app *ubaV1.Application) *ReportService {
	t.Helper()

	bctx := bootstrap.NewContextWithParam(
		context.Background(),
		&conf.AppInfo{}, &conf.Bootstrap{},
		log.DefaultLogger,
	)
	auth := NewAppAuthenticator(bctx, &fakeApplicationClient{app: app}, nil)

	return NewReportService(bctx, fb, nil, nil, nil, nil, auth)
}

func testFixedTime() time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
}

func longString(n int) string {
	return strings.Repeat("a", n)
}

func validBehaviorEvent() *ubaV1.ReportEvent {
	return &ubaV1.ReportEvent{
		EventId:   "evt-1",
		EventName: "page_view",
		EventTime: timestamppb.New(testFixedTime()),
		EventType: ubaV1.ReportEvent_BEHAVIOR,
		Payload: &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{}},
	}
}

func TestReportService_ValidateEvent(t *testing.T) {
	s := &ReportService{}

	// 合法 BEHAVIOR 事件。
	require.NoError(t, s.validateEvent(validBehaviorEvent()))

	// 合法 RISK 事件。
	risk := &ubaV1.ReportEvent{
		EventId:   "evt-2",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
	}
	require.NoError(t, s.validateEvent(risk))

	// 缺 event_id / event_name / event_time。
	noID := validBehaviorEvent()
	noID.EventId = ""
	require.Error(t, s.validateEvent(noID))

	noName := validBehaviorEvent()
	noName.EventName = ""
	require.Error(t, s.validateEvent(noName))

	noTime := validBehaviorEvent()
	noTime.EventTime = nil
	require.Error(t, s.validateEvent(noTime))

	// payload 缺失。
	noPayload := validBehaviorEvent()
	noPayload.Payload = nil
	require.Error(t, s.validateEvent(noPayload))

	// 不支持的事件类型。
	badType := validBehaviorEvent()
	badType.EventType = ubaV1.ReportEvent_EVENT_TYPE_UNSPECIFIED
	badType.Payload = nil
	require.Error(t, s.validateEvent(badType))
}

func TestReportService_ValidateEvents_GroupsErrorsAndSkipsNil(t *testing.T) {
	s := &ReportService{}

	bad1 := validBehaviorEvent()
	bad1.EventId = "bad-1"
	bad1.EventName = "" // 违规
	bad2 := validBehaviorEvent()
	bad2.EventId = "bad-2"
	bad2.EventName = "" // 违规
	ok := validBehaviorEvent()

	errs, valid := s.validateEvents([]*ubaV1.ReportEvent{nil, bad1, bad2, ok})

	// nil 元素直接丢弃，不进错误分组也不进 valid。
	require.Len(t, valid, 1)
	assert.Equal(t, "evt-1", valid[0].EventId)

	// 同类错误按事件类型分组。
	behaviorErrs := errs[ubaV1.ReportEvent_BEHAVIOR.String()]
	require.Len(t, behaviorErrs, 2)
	assert.Equal(t, "bad-1", behaviorErrs[0].GetEventId())
	assert.Equal(t, "bad-2", behaviorErrs[1].GetEventId())
	assert.Equal(t, "INVALID_EVENT", behaviorErrs[0].GetCode())
}

func TestReportService_PostReport_RequiresEvents(t *testing.T) {
	s := newTestReportService(t, &fakeBroker{}, testEnabledApp())

	_, err := s.PostReport(context.Background(), nil)
	require.Error(t, err)

	_, err = s.PostReport(context.Background(), &ubaV1.PostReportRequest{})
	require.Error(t, err)
}

func TestReportService_PostReport_SuccessOverridesTenant(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	// 客户端伪造 tenant_id=999，必须被应用权威租户 42 覆盖。
	ev := validBehaviorEvent()
	ev.TenantId = 999

	resp, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "app-001", AppSecret: "secret-abc", Events: []*ubaV1.ReportEvent{ev},
	})
	require.NoError(t, err)

	assert.True(t, resp.GetSuccess())
	assert.Equal(t, int32(1), resp.GetTotalCount())
	assert.Equal(t, int32(1), resp.GetSuccessCount())
	assert.Equal(t, int32(0), resp.GetFailedCount())
	assert.NotEmpty(t, resp.GetRequestId())

	// 投递到 raw 主题，且租户被覆盖。
	require.Len(t, fb.topics, 1)
	behavior := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.Equal(t, uint32(42), behavior.GetTenantId())
	assert.Equal(t, "evt-1", behavior.GetEventId())
}

func TestReportService_PostReport_BrokerFailureCountsAsFailed(t *testing.T) {
	fb := &fakeBroker{fail: true}
	s := newTestReportService(t, fb, testEnabledApp())

	resp, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "app-001", AppSecret: "secret-abc", Events: []*ubaV1.ReportEvent{validBehaviorEvent()},
	})
	require.NoError(t, err, "publish failure must be reported per-event, not abort the request")

	assert.False(t, resp.GetSuccess())
	assert.Equal(t, int32(1), resp.GetFailedCount())
	require.Len(t, resp.GetErrorsByType(), 1)
	detail := resp.GetErrorsByType()[0].GetErrors()[0]
	assert.Equal(t, "HANDLE_FAILED", detail.GetCode())
	assert.Equal(t, "evt-1", detail.GetEventId())
}

func TestReportService_PostReport_InvalidAuthPropagates(t *testing.T) {
	s := newTestReportService(t, &fakeBroker{}, nil) // gRPC 查无此应用

	_, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "ghost", AppSecret: "x", Events: []*ubaV1.ReportEvent{validBehaviorEvent()},
	})
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestReportService_HandleBehavior_MapsTopLevelFields(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventId:       "evt-9",
		EventName:     "pay_success",
		EventAction:   "pay",
		EventType:     ubaV1.ReportEvent_BEHAVIOR,
		ObjectType:    "order",
		ObjectId:      "o-1",
		ObjectName:    "订单",
		Amount:        "9.90",
		ErrorCode:     "",
		Os:            trans.Ptr("iOS"),
		AppVersion:    trans.Ptr("1.2.3"),
		Channel:       trans.Ptr("official"),
		Network:       trans.Ptr("WiFi"),
		OpResult:      trans.Ptr("success"),
		Platform:      "web",
		EventCategory: "TRADE",
		UserId:        trans.Ptr(uint32(7)),
		DeviceId:      "dev-1",
		SessionId:     "sess-1",
		TraceId:       "trace-1",
		Ip:             "10.0.0.2",
		Properties: map[string]string{
			"color": "red",
			"geo":   "39.9,116.4",
		},
		EventTime: timestamppb.New(testFixedTime()),
		Payload:   &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{}},
	}
	// server_time 由 PostReport 循环统一补全后透传；直调时模拟已补全。
	evt.ServerTime = timestamppb.New(testFixedTime())

	// broker 正常时应成功发布。
	require.NoError(t, s.handleBehavior(context.Background(), evt, nil))
	require.Len(t, fb.bodies, 1)

	b := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.Equal(t, "evt-9", b.GetEventId())
	assert.Equal(t, "pay_success", b.GetEventName())
	assert.Equal(t, "pay", b.GetEventAction())
	assert.Equal(t, "order", b.GetObjectType())
	assert.Equal(t, "o-1", b.GetObjectId())
	assert.Equal(t, "订单", b.GetObjectName())
	assert.Equal(t, "9.90", b.GetAmount())
	assert.Equal(t, uint32(7), b.GetUserId())
	assert.Equal(t, "dev-1", b.GetDeviceId())
	assert.Equal(t, "sess-1", b.GetSessionId())
	assert.Equal(t, "trace-1", b.GetTraceId())
	assert.Equal(t, "web", b.GetPlatform())
	assert.Equal(t, "TRADE", b.GetEventCategory())
	assert.Equal(t, "iOS", b.GetOs())
	assert.Equal(t, "1.2.3", b.GetAppVersion())
	assert.Equal(t, "official", b.GetChannel())
	assert.Equal(t, "WiFi", b.GetNetwork())
	assert.Equal(t, "success", b.GetOpResult())
	assert.Equal(t, "39.9,116.4", b.GetGeo())
	// properties → context 透传。
	assert.Equal(t, "red", b.GetContext()["color"])
	// server_time 透传。
	assert.True(t, b.GetServerTime().AsTime().Equal(testFixedTime()))
}

func TestReportService_HandleBehavior_OneofTakesPrecedence(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	// oneof 内已填的值优先，顶层同名字段不得覆盖。
	evt := &ubaV1.ReportEvent{
		EventId:   "evt-10",
		EventName: "pay_success",
		EventType: ubaV1.ReportEvent_BEHAVIOR,
		Os:        trans.Ptr("Android"),
		UserId:    trans.Ptr(uint32(7)),
		EventTime: timestamppb.New(testFixedTime()),
		Payload: &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{
			Os: trans.Ptr("iOS"),
		}},
	}

	require.NoError(t, s.handleBehavior(context.Background(), evt, nil))
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.Equal(t, "iOS", b.GetOs(), "oneof 内已填值不应被顶层回退覆盖")
}

func TestReportService_HandleBehavior_ClientInfoTrimmed(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := validBehaviorEvent()
	ci := &ubaV1.ClientInfo{
		UserAgent: "  " + longString(300) + "  ",
		Country:   "  China  ",
		City:      "  Beijing  ",
		Referer:   "  https://example.com/page  ",
	}

	require.NoError(t, s.handleBehavior(context.Background(), evt, ci))
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)

	assert.Len(t, b.GetUserAgent(), 256, "UA 截断到 256 字节")
	assert.Equal(t, "China", b.GetCountry())
	assert.Equal(t, "Beijing", b.GetIpCity())
	assert.Equal(t, "https://example.com/page", b.GetReferer())
}

func TestReportService_HandleRisk_FillsDefaultsAndFallbacks(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventId:   "evt-r1",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		UserId:    trans.Ptr(uint32(11)),
		DeviceId:  "dev-r1",
		EventType: ubaV1.ReportEvent_RISK,
		Payload: &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{
			RiskType: trans.Ptr("frequent_login"),
		}},
	}

	require.NoError(t, s.handleRisk(context.Background(), evt, nil))
	require.Len(t, fb.topics, 1)

	r := fb.bodies[0].(*ubaV1.RiskEvent)
	// 兜底生成 risk_event_id。
	assert.NotEmpty(t, r.GetRiskEventId())
	// occur_time 回退到事件时间。
	assert.Equal(t, testFixedTime().Unix(), r.GetOccurTime().AsTime().Unix())
	// report_time 补全。
	assert.NotNil(t, r.GetReportTime())
	// 顶层回退：user_id / device_id。
	assert.Equal(t, uint32(11), r.GetUserId())
	assert.Equal(t, "dev-r1", r.GetDeviceId())
	// oneof 内自带的字段保留。
	assert.Equal(t, "frequent_login", r.GetRiskType())
}

func TestReportService_HandleRisk_AnonymousUserStaysNil(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	// 顶层未报 user_id：不得写成 0（「存在的用户 id 0」）。
	evt := &ubaV1.ReportEvent{
		EventId:   "evt-r2",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
	}

	require.NoError(t, s.handleRisk(context.Background(), evt, nil))
	r := fb.bodies[0].(*ubaV1.RiskEvent)
	assert.Nil(t, r.UserId, "anonymous user must not become user_id=0")
}

func TestReportService_Publish_NilBrokerFails(t *testing.T) {
	// kafka 未配置：publish 必须报错（让 SDK 感知并重试），不能静默丢数据。
	s := newTestReportService(t, nil, testEnabledApp())
	require.Error(t, s.publish(context.Background(), "any-topic", &struct{}{}))
}

func TestReportService_HealthCheck(t *testing.T) {
	ok := newTestReportService(t, &fakeBroker{}, testEnabledApp())
	resp, err := ok.HealthCheck(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, collectorV1.HealthCheckResponse_OK, resp.GetStatus())

	degraded := newTestReportService(t, nil, testEnabledApp())
	resp, err = degraded.HealthCheck(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, collectorV1.HealthCheckResponse_DEGRADED, resp.GetStatus())
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	"google.golang.org/grpc"

	collectorV1 "go-wind-uba/api/gen/go/collector/service/v1"
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ───────────────────────────── 下游 gRPC 客户端 fake ─────────────────────────────

// fakeFeatureFlagClient 只覆写 EvaluateFlags,捕获入参便于断言权威租户与身份透传。
type fakeFeatureFlagClient struct {
	ubaV1.FeatureFlagServiceClient

	resp   *ubaV1.EvaluateFlagsResponse
	err    error
	gotReq *ubaV1.EvaluateFlagsRequest
}

func (f *fakeFeatureFlagClient) EvaluateFlags(_ context.Context, req *ubaV1.EvaluateFlagsRequest, _ ...grpc.CallOption) (*ubaV1.EvaluateFlagsResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// fakeVisualEventClient 只覆写 ListForApp。
type fakeVisualEventClient struct {
	ubaV1.VisualEventServiceClient

	resp   *ubaV1.ListVisualEventsForAppResponse
	err    error
	gotReq *ubaV1.ListVisualEventsForAppRequest
}

func (f *fakeVisualEventClient) ListForApp(_ context.Context, req *ubaV1.ListVisualEventsForAppRequest, _ ...grpc.CallOption) (*ubaV1.ListVisualEventsForAppResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// fakeReplayClient 只覆写 UploadChunk。
type fakeReplayClient struct {
	ubaV1.ReplayServiceClient

	resp   *ubaV1.UploadReplayChunkResponse
	err    error
	gotReq *ubaV1.UploadReplayChunkRequest
}

func (f *fakeReplayClient) UploadChunk(_ context.Context, req *ubaV1.UploadReplayChunkRequest, _ ...grpc.CallOption) (*ubaV1.UploadReplayChunkResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// injectClients 在 newTestReportService 的基础上注入下游客户端(同包直改私有字段)。
func injectClients(
	s *ReportService,
	flags ubaV1.FeatureFlagServiceClient,
	visual ubaV1.VisualEventServiceClient,
	replay ubaV1.ReplayServiceClient,
) *ReportService {
	s.featureFlagServiceClient = flags
	s.visualEventServiceClient = visual
	s.replayServiceClient = replay
	return s
}

func TestReportService_ValidateEvent_RiskPayloadRequired(t *testing.T) {
	// RISK 事件的 payload 缺失同样必须被拒(既有用例只钉了 BEHAVIOR 侧)。
	s := &ReportService{}

	risk := &ubaV1.ReportEvent{
		EventId:   "evt-risk-nopayload",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   nil,
	}
	err := s.validateEvent(risk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "risk payload required")
}

func TestReportService_PostReport_RiskEventPublishedToRiskTopic(t *testing.T) {
	// PostReport 级别的 RISK 链路:发布到风险主题,risk_event_id/occur_time 等兜底字段就位。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	resp, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		Events: []*ubaV1.ReportEvent{{
			EventId:   "evt-risk-1",
			EventName: "risk_hit",
			EventTime: timestamppb.New(testFixedTime()),
			EventType: ubaV1.ReportEvent_RISK,
			Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
		}},
	})
	require.NoError(t, err)

	assert.True(t, resp.GetSuccess())
	assert.Equal(t, int32(1), resp.GetSuccessCount())
	require.Len(t, fb.topics, 1)

	risk := fb.bodies[0].(*ubaV1.RiskEvent)
	assert.NotEmpty(t, risk.GetRiskEventId())
	assert.NotNil(t, risk.GetOccurTime())
	assert.Equal(t, int64(42), risk.GetTenantId(), "RISK 事件同样被应用权威租户覆盖")
}

// ───────────────────────────── trimAndLimit 多字节钉子 ─────────────────────────────

func TestTrimAndLimit_MultiByteUTF8PinsByteCut(t *testing.T) {
	// 钉住当前实现:按【字节】截断,不感知 UTF-8 边界 —— 中文(3 字节/字)截到 7 字节
	// 会切破第三个字符,产生非法 UTF-8 的脏数据入库。若未来实现改为 rune 安全截断,
	// 本用例失败属预期,请同步更新断言与注释。
	got := trimAndLimit("中文中文中文", 7)
	assert.Equal(t, 7, len(got), "按字节截断,结果长度应为 max 字节")
	assert.False(t, utf8.ValidString(got), "7 字节切在多字节字符中间,当前实现会产生非法 UTF-8")

	// 截断点恰好落在字符边界时不产生坏字节。
	assert.Equal(t, "中文", trimAndLimit("中文中文中文", 6))
	assert.True(t, utf8.ValidString(trimAndLimit("中文中文中文", 6)))

	// 4 字节 emoji 整体落在边界内时保持合法(5 = 1 + 4)。
	withEmoji := trimAndLimit("a😀b😀c", 5)
	assert.Equal(t, "a😀", withEmoji)
	assert.True(t, utf8.ValidString(withEmoji))
}

// ───────────────────────────── 批量上报计数 ─────────────────────────────

func TestReportService_PostReport_MixedBatchCounts(t *testing.T) {
	// 批量上报部分非法:合法事件照常发布,非法事件进错误分组,计数必须如实反映,
	// nil 元素也要计入 TotalCount(客户端收到完整的账目)。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	bad := validBehaviorEvent()
	bad.EventId = "bad-1"
	bad.EventName = "" // 非法:缺 event_name

	resp, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		Events: []*ubaV1.ReportEvent{validBehaviorEvent(), bad, nil},
	})
	require.NoError(t, err)

	assert.True(t, resp.GetSuccess(), "有事件成功即为 success")
	assert.Equal(t, int32(3), resp.GetTotalCount())
	assert.Equal(t, int32(1), resp.GetSuccessCount())
	assert.Equal(t, int32(2), resp.GetFailedCount(), "nil 元素与非法事件都计入失败")
	assert.Len(t, fb.bodies, 1, "仅合法事件被发布")

	// 校验失败按事件类型分组,事件 id 保留。
	require.Len(t, resp.GetErrorsByType(), 1)
	assert.Equal(t, ubaV1.ReportEvent_BEHAVIOR.String(), resp.GetErrorsByType()[0].GetType())
	behaviorErrs := resp.GetErrorsByType()[0].GetErrors()
	require.Len(t, behaviorErrs, 1)
	assert.Equal(t, "INVALID_EVENT", behaviorErrs[0].GetCode())
	assert.Equal(t, "bad-1", behaviorErrs[0].GetEventId())
}

func TestReportService_PostReport_UnsupportedEventTypeGroupedAsInvalid(t *testing.T) {
	// 未知事件类型(EVENT_TYPE_UNSPECIFIED)在 validateEvents 阶段即被拒:
	// 不产生任何发布,也不会触发 PostReport 循环里的 UNSUPPORTED_EVENT 分支
	// (该分支实际不可达,校验先行)。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	weird := validBehaviorEvent()
	weird.EventType = ubaV1.ReportEvent_EVENT_TYPE_UNSPECIFIED
	weird.Payload = &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{}}

	resp, err := s.PostReport(context.Background(), &ubaV1.PostReportRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		Events: []*ubaV1.ReportEvent{weird},
	})
	require.NoError(t, err)

	assert.False(t, resp.GetSuccess())
	assert.Equal(t, int32(1), resp.GetFailedCount())
	assert.Empty(t, fb.bodies, "非法事件不得发布到 Kafka")

	require.Len(t, resp.GetErrorsByType(), 1)
	assert.Equal(t, ubaV1.ReportEvent_EVENT_TYPE_UNSPECIFIED.String(), resp.GetErrorsByType()[0].GetType())
	assert.Contains(t, resp.GetErrorsByType()[0].GetErrors()[0].GetMessage(), "unsupported event_type")
}

// ───────────────────────────── handleBehavior 未覆盖分支 ─────────────────────────────

func TestReportService_HandleBehavior_FillsDefaultsWhenMissing(t *testing.T) {
	// handleBehavior 自身的兜底链路(直调防御路径):缺 event_id/event_name/event_time
	// 时分别兜底为 UUID / "behavior_event" / 当前时间,并同步写回 behavior oneof。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventType: ubaV1.ReportEvent_BEHAVIOR,
		Payload:   &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{}},
	}

	require.NoError(t, s.handleBehavior(context.Background(), evt, nil))
	require.Len(t, fb.bodies, 1)
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)

	assert.NotEmpty(t, evt.EventId, "event_id 应兜底生成 UUID")
	assert.Equal(t, "behavior_event", evt.EventName, "event_name 应兜底")
	assert.NotNil(t, evt.EventTime, "event_time 应兜底为当前时间")
	assert.Equal(t, evt.EventId, b.GetEventId(), "兜底后的 event_id 应同步到 behavior")
	assert.Equal(t, evt.EventName, b.GetEventName())
}

func TestReportService_HandleBehavior_MetricsFallback(t *testing.T) {
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	// 场景一:oneof 内 metrics 为空 → 回退顶层 ReportEvent.Metrics(map 可按 len 判空,安全回退)。
	evt1 := validBehaviorEvent()
	evt1.Metrics = map[string]float64{"latency_ms": 128}
	require.NoError(t, s.handleBehavior(context.Background(), evt1, nil))
	b1 := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.InDeltaMapValues(t, map[string]float64{"latency_ms": 128}, b1.GetMetrics(), 1e-9)

	// 场景二:oneof 内已有 metrics → 保留,不被顶层覆盖。
	evt2 := validBehaviorEvent()
	evt2.Metrics = map[string]float64{"top_level": 9}
	evt2.Payload = &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{
		Metrics: map[string]float64{"oneof": 1},
	}}
	require.NoError(t, s.handleBehavior(context.Background(), evt2, nil))
	b2 := fb.bodies[1].(*ubaV1.BehaviorEvent)
	assert.InDeltaMapValues(t, map[string]float64{"oneof": 1}, b2.GetMetrics(), 1e-9)
}

func TestReportService_HandleBehavior_NumericScalarsOnlyTrustOneof(t *testing.T) {
	// 数值标量(proto3 下无法区分「未设置」与「显式 0」)不做顶层回退:
	// oneof 内显式 0 必须保留(如 Score=0 最低分),顶层的非零值不得回填误杀合法 0。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := validBehaviorEvent()
	evt.SessionSeq = 99
	evt.DurationMs = 88
	evt.Quantity = 77
	evt.Score = 66
	evt.Payload = &ubaV1.ReportEvent_Behavior{Behavior: &ubaV1.BehaviorEvent{}}

	require.NoError(t, s.handleBehavior(context.Background(), evt, nil))
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.Equal(t, uint32(0), b.GetSessionSeq(), "session_seq 不得从顶层回退")
	assert.Equal(t, uint32(0), b.GetDurationMs(), "duration_ms 不得从顶层回退")
	assert.Equal(t, uint32(0), b.GetQuantity(), "quantity 不得从顶层回退")
	assert.Equal(t, int32(0), b.GetScore(), "score=0 是合法值,不得被顶层覆盖")
}

func TestReportService_HandleBehavior_GeoPropertyTrimmed(t *testing.T) {
	// properties["geo"] 截断到 128 字节,防止脏数据撑爆 geo 列。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := validBehaviorEvent()
	evt.Properties = map[string]string{"geo": "  " + longString(200) + "  "}

	require.NoError(t, s.handleBehavior(context.Background(), evt, nil))
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)
	assert.Len(t, b.GetGeo(), 128)
}

func TestReportService_HandleBehavior_BrokerFailureReturnsInternal(t *testing.T) {
	// 直调路径:发布失败返回 500 语义(而非静默成功),PostReport 会把它归入 HANDLE_FAILED。
	s := newTestReportService(t, &fakeBroker{fail: true}, testEnabledApp())

	err := s.handleBehavior(context.Background(), validBehaviorEvent(), nil)
	require.Error(t, err)
	assert.True(t, ubaV1.IsInternalServerError(err))
}

func TestReportService_HandleBehavior_NilPayloadFails(t *testing.T) {
	// payload 为 nil(理论上被 validateEvents 拦截,直调防御路径)不得 panic。
	s := newTestReportService(t, &fakeBroker{}, testEnabledApp())

	evt := &ubaV1.ReportEvent{EventType: ubaV1.ReportEvent_BEHAVIOR}
	require.Error(t, s.handleBehavior(context.Background(), evt, nil))
}

func TestReportService_HandleBehavior_NoClientInfoLeavesGeoFieldsNil(t *testing.T) {
	// 无 ClientInfo 且无请求上下文:geo/UA/referer 相关字段不得凭空生成。
	// 注意:Ip 会被无条件写成空串指针(trans.Ptr(evt.GetIp())),此处钉住该行为 ——
	// 若未来改为「空 IP 不落指针」,本用例失败请确认下游(OLAP 列语义)后再更新。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	require.NoError(t, s.handleBehavior(context.Background(), validBehaviorEvent(), nil))
	b := fb.bodies[0].(*ubaV1.BehaviorEvent)

	assert.Nil(t, b.Country)
	assert.Nil(t, b.IpCity)
	assert.Nil(t, b.UserAgent)
	assert.Nil(t, b.Referer)
	assert.Nil(t, b.Geo)
	require.NotNil(t, b.Ip, "当前实现无条件写 Ip 指针")
	assert.Empty(t, b.GetIp())
}

// ───────────────────────────── handleRisk 未覆盖分支 ─────────────────────────────

func TestReportService_HandleRisk_OccurTimeFallsBackToNow(t *testing.T) {
	// 事件时间也缺失时 occur_time 用当前时间兜底(occur_time 为 NOT NULL 且是
	// risk_events 表分区键 event_date 的来源),report_time 同样补全。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	before := time.Now()
	evt := &ubaV1.ReportEvent{
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
	}

	require.NoError(t, s.handleRisk(context.Background(), evt, nil))
	r := fb.bodies[0].(*ubaV1.RiskEvent)

	require.NotNil(t, r.GetOccurTime())
	assert.False(t, r.GetOccurTime().AsTime().Before(before), "occur_time 应兜底为当前时间")
	assert.NotNil(t, r.GetReportTime())
}

func TestReportService_HandleRisk_OneofTakesPrecedence(t *testing.T) {
	// risk oneof 内已填的主体字段不被顶层回退覆盖(与 handleBehavior 策略一致)。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventId:   "evt-r3",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		UserId:    trans.Ptr(int64(11)),
		DeviceId:  "top-dev",
		SessionId: "top-sess",
		EventType: ubaV1.ReportEvent_RISK,
		Payload: &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{
			UserId:    trans.Ptr(int64(5)),
			DeviceId:  trans.Ptr("oneof-dev"),
			SessionId: trans.Ptr("oneof-sess"),
		}},
	}

	require.NoError(t, s.handleRisk(context.Background(), evt, nil))
	r := fb.bodies[0].(*ubaV1.RiskEvent)
	assert.Equal(t, int64(5), r.GetUserId(), "oneof 内 user_id 不被顶层覆盖")
	assert.Equal(t, "oneof-dev", r.GetDeviceId())
	assert.Equal(t, "oneof-sess", r.GetSessionId())
}

func TestReportService_HandleRisk_TopLevelSessionFallback(t *testing.T) {
	// oneof 内无 session_id → 回退顶层 user_id/device_id/session_id(字符串空串视为未设置)。
	fb := &fakeBroker{}
	s := newTestReportService(t, fb, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventId:   "evt-r4",
		EventName: "risk_hit",
		EventTime: timestamppb.New(testFixedTime()),
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
		UserId:    trans.Ptr(int64(3)),
		DeviceId:  "top-dev",
		SessionId: "top-sess",
	}

	require.NoError(t, s.handleRisk(context.Background(), evt, nil))
	r := fb.bodies[0].(*ubaV1.RiskEvent)
	assert.Equal(t, int64(3), r.GetUserId())
	assert.Equal(t, "top-dev", r.GetDeviceId())
	assert.Equal(t, "top-sess", r.GetSessionId())
}

func TestReportService_HandleRisk_BrokerFailureReturnsInternal(t *testing.T) {
	s := newTestReportService(t, &fakeBroker{fail: true}, testEnabledApp())

	evt := &ubaV1.ReportEvent{
		EventType: ubaV1.ReportEvent_RISK,
		Payload:   &ubaV1.ReportEvent_Risk{Risk: &ubaV1.RiskEvent{}},
	}
	err := s.handleRisk(context.Background(), evt, nil)
	require.Error(t, err)
	assert.True(t, ubaV1.IsInternalServerError(err))
}

func TestReportService_HandleRisk_NilPayloadFails(t *testing.T) {
	s := newTestReportService(t, &fakeBroker{}, testEnabledApp())

	evt := &ubaV1.ReportEvent{EventType: ubaV1.ReportEvent_RISK}
	require.Error(t, s.handleRisk(context.Background(), evt, nil))
}

// ───────────────────────────── PostFlags ─────────────────────────────

func TestReportService_PostFlags_BadRequest(t *testing.T) {
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		&fakeFeatureFlagClient{}, nil, nil,
	)
	ctx := context.Background()

	_, err := s.PostFlags(ctx, nil)
	require.Error(t, err)
	assert.True(t, collectorV1.IsBadRequest(err))
	assert.Contains(t, err.Error(), "app_id is required")

	_, err = s.PostFlags(ctx, &ubaV1.PostFlagsRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_id is required")

	// app_id 有但两种身份都缺:同样 400。
	_, err = s.PostFlags(ctx, &ubaV1.PostFlagsRequest{AppId: "app-001"})
	require.Error(t, err)
	assert.True(t, collectorV1.IsBadRequest(err))
	assert.Contains(t, err.Error(), "user_id or device_id is required")
}

func TestReportService_PostFlags_NilClientReturnsEmpty(t *testing.T) {
	// flag 服务未装配:降级返回空结果(非 nil 的空 map),SDK 回退默认开关行为,不中断业务。
	s := newTestReportService(t, &fakeBroker{}, testEnabledApp())

	resp, err := s.PostFlags(context.Background(), &ubaV1.PostFlagsRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		DeviceId: trans.Ptr("dev-1"),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetFlags())
	assert.Empty(t, resp.GetFlags())
}

func TestReportService_PostFlags_SuccessPassthrough(t *testing.T) {
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		&fakeFeatureFlagClient{resp: &ubaV1.EvaluateFlagsResponse{
			Flags: map[string]string{"dark_mode": "on", "new_checkout": "off"},
		}},
		nil, nil,
	)

	resp, err := s.PostFlags(context.Background(), &ubaV1.PostFlagsRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		UserId: trans.Ptr(int64(7)), DeviceId: trans.Ptr("dev-1"),
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"dark_mode": "on", "new_checkout": "off"}, resp.GetFlags())

	// 评估请求必须携带应用权威租户与完整身份。
	flagClient := s.featureFlagServiceClient.(*fakeFeatureFlagClient)
	require.NotNil(t, flagClient.gotReq)
	assert.Equal(t, int64(42), flagClient.gotReq.GetTenantId(), "评估必须用应用权威租户")
	assert.Equal(t, int64(7), flagClient.gotReq.GetUserId())
	assert.Equal(t, "dev-1", flagClient.gotReq.GetDeviceId())
}

func TestReportService_PostFlags_AuthFailurePropagates(t *testing.T) {
	// 鉴权失败(查无应用)向上传播,不降级为空结果:拿不到开关应让 SDK 感知 401。
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, nil),
		&fakeFeatureFlagClient{}, nil, nil,
	)

	_, err := s.PostFlags(context.Background(), &ubaV1.PostFlagsRequest{
		AppId: "ghost", AppSecret: "x", DeviceId: trans.Ptr("dev-1"),
	})
	require.Error(t, err)
	assert.True(t, collectorV1.IsUnauthorized(err))
}

func TestReportService_PostFlags_EvaluateFailureDegradesToEmpty(t *testing.T) {
	// 下游评估失败:返回空结果而非报错(与 nil client 的降级语义一致)。
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		&fakeFeatureFlagClient{err: errors.New("core down")},
		nil, nil,
	)

	resp, err := s.PostFlags(context.Background(), &ubaV1.PostFlagsRequest{
		AppId: "app-001", AppSecret: "secret-abc",
		UserId: trans.Ptr(int64(7)),
	})
	require.NoError(t, err, "评估故障应降级为空结果而非报错")
	require.NotNil(t, resp.GetFlags())
	assert.Empty(t, resp.GetFlags())
}

// ───────────────────────────── ListVisualEvents ─────────────────────────────

func TestReportService_ListVisualEvents_BadRequest(t *testing.T) {
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, &fakeVisualEventClient{}, nil,
	)

	_, err := s.ListVisualEvents(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, collectorV1.IsBadRequest(err))
	assert.Contains(t, err.Error(), "app_id is required")

	_, err = s.ListVisualEvents(context.Background(), &ubaV1.ListVisualEventsForAppRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_id is required")
}

func TestReportService_ListVisualEvents_NilClientReturnsEmpty(t *testing.T) {
	// 规则服务未装配:返回空列表(不鉴权,守卫先行)。
	s := newTestReportService(t, &fakeBroker{}, testEnabledApp())

	resp, err := s.ListVisualEvents(context.Background(), &ubaV1.ListVisualEventsForAppRequest{AppId: trans.Ptr("app-001")})
	require.NoError(t, err)
	assert.Empty(t, resp.GetItems())
}

func TestReportService_ListVisualEvents_UnknownAppDegradesToEmpty(t *testing.T) {
	// 只读旁路鉴权失败:静默降级为空列表,SDK 只跑代码埋点,且不得向下游转发。
	visual := &fakeVisualEventClient{}
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, nil), // 查无此应用
		nil, visual, nil,
	)

	resp, err := s.ListVisualEvents(context.Background(), &ubaV1.ListVisualEventsForAppRequest{AppId: trans.Ptr("ghost")})
	require.NoError(t, err)
	assert.Empty(t, resp.GetItems())
	assert.Nil(t, visual.gotReq, "鉴权失败不得转发到下游")
}

func TestReportService_ListVisualEvents_SuccessWithAuthoritativeTenant(t *testing.T) {
	visual := &fakeVisualEventClient{resp: &ubaV1.ListVisualEventsForAppResponse{
		Items: []*ubaV1.VisualEvent{{}},
	}}
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, visual, nil,
	)

	// 客户端伪造 tenant_id=999,转发请求必须改用应用权威租户 42。
	resp, err := s.ListVisualEvents(context.Background(), &ubaV1.ListVisualEventsForAppRequest{
		AppId:    trans.Ptr("app-001"),
		TenantId: trans.Ptr(int64(999)),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetItems(), 1)

	require.NotNil(t, visual.gotReq)
	assert.Equal(t, int64(42), visual.gotReq.GetTenantId(), "下发规则必须限定在应用权威租户内")
	assert.Equal(t, "app-001", visual.gotReq.GetAppId())
}

func TestReportService_ListVisualEvents_DownstreamFailureDegrades(t *testing.T) {
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, &fakeVisualEventClient{err: errors.New("core down")}, nil,
	)

	resp, err := s.ListVisualEvents(context.Background(), &ubaV1.ListVisualEventsForAppRequest{AppId: trans.Ptr("app-001")})
	require.NoError(t, err, "规则服务故障应降级为空列表而非报错")
	assert.Empty(t, resp.GetItems())
}

// ───────────────────────────── UploadReplayChunk ─────────────────────────────

func TestReportService_UploadReplayChunk_GuardClauses(t *testing.T) {
	// nil 请求 / 空 session_id / 下游未装配:三种守卫都静默返回空响应(不报错)。
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, nil, &fakeReplayClient{resp: &ubaV1.UploadReplayChunkResponse{Id: 1}},
	)
	ctx := context.Background()

	resp, err := s.UploadReplayChunk(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	resp, err = s.UploadReplayChunk(ctx, &ubaV1.UploadReplayChunkRequest{AppId: "app-001"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	resp, err = s.UploadReplayChunk(ctx, &ubaV1.UploadReplayChunkRequest{AppId: "app-001", SessionId: "sess-1"})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestReportService_UploadReplayChunk_GuardSkipsDownstream(t *testing.T) {
	// 守卫分支命中时下游不得被调用(哪怕 client 已装配)。
	replay := &fakeReplayClient{}
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, nil, replay,
	)
	ctx := context.Background()

	_, err := s.UploadReplayChunk(ctx, nil)
	require.NoError(t, err)
	_, err = s.UploadReplayChunk(ctx, &ubaV1.UploadReplayChunkRequest{AppId: "app-001"})
	require.NoError(t, err)
	assert.Nil(t, replay.gotReq, "守卫命中不得转发")
}

func TestReportService_UploadReplayChunk_AuthFailureSilentlyDrops(t *testing.T) {
	// 回放是只增旁路数据:鉴权失败静默 200 且不转发,避免 SDK 录制重试风暴。
	replay := &fakeReplayClient{resp: &ubaV1.UploadReplayChunkResponse{Id: 1}}
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, nil), // 查无此应用
		nil, nil, replay,
	)

	resp, err := s.UploadReplayChunk(context.Background(), &ubaV1.UploadReplayChunkRequest{
		AppId: "ghost", SessionId: "sess-1", EventsJson: "[1,2,3]",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, replay.gotReq, "鉴权失败不得转发到下游")
}

func TestReportService_UploadReplayChunk_TenantOverriddenInPlace(t *testing.T) {
	// SDK 伪造 tenant_id=999 必须被应用权威租户 42 就地覆盖后转发,响应原样透传。
	replay := &fakeReplayClient{resp: &ubaV1.UploadReplayChunkResponse{Id: 77}}
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, nil, replay,
	)

	req := &ubaV1.UploadReplayChunkRequest{
		AppId: "app-001", SessionId: "sess-1", TenantId: trans.Ptr(int64(999)),
	}
	resp, err := s.UploadReplayChunk(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int64(77), resp.GetId(), "下游响应应原样透传")

	require.NotNil(t, replay.gotReq)
	assert.Same(t, req, replay.gotReq, "应转发原请求对象(就地覆盖租户)")
	assert.Equal(t, int64(42), replay.gotReq.GetTenantId())
}

func TestReportService_UploadReplayChunk_DownstreamFailureSilentlyDrops(t *testing.T) {
	s := injectClients(
		newTestReportService(t, &fakeBroker{}, testEnabledApp()),
		nil, nil, &fakeReplayClient{err: errors.New("core down")},
	)

	resp, err := s.UploadReplayChunk(context.Background(), &ubaV1.UploadReplayChunkRequest{
		AppId: "app-001", SessionId: "sess-1",
	})
	require.NoError(t, err, "转发失败应静默返回空响应")
	require.NotNil(t, resp)
}

package uba

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 记录式假服务端：可编程状态码队列 + 可编程 handler 覆写。
type fakeServer struct {
	*httptest.Server

	mu       sync.Mutex
	bodies   []map[string]any
	statuses []int
	queue    []int // 待返回的状态码队列；空则 200
	override func(w http.ResponseWriter, r *http.Request, n int) // 按请求序号覆写
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	fs := &fakeServer{}
	var seq atomic.Int64
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(seq.Add(1))
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)

		fs.mu.Lock()
		fs.bodies = append(fs.bodies, m)
		override := fs.override
		status := http.StatusOK
		if override == nil && len(fs.queue) > 0 {
			status = fs.queue[0]
			fs.queue = fs.queue[1:]
		}
		fs.statuses = append(fs.statuses, status)
		fs.mu.Unlock()

		if override != nil {
			override(w, r, n)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"success": true, "totalCount": 1, "successCount": 1, "failedCount": 0}`))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeServer) requestCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.bodies)
}

func (fs *fakeServer) lastBody() map[string]any {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.bodies[len(fs.bodies)-1]
}

func totalEvents(fs *fakeServer) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n := 0
	for _, b := range fs.bodies {
		if evts, ok := b["events"].([]any); ok {
			n += len(evts)
		}
	}
	return n
}

func fastConfig(endpoint string) Config {
	return Config{
		AppID: "app_001", AppSecret: "secret_001", Endpoint: endpoint,
		BatchSize: 100,
		// -1 = 不启动定时器（0 会被 fillDefaults 规整为默认 5s）
		FlushInterval: -1,
		MaxRetries:    1,
		RetryBaseDelay: time.Millisecond,
	}
}

func TestClientTrackGolden(t *testing.T) {
	fs := newFakeServer(t)
	client, err := NewClient(fastConfig(fs.URL))
	if err != nil {
		t.Fatal(err)
	}
	client.Identify(1001)
	client.SetSuperProperties(map[string]string{"env": "prod"})
	client.Track("order_paid", P{"orderId": "ORD-001"}, &TrackOptions{
		EventCategory: "order",
		EventAction:   "pay",
		ObjectType:    "order",
		ObjectID:      "ORD-001",
		ObjectName:    "订单",
		DurationMs:    3500,
		Amount:        "99.90",
		Quantity:      2,
		Score:         88,
		Metrics:       map[string]float64{"amount_num": 99.9},
		ServerID:      "s1",
		Level:         42,
		TraceID:       "trace-1",
		Network:       "WiFi",
		OpResult:      "success",
	})
	if r := client.Flush(); !r.Success || r.Sent != 1 {
		t.Fatalf("flush 结果: %+v", r)
	}
	client.Close()

	envelope := fs.lastBody()
	if envelope["appId"] != "app_001" || envelope["appSecret"] != "secret_001" {
		t.Fatal("鉴权字段缺失")
	}
	if ci, ok := envelope["clientInfo"].(map[string]any); !ok || ci["userAgent"] == "" {
		t.Fatal("clientInfo.userAgent 缺失")
	}
	events := envelope["events"].([]any)
	evt := events[0].(map[string]any)
	if evt["eventType"] != "BEHAVIOR" || evt["eventName"] != "order_paid" {
		t.Fatalf("事件头错误: %v", evt)
	}
	if evt["userId"] != float64(1001) {
		t.Fatalf("userId 应为数字: %v", evt["userId"])
	}
	if evt["properties"].(map[string]any)["env"] != "prod" {
		t.Fatal("super 属性未合并")
	}
	// 服务端专属字段与数值标量绝不能出现在顶层
	for _, k := range []string{"tenantId", "serverTime", "ip", "durationMs", "quantity", "score", "level", "sessionSeq"} {
		if _, ok := evt[k]; ok {
			t.Fatalf("顶层不应出现 %s", k)
		}
	}
	behavior := evt["behavior"].(map[string]any)
	for k, want := range map[string]any{"durationMs": 3500, "quantity": 2, "score": 88, "level": 42, "serverId": "s1"} {
		var expected any = want
		if n, ok := want.(int); ok {
			expected = float64(n)
		}
		if behavior[k] != expected {
			t.Fatalf("behavior.%s = %v, want %v", k, behavior[k], want)
		}
	}
	if behavior["os"] == "" || behavior["appVersion"] == "" {
		t.Fatal("behavior 应带 os/appVersion")
	}
	if evt["eventTime"] == "" {
		t.Fatal("eventTime 缺失")
	}
}

func TestClientMinimalTrack(t *testing.T) {
	fs := newFakeServer(t)
	client, _ := NewClient(fastConfig(fs.URL))
	client.Track("heartbeat", nil, nil)
	client.Flush()
	client.Close()

	evt := fs.lastBody()["events"].([]any)[0].(map[string]any)
	if _, ok := evt["userId"]; ok {
		t.Fatal("未 identify 不应带 userId")
	}
	if _, ok := evt["properties"]; ok {
		t.Fatal("无属性不应发 properties")
	}
	if _, ok := evt["behavior"]; !ok {
		t.Fatal("behavior oneof 必须存在（服务端校验 payload 与 eventType 匹配）")
	}
}

func TestClientTrackRisk(t *testing.T) {
	fs := newFakeServer(t)
	client, _ := NewClient(fastConfig(fs.URL))
	client.TrackRisk("login_failed", &RiskEvent{
		RiskType:        "login_anomaly",
		RiskLevel:       "HIGH",
		RiskScore:       88.5,
		RuleID:          101,
		RelatedEventIDs: []string{"e1"},
		RuleContext:     map[string]any{"threshold": 5},
	}, P{"try_count": "8"}, nil)
	client.Flush()
	client.Close()

	evt := fs.lastBody()["events"].([]any)[0].(map[string]any)
	if evt["eventType"] != "RISK" {
		t.Fatalf("eventType: %v", evt["eventType"])
	}
	if _, ok := evt["behavior"]; ok {
		t.Fatal("risk 事件不应带 behavior")
	}
	risk := evt["risk"].(map[string]any)
	if risk["riskScore"] != 88.5 || risk["ruleId"] != float64(101) {
		t.Fatalf("risk 载荷错误: %v", risk)
	}
}

func TestClientNoRetryOn4xx(t *testing.T) {
	fs := newFakeServer(t)
	fs.mu.Lock()
	fs.queue = []int{http.StatusUnauthorized}
	fs.mu.Unlock()

	client, _ := NewClient(fastConfig(fs.URL))
	client.Track("t", nil, nil)
	r := client.Flush()
	if r.Success || r.Retryable || r.Dropped != 1 {
		t.Fatalf("401 应确定性丢弃: %+v", r)
	}
	if fs.requestCount() != 1 {
		t.Fatalf("401 不应重试，实际请求 %d 次", fs.requestCount())
	}
	if client.PendingCount() != 0 {
		t.Fatal("确定性失败不应回队")
	}
	client.Close()
}

func TestClientRetryOn5xx(t *testing.T) {
	fs := newFakeServer(t)
	fs.mu.Lock()
	fs.queue = []int{http.StatusInternalServerError, http.StatusBadGateway}
	fs.mu.Unlock()

	cfg := fastConfig(fs.URL)
	cfg.MaxRetries = 2 // 3 次尝试：500 → 502 → 200
	client, _ := NewClient(cfg)
	client.Track("t", nil, nil)
	r := client.Flush()
	if !r.Success {
		t.Fatalf("第 3 次应成功: %+v", r)
	}
	if fs.requestCount() != 3 {
		t.Fatalf("应重试 2 次，实际 %d 次", fs.requestCount())
	}
	client.Close()
}

func TestClient429Retryable(t *testing.T) {
	fs := newFakeServer(t)
	fs.mu.Lock()
	fs.queue = []int{http.StatusTooManyRequests}
	fs.mu.Unlock()

	cfg := fastConfig(fs.URL)
	cfg.MaxRetries = -1 // 禁用重试：单次尝试即判可重试耗尽
	client, _ := NewClient(cfg)
	client.Track("t", nil, nil)
	r := client.Flush()
	if r.Success || !r.Retryable {
		t.Fatalf("429 应可重试: %+v", r)
	}
	if client.PendingCount() != 1 {
		t.Fatal("429 耗尽应回队")
	}
	client.Close()
}

func TestClientRetryExhaustedStaysQueued(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	fs := newFakeServer(t)
	fs.override = func(w http.ResponseWriter, _ *http.Request, _ int) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success": true, "failedCount": 0}`))
	}

	client, _ := NewClient(fastConfig(fs.URL))
	client.Track("t", nil, nil)
	r := client.Flush()
	if r.Success || !r.Retryable {
		t.Fatalf("重试耗尽应为可重试失败: %+v", r)
	}
	if client.PendingCount() != 1 {
		t.Fatal("可重试失败应回队等待补发")
	}
	fail.Store(false)
	if r := client.Flush(); !r.Success || client.PendingCount() != 0 {
		t.Fatalf("补发失败: %+v", r)
	}
	client.Close()
}

func TestClientPartialFailure(t *testing.T) {
	fs := newFakeServer(t)
	fs.override = func(w http.ResponseWriter, _ *http.Request, _ int) {
		_, _ = w.Write([]byte(`{"success": true, "totalCount": 3, "successCount": 1, "failedCount": 2}`))
	}

	client, _ := NewClient(fastConfig(fs.URL))
	for _, name := range []string{"a", "b", "c"} {
		client.Track(name, nil, nil)
	}
	r := client.Flush()
	if !r.Success {
		t.Fatal("HTTP 200 部分失败仍算传输成功")
	}
	if r.Sent != 1 || r.Dropped != 2 {
		t.Fatalf("部分失败计数错误: %+v", r)
	}
	if client.PendingCount() != 0 {
		t.Fatal("服务端明确拒绝的条目不应回队")
	}
	client.Close()
}

func TestClientBatchSizeTrigger(t *testing.T) {
	fs := newFakeServer(t)
	cfg := fastConfig(fs.URL)
	cfg.BatchSize = 2
	client, _ := NewClient(cfg)
	client.Track("a", nil, nil)
	client.Track("b", nil, nil)

	deadline := time.Now().Add(2 * time.Second)
	for fs.requestCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fs.requestCount() != 1 {
		t.Fatalf("达到 batchSize 应自动 flush，请求数 %d", fs.requestCount())
	}
	client.Close()
}

func TestClientConcurrentTrack(t *testing.T) {
	fs := newFakeServer(t)
	cfg := fastConfig(fs.URL)
	cfg.BatchSize = 50
	client, _ := NewClient(cfg)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				client.Track("concurrent", nil, nil)
			}
		}()
	}
	wg.Wait()

	// 等 auto-flush 清空队列（含在途请求）
	deadline := time.Now().Add(3 * time.Second)
	for (client.PendingCount() > 0 || fs.requestCount() < 20) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	client.Flush()
	if got := totalEvents(fs); got != 1000 {
		t.Fatalf("并发 track 丢事件: 服务端收到 %d，want 1000", got)
	}
	client.Close()
}

func TestClientSamplingOff(t *testing.T) {
	fs := newFakeServer(t)
	cfg := fastConfig(fs.URL)
	cfg.SampleRate = 0.000001
	client, _ := NewClient(cfg)
	for i := 0; i < 10; i++ {
		client.Track("t", nil, nil)
	}
	if client.PendingCount() != 0 {
		t.Fatal("采样关闭时应全部丢弃")
	}
	client.Close()
}

func TestClientFlags(t *testing.T) {
	fs := newFakeServer(t)
	fs.override = func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.URL.Path == "/uba/v1/flags" {
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			if m["deviceId"] == "" {
				t.Error("flags 请求应带 deviceId")
			}
			_, _ = w.Write([]byte(`{"flags": {"exp": "red", "ctl": "control"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success": true, "failedCount": 0}`))
	}

	client, _ := NewClient(fastConfig(fs.URL))
	client.Identify(7)
	flags, err := client.LoadFlags()
	if err != nil {
		t.Fatal(err)
	}
	if flags["exp"] != "red" {
		t.Fatalf("flags 错误: %v", flags)
	}
	if v := client.GetFlag("exp"); v != "red" {
		t.Fatalf("GetFlag: %s", v)
	}
	if client.IsFeatureEnabled("ctl") {
		t.Fatal("control 不应命中")
	}
	if client.GetFlag("missing") != "" {
		t.Fatal("缺失开关应返回空")
	}

	client.Flush()
	var exposure map[string]any
	for _, b := range fs.bodies {
		if evts, ok := b["events"].([]any); ok {
			for _, e := range evts {
				evt := e.(map[string]any)
				if evt["eventName"] == "$feature_flag" {
					exposure = evt
				}
			}
		}
	}
	if exposure == nil {
		t.Fatal("应产生一次曝光事件")
	}
	if exposure["eventCategory"] != "experiment" {
		t.Fatalf("曝光事件分类错误: %v", exposure)
	}
	if props := exposure["properties"].(map[string]any); props["flag_key"] != "exp" || props["variant"] != "red" {
		t.Fatalf("曝光属性错误: %v", props)
	}

	// 同会话去重
	client.GetFlag("exp")
	if client.PendingCount() != 0 {
		t.Fatal("同会话同开关不应重复曝光")
	}
	client.Close()
}

func TestClientTrackOptionsOverrideIdentity(t *testing.T) {
	fs := newFakeServer(t)
	client, _ := NewClient(fastConfig(fs.URL))
	client.Identify(1)
	client.Track("t", nil, &TrackOptions{UserID: 999, DeviceID: "custom", SessionID: "custom-sess", Platform: "game"})
	client.Flush()
	client.Close()

	evt := fs.lastBody()["events"].([]any)[0].(map[string]any)
	if evt["userId"] != float64(999) || evt["deviceId"] != "custom" || evt["sessionId"] != "custom-sess" || evt["platform"] != "game" {
		t.Fatalf("TrackOptions 覆盖失败: %v", evt)
	}
}

func TestClientSessionRotationOnTimeout(t *testing.T) {
	fs := newFakeServer(t)
	cfg := fastConfig(fs.URL)
	cfg.SessionTimeout = 30 * time.Millisecond
	client, _ := NewClient(cfg)
	client.Track("a", nil, nil)
	client.Flush()
	sess1 := fs.lastBody()["events"].([]any)[0].(map[string]any)["sessionId"].(string)

	time.Sleep(40 * time.Millisecond)
	client.Track("b", nil, nil)
	client.Flush()
	client.Close()
	sess2 := fs.lastBody()["events"].([]any)[0].(map[string]any)["sessionId"].(string)
	if sess1 == "" || sess1 == sess2 {
		t.Fatalf("超时后应轮换会话: %s vs %s", sess1, sess2)
	}
}

func TestClientClosedRejects(t *testing.T) {
	fs := newFakeServer(t)
	client, _ := NewClient(fastConfig(fs.URL))
	client.Track("last", nil, nil)
	client.Close()
	before := fs.requestCount()
	client.Track("after-close", nil, nil)
	if client.PendingCount() != 0 {
		t.Fatal("close 后不应入队")
	}
	if fs.requestCount() != before {
		t.Fatal("close 后不应产生新请求")
	}
}

// 定时 flush 循环：可重试失败回队的残余由定时器补发
func TestClientTimerFlushRetriesResidual(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	fs := newFakeServer(t)
	fs.override = func(w http.ResponseWriter, _ *http.Request, _ int) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success": true, "failedCount": 0}`))
	}

	cfg := Config{
		AppID: "app_001", AppSecret: "secret_001", Endpoint: fs.URL,
		BatchSize: 100, FlushInterval: 30 * time.Millisecond,
		MaxRetries: 1, RetryBaseDelay: time.Millisecond,
	}
	client, _ := NewClient(cfg)
	client.Track("residual", nil, nil)
	r := client.Flush()
	if r.Success || !r.Retryable || client.PendingCount() != 1 {
		t.Fatalf("第一次应失败且回队: %+v", r)
	}
	fail.Store(false)

	deadline := time.Now().Add(3 * time.Second)
	for client.PendingCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if client.PendingCount() != 0 {
		t.Fatal("定时 flush 应补发回队残余")
	}
	client.Close()
}

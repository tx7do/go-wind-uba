package uba

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDV4(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := uuidV4()
		if !uuidPattern.MatchString(id) {
			t.Fatalf("uuid 格式错误: %s", id)
		}
		if seen[id] {
			t.Fatalf("uuid 重复: %s", id)
		}
		seen[id] = true
	}
}

func TestToRFC3339Ms(t *testing.T) {
	got := toRFC3339Ms(time.Date(2026, 3, 27, 12, 0, 0, 123_000_000, time.UTC))
	if got != "2026-03-27T12:00:00.123Z" {
		t.Fatalf("got %s", got)
	}
	got = toRFC3339Ms(time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC))
	if got != "2026-01-02T03:04:05.006Z" {
		t.Fatalf("前零补齐失败: %s", got)
	}
}

func TestSessionInSample(t *testing.T) {
	if !sessionInSample("s", 1) {
		t.Fatal("采样率 1 应全采")
	}
	if sessionInSample("s", 0) {
		t.Fatal("采样率 0 应全丢")
	}
	r := sessionInSample("fixed-session", 0.5)
	for i := 0; i < 10; i++ {
		if sessionInSample("fixed-session", 0.5) != r {
			t.Fatal("同会话采样结果应恒定")
		}
	}
}

func TestMergeProperties(t *testing.T) {
	super := map[string]string{"a": "1", "b": "1"}
	merged := mergeProperties(super, map[string]string{"b": "2", "c": "3"})
	if merged["a"] != "1" || merged["b"] != "2" || merged["c"] != "3" {
		t.Fatalf("合并结果错误: %v", merged)
	}
	merged["a"] = "mutated"
	if super["a"] != "1" {
		t.Fatal("不应污染 super 属性")
	}
	if mergeProperties(nil, nil) != nil {
		t.Fatal("两侧为空应返回 nil")
	}
}

func TestEventJSONContract(t *testing.T) {
	// 金样：键集合精确对齐 proto json_name，数值标量不落顶层
	evt := &Event{
		EventType:     "BEHAVIOR",
		EventID:       "e1",
		EventName:     "order_paid",
		EventTime:     "2026-03-27T12:00:00.123Z",
		UserID:        1001,
		DeviceID:      "host-1",
		SessionID:     "sess-1",
		Platform:      "server",
		EventCategory: "order",
		Properties:    map[string]string{"pay_type": "wechat"},
		Metrics:       map[string]float64{"amount_num": 99.9},
		TraceID:       "trace-1",
		EventAction:   "pay",
		ObjectType:    "order",
		ObjectID:      "ORD-001",
		ObjectName:    "订单",
		Amount:        "99.90",
		OpResult:      "success",
		Behavior: &behaviorPayload{
			EventAction: "pay",
			ObjectType:  "order",
			ObjectID:    "ORD-001",
			OS:          "linux",
			AppVersion:  "1.0.0",
			DurationMs:  3500,
			Quantity:    2,
			Score:       88,
			Metrics:     map[string]float64{"amount_num": 99.9},
			ServerID:    "s1",
			Level:       42,
		},
	}
	raw, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"eventType": true, "eventId": true, "eventName": true, "eventTime": true,
		"userId": true, "deviceId": true, "sessionId": true, "platform": true,
		"eventCategory": true, "properties": true, "metrics": true, "traceId": true,
		"eventAction": true, "objectType": true, "objectId": true, "objectName": true,
		"amount": true, "opResult": true, "behavior": true,
	}
	for k := range m {
		if !want[k] {
			t.Fatalf("多余的顶层键: %s", k)
		}
		delete(want, k)
	}
	if len(want) > 0 {
		t.Fatalf("缺失的顶层键: %v", want)
	}

	// 服务端专属字段绝不能出现
	for _, k := range []string{"tenantId", "serverTime", "ip", "durationMs", "quantity", "score", "level"} {
		if _, ok := m[k]; ok {
			t.Fatalf("顶层不应出现 %s", k)
		}
	}

	behavior := m["behavior"].(map[string]any)
	for k, want := range map[string]any{"durationMs": 3500, "quantity": 2, "score": 88, "level": 42} {
		if behavior[k] != float64(want.(int)) {
			t.Fatalf("behavior.%s = %v, want %v", k, behavior[k], want)
		}
	}
	if m["userId"] != float64(1001) {
		t.Fatalf("userId 应为数字 1001，got %v (%T)", m["userId"], m["userId"])
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{AppID: "a", AppSecret: "s", Endpoint: "http://x/"}
	if err := cfg.fillDefaults(); err != nil {
		t.Fatal(err)
	}
	if cfg.BatchSize != 20 || cfg.FlushInterval != 5*time.Second || cfg.MaxRetries != 3 ||
		cfg.Timeout != 8*time.Second || cfg.RetryBaseDelay != time.Second {
		t.Fatalf("默认值与 C#/Flutter SDK 不一致: %+v", cfg)
	}
	if cfg.Path != "/uba/v1/report" || cfg.FlagsPath != "/uba/v1/flags" {
		t.Fatalf("默认路径错误: %s %s", cfg.Path, cfg.FlagsPath)
	}
	if cfg.Platform != "server" || cfg.DeviceID == "" {
		t.Fatalf("服务端默认身份错误: %s %s", cfg.Platform, cfg.DeviceID)
	}
	if cfg.ReportURL() != "http://x/uba/v1/report" {
		t.Fatalf("尾斜杠未规整: %s", cfg.ReportURL())
	}
}

func TestConfigValidation(t *testing.T) {
	cases := []Config{
		{AppSecret: "s", Endpoint: "http://x"},
		{AppID: "a", Endpoint: "http://x"},
		{AppID: "a", AppSecret: "s"},
		{AppID: "a", AppSecret: "s", Endpoint: "http://x", Timeout: 10 * time.Second},
	}
	for i, cfg := range cases {
		if err := cfg.fillDefaults(); err == nil {
			t.Fatalf("case %d 应校验失败", i)
		}
	}
	okCfg := Config{AppID: "a", AppSecret: "s", Endpoint: "http://x", Timeout: 9999 * time.Millisecond}
	if err := okCfg.fillDefaults(); err != nil {
		t.Fatalf("<10s 应合法: %v", err)
	}
}

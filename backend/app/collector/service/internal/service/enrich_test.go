package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mileusna/useragent"
	"github.com/tx7do/go-utils/trans"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

func TestRealClientIP(t *testing.T) {
	build := func(headers map[string]string, remoteAddr string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/report", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		// httptest.NewRequest 默认填 192.0.2.1，这里无条件覆盖以控制测试输入。
		req.RemoteAddr = remoteAddr
		return req
	}

	// XFF 链首（最外层代理应覆写该头，取第一个合法 IP）。
	req := build(map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1, 198.51.100.2"}, "")
	if got := realClientIP(req); got != "203.0.113.7" {
		t.Fatalf("XFF first valid ip: got %q", got)
	}

	// XFF 里的非法项应跳过（伪造的垃圾值不阻断，继续往后找）。
	req = build(map[string]string{"X-Forwarded-For": "not-an-ip, 198.51.100.2"}, "")
	if got := realClientIP(req); got != "198.51.100.2" {
		t.Fatalf("XFF skips invalid entries: got %q", got)
	}

	// 无 XFF 时回退 X-Real-IP。
	req = build(map[string]string{"X-Real-IP": "203.0.113.9"}, "10.0.0.5:1234")
	if got := realClientIP(req); got != "203.0.113.9" {
		t.Fatalf("X-Real-IP fallback: got %q", got)
	}

	// X-Real-IP 非法时继续回退 RemoteAddr。
	req = build(map[string]string{"X-Real-IP": "garbage"}, "10.0.0.5:1234")
	if got := realClientIP(req); got != "10.0.0.5" {
		t.Fatalf("invalid X-Real-IP falls to RemoteAddr: got %q", got)
	}

	// 全缺失时取 RemoteAddr 并去端口。
	req = build(nil, "198.51.100.77:5678")
	if got := realClientIP(req); got != "198.51.100.77" {
		t.Fatalf("RemoteAddr host strip: got %q", got)
	}

	// RemoteAddr 无端口（如 unix 域或测试环境）也应原样返回。
	req = build(nil, "198.51.100.78")
	if got := realClientIP(req); got != "198.51.100.78" {
		t.Fatalf("RemoteAddr without port: got %q", got)
	}

	// 全部为空返回空串。
	req = build(nil, "")
	if got := realClientIP(req); got != "" {
		t.Fatalf("empty request should give empty ip, got %q", got)
	}

	// nil 请求防御。
	if got := realClientIP(nil); got != "" {
		t.Fatalf("nil request should give empty ip, got %q", got)
	}
}

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"203.0.113.7", true},    // 公网
		{"127.0.0.1", false},     // 回环
		{"10.1.2.3", false},      // 私网
		{"192.168.1.1", false},   // 私网
		{"172.16.0.9", false},    // 私网
		{"169.254.1.1", false},   // 链路本地
		{"0.0.0.0", false},       // 未指定
		{"not-an-ip", false},     // 解析失败
		{"", false},              // 空串
		{"::1", false},            // IPv6 回环
		{"2606:4700::1111", true}, // IPv6 公网
	}
	for _, c := range cases {
		if got := isPublicIP(c.ip); got != c.want {
			t.Fatalf("isPublicIP(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestDeviceType(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", "desktop"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1", "mobile"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Safari/604.1", "tablet"},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "bot"},
		{"", "unknown"},
		{"curl/8.0", "unknown"},
	}
	for _, c := range cases {
		if got := deviceType(useragent.Parse(c.ua)); got != c.want {
			t.Fatalf("deviceType(%q) = %q, want %q", c.ua, got, c.want)
		}
	}
}

func TestTrimAndLimit(t *testing.T) {
	// 去首尾空格。
	if got := trimAndLimit("  hello  ", 32); got != "hello" {
		t.Fatalf("trim: got %q", got)
	}
	// 超长截断到 max 字节。
	if got := trimAndLimit("abcdefghij", 5); got != "abcde" {
		t.Fatalf("limit: got %q", got)
	}
	// 未超长原样返回。
	if got := trimAndLimit("abc", 10); got != "abc" {
		t.Fatalf("within limit: got %q", got)
	}
	// 空串。
	if got := trimAndLimit("", 8); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestEnrichBehaviorFromRequest_NoRequest(t *testing.T) {
	// 非 HTTP 传输（requestFromContext 返回 nil）：UA 解析仍应工作。
	ev := &ubaV1.BehaviorEvent{}
	ci := &ubaV1.ClientInfo{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0"}
	enrichBehaviorFromRequest(context.Background(), ev, ci)

	if ev.GetOs() == "" {
		t.Fatal("os should be parsed from user agent")
	}
	if ev.GetContext()["browser"] == "" {
		t.Fatal("browser should be parsed into context")
	}
	if ev.GetContext()["device_type"] != "desktop" {
		t.Fatalf("device_type = %q, want desktop", ev.GetContext()["device_type"])
	}
	// 无请求头时 referer/geo 不应被凭空补出。
	if ev.Referer != nil {
		t.Fatalf("referer should stay unset without request header, got %q", *ev.Referer)
	}
}

func TestEnrichBehaviorFromRequest_KeepsExplicitValues(t *testing.T) {
	// SDK 显式上报的同名属性不能被服务端补全覆盖。
	ev := &ubaV1.BehaviorEvent{
		Os: trans.Ptr("iOS"),
		Context: map[string]string{
			"browser":     "MyBrowser 1.0",
			"device_type": "custom",
		},
	}
	ci := &ubaV1.ClientInfo{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0"}
	enrichBehaviorFromRequest(context.Background(), ev, ci)

	if got := ev.GetOs(); got != "iOS" {
		t.Fatalf("explicit os overwritten: got %q", got)
	}
	if got := ev.GetContext()["browser"]; got != "MyBrowser 1.0" {
		t.Fatalf("explicit browser overwritten: got %q", got)
	}
	if got := ev.GetContext()["device_type"]; got != "custom" {
		t.Fatalf("explicit device_type overwritten: got %q", got)
	}
}

func TestEnrichBehaviorFromRequest_GeoSkippedForPrivateIP(t *testing.T) {
	// 回环/私网 IP 不值得做地理查询：country/city 保持为空。
	ev := &ubaV1.BehaviorEvent{Ip: trans.Ptr("10.0.0.1")}
	enrichBehaviorFromRequest(context.Background(), ev, nil)

	if ev.Country != nil || ev.IpCity != nil {
		t.Fatalf("geo lookup should be skipped for private ip, got country=%v city=%v", ev.Country, ev.IpCity)
	}
}

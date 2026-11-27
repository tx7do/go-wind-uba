package uba

import (
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// uuidV4 生成 uuid v4（服务端埋点的事件去重键）。
func uuidV4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败基本意味着系统熵源异常，退化为时间戳占位（不应发生）
		return fmt.Sprintf("uba-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// toRFC3339Ms UTC 时间 → RFC3339 毫秒精度字符串，与其他端 SDK 精度一致。
func toRFC3339Ms(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// trimSlash 去掉 endpoint 尾斜杠。
func trimSlash(s string) string { return strings.TrimSuffix(s, "/") }

// hostname 取主机名作为服务端实例标识（deviceId 语义）。
func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	return h
}

// fnv1a32 FNV-1a 32 位哈希，用于按 sessionId 确定性采样。
func fnv1a32(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}

// sessionInSample 会话是否落在采样区间（同一会话结果恒定）。
func sessionInSample(sessionID string, rate float64) bool {
	if rate >= 1 {
		return true
	}
	if rate <= 0 {
		return false
	}
	return float64(fnv1a32(sessionID)%1000)/1000 < rate
}

// mergeProperties super 属性在前、调用属性在后（后者覆盖前者）。
// 返回副本，不污染 super 属性本身。
func mergeProperties(super map[string]string, props map[string]string) map[string]string {
	if len(super) == 0 && len(props) == 0 {
		return nil
	}
	merged := make(map[string]string, len(super)+len(props))
	for k, v := range super {
		merged[k] = v
	}
	for k, v := range props {
		merged[k] = v
	}
	return merged
}

// validHTTPURL 校验 endpoint 是可解析的 http(s) URL。
func validHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

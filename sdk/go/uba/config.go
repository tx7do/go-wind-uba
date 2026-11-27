// Package uba 是 GoWind UBA 风行用户行为分析平台的 Go 服务端埋点 SDK。
//
// 面向后端服务发事件（支付回调、风控触发、订单流转等服务端场景），
// 直连 collector：POST {endpoint}/uba/v1/report，鉴权在请求 body（appId+appSecret），
// camelCase JSON，无 gzip，单批请求体压在 1MiB 内，超时默认 8s（必须小于服务端 10s）。
//
// 用法：
//
//	client, err := uba.NewClient(uba.Config{
//		AppID:     "your_app_id",
//		AppSecret: "your_app_secret",
//		Endpoint:  "http://localhost:5700",
//	})
//	if err != nil { ... }
//	defer client.Close()
//
//	client.Identify(1001)
//	client.Track("order_paid", uba.P{"orderId": "ORD-001"},
//		&uba.TrackOptions{ObjectType: "order", Amount: "99.90"})
//
// 契约铁律（与 Web/C#/Flutter SDK 一致）：
//   - 字段 camelCase 对齐后端 proto json_name，禁止手造字段；
//   - tenantId/serverTime/ip 服务端补全，SDK 不发送；
//   - 数值标量（durationMs/quantity/score/sessionSeq/level）只进 behavior oneof。
package uba

import (
	"errors"
	"time"
)

// Config 客户端配置，默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 /
// 1s 退避基数 / 8s 超时（必须 < 服务端 10s）。
type Config struct {
	// AppID 应用唯一标识（管理后台「数据采集/应用管理」生成），必填。
	AppID string
	// AppSecret 应用密钥，鉴权在请求 body 中完成，必填。
	AppSecret string
	// Endpoint collector 服务地址，如 http://localhost:5700，必填。
	Endpoint string
	// Path 事件上报路径，默认 /uba/v1/report。
	Path string
	// FlagsPath AB 实验开关拉取路径，默认 /uba/v1/flags。
	FlagsPath string
	// BatchSize 批量触发阈值，默认 20。
	BatchSize int
	// FlushInterval 定时 flush 间隔，默认 5s；设为负数禁用定时 flush。
	FlushInterval time.Duration
	// MaxRetries 失败重试次数（总尝试 = MaxRetries+1），默认 3；设为负数禁用重试。
	MaxRetries int
	// Timeout 单次 HTTP 请求超时，默认 8s。
	Timeout time.Duration
	// RetryBaseDelay 重试退避基数，第 n 次重试延迟 = RetryBaseDelay*2^n，默认 1s。
	RetryBaseDelay time.Duration
	// Debug 调试日志开关（输出到标准 log）。
	Debug bool
	// DeviceID 服务端实例标识（服务端没有设备概念，用实例/主机名区分数据源），
	// 为空时自动取 hostname。
	DeviceID string
	// Platform 上报平台标识，默认 "server"。
	Platform string
	// AppVersion 应用版本，随事件上报 behavior.appVersion。
	AppVersion string
	// Channel 渠道标识。
	Channel string
	// SessionTimeout 会话超时：Go 服务端是常驻进程，默认不轮换（0 = 进程级会话）。
	SessionTimeout time.Duration
	// SampleRate 采样率 0~1，按 sessionId 确定性采样，默认 1。
	SampleRate float64
	// HTTPClient 自定义 HTTP 客户端（超时/代理/连接池），为空时用默认实现。
	HTTPClient HTTPClient
}

func (c *Config) fillDefaults() error {
	if c.AppID == "" {
		return errors.New("uba: AppID 不能为空")
	}
	if c.AppSecret == "" {
		return errors.New("uba: AppSecret 不能为空")
	}
	if c.Endpoint == "" {
		return errors.New("uba: Endpoint 不能为空")
	}
	if c.Path == "" {
		c.Path = "/uba/v1/report"
	}
	if c.FlagsPath == "" {
		c.FlagsPath = "/uba/v1/flags"
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 20
	}
	if c.FlushInterval == 0 {
		c.FlushInterval = 5 * time.Second
	}
	// 0 = 未设置（默认 3）；负数 = 显式禁用重试
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	} else if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.Timeout <= 0 {
		c.Timeout = 8 * time.Second
	}
	if c.Timeout >= 10*time.Second {
		return errors.New("uba: Timeout 必须 < 10s（服务端超时 10s）")
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = time.Second
	}
	if c.DeviceID == "" {
		c.DeviceID = hostname()
	}
	if c.Platform == "" {
		c.Platform = "server"
	}
	if c.SampleRate <= 0 {
		c.SampleRate = 1
	}
	return nil
}

// ReportURL 事件上报完整地址（endpoint 规整尾斜杠后拼接）。
func (c *Config) ReportURL() string { return trimSlash(c.Endpoint) + c.Path }

// FlagsURL AB 开关拉取完整地址。
func (c *Config) FlagsURL() string { return trimSlash(c.Endpoint) + c.FlagsPath }

// TrackOptions 单次上报维度，字段与其他端 SDK 的 TrackOptions 对齐。
// 传入的字段优先级高于客户端状态。
type TrackOptions struct {
	EventCategory string
	UserID        int64
	DeviceID      string
	SessionID     string
	Platform      string
	EventAction   string
	ObjectType    string
	ObjectID      string
	ObjectName    string
	DurationMs    int64
	Amount        string
	Quantity      int64
	Score         int32
	Metrics       map[string]float64
	Network       string
	OpResult      string
	ErrorCode     string
	ServerID      string
	Level         int32
	TraceID       string
}

// P track 的业务属性（properties map）的类型别名。
type P = map[string]string

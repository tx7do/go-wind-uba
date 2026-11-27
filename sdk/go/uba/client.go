package uba

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// sdkVersion SDK 版本，用于 User-Agent；与发布版本保持一致。
const sdkVersion = "1.0.0"

// maxBatchBytes 单批请求体字节上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内。
const maxBatchBytes = 900 * 1024

// maxQueueLen 内存队列上限（防持续故障下无界增长）；超限丢弃最旧事件。
const maxQueueLen = 10000

// HTTPClient HTTP 传输抽象，默认 *http.Client；可替换为自定义实现（代理/网关/加签）。
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client Go 服务端埋点客户端。并发安全。
type Client struct {
	cfg        Config
	http       HTTPClient
	userAgent  string

	mu    sync.Mutex
	queue []*Event
	// flushing 防重入：并发 Flush 直接返回
	flushing bool
	closed   bool

	stopOnce sync.Once
	stopCh   chan struct{}

	userID         atomic.Int64 // 0 = 匿名
	superProps     map[string]string
	flags          map[string]string
	exposed        map[string]bool
	exposedSession string

	sessionID   string
	lastEventAt time.Time
}

// NewClient 创建并启动客户端（定时 flush 协程随 Close 停止）。
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.fillDefaults(); err != nil {
		return nil, err
	}
	if !validHTTPURL(cfg.Endpoint) {
		return nil, fmt.Errorf("uba: Endpoint 不是合法的 http(s) 地址: %s", cfg.Endpoint)
	}

	c := &Client{
		cfg:      cfg,
		http:     cfg.HTTPClient,
		stopCh:   make(chan struct{}),
		superProps: map[string]string{},
		flags:    map[string]string{},
		exposed:  map[string]bool{},
		sessionID: uuidV4(),
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: cfg.Timeout}
	}
	c.userAgent = fmt.Sprintf("uba-go/%s (%s)", sdkVersion, runtime.GOOS)

	if cfg.FlushInterval > 0 {
		go c.flushLoop()
	}
	return c, nil
}

// Identify 绑定用户 ID（服务端事件源：订单/支付/风控消息里的用户）。0 = 匿名。
func (c *Client) Identify(userID int64) { c.userID.Store(userID) }

// ResetUser 清空用户绑定。
func (c *Client) ResetUser() { c.userID.Store(0) }

// SetSuperProperties 设置全局公共属性：合并进之后每个事件的 properties（同名覆盖）。
func (c *Client) SetSuperProperties(props map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range props {
		c.superProps[k] = v
	}
}

// ClearSuperProperties 清空全局公共属性。
func (c *Client) ClearSuperProperties() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.superProps = map[string]string{}
}

// Track 上报自定义行为事件。可重试失败留在内存队列，等待下次定时 flush 补发。
func (c *Client) Track(eventName string, props P, opts *TrackOptions) {
	c.track(eventName, props, opts, nil)
}

// TrackRisk 上报风险事件（进入 uba_risk_events 通道）。
func (c *Client) TrackRisk(eventName string, risk *RiskEvent, props P, opts *TrackOptions) {
	if risk == nil {
		return
	}
	c.track(eventName, props, opts, (*riskPayload)(risk))
}

func (c *Client) track(eventName string, props P, opts *TrackOptions, risk *riskPayload) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		c.debugf("客户端已关闭，丢弃事件 %s", eventName)
		return
	}

	sessionID := c.currentSessionID()
	if !sessionInSample(sessionID, c.cfg.SampleRate) {
		return // 会话级确定性采样：整会话要么全采要么全丢
	}

	uid := int64(0)
	if opts != nil && opts.UserID != 0 {
		uid = opts.UserID
	} else {
		uid = c.userID.Load()
	}
	deviceID := c.cfg.DeviceID
	if opts != nil && opts.DeviceID != "" {
		deviceID = opts.DeviceID
	}
	session := sessionID
	if opts != nil && opts.SessionID != "" {
		session = opts.SessionID
	}
	platform := c.cfg.Platform
	if opts != nil && opts.Platform != "" {
		platform = opts.Platform
	}

	evt := &Event{
		EventType:     "BEHAVIOR",
		EventID:       uuidV4(),
		EventName:     eventName,
		EventTime:     toRFC3339Ms(time.Now()),
		UserID:        uid,
		DeviceID:      deviceID,
		SessionID:     session,
		Platform:      platform,
		Properties:    mergeProperties(c.snapshotSuperProps(), props),
		TraceID:       field(opts, func(x *TrackOptions) string { return x.TraceID }),
		EventCategory: field(opts, func(x *TrackOptions) string { return x.EventCategory }),
		EventAction:   field(opts, func(x *TrackOptions) string { return x.EventAction }),
		ObjectType:    field(opts, func(x *TrackOptions) string { return x.ObjectType }),
		ObjectID:      field(opts, func(x *TrackOptions) string { return x.ObjectID }),
		ObjectName:    field(opts, func(x *TrackOptions) string { return x.ObjectName }),
		Amount:        field(opts, func(x *TrackOptions) string { return x.Amount }),
		OpResult:      field(opts, func(x *TrackOptions) string { return x.OpResult }),
		ErrorCode:     field(opts, func(x *TrackOptions) string { return x.ErrorCode }),
	}
	if risk != nil {
		evt.EventType = "RISK"
		evt.Risk = risk
	} else {
		// behavior oneof 必须带载荷（服务端校验 payload 与 eventType 匹配）
		evt.Behavior = c.buildBehavior(opts)
		if opts != nil && opts.Metrics != nil {
			evt.Metrics = opts.Metrics
		}
	}

	c.enqueue(evt)
}

func field[T any](o *TrackOptions, f func(*TrackOptions) T) T {
	var zero T
	if o == nil {
		return zero
	}
	return f(o)
}

func (c *Client) buildBehavior(o *TrackOptions) *behaviorPayload {
	b := &behaviorPayload{
		OS:         runtime.GOOS,
		AppVersion: c.cfg.AppVersion,
		Channel:    c.cfg.Channel,
	}
	if o == nil {
		return b
	}
	b.EventAction = o.EventAction
	b.ObjectType = o.ObjectType
	b.ObjectID = o.ObjectID
	b.ObjectName = o.ObjectName
	b.Network = o.Network
	b.DurationMs = o.DurationMs
	b.Amount = o.Amount
	b.Quantity = o.Quantity
	b.Score = o.Score
	b.Metrics = o.Metrics
	b.OpResult = o.OpResult
	b.ErrorCode = o.ErrorCode
	b.ServerID = o.ServerID
	b.Level = o.Level
	return b
}

func (c *Client) snapshotSuperProps() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.superProps) == 0 {
		return nil
	}
	out := make(map[string]string, len(c.superProps))
	for k, v := range c.superProps {
		out[k] = v
	}
	return out
}

// currentSessionID 取会话 ID；SessionTimeout > 0 且距上一事件超时则轮换新会话。
func (c *Client) currentSessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.cfg.SessionTimeout > 0 && !c.lastEventAt.IsZero() &&
		now.Sub(c.lastEventAt) > c.cfg.SessionTimeout {
		c.sessionID = uuidV4()
		c.exposedSession = ""
	}
	c.lastEventAt = now
	return c.sessionID
}

func (c *Client) enqueue(evt *Event) {
	var auto bool
	c.mu.Lock()
	c.queue = append(c.queue, evt)
	if len(c.queue) > maxQueueLen {
		c.queue = c.queue[len(c.queue)-maxQueueLen:]
	}
	auto = len(c.queue) >= c.cfg.BatchSize
	c.mu.Unlock()
	if auto {
		go c.Flush()
	}
}

// PendingCount 队列中待上报条数。
func (c *Client) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queue)
}

// Flush 手动冲刷队列：按批量/字节上限分批发送，直到清空或遇到失败。
// 并发调用安全：正在 flush 时的再次调用直接返回。
func (c *Client) Flush() FlushResult {
	c.mu.Lock()
	if c.flushing || c.closed {
		c.mu.Unlock()
		return FlushResult{Success: true}
	}
	c.flushing = true
	c.mu.Unlock()

	var sent, dropped int
	var message string
	var retryable bool
	ok := true
	for {
		c.mu.Lock()
		if len(c.queue) == 0 {
			c.mu.Unlock()
			break
		}
		batch := c.takeBatchLocked()
		c.mu.Unlock()

		r := c.sendWithRetry(batch)
		sent += r.Sent
		dropped += r.Dropped
		if !r.Success {
			ok = false
			message = r.Message
			retryable = r.Retryable
			// 可重试失败（网络/5xx/429）回到队首等下次补发；确定性 4xx 直接丢弃
			if r.Retryable {
				c.mu.Lock()
				c.queue = append(batch, c.queue...)
				c.mu.Unlock()
			}
			break
		}
	}

	c.mu.Lock()
	c.flushing = false
	c.mu.Unlock()
	return FlushResult{
		Success:   ok,
		Sent:      sent,
		Dropped:   dropped,
		Retryable: retryable,
		Message:   message,
	}
}

// takeBatchLocked 取一批：受 BatchSize 与 maxBatchBytes 双重限制。调用方需持锁。
func (c *Client) takeBatchLocked() []*Event {
	var batch []*Event
	var size int
	for len(c.queue) > 0 && len(batch) < c.cfg.BatchSize {
		encoded, err := json.Marshal(c.queue[0])
		if err == nil && len(batch) > 0 && size+len(encoded) > maxBatchBytes {
			break
		}
		if err == nil {
			size += len(encoded)
		}
		batch = append(batch, c.queue[0])
		c.queue = c.queue[1:]
	}
	return batch
}

// sendWithRetry 带重试发送一批：指数退避，4xx（除 429）确定性失败不重试。
func (c *Client) sendWithRetry(batch []*Event) FlushResult {
	body, err := json.Marshal(envelope{
		AppID:      c.cfg.AppID,
		AppSecret:  c.cfg.AppSecret,
		ClientInfo: &clientInfo{UserAgent: c.userAgent},
		Events:     batch,
	})
	if err != nil {
		return FlushResult{Success: false, Dropped: len(batch), Message: err.Error()}
	}

	for attempt := 0; ; attempt++ {
		status, respBody, doErr := c.post(c.cfg.ReportURL(), body)
		if doErr == nil && status >= 200 && status < 300 {
			var parsed reportResponse
			failed := 0
			if json.Unmarshal(respBody, &parsed) == nil {
				failed = parsed.FailedCount
				if failed > 0 {
					c.debugf("部分事件被服务端拒绝：%s", parsed.Message)
				}
			}
			return FlushResult{Success: true, Sent: len(batch) - failed, Dropped: failed}
		}

		// 4xx（除 429）是确定性失败：整批丢弃，不重试
		if doErr == nil && status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			c.debugf("HTTP %d（不可重试），丢弃 %d 条", status, len(batch))
			return FlushResult{Success: false, Dropped: len(batch), Message: fmt.Sprintf("HTTP %d", status)}
		}

		if attempt >= c.cfg.MaxRetries {
			reason := fmt.Sprintf("HTTP %d", status)
			if doErr != nil {
				reason = doErr.Error()
			}
			c.debugf("重试耗尽（%s），%d 条待下次补发", reason, len(batch))
			return FlushResult{
				Success:   false,
				Dropped:   len(batch),
				Retryable: true,
				Message:   fmt.Sprintf("%s after %d attempts", reason, attempt+1),
			}
		}

		delay := time.Duration(int64(c.cfg.RetryBaseDelay) << uint(attempt))
		c.debugf("HTTP %d，%s 后第 %d 次重试", status, delay, attempt+1)
		time.Sleep(delay)
	}
}

func (c *Client) post(url string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func (c *Client) flushLoop() {
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if c.PendingCount() > 0 {
				go c.Flush()
			}
		}
	}
}

// Close 停止定时 flush 并冲刷剩余队列。之后 Track 不再入队。
func (c *Client) Close() FlushResult {
	// 先停定时器再冲刷：Flush 的防重入会拒绝 closed 状态下的调用
	c.stopOnce.Do(func() { close(c.stopCh) })
	r := c.Flush()
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return r
}

// ─── AB 实验开关 ────────────────────────────────────────────────────────────

// LoadFlags 拉取最新实验开关并缓存。失败时保留旧缓存并返回 error。
func (c *Client) LoadFlags() (map[string]string, error) {
	uid := c.userID.Load()
	reqBody := map[string]any{
		"appId":    c.cfg.AppID,
		"appSecret": c.cfg.AppSecret,
		"deviceId": c.cfg.DeviceID,
	}
	if uid != 0 {
		reqBody["userId"] = uid
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return c.flagSnapshot(), err
	}
	status, respBody, doErr := c.post(c.cfg.FlagsURL(), body)
	if doErr != nil {
		return c.flagSnapshot(), doErr
	}
	if status < 200 || status >= 300 {
		return c.flagSnapshot(), fmt.Errorf("uba: 拉取 flags 失败 HTTP %d", status)
	}
	var parsed flagsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return c.flagSnapshot(), err
	}
	c.mu.Lock()
	if parsed.Flags == nil {
		parsed.Flags = map[string]string{}
	}
	c.flags = parsed.Flags
	c.mu.Unlock()
	return c.flagSnapshot(), nil
}

// GetFlag 读取开关变体；命中时自动上报一次 $feature_flag 曝光事件（每会话每开关一次）。
func (c *Client) GetFlag(key string) string {
	c.mu.Lock()
	variant, ok := c.flags[key]
	c.mu.Unlock()
	if !ok {
		return ""
	}
	c.trackFlagExposure(key, variant)
	return variant
}

// IsFeatureEnabled 变体非空且非 control 即视为命中（不触发曝光事件）。
func (c *Client) IsFeatureEnabled(key string) bool {
	c.mu.Lock()
	variant, ok := c.flags[key]
	c.mu.Unlock()
	return ok && variant != "" && variant != "control"
}

func (c *Client) flagSnapshot() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.flags))
	for k, v := range c.flags {
		out[k] = v
	}
	return out
}

func (c *Client) trackFlagExposure(key, variant string) {
	sessionID := c.currentSessionID()
	c.mu.Lock()
	if c.exposedSession != sessionID {
		c.exposed = map[string]bool{}
		c.exposedSession = sessionID
	}
	first := !c.exposed[key]
	if first {
		c.exposed[key] = true
	}
	c.mu.Unlock()
	if first {
		c.track("$feature_flag", P{"flag_key": key, "variant": variant},
			&TrackOptions{EventCategory: "experiment"}, nil)
	}
}

func (c *Client) debugf(format string, args ...any) {
	if c.cfg.Debug {
		log.Printf("[uba-go] "+format, args...)
	}
}

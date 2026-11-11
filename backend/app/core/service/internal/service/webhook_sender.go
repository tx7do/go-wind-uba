package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-wind-uba/app/core/service/internal/data"
	"go-wind-uba/app/core/service/internal/data/ent"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

const (
	// webhookTimeout 单次投递超时；低频告警场景下无需长连接。
	webhookTimeout = 5 * time.Second
	// webhookMaxAttempts 总尝试次数（含首次）。
	webhookMaxAttempts = 3
	// webhookRetryBackoff 相邻尝试的固定间隔。
	webhookRetryBackoff = 2 * time.Second
	// webhookResponseTruncate 响应体落库截断长度。
	webhookResponseTruncate = 1024
)

// WebhookSender 把事件投递到匹配的网络钩子：签名、重试、投递日志一体完成。
type WebhookSender struct {
	log *log.Helper

	webhookRepo  *data.WebhookRepo
	deliveryRepo *data.WebhookDeliveryLogRepo

	httpClient *http.Client
}

func NewWebhookSender(
	ctx *bootstrap.Context,
	webhookRepo *data.WebhookRepo,
	deliveryRepo *data.WebhookDeliveryLogRepo,
) *WebhookSender {
	return &WebhookSender{
		log:          ctx.NewLoggerHelper("webhook-sender/service/core-service"),
		webhookRepo:  webhookRepo,
		deliveryRepo: deliveryRepo,
		httpClient: &http.Client{
			Timeout: webhookTimeout,
		},
	}
}

// SendPayload 投递的事件负载（data 字段内容由调用方组装）。
type SendPayload struct {
	TenantID int64
	AppID    string
	// EventType 触发事件类型，如 alert.triggered / risk.high。
	EventType string
	// Data 事件数据，序列化后作为 payload.data。
	Data any
}

// Send 把事件投递给所有匹配的启用 Webhook。单个 Webhook 失败不影响其他投递。
func (s *WebhookSender) Send(ctx context.Context, p SendPayload) {
	webhooks, err := s.webhookRepo.ListEnabledByTenantApp(ctx, p.TenantID, p.AppID)
	if err != nil {
		s.log.Errorf("list webhooks for delivery failed: %s", err.Error())
		return
	}

	for _, hook := range webhooks {
		if !webhookMatches(hook, p.EventType) {
			continue
		}

		go s.deliver(hook, p)
	}
}

// webhookMatches 判断 Webhook 是否订阅了该事件类型（未配置 event_types 视为订阅全部）。
func webhookMatches(hook *ent.Webhook, eventType string) bool {
	if len(hook.EventTypes) == 0 {
		return true
	}
	return slices.Contains(hook.EventTypes, eventType)
}

// deliver 对单个 Webhook 执行投递（含重试与日志），独立 goroutine 运行。
func (s *WebhookSender) deliver(hook *ent.Webhook, p SendPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout*time.Duration(webhookMaxAttempts))
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"eventType": p.EventType,
		"tenantId":  p.TenantID,
		"appId":     p.AppID,
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      p.Data,
	})
	if err != nil {
		s.log.Errorf("marshal webhook payload failed: %s", err.Error())
		return
	}

	var (
		lastStatus int
		lastBody   string
		lastErr    string
		success    bool
	)

	for attempt := 1; attempt <= webhookMaxAttempts; attempt++ {
		lastStatus, lastBody, lastErr = s.post(ctx, hook, p.EventType, body)
		if lastErr == "" && lastStatus >= 200 && lastStatus < 300 {
			success = true
			break
		}
		if attempt < webhookMaxAttempts {
			time.Sleep(webhookRetryBackoff)
		}
	}

	if p.TenantID != 0 {
		s.recordDelivery(hook, p, body, success, webhookMaxAttempts-1, lastStatus, lastBody, lastErr)
	}

	if success {
		s.markTriggered(hook)
	} else {
		s.markFailed(hook, lastErr)
	}
}

// post 执行一次 HTTP 投递，返回 (状态码, 响应体截断, 错误)。
func (s *WebhookSender) post(ctx context.Context, hook *ent.Webhook, eventType string, body []byte) (int, string, string) {
	if hook.URL == nil || *hook.URL == "" {
		return 0, "", "webhook url is empty"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *hook.URL, bytes.NewReader(body))
	if err != nil {
		return 0, "", "build request failed: " + err.Error()
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Uba-Event-Type", eventType)
	req.Header.Set("X-Uba-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-Uba-Signature", "sha256="+s.sign(derefStr(hook.Secret), body))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", err.Error()
	}
	defer resp.Body.Close()

	buf := make([]byte, webhookResponseTruncate)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n]), ""
}

// sign 计算 HMAC-SHA256 签名；未配置密钥时返回空签名。
func (s *WebhookSender) sign(secret string, body []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *WebhookSender) recordDelivery(hook *ent.Webhook, p SendPayload, body []byte, success bool, retryCount int64, statusCode int, responseBody, errMsg string) {
	err := s.deliveryRepo.Create(context.Background(), &ubaV1.WebhookDeliveryLog{
		WebhookId:    hook.ID,
		TenantId:     p.TenantID,
		EventType:    p.EventType,
		Payload:      string(body),
		StatusCode:   int32(statusCode),
		ResponseBody: responseBody,
		RetryCount:   uint32(retryCount),
		Success:      success,
		ErrorMessage: errMsg,
	})
	if err != nil {
		s.log.Errorf("record webhook delivery log failed: %s", err.Error())
	}
}

// markTriggered 投递成功后更新最后触发时间并清零失败计数。
func (s *WebhookSender) markTriggered(hook *ent.Webhook) {
	now := time.Now()
	if err := s.webhookRepo.UpdateDeliveryState(context.Background(), hook.ID, &now, nil); err != nil {
		s.log.Errorf("update webhook trigger state failed: %s", err.Error())
	}
}

// markFailed 投递失败后累加失败计数。
func (s *WebhookSender) markFailed(hook *ent.Webhook, _ string) {
	count := int64(hook.FailureCount) + 1
	if err := s.webhookRepo.UpdateDeliveryState(context.Background(), hook.ID, nil, &count); err != nil {
		s.log.Errorf("update webhook failure count failed: %s", err.Error())
	}
}

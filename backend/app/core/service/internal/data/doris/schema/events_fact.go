package schema

import "time"

type EventsFact struct {
	EventID       *string            `json:"event_id"`
	TenantID      *uint32            `json:"tenant_id"`
	UserID        *uint32            `json:"user_id"`
	DeviceID      *string            `json:"device_id"`
	AccountID     *string            `json:"account_id"`
	GlobalUserID  *string            `json:"global_user_id"`
	EventTime     *time.Time         `json:"event_time"`
	EventTs       *int64             `json:"-"`
	ServerTime    *time.Time         `json:"server_time"`
	EventCategory *string            `json:"event_category"`
	EventName     *string            `json:"event_name"`
	EventAction   *string            `json:"event_action"`
	ObjectType    *string            `json:"object_type"`
	ObjectID      *string            `json:"object_id"`
	ObjectName    *string            `json:"object_name"`
	SessionID     *string            `json:"session_id"`
	SessionSeq    *uint32            `json:"session_seq"`
	Platform      *string            `json:"platform"`
	Os            *string            `json:"os"`
	AppVersion    *string            `json:"app_version"`
	Channel       *string            `json:"channel"`
	UserAgent     *string            `json:"user_agent"`
	Ip            *string            `json:"ip"`
	IpCity        *string            `json:"ip_city"`
	Country       *string            `json:"country"`
	Geo           *string            `json:"geo"`
	Network       *string            `json:"network"`
	Referer       *string            `json:"referer"`
	Context       map[string]string  `json:"context"`
	Metrics       map[string]float64 `json:"metrics"`
	Properties    map[string]string  `json:"properties"`
	OpResult      *string            `json:"op_result"`
	RiskLevel     *string            `json:"risk_level"`
	ErrorCode     *string            `json:"error_code"`
	Score         *int32             `json:"score"`
	Quantity      *uint32            `json:"quantity"`
	Amount        *uint32            `json:"amount"`
	DurationMs    *uint32            `json:"duration_ms"`
}

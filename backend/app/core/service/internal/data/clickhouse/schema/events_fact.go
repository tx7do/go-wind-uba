package schema

import "time"

// 事件事实表
// 对应表：gw_uba.events_fact

type EventsFact struct {
	EventID       *string            `ch:"event_id"`
	TenantID      *uint32            `ch:"tenant_id"`
	UserID        *uint32            `ch:"user_id"`
	DeviceID      *string            `ch:"device_id"`
	AccountID     *string            `ch:"account_id"`
	GlobalUserID  *string            `ch:"global_user_id"`
	EventTime     *time.Time         `ch:"event_time"`
	EventDate     *time.Time         `ch:"-"`
	EventTs       *int64             `ch:"-"`
	ServerTime    *time.Time         `ch:"server_time"`
	EventCategory *string            `ch:"event_category"`
	EventName     *string            `ch:"event_name"`
	EventAction   *string            `ch:"event_action"`
	ObjectType    *string            `ch:"object_type"`
	ObjectID      *string            `ch:"object_id"`
	ObjectName    *string            `ch:"object_name"`
	SessionID     *string            `ch:"session_id"`
	SessionSeq    *uint32            `ch:"session_seq"`
	Platform      *string            `ch:"platform"`
	Os            *string            `ch:"os"`
	AppVersion    *string            `ch:"app_version"`
	Channel       *string            `ch:"channel"`
	UserAgent     *string            `ch:"user_agent"`
	Ip            *string            `ch:"ip"`
	IpCity        *string            `ch:"ip_city"`
	Country       *string            `ch:"country"`
	Geo           *string            `ch:"geo"`
	Network       *string            `ch:"network"`
	Referer       *string            `ch:"referer"`
	Context       map[string]string  `ch:"context"`
	Metrics       map[string]float64 `ch:"metrics"`
	Properties    map[string]string  `ch:"properties"`
	OpResult      *string            `ch:"op_result"`
	RiskLevel     *string            `ch:"risk_level"`
	ErrorCode     *string            `ch:"error_code"`
	Score         *int32             `ch:"score"`
	Quantity      *uint32            `ch:"quantity"`
	Amount        *uint32            `ch:"amount"`
	DurationMs    *uint32            `ch:"duration_ms"`
}

package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type EventsFact struct {
	EventID       *string          `db:"event_id"`
	TenantID      *uint32          `db:"tenant_id"`
	UserID        *uint32          `db:"user_id"`
	DeviceID      *string          `db:"device_id"`
	AccountID     *string          `db:"account_id"`
	GlobalUserID  *string          `db:"global_user_id"`
	EventTime     *time.Time       `db:"event_time"`
	EventTs       *int64           `db:"event_ts,readonly"`
	ServerTime    *time.Time       `db:"server_time"`
	EventCategory *string          `db:"event_category"`
	EventName     *string          `db:"event_name"`
	EventAction   *string          `db:"event_action"`
	ObjectType    *string          `db:"object_type"`
	ObjectID      *string          `db:"object_id"`
	ObjectName    *string          `db:"object_name"`
	SessionID     *string          `db:"session_id"`
	SessionSeq    *uint32          `db:"session_seq"`
	Platform      *string          `db:"platform"`
	Os            *string          `db:"os"`
	AppVersion    *string          `db:"app_version"`
	Channel       *string          `db:"channel"`
	UserAgent     *string          `db:"user_agent"`
	Ip            *string          `db:"ip"`
	IpCity        *string          `db:"ip_city"`
	Country       *string          `db:"country"`
	Geo           *string          `db:"geo"`
	Network       *string          `db:"network"`
	Referer       *string          `db:"referer"`
	Context       MapStringString  `db:"context"`
	Metrics       MapStringFloat64 `db:"metrics"`
	Properties    MapStringString  `db:"properties"`
	OpResult      *string          `db:"op_result"`
	RiskLevel     *string          `db:"risk_level"`
	ErrorCode     *string          `db:"error_code"`
	TraceID       *string          `db:"trace_id"`
	Score         *int32           `db:"score"`
	Quantity      *uint32          `db:"quantity"`
	Amount        *decimal.Decimal `db:"amount"`
	DurationMs    *uint32          `db:"duration_ms"`
	CreatedAt     *time.Time       `db:"created_at"`
	UpdatedAt     *time.Time       `db:"updated_at"`
}

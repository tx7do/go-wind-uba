package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type SessionsFact struct {
	ID            *uint64           `ch:"id"`
	TenantID      *uint32           `json:"tenant_id"`
	UserID        *uint32           `json:"user_id"`
	DeviceID      *string           `json:"device_id"`
	GlobalUserID  *string           `json:"global_user_id"`
	StartTime     *time.Time        `json:"start_time"`
	EndTime       *time.Time        `json:"end_time"`
	SessionDate   *time.Time        `json:"session_date"`
	DurationMs    *uint64           `json:"duration_ms"`
	EventCount    *uint32           `json:"event_count"`
	PageViewCount *uint32           `json:"page_view_count"`
	ActionCount   *uint32           `json:"action_count"`
	EntryPage     *string           `json:"entry_page"`
	ExitPage      *string           `json:"exit_page"`
	IsBounce      *uint8            `json:"is_bounce"`
	Platform      *string           `json:"platform"`
	Os            *string           `json:"os"`
	AppVersion    *string           `json:"app_version"`
	IpCity        *string           `json:"ip_city"`
	Country       *string           `json:"country"`
	TotalAmount   *decimal.Decimal  `json:"total_amount"`
	PayEventCount *uint32           `json:"pay_event_count"`
	RiskLevel     *string           `json:"risk_level"`
	RiskTags      []string          `json:"risk_tags"`
	Context       map[string]string `json:"context"`
	CreatedAt     *time.Time        `json:"created_at"`
	UpdatedAt     *time.Time        `json:"updated_at"`
}

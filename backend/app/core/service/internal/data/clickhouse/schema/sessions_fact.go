package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

// 会话事实表
// 对应表：gw_uba.sessions_fact

type SessionsFact struct {
	SessionID *string `ch:"session_id"`

	TenantID     *int64  `ch:"tenant_id"`
	UserID       *int64  `ch:"user_id"`
	DeviceID     *string `ch:"device_id"`
	GlobalUserID *string `ch:"global_user_id"`

	StartTime   *time.Time `ch:"start_time"`
	EndTime     *time.Time `ch:"end_time"`
	SessionDate *time.Time `ch:"session_date"`
	DurationMs  *int64     `ch:"duration_ms"`

	EventCount    *int64  `ch:"event_count"`
	PageViewCount *int64  `ch:"page_view_count"`
	ActionCount   *int64  `ch:"action_count"`
	EntryPage     *string `ch:"entry_page"`
	ExitPage      *string `ch:"exit_page"`
	IsBounce      *uint8  `ch:"is_bounce"`

	Platform   *string `ch:"platform"`
	Os         *string `ch:"os"`
	AppVersion *string `ch:"app_version"`
	IpCity     *string `ch:"ip_city"`
	Country    *string `ch:"country"`

	TotalAmount   *decimal.Decimal `ch:"total_amount"`
	PayEventCount *int64           `ch:"pay_event_count"`

	RiskLevel *string  `ch:"risk_level"`
	RiskTags  []string `ch:"risk_tags"`

	Context map[string]string `ch:"context"`

	CreatedAt *time.Time `ch:"created_at"`
	UpdatedAt *time.Time `ch:"updated_at"`
}

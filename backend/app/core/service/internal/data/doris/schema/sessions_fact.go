package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type SessionsFact struct {
	SessionID *string `db:"session_id"`

	TenantID     *int64  `db:"tenant_id"`
	UserID       *int64  `db:"user_id"`
	DeviceID     *string `db:"device_id"`
	GlobalUserID *string `db:"global_user_id"`

	StartTime   *time.Time `db:"start_time"`
	EndTime     *time.Time `db:"end_time"`
	SessionDate *time.Time `db:"session_date"`
	DurationMs  *int64     `db:"duration_ms"`

	EventCount    *int64  `db:"event_count"`
	PageViewCount *int64  `db:"page_view_count"`
	ActionCount   *int64  `db:"action_count"`
	EntryPage     *string `db:"entry_page"`
	ExitPage      *string `db:"exit_page"`
	IsBounce      *uint8  `db:"is_bounce"`

	Platform   *string `db:"platform"`
	Os         *string `db:"os"`
	AppVersion *string `db:"app_version"`
	IpCity     *string `db:"ip_city"`
	Country    *string `db:"country"`

	TotalAmount   *decimal.Decimal `db:"total_amount"`
	PayEventCount *int64           `db:"pay_event_count"`

	RiskLevel *string     `db:"risk_level"`
	RiskTags  StringArray `db:"risk_tags"`

	Context MapStringString `db:"context"`

	CreatedAt *time.Time `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

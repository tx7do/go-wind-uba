package schema

import "time"

// 会话事实表
// 对应表：gw_uba.sessions_fact

type SessionsFact struct {
	SessionID     *uint32           `ch:"session_id"`
	TenantID      *uint32           `ch:"tenant_id"`
	UserID        *uint32           `ch:"user_id"`
	DeviceID      *string           `ch:"device_id"`
	GlobalUserID  *string           `ch:"global_user_id"`
	StartTime     *time.Time        `ch:"start_time"`
	EndTime       *time.Time        `ch:"end_time"`
	SessionDate   *time.Time        `ch:"session_date"`
	DurationMs    *uint64           `ch:"duration_ms"`
	EventCount    *uint32           `ch:"event_count"`
	PageViewCount *uint32           `ch:"page_view_count"`
	ActionCount   *uint32           `ch:"action_count"`
	EntryPage     *string           `ch:"entry_page"`
	ExitPage      *string           `ch:"exit_page"`
	IsBounce      *uint8            `ch:"is_bounce"`
	Platform      *string           `ch:"platform"`
	Os            *string           `ch:"os"`
	AppVersion    *string           `ch:"app_version"`
	IpCity        *string           `ch:"ip_city"`
	Country       *string           `ch:"country"`
	TotalAmount   *float64          `ch:"total_amount"`
	PayEventCount *uint32           `ch:"pay_event_count"`
	RiskLevel     *string           `ch:"risk_level"`
	RiskTags      []string          `ch:"risk_tags"`
	Context       map[string]string `ch:"context"`
	CreatedAt     *time.Time        `ch:"created_at"`
	UpdatedAt     *time.Time        `ch:"updated_at"`
}

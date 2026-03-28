package schema

import (
	"time"
)

type RiskEvents struct {
	RiskEventID     *string         `db:"risk_event_id"`
	TenantID        *uint32         `db:"tenant_id"`
	UserID          *uint32         `db:"user_id"`
	DeviceID        *string         `db:"device_id"`
	GlobalUserID    *string         `db:"global_user_id"`
	RiskType        *string         `db:"risk_type"`
	RiskLevel       *string         `db:"risk_level"`
	RiskScore       *float32        `db:"risk_score"`
	RuleID          *uint32         `db:"rule_id"`
	RuleName        *string         `db:"rule_name"`
	RuleContext     MapStringString `db:"rule_context"`
	RelatedEventIDs StringArray     `db:"related_event_ids"`
	SessionID       *string         `db:"session_id"`
	Description     *string         `db:"description"`
	Evidence        MapStringString `db:"evidence"`
	Status          *string         `db:"status"`
	HandlerID       *string         `db:"handler_id"`
	HandledTime     *time.Time      `db:"handled_time"`
	HandleRemark    *string         `db:"handle_remark"`
	OccurTime       *time.Time      `db:"occur_time"`
	ReportTime      *time.Time      `db:"report_time"`
	EventDate       *time.Time      `db:"event_date"`
	CreatedAt       *time.Time      `db:"created_at"`
	UpdatedAt       *time.Time      `db:"updated_at"`
}

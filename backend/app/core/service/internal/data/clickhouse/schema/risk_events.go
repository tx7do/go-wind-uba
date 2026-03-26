package schema

import (
	"time"
)

// 风险事件表
// 对应表：gw_uba.risk_events

type RiskEvents struct {
	ID              *uint64           `ch:"id"`
	TenantID        *uint32           `ch:"tenant_id"`
	UserID          *uint32           `ch:"user_id"`
	DeviceID        *string           `ch:"device_id"`
	GlobalUserID    *string           `ch:"global_user_id"`
	RiskType        *string           `ch:"risk_type"`
	RiskLevel       *string           `ch:"risk_level"`
	RiskScore       *float32          `ch:"risk_score"`
	RuleID          *uint32           `ch:"rule_id"`
	RuleName        *string           `ch:"rule_name"`
	RuleContext     map[string]string `ch:"rule_context"`
	RelatedEventIDs []string          `ch:"related_event_ids"`
	SessionID       *uint64           `ch:"session_id"`
	Description     *string           `ch:"description"`
	Evidence        map[string]string `ch:"evidence"`
	Status          *string           `ch:"status"`
	HandlerID       *string           `ch:"handler_id"`
	HandledTime     *time.Time        `ch:"handled_time"`
	HandleRemark    *string           `ch:"handle_remark"`
	OccurTime       *time.Time        `ch:"occur_time"`
	ReportTime      *time.Time        `ch:"report_time"`
	EventDate       *time.Time        `ch:"event_date"`
	CreatedAt       *time.Time        `ch:"created_at"`
	UpdatedAt       *time.Time        `ch:"updated_at"`
}

package schema

import "time"

type RiskEvents struct {
	RiskID          *string           `json:"risk_id"`
	TenantID        *uint32           `json:"tenant_id"`
	UserID          *uint32           `json:"user_id"`
	DeviceID        *string           `json:"device_id"`
	GlobalUserID    *string           `json:"global_user_id"`
	RiskType        *string           `json:"risk_type"`
	RiskLevel       *string           `json:"risk_level"`
	RiskScore       *float32          `json:"risk_score"`
	RuleID          *uint32           `json:"rule_id"`
	RuleName        *string           `json:"rule_name"`
	RuleContext     map[string]string `json:"rule_context"`
	RelatedEventIDs []string          `json:"related_event_ids"`
	SessionID       *uint32           `json:"session_id"`
	Description     *string           `json:"description"`
	Evidence        map[string]string `json:"evidence"`
	Status          *string           `json:"status"`
	HandlerID       *string           `json:"handler_id"`
	HandledTime     *time.Time        `json:"handled_time"`
	HandleRemark    *string           `json:"handle_remark"`
	OccurTime       *time.Time        `json:"occur_time"`
	ReportTime      *time.Time        `json:"report_time"`
	EventDate       *time.Time        `json:"event_date"`
	CreatedAt       *time.Time        `json:"created_at"`
	UpdatedAt       *time.Time        `json:"updated_at"`
}

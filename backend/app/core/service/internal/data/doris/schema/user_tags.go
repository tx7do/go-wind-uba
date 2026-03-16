package schema

import "time"

type UserTags struct {
	TenantID      *uint32    `json:"tenant_id"`
	UserID        *uint32    `json:"user_id"`
	TagID         *uint32    `json:"tag_id"`
	TagValue      *string    `json:"tag_value"`
	ValueLabel    *string    `json:"value_label"`
	Confidence    *float32   `json:"confidence"`
	Source        *string    `json:"source"`
	SourceRuleID  *uint32    `json:"source_rule_id"`
	EffectiveTime *time.Time `json:"effective_time"`
	ExpireTime    *time.Time `json:"expire_time"`
	ExpireDate    *time.Time `json:"expire_date"`
	IsActive      *uint8     `json:"is_active"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

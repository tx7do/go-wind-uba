package schema

import "time"

// 用户标签表
// 对应表：gw_uba.user_tags

type UserTags struct {
	TenantID      *int64     `ch:"tenant_id"`
	UserID        *int64     `ch:"user_id"`
	TagID         *int64     `ch:"tag_id"`
	TagValue      *string    `ch:"tag_value"`
	ValueLabel    *string    `ch:"value_label"`
	Confidence    *float32   `ch:"confidence"`
	Source        *string    `ch:"source"`
	SourceRuleID  *int64     `ch:"source_rule_id"`
	EffectiveTime *time.Time `ch:"effective_time"`
	ExpireTime    *time.Time `ch:"expire_time"`
	ExpireDate    *time.Time `ch:"expire_date"`
	IsActive      *uint8     `ch:"is_active"`
	CreatedAt     *time.Time `ch:"created_at"`
	UpdatedAt     *time.Time `ch:"updated_at"`
}

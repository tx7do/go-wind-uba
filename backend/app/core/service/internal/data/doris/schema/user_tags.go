package schema

import "time"

type UserTags struct {
	TenantID      *uint32    `db:"tenant_id"`
	UserID        *uint32    `db:"user_id"`
	TagID         *uint32    `db:"tag_id"`
	TagValue      *string    `db:"tag_value"`
	ValueLabel    *string    `db:"value_label"`
	Confidence    *float32   `db:"confidence"`
	Source        *string    `db:"source"`
	SourceRuleID  *uint32    `db:"source_rule_id"`
	EffectiveTime *time.Time `db:"effective_time"`
	ExpireTime    *time.Time `db:"expire_time"`
	ExpireDate    *time.Time `db:"expire_date"`
	IsActive      *uint8     `db:"is_active"`
	CreatedAt     *time.Time `db:"created_at"`
	UpdatedAt     *time.Time `db:"updated_at"`
}

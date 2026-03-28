package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type UsersDim struct {
	TenantID         *uint32          `db:"tenant_id"`
	UserID           *uint32          `db:"user_id"`
	RegisterTime     *time.Time       `db:"register_time"`
	RegisterChannel  *string          `db:"register_channel"`
	FirstActiveDate  *time.Time       `db:"first_active_date"`
	LastActiveDate   *time.Time       `db:"last_active_date"`
	UserLevel        *uint16          `db:"user_level"`
	VipLevel         *uint8           `db:"vip_level"`
	UserRole         *string          `db:"user_role"`
	TotalEvents      *uint64          `db:"total_events"`
	TotalSessions    *uint32          `db:"total_sessions"`
	TotalPayAmount   *decimal.Decimal `db:"total_pay_amount"`
	LastPayTime      *time.Time       `db:"last_pay_time"`
	PreferCategories StringArray      `db:"prefer_categories"`
	PreferObjects    StringArray      `db:"prefer_objects"`
	RiskScore        *uint8           `db:"risk_score"`
	RiskLevel        *string          `db:"risk_level"`
	RiskTags         StringArray      `db:"risk_tags"`
	LastRiskTime     *time.Time       `db:"last_risk_time"`
	Profile          MapStringString  `db:"profile"`
	Geo              MapStringString  `db:"geo"`
	DeviceType       *string          `db:"device_type"`
	Platform         *string          `db:"platform"`
	Country          *string          `db:"country"`
	Ver              *uint64          `db:"ver"`
	CreatedAt        *time.Time       `db:"created_at"`
	UpdatedAt        *time.Time       `db:"updated_at"`
}

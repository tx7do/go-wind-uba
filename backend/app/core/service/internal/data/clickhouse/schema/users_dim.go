package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

// 用户维度表
// 对应表：gw_uba.users_dim

type UsersDim struct {
	TenantID         *uint32           `ch:"tenant_id"`
	UserID           *uint32           `ch:"user_id"`
	RegisterTime     *time.Time        `ch:"register_time"`
	RegisterChannel  *string           `ch:"register_channel"`
	FirstActiveDate  *time.Time        `ch:"first_active_date"`
	LastActiveDate   *time.Time        `ch:"last_active_date"`
	UserLevel        *uint16           `ch:"user_level"`
	VipLevel         *uint8            `ch:"vip_level"`
	UserRole         *string           `ch:"user_role"`
	TotalEvents      *uint64           `ch:"total_events"`
	TotalSessions    *uint32           `ch:"total_sessions"`
	TotalPayAmount   *decimal.Decimal  `ch:"total_pay_amount"`
	LastPayTime      *time.Time        `ch:"last_pay_time"`
	PreferCategories []string          `ch:"prefer_categories"`
	PreferObjects    []string          `ch:"prefer_objects"`
	RiskScore        *uint8            `ch:"risk_score"`
	RiskLevel        *string           `ch:"risk_level"`
	RiskTags         []string          `ch:"risk_tags"`
	LastRiskTime     *time.Time        `ch:"last_risk_time"`
	Profile          map[string]string `ch:"profile"`
	Geo              map[string]string `ch:"geo"`
	DeviceType       *string           `ch:"device_type"`
	Platform         *string           `ch:"platform"`
	Country          *string           `ch:"country"`
	Ver              *uint64           `ch:"ver"`
	CreatedAt        *time.Time        `ch:"created_at"`
	UpdatedAt        *time.Time        `ch:"updated_at"`
}

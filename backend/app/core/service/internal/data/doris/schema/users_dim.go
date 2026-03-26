package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type UsersDim struct {
	TenantID         *uint32           `json:"tenant_id"`
	UserID           *uint32           `json:"user_id"`
	RegisterTime     *time.Time        `json:"register_time"`
	RegisterChannel  *string           `json:"register_channel"`
	FirstActiveDate  *time.Time        `json:"first_active_date"`
	LastActiveDate   *time.Time        `json:"last_active_date"`
	UserLevel        *uint16           `json:"user_level"`
	VipLevel         *uint8            `json:"vip_level"`
	UserRole         *string           `json:"user_role"`
	TotalEvents      *uint64           `json:"total_events"`
	TotalSessions    *uint32           `json:"total_sessions"`
	TotalPayAmount   *decimal.Decimal  `json:"total_pay_amount"`
	LastPayTime      *time.Time        `json:"last_pay_time"`
	PreferCategories []string          `json:"prefer_categories"`
	PreferObjects    []string          `json:"prefer_objects"`
	RiskScore        *uint8            `json:"risk_score"`
	RiskLevel        *string           `json:"risk_level"`
	RiskTags         []string          `json:"risk_tags"`
	LastRiskTime     *time.Time        `json:"last_risk_time"`
	Profile          map[string]string `json:"profile"`
	Geo              map[string]string `json:"geo"`
	DeviceType       *string           `json:"device_type"`
	Platform         *string           `ch:"platform"`
	Ver              *uint64           `json:"ver"`
	CreatedAt        *time.Time        `json:"created_at"`
	UpdatedAt        *time.Time        `json:"updated_at"`
}

package schema

import "time"

// ID映射表
// 对应表：gw_uba.id_mapping

type IDMapping struct {
	GlobalUserID string    `ch:"global_user_id"`
	TenantID     uint32    `ch:"tenant_id"`
	IDType       string    `ch:"id_type"`
	IDValue      string    `ch:"id_value"`
	Confidence   float32   `ch:"confidence"`
	LinkSource   string    `ch:"link_source"`
	FirstSeen    time.Time `ch:"first_seen"`
	LastSeen     time.Time `ch:"last_seen"`
	IsActive     uint8     `ch:"is_active"`
	CreatedAt    time.Time `ch:"created_at"`
	UpdatedAt    time.Time `ch:"updated_at"`
	UpdatedDate  time.Time `ch:"updated_date"`
}

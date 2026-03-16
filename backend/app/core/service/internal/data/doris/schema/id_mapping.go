package schema

import "time"

type IDMapping struct {
	GlobalUserID *string    `json:"global_user_id"`
	TenantID     *uint32    `json:"tenant_id"`
	IDType       *string    `json:"id_type"`
	IDValue      *string    `json:"id_value"`
	Confidence   *float32   `json:"confidence"`
	LinkSource   *string    `json:"link_source"`
	FirstSeen    *time.Time `json:"first_seen"`
	LastSeen     *time.Time `json:"last_seen"`
	IsActive     *uint8     `json:"is_active"`
	CreatedAt    *time.Time `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
	UpdatedDate  *time.Time `json:"updated_date"`
}

package schema

import "time"

type IDMapping struct {
	GlobalUserID *string    `db:"global_user_id"`
	TenantID     *uint32    `db:"tenant_id"`
	IDType       *string    `db:"id_type"`
	IDValue      *string    `db:"id_value"`
	Confidence   *float32   `db:"confidence"`
	LinkSource   *string    `db:"link_source"`
	FirstSeen    *time.Time `db:"first_seen"`
	LastSeen     *time.Time `db:"last_seen"`
	IsActive     *uint8     `db:"is_active"`
	CreatedAt    *time.Time `db:"created_at"`
	UpdatedAt    *time.Time `db:"updated_at"`
	UpdatedDate  *time.Time `db:"updated_date"`
}

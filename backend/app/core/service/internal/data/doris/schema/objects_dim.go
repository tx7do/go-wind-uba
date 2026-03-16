package schema

import "time"

type ObjectsDim struct {
	TenantID     *uint32           `json:"tenant_id"`
	ObjectType   *string           `json:"object_type"`
	ObjectID     *string           `json:"object_id"`
	ObjectName   *string           `json:"object_name"`
	CategoryPath *string           `json:"category_path"`
	Price        *float64          `json:"price"`
	Currency     *string           `json:"currency"`
	Rarity       *string           `json:"rarity"`
	Attributes   map[string]string `json:"attributes"`
	Status       *string           `json:"status"`
	ValidFrom    *time.Time        `json:"valid_from"`
	ValidTo      *time.Time        `json:"valid_to"`
	CreatedAt    *time.Time        `json:"created_at"`
	UpdatedAt    *time.Time        `json:"updated_at"`
}

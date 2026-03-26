package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type ObjectsDim struct {
	TenantID     *uint32           `json:"tenant_id"`
	ID           *string           `json:"id"`
	ObjectType   *string           `json:"object_type"`
	ObjectName   *string           `json:"object_name"`
	CategoryPath *string           `json:"category_path"`
	Price        *decimal.Decimal  `json:"price"`
	Currency     *string           `json:"currency"`
	Rarity       *string           `json:"rarity"`
	Attributes   map[string]string `json:"attributes"`
	Status       *string           `json:"status"`
	ValidFrom    *time.Time        `json:"valid_from"`
	ValidTo      *time.Time        `json:"valid_to"`
	CreatedAt    *time.Time        `json:"created_at"`
	UpdatedAt    *time.Time        `json:"updated_at"`
}

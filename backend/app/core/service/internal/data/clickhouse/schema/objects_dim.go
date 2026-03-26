package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

// 对象维度表
// 对应表：gw_uba.objects_dim

type ObjectsDim struct {
	TenantID     *uint32           `ch:"tenant_id"`
	ID           *string           `ch:"id"`
	ObjectType   *string           `ch:"object_type"`
	ObjectName   *string           `ch:"object_name"`
	CategoryPath *string           `ch:"category_path"`
	Price        *decimal.Decimal  `ch:"price"`
	Currency     *string           `ch:"currency"`
	Rarity       *string           `ch:"rarity"`
	Attributes   map[string]string `ch:"attributes"`
	Status       *string           `ch:"status"`
	ValidFrom    *time.Time        `ch:"valid_from"`
	ValidTo      *time.Time        `ch:"valid_to"`
	CreatedAt    *time.Time        `ch:"created_at"`
	UpdatedAt    *time.Time        `ch:"updated_at"`
}

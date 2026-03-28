package schema

import (
	"time"

	"github.com/shopspring/decimal"
)

type ObjectsDim struct {
	TenantID     *uint32          `db:"tenant_id"`
	ObjectID     *string          `db:"object_id"`
	ObjectType   *string          `db:"object_type"`
	ObjectName   *string          `db:"object_name"`
	CategoryPath *string          `db:"category_path"`
	Price        *decimal.Decimal `db:"price"`
	Currency     *string          `db:"currency"`
	Rarity       *string          `db:"rarity"`
	Attributes   MapStringString  `db:"attributes"`
	Status       *string          `db:"status"`
	ValidFrom    *time.Time       `db:"valid_from"`
	ValidTo      *time.Time       `db:"valid_to"`
	CreatedAt    *time.Time       `db:"created_at"`
	UpdatedAt    *time.Time       `db:"updated_at"`
}

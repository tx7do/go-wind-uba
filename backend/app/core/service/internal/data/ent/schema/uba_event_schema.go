package schema

import (
	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// EventSchema holds the schema definition for the EventSchema entity.
// 用于事件元数据管理：登记合法事件名、属性与类型，配合上报校验。
type EventSchema struct {
	ent.Schema
}

func (EventSchema) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_event_schemas",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA事件Schema定义表"),
	}
}

func (EventSchema) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_name").
			Comment("事件名，唯一业务标识，对应 events_fact.event_name").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("display_name").
			Comment("显示名").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("category").
			Comment("事件类别").
			Optional().
			Nillable(),

		field.String("description").
			Comment("事件描述").
			Optional().
			Nillable(),

		// 属性 schema 列表，结构化存储
		field.JSON("properties", []*ubaV1.EventPropertySchema{}).
			Comment("属性 schema 列表").
			Optional(),

		field.String("status").
			Comment("启用状态：ENABLED/DISABLED").
			Default("ENABLED").
			Optional().
			Nillable(),
	}
}

func (EventSchema) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID[uint32]{},
	}
}

func (EventSchema) Indexes() []ent.Index {
	return []ent.Index{
		// 同一租户下事件名唯一
		index.Fields("tenant_id", "event_name").
			Unique().
			StorageKey("uix_uba_event_schemas_tenant_event_name"),

		index.Fields("tenant_id").
			StorageKey("idx_uba_event_schemas_tenant_id"),

		index.Fields("category").
			StorageKey("idx_uba_event_schemas_category"),

		index.Fields("status").
			StorageKey("idx_uba_event_schemas_status"),

		index.Fields("created_at").
			StorageKey("idx_uba_event_schemas_created_at"),
	}
}

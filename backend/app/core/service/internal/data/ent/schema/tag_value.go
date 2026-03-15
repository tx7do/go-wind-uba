package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// TagValue holds the schema definition for the TagValue entity.
type TagValue struct {
	ent.Schema
}

func (TagValue) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_tag_values",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA标签值表"),
	}
}

func (TagValue) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tag_id").
			Comment("标签定义ID，关联 uba_tag_definitions.id").
			Optional().
			Nillable(),

		field.String("value").
			Comment("标签值，业务唯一标识").
			NotEmpty(),

		field.String("label").
			Comment("显示名称").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述").
			Optional().
			Nillable(),

		field.String("color").
			Comment("颜色标识").
			Optional().
			Nillable(),

		field.String("icon").
			Comment("图标").
			Optional().
			Nillable(),
	}
}

func (TagValue) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.SortOrder{},
	}
}

func (TagValue) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：同一租户下同一标签定义下 value 唯一
		index.Fields("tenant_id", "tag_id", "value").
			Unique().
			StorageKey("idx_tag_value"),
	}
}

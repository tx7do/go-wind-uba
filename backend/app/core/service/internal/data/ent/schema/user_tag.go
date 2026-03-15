package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// UserTag holds the schema definition for the UserTag entity.
type UserTag struct {
	ent.Schema
}

func (UserTag) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_user_tags",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("用户标签关联表"),
	}
}

func (UserTag) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("user_id").
			Comment("业务用户ID").
			Optional().
			Nillable(),

		field.Uint32("tag_id").
			Comment("标签定义ID，关联 uba_tag_definitions.id").
			Optional().
			Nillable(),

		field.String("value").
			Comment("标签值，实际存储值").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("value_label").
			Comment("显示名称").
			Optional().
			Nillable(),

		field.Float("confidence").
			Comment("置信度，算法打标").
			Default(1.0).
			Optional().
			Nillable(),

		field.Enum("source").
			Comment("标签来源").
			NamedValues(
				"TagSourceManual", "TAG_SOURCE_MANUAL",
				"TagSourceRule", "TAG_SOURCE_RULE",
				"TagSourceModel", "TAG_SOURCE_MODEL",
				"TagSourceImport", "TAG_SOURCE_IMPORT",
			).
			Optional().
			Nillable(),

		field.Uint32("source_rule_id").
			Comment("来源规则ID，关联规则表").
			Optional().
			Nillable(),

		field.Time("effective_time").
			Comment("生效时间").
			Optional().
			Nillable(),

		field.Time("expire_time").
			Comment("过期时间，NULL表示永久").
			Optional().
			Nillable(),

		field.Bool("is_active").
			Comment("是否激活，1为激活，0为未激活").
			Default(true).
			Optional().
			Nillable(),
	}
}

func (UserTag) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
		mixin.OperatorID{},
	}
}

func (UserTag) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id + user_id
		index.Fields("tenant_id", "user_id").
			StorageKey("idx_tenant_user"),

		// 索引：tenant_id + tag_id
		index.Fields("tenant_id", "tag_id").
			StorageKey("idx_tag_id"),

		// 索引：tenant_id + is_active
		index.Fields("tenant_id", "is_active").
			StorageKey("idx_user_tag_active"),

		// 索引：tenant_id + expire_time + is_active
		index.Fields("tenant_id", "expire_time", "is_active").
			StorageKey("idx_expire"),

		// 唯一索引：tenant_id + user_id + tag_id + effective_time
		index.Fields("tenant_id", "user_id", "tag_id", "effective_time").
			Unique().
			StorageKey("idx_user_tag"),
	}
}

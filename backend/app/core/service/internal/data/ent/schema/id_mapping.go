package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// IDMapping holds the schema definition for the IDMapping entity.
type IDMapping struct {
	ent.Schema
}

func (IDMapping) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_id_mappings",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("ID关联映射表"),
	}
}

func (IDMapping) Fields() []ent.Field {
	return []ent.Field{
		field.String("global_user_id").
			Comment("打通后的全局用户ID").
			NotEmpty(),

		field.String("id_type").
			Comment("身份类型：user_id/device_id/cookie/email/phone").
			NotEmpty(),

		field.String("id_value").
			Comment("身份值，具体的用户/设备/邮箱/手机号等标识").
			NotEmpty(),

		field.Float("confidence").
			Comment("关联置信度，默认1.0").
			Default(1.0),

		field.String("link_source").
			Comment("关联来源：login/bind/algorithm").
			Default("login"),

		field.Time("first_seen").
			Comment("首次关联时间").
			Optional().
			Nillable(),

		field.Time("last_seen").
			Comment("最近关联时间").
			Optional().
			Nillable(),

		field.Bool("is_active").
			Comment("是否激活，1为激活，0为未激活").
			Default(true),
	}
}

func (IDMapping) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TenantID[uint32]{},
		mixin.TimeAt{},
	}
}

func (IDMapping) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id + global_user_id
		index.Fields("tenant_id", "global_user_id").
			StorageKey("idx_global_user"),

		// 索引：tenant_id + id_type + id_value
		index.Fields("tenant_id", "id_type", "id_value").
			StorageKey("idx_id_type_value"),

		// 索引：tenant_id + is_active
		index.Fields("tenant_id", "is_active").
			StorageKey("idx_id_mapping_active"),

		// 唯一索引：tenant_id + id_type + id_value
		index.Fields("tenant_id", "id_type", "id_value").
			Unique().
			StorageKey("idx_type_value"),
	}
}

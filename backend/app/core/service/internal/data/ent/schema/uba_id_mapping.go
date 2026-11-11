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
			Comment("全局用户唯一标识").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("id_type").
			Comment("ID类型").
			Optional().
			Nillable(),

		field.String("id_value").
			Comment("ID值").
			NotEmpty().
			Optional().
			Nillable(),

		field.Float32("confidence").
			Comment("置信度，映射关系可信度评分，范围0~1，默认1.0").
			Default(1.0).
			Optional().
			Nillable(),

		field.String("link_source").
			Comment("关联来源：login/bind/algorithm").
			Default("login").
			Optional().
			Nillable(),

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
			Default(true).
			Optional().
			Nillable(),

		field.JSON("properties", map[string]string{}).
			Comment("扩展属性").
			Optional(),
	}
}

func (IDMapping) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TenantID[int64]{},
		mixin.OperatorID64{},
		mixin.TimeAt{},
	}
}

func (IDMapping) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id + global_user_id
		index.Fields("tenant_id", "global_user_id").
			StorageKey("idx_id_mapping_global_user"),

		// 索引：tenant_id + id_type + id_value
		index.Fields("tenant_id", "id_type", "id_value").
			StorageKey("idx_id_mapping_id_type_value"),

		// 索引：tenant_id + is_active
		index.Fields("tenant_id", "is_active").
			StorageKey("idx_id_mapping_active"),

		// 唯一索引：tenant_id + id_type + id_value
		index.Fields("tenant_id", "id_type", "id_value").
			Unique().
			StorageKey("idx_id_mapping_type_value"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (IDMapping) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

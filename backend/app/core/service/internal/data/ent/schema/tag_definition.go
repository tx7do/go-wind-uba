package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// TagDefinition holds the schema definition for the TagDefinition entity.
type TagDefinition struct {
	ent.Schema
}

func (TagDefinition) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_tag_definitions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA标签定义表"),
	}
}

func (TagDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("标签名称，显示名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("标签描述，详细说明").
			Optional().
			Nillable(),

		field.Enum("category").
			Comment("标签分类，如用户属性、行为偏好、风险、业务等").
			NamedValues(
				"TagCategoryUser", "TAG_CATEGORY_USER",
				"TagCategoryBehavior", "TAG_CATEGORY_BEHAVIOR",
				"TagCategoryRisk", "TAG_CATEGORY_RISK",
				"TagCategoryBusiness", "TAG_CATEGORY_BUSINESS",
			).
			Optional().
			Nillable(),

		field.Enum("tag_type").
			Comment("标签类型，如布尔、枚举、数值、字符串、列表等").
			NamedValues(
				"TagTypeBoolean", "TAG_TYPE_BOOLEAN",
				"TagTypeEnum", "TAG_TYPE_ENUM",
				"TagTypeNumeric", "TAG_TYPE_NUMERIC",
				"TagTypeString", "TAG_TYPE_STRING",
				"TagTypeList", "TAG_TYPE_LIST",
			).
			Optional().
			Nillable(),

		field.JSON("rule", map[string]any{}).
			Comment("计算规则，简化，实际可用表达式引擎，如 CEL/SQL").
			Optional(),

		field.JSON("allowed_values", []map[string]any{}).
			Comment("取值范围，枚举型标签的允许值列表").
			Optional(),

		field.Bool("is_system").
			Comment("是否系统预置，true: 系统预置，false: 用户自定义").
			Default(false).
			Optional().
			Nillable(),

		field.Bool("is_dynamic").
			Comment("是否动态标签，true: 动态计算，false: 静态打标").
			Default(false).
			Optional().
			Nillable(),

		field.Uint32("refresh_interval_seconds").
			Comment("动态标签刷新间隔，单位秒").
			Default(0).
			Optional().
			Nillable(),

		field.String("code").
			Comment("标签唯一代码，业务唯一标识，支持英文、数字、下划线").
			NotEmpty().
			Optional().
			Nillable(),
	}
}

func (TagDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID[uint32]{},
	}
}

func (TagDefinition) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：同一租户下 tag_id 唯一
		index.Fields("tenant_id", "id").
			Unique().
			StorageKey("uix_uba_tag_definitions_tenant_tag_id"),

		// 按租户快速筛选
		index.Fields("tenant_id").
			StorageKey("idx_uba_tag_definitions_tenant_id"),

		// 按类型过滤
		index.Fields("tag_type").
			StorageKey("idx_uba_tag_definitions_tag_type"),

		// 按分类过滤
		index.Fields("category").
			StorageKey("idx_uba_tag_definitions_category"),

		// 按启用状态过滤
		index.Fields("is_system").
			StorageKey("idx_uba_tag_definitions_is_system"),

		// 按创建时间分页与区间查询
		index.Fields("created_at").
			StorageKey("idx_uba_tag_definitions_created_at"),
	}
}

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Application holds the schema definition for the Application entity.
type Application struct {
	ent.Schema
}

func (Application) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_applications",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA应用表"),
	}
}

func (Application) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("UBA应用名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("app_id").
			Comment("UBA应用唯一标识（上报时使用）").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("app_key").
			Comment("密钥（签名/鉴权）").
			Optional().
			Nillable(),

		field.String("app_secret").
			Comment("密钥").
			Optional().
			Nillable(),

		field.String("type").
			Comment("应用类型").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("应用状态").
			NamedValues(
				"On", "ON",
				"Off", "OFF",
			).
			Default("ON").
			Optional().
			Nillable(),

		field.Strings("platforms").
			Comment("应用支持的平台列表").
			Optional(),

		field.String("remark").
			Comment("备注信息").
			Optional().
			Nillable(),

		field.Bool("desensitize").
			Comment("是否开启脱敏").
			Optional().
			Nillable(),

		field.String("webhook_url").
			Comment("事件回调 URL").
			Optional().
			Nillable(),

		field.String("webhook_secret").
			Comment("回调签名密钥").
			Optional().
			Nillable(),
	}
}

func (Application) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID[uint32]{},
	}
}

func (Application) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：同一租户下 app_id 唯一
		index.Fields("tenant_id", "app_id").
			Unique().
			StorageKey("uix_uba_applications_tenant_app_id"),

		// 按租户快速筛选
		index.Fields("tenant_id").
			StorageKey("idx_uba_applications_tenant_id"),

		// 按应用状态过滤
		index.Fields("status").
			StorageKey("idx_uba_applications_status"),

		// 按类型过滤
		index.Fields("type").
			StorageKey("idx_uba_applications_type"),

		// 按创建时间分页与区间查询
		index.Fields("created_at").
			StorageKey("idx_uba_applications_created_at"),
	}
}

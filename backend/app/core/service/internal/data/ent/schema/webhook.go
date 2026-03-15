package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Webhook holds the schema definition for the Webhook entity.
type Webhook struct {
	ent.Schema
}

func (Webhook) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_webhooks",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("Webhook告警配置表"),
	}
}

func (Webhook) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Webhook名称").
			NotEmpty(),

		field.String("url").
			Comment("回调URL").
			NotEmpty(),

		field.String("secret").
			Comment("签名密钥").
			Optional().
			Nillable(),

		field.JSON("event_types", []string{}).
			Comment("触发事件类型列表，如[\\\"risk.high\\\", \\\"risk.critical\\\"]").
			Optional(),

		field.JSON("filters", map[string]interface{}{}).
			Comment("过滤条件，结构化JSON").
			Optional(),

		field.Bool("enabled").
			Comment("是否启用，1为启用，0为禁用").
			Default(true),

		field.Time("last_triggered_at").
			Comment("最后触发时间").
			Optional().
			Nillable(),

		field.Int("failure_count").
			Comment("失败次数").
			Default(0),
	}
}

func (Webhook) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID[uint32]{},
	}
}

func (Webhook) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id
		index.Fields("tenant_id").
			StorageKey("idx_tenant_id"),

		// 索引：tenant_id + enabled
		index.Fields("tenant_id", "enabled").
			StorageKey("idx_enabled"),
	}
}

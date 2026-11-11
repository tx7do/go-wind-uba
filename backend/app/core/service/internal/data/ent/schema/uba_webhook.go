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
			NotEmpty().
			Optional().
			Nillable(),

		field.String("app_id").
			Comment("关联应用ID").
			Optional().
			Nillable(),

		field.String("url").
			Comment("回调URL").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("secret").
			Comment("签名密钥").
			Optional().
			Nillable(),

		field.JSON("event_types", []string{}).
			Comment("触发事件类型列表，如[\\\"risk.high\\\", \\\"risk.critical\\\"]").
			Optional(),

		//field.JSON("filter", &ubaV1.WebhookFilter{}).
		//	Comment("过滤条件，结构化JSON").
		//	Optional(),

		field.Bool("enabled").
			Comment("是否启用，1为启用，0为禁用").
			Default(true),

		field.Time("last_triggered_at").
			Comment("最后触发时间").
			Optional().
			Nillable(),

		field.Uint32("failure_count").
			Comment("失败次数").
			Default(0),
	}
}

func (Webhook) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (Webhook) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id
		index.Fields("tenant_id").
			StorageKey("idx_webhook_tenant_id"),

		// 索引：tenant_id + enabled
		index.Fields("tenant_id", "enabled").
			StorageKey("idx_webhook_tenant_id_enabled"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (Webhook) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

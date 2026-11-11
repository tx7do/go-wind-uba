package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// WebhookDeliveryLog holds the schema definition for the WebhookDeliveryLog entity.
type WebhookDeliveryLog struct {
	ent.Schema
}

func (WebhookDeliveryLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_webhook_delivery_logs",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("Webhook投递日志表"),
	}
}

func (WebhookDeliveryLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("webhook_id").
			Comment("Webhook配置ID"),

		field.String("event_type").
			Comment("触发事件类型，如 risk.high / alert.triggered").
			Optional().
			Nillable(),

		field.Text("payload").
			Comment("发送的payload内容").
			Optional(),

		field.Int32("status_code").
			Comment("HTTP状态码，0表示未收到响应").
			Default(0),

		field.Text("response_body").
			Comment("响应内容（截断）").
			Optional(),

		field.Uint32("retry_count").
			Comment("重试次数").
			Default(0),

		field.Bool("success").
			Comment("是否投递成功").
			Default(false),

		field.String("error_message").
			Comment("错误信息").
			Optional().
			Nillable(),

		field.Time("sent_at").
			Comment("发送时间").
			Optional().
			Nillable(),
	}
}

func (WebhookDeliveryLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.TenantID[int64]{},
	}
}

func (WebhookDeliveryLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("webhook_id").
			StorageKey("idx_webhook_delivery_log_webhook_id"),

		index.Fields("webhook_id", "created_at").
			StorageKey("idx_webhook_delivery_log_webhook_created"),
	}
}

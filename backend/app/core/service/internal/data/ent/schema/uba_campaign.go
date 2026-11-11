package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Campaign holds the schema definition for the Campaign entity.
type Campaign struct {
	ent.Schema
}

func (Campaign) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_campaigns",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("营销触达活动表"),
	}
}

func (Campaign) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("活动名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("活动描述").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID").
			Optional().
			Nillable(),

		field.Int64("segment_id").
			Comment("目标分群ID（uba_segments.id）"),

		field.String("title").
			Comment("消息标题（站内信）").
			Optional().
			Nillable(),

		field.Text("content").
			Comment("消息正文（站内信）").
			Optional().
			Nillable(),

		field.String("template_code").
			Comment("通知模板编码（配置后按模板渲染，title/content 忽略）").
			Optional().
			Nillable(),

		field.String("send_mode").
			Comment("发送模式：MANUAL / SCHEDULED").
			Default("MANUAL"),

		field.Time("scheduled_at").
			Comment("计划发送时间（SCHEDULED 模式）").
			Optional().
			Nillable(),

		field.String("status").
			Comment("状态：DRAFT / SCHEDULED / SENDING / SENT / PARTIAL / FAILED / CANCELLED").
			Default("DRAFT"),

		field.Uint64("total_count").
			Comment("目标人数").
			Default(0),

		field.Uint64("sent_count").
			Comment("成功人数").
			Default(0),

		field.Uint64("failed_count").
			Comment("失败人数").
			Default(0),

		field.Time("last_sent_at").
			Comment("最近发送时间").
			Optional().
			Nillable(),
	}
}

func (Campaign) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (Campaign) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_campaign_tenant_id"),

		index.Fields("tenant_id", "status").
			StorageKey("idx_campaign_tenant_id_status"),

		index.Fields("segment_id").
			StorageKey("idx_campaign_segment_id"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (Campaign) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

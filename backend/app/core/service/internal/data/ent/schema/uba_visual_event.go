package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// VisualEvent holds the schema definition for the VisualEvent entity.
type VisualEvent struct {
	ent.Schema
}

func (VisualEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_visual_events",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("可视化埋点规则表"),
	}
}

func (VisualEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("app_id").
			Comment("关联应用标识").
			Optional().
			Nillable(),

		field.String("event_name").
			Comment("事件名（点击命中后上报）").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("display_name").
			Comment("显示名").
			Optional().
			Nillable(),

		field.String("page_url").
			Comment("规则作用页面 URL 前缀，空为全部页面").
			Optional().
			Nillable(),

		field.Text("element_xpath").
			Comment("元素 XPath（SDK 点击采集同口径生成）").
			NotEmpty().
			Optional(),

		field.String("match_type").
			Comment("匹配方式：XPATH_EXACT / XPATH_PREFIX").
			Default("XPATH_EXACT"),

		field.JSON("properties", map[string]string{}).
			Comment("随事件上报的静态属性").
			Optional(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.String("remark").
			Comment("备注").
			Optional().
			Nillable(),
	}
}

func (VisualEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (VisualEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_visual_event_tenant_id"),

		index.Fields("tenant_id", "enabled").
			StorageKey("idx_visual_event_tenant_id_enabled"),

		index.Fields("app_id").
			StorageKey("idx_visual_event_app_id"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (VisualEvent) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

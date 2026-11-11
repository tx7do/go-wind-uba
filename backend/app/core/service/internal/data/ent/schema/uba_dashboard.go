package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Dashboard holds the schema definition for the Dashboard entity.
type Dashboard struct {
	ent.Schema
}

func (Dashboard) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_dashboards",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("自定义看板表"),
	}
}

func (Dashboard) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("看板名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("看板描述").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID").
			Optional().
			Nillable(),

		field.JSON("widgets", []map[string]any{}).
			Comment("看板组件列表：[{id,title,model,paramsJson,span}] 的 JSON 快照"),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),
	}
}

func (Dashboard) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (Dashboard) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_dashboard_tenant_id"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (Dashboard) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

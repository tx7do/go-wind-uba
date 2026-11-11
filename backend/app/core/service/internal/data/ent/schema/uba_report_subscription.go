package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// ReportSubscription holds the schema definition for the ReportSubscription entity.
type ReportSubscription struct {
	ent.Schema
}

func (ReportSubscription) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_report_subscriptions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("定时报告订阅表"),
	}
}

func (ReportSubscription) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("订阅名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("订阅描述").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID").
			Optional().
			Nillable(),

		field.String("model").
			Comment("分析模型标识：event-trend / active-users / group-by / funnel / retention").
			NotEmpty().
			Optional().
			Nillable(),

		field.Text("params_json").
			Comment("分析请求参数 JSON 快照（与对应分析 API 请求体一致）").
			Optional().
			Nillable(),

		field.Int32("interval_minutes").
			Comment("执行间隔（分钟），最小 60").
			Default(1440),

		field.JSON("channels", []string{}).
			Comment("推送渠道列表：internal_message / webhook").
			Optional(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.Time("last_run_at").
			Comment("最近一次执行时间").
			Optional().
			Nillable(),
	}
}

func (ReportSubscription) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (ReportSubscription) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_report_subscription_tenant_id"),

		index.Fields("tenant_id", "enabled").
			StorageKey("idx_report_subscription_tenant_id_enabled"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (ReportSubscription) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

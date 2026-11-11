package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// AlertRule holds the schema definition for the AlertRule entity.
type AlertRule struct {
	ent.Schema
}

func (AlertRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_alert_rules",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("指标阈值告警规则表"),
	}
}

func (AlertRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("规则名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("规则描述").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID（分析域数字ID），0为全部应用").
			Optional().
			Nillable(),

		field.String("event_name").
			Comment("监控的事件名，空为全部事件（此时 COUNT 即总事件量）").
			Optional().
			Nillable(),

		field.String("aggregator").
			Comment("聚合方式：COUNT / ACTIVE_USERS / SUM_AMOUNT / AVG_AMOUNT").
			Default("COUNT"),

		field.String("comparison").
			Comment("比较算子：GT / GTE / LT / LTE").
			Default("GT"),

		field.Float("threshold").
			Comment("阈值").
			Default(0),

		field.Int32("window_minutes").
			Comment("统计窗口长度（分钟）").
			Default(60),

		field.Int32("silence_minutes").
			Comment("告警静默时长（分钟），触发后在此窗口内不重复通知").
			Default(60),

		field.JSON("channels", []string{}).
			Comment("通知渠道列表：webhook / internal_message").
			Optional(),

		field.Bool("enabled").
			Comment("是否启用").
			Default(true),

		field.Time("last_triggered_at").
			Comment("最后触发时间").
			Optional().
			Nillable(),
	}
}

func (AlertRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (AlertRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_alert_rule_tenant_id"),

		index.Fields("tenant_id", "enabled").
			StorageKey("idx_alert_rule_tenant_id_enabled"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (AlertRule) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

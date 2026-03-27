package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// RiskRule holds the schema definition for the RiskRule entity.
type RiskRule struct {
	ent.Schema
}

func (RiskRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_risk_rules",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA风险规则表"),
	}
}

func (RiskRule) Fields() []ent.Field {
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

		field.String("risk_type").
			Comment("风险类型，规则对应的风险类型").
			Optional().
			Nillable(),

		field.String("default_level").
			Comment("默认风险等级，规则对应的默认风险等级").
			Optional().
			Nillable(),

		field.JSON("condition", map[string]any{}).
			Comment("规则条件，简化版，实际可用 CEL/JSON Schema").
			Optional(),

		field.JSON("actions", []*ubaV1.RiskAction{}).
			Comment("动作配置，规则触发时的处置动作列表").
			Optional(),

		field.Bool("enabled").
			Comment("是否启用，true: 启用，false: 禁用").
			Default(true).
			Optional().
			Nillable(),

		field.Uint32("priority").
			Comment("优先级，越小优先级越高").
			Default(100).
			Optional().
			Nillable(),

		field.String("code").
			Comment("规则编码，业务唯一标识，支持英文、数字、下划线").
			NotEmpty().
			Optional().
			Nillable(),

		field.Text("rule_expression").
			Comment("规则表达式（CEL/SQL）").
			Optional().
			Nillable(),

		field.JSON("rule_config", map[string]any{}).
			Comment("规则配置参数").
			Optional(),

		field.Enum("exec_mode").
			Comment("执行模式：realtime/batch").
			NamedValues(
				"Realtime", "REALTIME",
				"Batch", "BATCH",
			).
			Default("REALTIME").
			Optional().
			Nillable(),

		field.Uint64("trigger_count").
			Comment("触发次数").
			Default(0).
			Optional().
			Nillable(),

		field.Time("last_triggered_at").
			Comment("最后触发时间").
			Optional().
			Nillable(),
	}
}

func (RiskRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.TenantID[uint32]{},
	}
}

func (RiskRule) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：同一租户下 rule_id 唯一
		index.Fields("tenant_id", "id").
			Unique().
			StorageKey("uix_uba_risk_rules_tenant_rule_id"),

		// 按租户快速筛选
		index.Fields("tenant_id").
			StorageKey("idx_uba_risk_rules_tenant_id"),

		// 按启用状态过滤
		index.Fields("enabled").
			StorageKey("idx_uba_risk_rules_enabled"),

		// 按风险类型过滤
		index.Fields("risk_type").
			StorageKey("idx_uba_risk_rules_risk_type"),

		// 按优先级排序
		index.Fields("priority").
			StorageKey("idx_uba_risk_rules_priority"),

		// 按创建时间分页与区间查询
		index.Fields("created_at").
			StorageKey("idx_uba_risk_rules_created_at"),
	}
}

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// RiskRuleCondition holds the schema definition for the RiskRuleCondition entity.
type RiskRuleCondition struct {
	ent.Schema
}

func (RiskRuleCondition) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_risk_rule_conditions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA风险规则条件表"),
	}
}

func (RiskRuleCondition) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("rule_id").
			Comment("风险规则ID，关联 uba_risk_rules.id").
			Optional().
			Nillable(),

		field.String("field_name").
			Comment("字段名，条件配置").
			NotEmpty(),

		field.String("operator").
			Comment("操作符：eq/gt/lt/contains/in").
			NotEmpty(),

		field.Text("field_value").
			Comment("字段值，条件配置").
			Optional().
			Nillable(),

		field.String("logic_operator").
			Comment("逻辑关系：AND/OR").
			Default("AND"),

		field.Uint32("group_id").
			Comment("条件组ID，逻辑分组").
			Optional().
			Nillable(),
	}
}

func (RiskRuleCondition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.SortOrder{},
		mixin.TenantID[uint32]{},
	}
}

func (RiskRuleCondition) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id + rule_id
		index.Fields("tenant_id", "rule_id").
			StorageKey("idx_risk_rule_condition_rule_id"),

		// 索引：tenant_id + rule_id + field_name
		index.Fields("tenant_id", "rule_id", "field_name").
			StorageKey("idx_risk_rule_condition_field_name"),
	}
}

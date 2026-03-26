package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// RiskRuleVersion holds the schema definition for the RiskRuleVersion entity.
type RiskRuleVersion struct {
	ent.Schema
}

func (RiskRuleVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_risk_rule_versions",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("UBA风险规则版本表"),
	}
}

func (RiskRuleVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("rule_id").
			Comment("风险规则ID，关联 uba_risk_rules.id").
			Optional().
			Nillable(),

		field.String("name").
			Comment("规则名称，版本快照").
			NotEmpty(),

		field.Text("rule_expression").
			Comment("规则表达式，版本快照"),

		field.JSON("rule_config", map[string]any{}).
			Comment("规则配置，版本快照"),

		field.JSON("actions", []map[string]any{}).
			Comment("动作配置，版本快照"),

		field.String("risk_level").
			Comment("风险等级，版本快照").
			Optional().
			Nillable(),

		field.String("change_summary").
			Comment("变更说明").
			Optional().
			Nillable(),

		field.String("change_reason").
			Comment("变更原因").
			Optional().
			Nillable(),

		field.Enum("status").
			Comment("版本状态：draft/published/archived").
			NamedValues(
				"Draft", "DRAFT",
				"Published", "PUBLISHED",
				"Archived", "ARCHIVED",
			).
			Default("DRAFT"),

		field.Time("published_at").
			Comment("发布时间").
			Optional().
			Nillable(),

		field.Uint32("published_by").
			Comment("发布者用户ID").
			Optional().
			Nillable(),
	}
}

func (RiskRuleVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId{},
		mixin.TimeAt{},
		mixin.OperatorID{},
		mixin.Version{},
		mixin.TenantID[uint32]{},
	}
}

func (RiskRuleVersion) Indexes() []ent.Index {
	return []ent.Index{
		// 索引：tenant_id + rule_id
		index.Fields("tenant_id", "rule_id").
			StorageKey("idx_risk_rule_version_rule_id"),

		// 索引：tenant_id + rule_id + version
		index.Fields("tenant_id", "rule_id", "version").
			StorageKey("idx_risk_rule_version_version"),

		// 索引：tenant_id + status
		index.Fields("tenant_id", "status").
			StorageKey("idx_risk_rule_version_status"),
	}
}

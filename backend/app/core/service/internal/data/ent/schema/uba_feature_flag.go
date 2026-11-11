package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// FeatureVariant 实验变体（多变量 A/B/N 的分支定义）。
// Weight 为千分比权重（0~1000），分配时按累计权重切分。
type FeatureVariant struct {
	Key    string `json:"key"`
	Name   string `json:"name,omitempty"`
	Weight int32  `json:"weight"`
}

// FeatureFlag holds the schema definition for the FeatureFlag entity.
type FeatureFlag struct {
	ent.Schema
}

func (FeatureFlag) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_feature_flags",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("功能开关/AB实验表"),
	}
}

func (FeatureFlag) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").
			Comment("开关唯一标识（租户内唯一，如 new-checkout-flow）").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("name").
			Comment("显示名称").
			Optional().
			Nillable(),

		field.String("description").
			Comment("描述（实验假设等）").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID，0/空为该租户全部应用").
			Optional().
			Nillable(),

		field.JSON("variants", []FeatureVariant{}).
			Comment("实验变体列表（含 control），Weight 千分比，和不必为 1000，按比例归一"),

		field.Int32("rollout_percentage").
			Comment("放量百分比 0~100，按 distinct_id 一致性哈希判定是否命中").
			Default(100),

		// 可选目标事件：配置后实验结果页展示各变体的转化率
		field.String("goal_event_name").
			Comment("目标转化事件名（A/B 实验分析用，可空）").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用，停用后评估一律返回未命中").
			Default(true),

		field.Time("started_at").
			Comment("实验开始时间").
			Optional().
			Nillable(),

		field.Time("ended_at").
			Comment("实验结束时间").
			Optional().
			Nillable(),
	}
}

func (FeatureFlag) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (FeatureFlag) Indexes() []ent.Index {
	return []ent.Index{
		// 唯一索引：租户内 key 唯一
		index.Fields("tenant_id", "key").
			Unique().
			StorageKey("uniq_flag_tenant_key"),

		index.Fields("tenant_id", "enabled").
			StorageKey("idx_flag_tenant_id_enabled"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (FeatureFlag) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

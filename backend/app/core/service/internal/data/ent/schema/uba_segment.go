package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// Segment holds the schema definition for the Segment entity.
type Segment struct {
	ent.Schema
}

func (Segment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_segments",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("用户分群表"),
	}
}

func (Segment) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("分群名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("description").
			Comment("分群描述").
			Optional().
			Nillable(),

		field.Int64("app_id").
			Comment("关联应用ID").
			Optional().
			Nillable(),

		field.JSON("definition", map[string]any{}).
			Comment("圈选定义（include/exclude 条件与时间范围，对应 SegmentationRequest 的 JSON 快照）"),

		field.Uint32("refresh_interval_minutes").
			Comment("定时刷新间隔（分钟），0 表示仅手动刷新").
			Default(0),

		field.Uint32("max_users").
			Comment("人群规模上限，超出截断").
			Default(50000),

		field.Uint64("user_count").
			Comment("最近一次刷新命中的人数").
			Default(0),

		field.Time("last_refreshed_at").
			Comment("最近一次刷新时间").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Comment("是否启用（停用后不参与定时刷新）").
			Default(true),
	}
}

func (Segment) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (Segment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_segment_tenant_id"),

		index.Fields("tenant_id", "enabled").
			StorageKey("idx_segment_tenant_id_enabled"),
	}
}

// Policy 追加租户变更防护：go-crud TenantPrivacy 只覆盖 Query/Create，
// 本规则为 Update/UpdateOne/Delete/DeleteOne 注入 tenant_id 过滤，
// 防止知道 ID 的跨租户篡改与删除（平台/系统上下文放行）。
func (Segment) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

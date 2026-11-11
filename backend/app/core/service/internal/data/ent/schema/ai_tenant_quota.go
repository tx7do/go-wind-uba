package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// AiTenantQuota AI 租户配额：一行 = 一个租户的月度 token 上限。
// 平台自用（tenant_id=0）不设限；未建行的租户视为不限量——
// 只有平台管理员显式设置的租户才受配额约束（fail-open，避免误伤）。
type AiTenantQuota struct {
	ent.Schema
}

func (AiTenantQuota) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "sys_ai_tenant_quotas",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("AI 租户月度 token 配额"),
	}
}

func (AiTenantQuota) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("tenant_id").Comment("租户ID（租户维度唯一）"),
		field.Uint64("monthly_token_limit").Comment("月度 token 上限（自然月，0=取消配额）"),
	}
}

func (AiTenantQuota) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
	}
}

func (AiTenantQuota) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").Unique().StorageKey("uix_sys_ai_tenant_quotas_tenant"),
	}
}

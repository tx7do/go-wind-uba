package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// ImportJob holds the schema definition for the ImportJob entity.
type ImportJob struct {
	ent.Schema
}

func (ImportJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_import_jobs",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("历史数据导入任务表"),
	}
}

func (ImportJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("任务名称").
			NotEmpty().
			Optional().
			Nillable(),

		field.String("app_id").
			Comment("关联应用标识").
			Optional().
			Nillable(),

		field.Text("events_json").
			Comment("原始事件 JSON 数组（仅创建时提交后缓存，避免重复上传；查询不返回）").
			Optional(),

		field.Uint32("total_count").
			Comment("总事件数").
			Default(0),

		field.Uint32("imported_count").
			Comment("已导入数").
			Default(0),

		field.Uint32("failed_count").
			Comment("失败数").
			Default(0),

		field.String("status").
			Comment("状态：PENDING / RUNNING / COMPLETED / PARTIAL / FAILED / CANCELLED").
			Default("PENDING"),

		field.JSON("errors", []string{}).
			Comment("错误摘要（前 20 条）").
			Optional(),

		field.Time("started_at").
			Comment("开始时间").
			Optional().
			Nillable(),

		field.Time("finished_at").
			Comment("完成时间").
			Optional().
			Nillable(),

		field.Uint32("duration_ms").
			Comment("耗时（毫秒）").
			Default(0),
	}
}

func (ImportJob) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.OperatorID64{},
		mixin.TenantID[int64]{},
	}
}

func (ImportJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			StorageKey("idx_import_job_tenant_id"),

		index.Fields("tenant_id", "status").
			StorageKey("idx_import_job_tenant_id_status"),
	}
}

// Policy 追加租户变更防护。
func (ImportJob) Policy() ent.Policy {
	return &TenantMutationGuardPolicy{}
}

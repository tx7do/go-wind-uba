package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// ReplayChunk holds the schema definition for the ReplayChunk entity.
type ReplayChunk struct {
	ent.Schema
}

func (ReplayChunk) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_replay_chunks",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("会话回放录制分块表（rrweb 事件 JSON）"),
	}
}

func (ReplayChunk) Fields() []ent.Field {
	return []ent.Field{
		field.String("app_id").
			Comment("关联应用标识").
			Optional().
			Nillable(),

		field.String("session_id").
			Comment("会话 ID（SDK sessionId）").
			NotEmpty(),

		field.String("device_id").
			Comment("设备 ID").
			Optional().
			Nillable(),

		field.Int64("user_id").
			Comment("登录用户 ID").
			Default(0),

		field.String("page_url").
			Comment("录制起始页面").
			Optional().
			Nillable(),

		field.Uint32("chunk_seq").
			Comment("分块序号（0 起，同会话内递增）").
			Default(0),

		field.Text("events_json").
			Comment("rrweb 事件 JSON 数组字符串"),

		field.Uint32("event_count").
			Comment("块内事件数").
			Default(0),

		field.Uint32("duration_ms").
			Comment("块内时间跨度（毫秒）").
			Default(0),
	}
}

func (ReplayChunk) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.TenantID[int64]{},
	}
}

func (ReplayChunk) Indexes() []ent.Index {
	return []ent.Index{
		// 幂等锚：同会话同序号的分块只留一份
		index.Fields("session_id", "chunk_seq").
			Unique().
			StorageKey("uniq_replay_session_seq"),

		index.Fields("session_id").
			StorageKey("idx_replay_session_id"),

		index.Fields("user_id").
			StorageKey("idx_replay_user_id"),

		index.Fields("created_at").
			StorageKey("idx_replay_created_at"),
	}
}

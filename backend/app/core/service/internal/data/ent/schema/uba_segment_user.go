package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// SegmentUser holds the schema definition for the SegmentUser entity.
type SegmentUser struct {
	ent.Schema
}

func (SegmentUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "uba_segment_users",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_unicode_ci",
		},
		entsql.WithComments(true),
		schema.Comment("用户分群成员表"),
	}
}

func (SegmentUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("segment_id").
			Comment("分群ID"),

		field.Int64("user_id").
			Comment("用户ID（events_fact.user_id）"),
	}
}

func (SegmentUser) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.AutoIncrementId64{},
		mixin.TimeAt{},
		mixin.TenantID[int64]{},
	}
}

func (SegmentUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("segment_id", "user_id").
			Unique().
			StorageKey("uniq_segment_user"),

		index.Fields("segment_id").
			StorageKey("idx_segment_user_segment_id"),
	}
}

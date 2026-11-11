package clickhouse

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestValidateAdhocQuery_Allowed(t *testing.T) {
	allowed := []string{
		"SELECT 1",
		"select * from events_fact limit 10",
		"  SELECT event_name, count() FROM events_fact GROUP BY event_name  ",
		"SHOW TABLES",
		"DESC events_fact",
		"DESCRIBE events_fact",
		"EXPLAIN SELECT 1",
		"SELECT 1;", // 结尾单个分号容忍
	}
	for _, q := range allowed {
		if err := ValidateAdhocQuery(q); err != nil {
			t.Fatalf("query %q should be allowed, got %v", q, err)
		}
	}
}

func TestValidateAdhocQuery_Rejected(t *testing.T) {
	cases := map[string]string{
		"":                                  "empty",
		"   ":                               "blank",
		"SELECT 1; DROP TABLE events_fact":  "multi statement",
		"SELECT 1;;SELECT 2":                "double semicolon",
		"INSERT INTO events_fact VALUES(1)": "insert",
		"UPDATE events_fact SET a=1":        "update",
		"DELETE FROM events_fact":           "delete",
		"DROP TABLE events_fact":            "drop",
		"CREATE TABLE t(a int)":             "create",
		"ALTER TABLE t ADD c int":           "alter",
		"SET GLOBAL a=1":                    "set",
		"GRANT ALL ON *.* TO u":             "grant",
		"USE gw_uba":                        "use",
		"SELECT 1 -- hidden":                "line comment",
		"SELECT /*x*/ 1":                    "block comment",
		"SELECT 1 #hidden":                  "hash comment",
	}
	for q, why := range cases {
		if err := ValidateAdhocQuery(q); err == nil {
			t.Fatalf("query %q (%s) should be rejected", q, why)
		}
	}
}

func TestChToStructValue(t *testing.T) {
	// 驱动专有类型逐层兜底：时间/字节串 → 切片/Map → Stringer → fmt.Sprint。
	cases := []struct {
		name string
		in   any
		want string // kind 判定用的简要描述
		kind string // null | string | number | list | struct
	}{
		{"nil", nil, "", "null"},
		{"time", time.Unix(0, 0).UTC(), "time", "string"},
		{"string", "a", "a", "string"},
		{"int64", int64(7), "7", "number"},
		{"bytes", []byte("b"), "b", "string"},
		{"slice", []string{"x", "y"}, "x,y", "list"},
		{"map", map[string]string{"k": "v"}, "k=v", "struct"},
	}
	for _, c := range cases {
		v, err := chToStructValue(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		switch c.kind {
		case "null":
			if _, ok := v.GetKind().(*structpb.Value_NullValue); !ok {
				t.Errorf("%s: want null", c.name)
			}
		case "string":
			if v.GetStringValue() == "" && c.want != "" {
				t.Errorf("%s: want string %q, got %v", c.name, c.want, v)
			}
		case "number":
			if v.GetNumberValue() != 7 {
				t.Errorf("%s: want 7, got %v", c.name, v)
			}
		case "list":
			if len(v.GetListValue().GetValues()) != 2 {
				t.Errorf("%s: want 2 items, got %v", c.name, v)
			}
		case "struct":
			if v.GetStructValue().GetFields()["k"].GetStringValue() != "v" {
				t.Errorf("%s: want map, got %v", c.name, v)
			}
		}
	}
}

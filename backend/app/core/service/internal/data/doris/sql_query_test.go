package doris

import "testing"

func TestValidateAdhocQuery_Allowed(t *testing.T) {
	allowed := []string{
		"SELECT 1",
		"select * from events_fact limit 10",
		"  SELECT event_name, COUNT(*) FROM events_fact GROUP BY event_name  ",
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

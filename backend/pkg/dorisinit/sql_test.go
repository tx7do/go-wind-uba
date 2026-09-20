package dorisinit

import (
	"strings"
	"testing"
)

func TestStripBOM(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"strips", []byte("\xEF\xBB\xBF-- x"), "-- x"},
		{"leaves clean input", []byte("-- x"), "-- x"},
		{"only removes the prefix", []byte("a\xEF\xBB\xBFb"), "a\xEF\xBB\xBFb"},
		{"empty", nil, ""},
		{"partial mark", []byte("\xEF\xBB"), "\xEF\xBB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripBOM(tt.in); string(got) != tt.want {
				t.Errorf("StripBOM() = %q, want %q", got, tt.want)
			}
		})
	}
}

// statementsOf splits src and fails the test on error, returning just the statement texts.
func statementsOf(t *testing.T, src string) []Statement {
	t.Helper()
	stmts, err := SplitStatements([]byte(src))
	if err != nil {
		t.Fatalf("SplitStatements(%q): unexpected error: %v", src, err)
	}
	return stmts
}

func TestSplitStatements_shape(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    []string // expected Text of each statement
		wantCmd []string // expected Code of each statement
		lines   []int
	}{
		{
			name:  "two statements",
			src:   "CREATE TABLE a (x INT);\nCREATE TABLE b (y INT);\n",
			want:  []string{"CREATE TABLE a (x INT)", "CREATE TABLE b (y INT)"},
			lines: []int{1, 2},
		},
		{
			name: "semicolon inside a string",
			src:  `INSERT INTO t VALUES ('a;b'), ('c');`,
			want: []string{`INSERT INTO t VALUES ('a;b'), ('c')`},
		},
		{
			name: "escaped quotes in jsonpaths",
			src:  `PROPERTIES ("jsonpaths" = "[\"$.a\", \"$.b\"]", "format" = "json")\nFROM KAFKA ("kafka_topic" = "x");`,
			// The source is a raw string, so the \n stays two literal characters.
			want: []string{`PROPERTIES ("jsonpaths" = "[\"$.a\", \"$.b\"]", "format" = "json")` + `\nFROM KAFKA ("kafka_topic" = "x")`},
		},
		{
			name: "doubled quote",
			src:  `INSERT INTO t VALUES ('it''s;ok');`,
			want: []string{`INSERT INTO t VALUES ('it''s;ok')`},
		},
		{
			name: "backslash escaped quote",
			src:  `SELECT 'a\';b';`,
			want: []string{`SELECT 'a\';b'`},
		},
		{
			name: "double quoted identifier",
			src:  `SELECT "we;ird" FROM t;`,
			want: []string{`SELECT "we;ird" FROM t`},
		},
		{
			name: "backtick identifier",
			src:  "SELECT `a;b` FROM t;",
			want: []string{"SELECT `a;b` FROM t"},
		},
		{
			name: "comment-only script",
			src:  "-- nothing here\n-- really\n",
			want: nil,
		},
		{
			name: "empty script",
			src:  "   \n\n",
			want: nil,
		},
		{
			name:  "stray semicolon",
			src:   ";;;\nSELECT 1;",
			want:  []string{"SELECT 1"},
			lines: []int{2},
		},
		{
			name:  "header comment is not part of the statement",
			src:   "-- 1. 填充 sessions_fact\nINSERT INTO s SELECT 1;\n",
			want:  []string{"INSERT INTO s SELECT 1"},
			lines: []int{2},
		},
		{
			name: "trailing comment after the last semicolon",
			src:  "SELECT 1;\n-- done",
			want: []string{"SELECT 1"},
		},
		{
			name: "statement without a terminator at EOF",
			src:  "SELECT 1;\nSELECT 2",
			want: []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:    "inline comments keep the statement whole",
			src:     "INSERT -- target\nINTO s /* pick */ SELECT 1;\n",
			want:    []string{"INSERT -- target\nINTO s /* pick */ SELECT 1"},
			wantCmd: []string{"INSERT INTO s SELECT 1"},
		},
		{
			name:  "hash comment",
			src:   "# note\nSELECT 1;",
			want:  []string{"SELECT 1"},
			lines: []int{2},
		},
		{
			name: "block comment swallows a semicolon",
			src:  "SELECT 1 /* a;b */;",
			want: []string{"SELECT 1 /* a;b */"},
		},
		{
			name: "double dash that is not a comment",
			src:  "SELECT 1--2;\n",
			want: []string{"SELECT 1--2"},
		},
		{
			name: "double dash at end of line",
			src:  "SELECT 1 --\nFROM t;\n",
			want: []string{"SELECT 1 --\nFROM t"},
		},
		{
			name:  "multi-line string keeps line numbering honest",
			src:   "SELECT 'a\nb';\nSELECT 2;",
			want:  []string{"SELECT 'a\nb'", "SELECT 2"},
			lines: []int{1, 3},
		},
		{
			name: "doris style script",
			src: strings.Join([]string{
				"-- ============================================================",
				"-- 1. 行为事件",
				"-- ============================================================",
				"USE gw_uba;",
				"",
				"CREATE ROUTINE LOAD gw_uba.job_a",
				"ON events_fact",
				`COLUMNS(event_id, amount)`,
				`PROPERTIES ("jsonpaths" = "[\"$.eventId\", \"$.amount\"]")`,
				`FROM KAFKA ("kafka_broker_list" = "kafka:9092", "kafka_topic" = "uba_events_raw");`,
				"",
			}, "\n"),
			want:  []string{"USE gw_uba", "CREATE ROUTINE LOAD gw_uba.job_a\nON events_fact\nCOLUMNS(event_id, amount)\nPROPERTIES (\"jsonpaths\" = \"[\\\"$.eventId\\\", \\\"$.amount\\\"]\")\nFROM KAFKA (\"kafka_broker_list\" = \"kafka:9092\", \"kafka_topic\" = \"uba_events_raw\")"},
			lines: []int{4, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts := statementsOf(t, tt.src)
			if len(stmts) != len(tt.want) {
				t.Fatalf("got %d statements %q, want %d %q", len(stmts), texts(stmts), len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if stmts[i].Text != want {
					t.Errorf("statement %d text =\n%q\nwant\n%q", i, stmts[i].Text, want)
				}
				if tt.lines != nil && stmts[i].Line != tt.lines[i] {
					t.Errorf("statement %d line = %d, want %d", i, stmts[i].Line, tt.lines[i])
				}
				if i < len(tt.wantCmd) && stmts[i].Code != tt.wantCmd[i] {
					t.Errorf("statement %d code = %q, want %q", i, stmts[i].Code, tt.wantCmd[i])
				}
			}
		})
	}
}

func TestSplitStatements_codeCollapsesWhitespace(t *testing.T) {
	stmts := statementsOf(t, "SELECT   a,\n  b -- pick\nFROM t\nWHERE x = 'y;z';\n")
	if len(stmts) != 1 {
		t.Fatalf("got %d statements, want 1", len(stmts))
	}
	want := "SELECT a, b FROM t WHERE x = 'y;z'"
	if got := stmts[0].Code; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
}

func TestFirstWord(t *testing.T) {
	tests := []struct{ code, want string }{
		{"CREATE ROUTINE LOAD db.job ON t", "CREATE"},
		{"  \n drop routine load for job", "drop"},
		{"(INSERT INTO t VALUES (1))", "INSERT"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := (Statement{Code: tt.code}).FirstWord(); got != tt.want {
			t.Errorf("FirstWord(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestSplitStatements_errors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{"unterminated string", "SELECT 'abc;", "unterminated"},
		{"unterminated identifier", "SELECT `abc;", "unterminated"},
		{"delimiter", "DELIMITER $$\nSELECT 1$$\n", "DELIMITER is not supported"},
		{"invalid utf8", "SELECT \xFF\xFE FROM t;\n", "invalid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SplitStatements([]byte(tt.src))
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func texts(stmts []Statement) []string {
	out := make([]string, len(stmts))
	for i, s := range stmts {
		out[i] = s.Text
	}
	return out
}

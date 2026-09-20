package dorisinit

import (
	"fmt"
	"strings"
	"testing"
)

func mustStatements(t *testing.T, src string) []Statement {
	t.Helper()
	stmts, err := SplitStatements([]byte(src))
	if err != nil {
		t.Fatalf("SplitStatements: %v", err)
	}
	return stmts
}

func TestSplitSections(t *testing.T) {
	src := `-- header
USE db;

-- ============================================================
-- 1. 会话表
-- ============================================================
INSERT INTO a SELECT 1;

-- 说明：
-- 1) 这不是小节标题
-- 2) 这也不是
-- 1. 重复的小节标题，下面没有语句

-- 2.1 开启部分列更新
SET x = true;

-- 2.2 主写入
INSERT INTO b SELECT 2;
SET x = false;

-- 6. 校验
SELECT count(*) FROM a;
`
	stmts := mustStatements(t, src)
	if len(stmts) != 6 {
		t.Fatalf("got %d statements, want 6: %q", len(stmts), texts(stmts))
	}

	// Every numbered header opens a section, in order, even when it holds no statement;
	// the "-- N)" prose bullets must not be mistaken for headers.
	var numbers, counts []string
	for _, s := range SplitSections([]byte(src), stmts) {
		numbers = append(numbers, s.Number)
		counts = append(counts, fmt.Sprintf("%s:%d", s.Number, len(s.Statements)))
	}
	if got, want := strings.Join(numbers, ","), ",1,1,2.1,2.2,6"; got != want {
		t.Errorf("section numbers = %q, want %q", got, want)
	}
	if got, want := strings.Join(counts, " "), ":1 1:1 1:0 2.1:1 2.2:2 6:1"; got != want {
		t.Errorf("statements per section = %q, want %q", got, want)
	}
}

func TestSelectSections(t *testing.T) {
	src := `-- preamble
USE db;
-- 1. one
INSERT INTO a SELECT 1;
-- 1.1 sub
INSERT INTO a2 SELECT 1;
-- 2. two
INSERT INTO b SELECT 2;
-- 3. three
SELECT 3;
`
	all := SplitSections([]byte(src), mustStatements(t, src))

	got, err := SelectSections(all, "1", "2")
	if err != nil {
		t.Fatalf("SelectSections: %v", err)
	}
	var numbers, stmts []string
	for _, s := range got {
		numbers = append(numbers, s.Number)
		for _, st := range s.Statements {
			stmts = append(stmts, st.Text)
		}
	}
	if strings.Join(numbers, ",") != "1,1.1,2" {
		t.Errorf("selected sections = %v, want [1 1.1 2]", numbers)
	}
	if strings.Join(stmts, " | ") != "INSERT INTO a SELECT 1 | INSERT INTO a2 SELECT 1 | INSERT INTO b SELECT 2" {
		t.Errorf("selected statements = %q", stmts)
	}

	if _, err := SelectSections(all, "9"); err == nil || !strings.Contains(err.Error(), `section "9" not found`) {
		t.Errorf("SelectSections(missing) error = %v, want not-found", err)
	}
}

func TestSelectSections_rejectsAmbiguousDuplicate(t *testing.T) {
	src := `-- 1. first
INSERT INTO a SELECT 1;
-- 1. again
INSERT INTO b SELECT 2;
`
	_, err := SelectSections(SplitSections([]byte(src), mustStatements(t, src)), "1")
	if err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("error = %v, want a duplicate-section error", err)
	}
}

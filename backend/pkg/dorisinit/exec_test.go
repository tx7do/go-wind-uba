package dorisinit

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

// fakeConn records what was sent and replays canned answers, so Apply/RunETL can be
// exercised without a Doris anywhere near the test suite.
type fakeConn struct {
	execed  []string
	queries []string

	showResult Rows
	showErr    error
	execErr    func(query string) error

	closed bool
}

func (f *fakeConn) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	if f.execErr != nil {
		if err := f.execErr(query); err != nil {
			return nil, err
		}
	}
	f.execed = append(f.execed, query)
	// Doris reports affected rows for INSERT and 0 for session statements.
	var rows int64
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "INSERT") {
		rows = 7
	}
	return fakeResult{rows: rows}, nil
}

type fakeResult struct{ rows int64 }

func (f fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (f fakeResult) RowsAffected() (int64, error) { return f.rows, nil }

func (f *fakeConn) QueryContext(_ context.Context, query string, _ ...any) (Rows, error) {
	f.queries = append(f.queries, query)
	if f.showErr != nil {
		return nil, f.showErr
	}
	return f.showResult, nil
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

func (f *fakeConn) single() Source {
	return SourceFunc(func(context.Context) (Conn, error) { return f, nil })
}

var errBoom = errors.New("boom")

func TestApply_runsPlainThenEnsures(t *testing.T) {
	conn := &fakeConn{showResult: &fakeRows{
		columns: []string{"Name", "DbName", "State"},
		rows: [][]any{
			{"job_paused", "gw_uba", "PAUSED"},
			{"job_running", "gw_uba", "RUNNING"},
			{"job_stopped", "gw_uba", "STOPPED"},
		},
	}}

	scripts := []Script{
		{Name: "1_base.sql", Bytes: []byte("CREATE TABLE events_fact (x INT);\nCREATE TABLE risk_events (x INT);\n")},
		{Name: "02_kafka.sql", Bytes: []byte(`
USE gw_uba;
CREATE ROUTINE LOAD gw_uba.job_events_to_fact ON events_fact FROM KAFKA("kafka_topic" = "t");
CREATE ROUTINE LOAD gw_uba.job_risk_events_to_fact ON risk_events FROM KAFKA("kafka_topic" = "r");
CREATE ROUTINE LOAD gw_uba.job_paused ON risk_events FROM KAFKA("kafka_topic" = "p");
CREATE ROUTINE LOAD gw_uba.job_running ON risk_events FROM KAFKA("kafka_topic" = "q");
CREATE ROUTINE LOAD gw_uba.job_stopped ON risk_events FROM KAFKA("kafka_topic" = "s");
`)},
	}
	report, err := Apply(context.Background(), conn.single(), scripts, Params{}, ApplyOptions{
		Policy: EnsurePolicy{Database: "gw_uba", ResumePaused: true},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	gotExec := conn.execed
	wantExec := []string{
		"CREATE TABLE events_fact (x INT)",
		"CREATE TABLE risk_events (x INT)",
		"USE gw_uba",
		`CREATE ROUTINE LOAD gw_uba.job_events_to_fact ON events_fact FROM KAFKA("kafka_topic" = "t")`,
		`CREATE ROUTINE LOAD gw_uba.job_risk_events_to_fact ON risk_events FROM KAFKA("kafka_topic" = "r")`,
		"RESUME ROUTINE LOAD FOR `gw_uba`.`job_paused`",
	}
	if strings.Join(gotExec, "\n") != strings.Join(wantExec, "\n") {
		t.Errorf("executed:\n%s\nwant:\n%s", strings.Join(gotExec, "\n"), strings.Join(wantExec, "\n"))
	}

	wantActions := map[string]Action{
		"job_events_to_fact":      ActionCreated,
		"job_risk_events_to_fact": ActionCreated,
		"job_paused":              ActionResumed,
		"job_running":             ActionAlreadyActive,
		"job_stopped":             ActionNeedsHuman,
	}
	if len(report.Results) != len(wantActions) {
		t.Fatalf("results = %d, want %d: %+v", len(report.Results), len(wantActions), report.Results)
	}
	for _, r := range report.Results {
		if want, ok := wantActions[r.Job.Name]; !ok {
			t.Errorf("unexpected job %q", r.Job.Name)
		} else if r.Action != want {
			t.Errorf("job %q action = %q, want %q", r.Job.Name, r.Action, want)
		}
	}
	if needs := report.NeedsHuman(); len(needs) != 1 || needs[0].Job.Name != "job_stopped" {
		t.Errorf("NeedsHuman() = %+v, want just job_stopped", needs)
	}
	if !conn.closed {
		t.Error("the connection must be released even on success")
	}
}

func TestApply_dryRunSendsNothingButTheStatusQuery(t *testing.T) {
	conn := &fakeConn{showResult: &fakeRows{columns: []string{"Name", "DbName", "State"}}}

	report, err := Apply(context.Background(), conn.single(), []Script{{
		Name:  "02.sql",
		Bytes: []byte(`CREATE ROUTINE LOAD db.job_a ON t FROM KAFKA("kafka_topic" = "x");`),
	}}, Params{}, ApplyOptions{Policy: EnsurePolicy{Database: "db", ResumePaused: true, DryRun: true}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(conn.execed) != 0 {
		t.Errorf("dry run executed %q", conn.execed)
	}
	if strings.Join(conn.queries, ";") != showAllRoutineLoad {
		t.Errorf("queries = %q, want only the listing", conn.queries)
	}
	if len(report.Results) != 1 || report.Results[0].Action != ActionCreated || !report.Plan {
		t.Errorf("results = %+v (plan=%v)", report.Results, report.Plan)
	}
	if summary := strings.Join(report.Summary(), "\n"); !strings.Contains(summary, "would created") {
		t.Errorf("summary should say what it would do:\n%s", summary)
	}
}

func TestApply_refusesRoutineLoadMutations(t *testing.T) {
	for _, stmt := range []string{
		"STOP ROUTINE LOAD FOR gw_uba.job_a;",
		"DROP ROUTINE LOAD IF EXISTS gw_uba.job_a;",
	} {
		conn := &fakeConn{}
		_, err := Apply(context.Background(), conn.single(), []Script{{Name: "02.sql", Bytes: []byte(stmt)}}, Params{}, ApplyOptions{})
		if err == nil || !strings.Contains(err.Error(), "mutates routine load jobs") {
			t.Errorf("Apply(%q) error = %v, want the mutation refusal", stmt, err)
		}
		if len(conn.execed) != 0 {
			t.Errorf("Apply(%q) executed %q before refusing", stmt, conn.execed)
		}
	}

	conn := &fakeConn{}
	if _, err := Apply(context.Background(), conn.single(), []Script{
		{Name: "02.sql", Bytes: []byte("CREATE TABLE t (x INT);\nDROP ROUTINE LOAD IF EXISTS gw_uba.job_a;\nCREATE TABLE u (x INT);\n")},
	}, Params{}, ApplyOptions{AllowRoutineLoadMutations: true}); err != nil {
		t.Fatalf("with the override the statement must run: %v", err)
	}
	want := []string{
		"CREATE TABLE t (x INT)",
		"DROP ROUTINE LOAD IF EXISTS gw_uba.job_a",
		"CREATE TABLE u (x INT)",
	}
	if strings.Join(conn.execed, "\n") != strings.Join(want, "\n") {
		t.Errorf("executed out of source order:\n%s\nwant:\n%s", strings.Join(conn.execed, "\n"), strings.Join(want, "\n"))
	}
}

func TestApply_reportsHowFarItGot(t *testing.T) {
	conn := &fakeConn{execErr: func(q string) error {
		if strings.HasPrefix(q, "CREATE TABLE b") {
			return errBoom
		}
		return nil
	}}

	report, err := Apply(context.Background(), conn.single(), []Script{{Name: "1.sql", Bytes: []byte(
		"CREATE TABLE a (x INT); CREATE TABLE b (x INT); CREATE TABLE c (x INT);")}}, Params{}, ApplyOptions{})
	if err == nil {
		t.Fatal("want the failure to surface")
	}
	if !strings.Contains(err.Error(), "1.sql:1") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should name the script, line and cause: %v", err)
	}
	if len(report.Applied) != 1 || report.Applied[0].Text != "CREATE TABLE a (x INT)" {
		t.Errorf("report should list what succeeded: %+v", report.Applied)
	}
}

func TestApply_failsWhenShowRoutineLoadFails(t *testing.T) {
	conn := &fakeConn{showErr: errBoom}
	_, err := Apply(context.Background(), conn.single(), []Script{{Name: "02.sql", Bytes: []byte(
		"CREATE ROUTINE LOAD db.job ON t FROM KAFKA();")}}, Params{}, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), showAllRoutineLoad) {
		t.Errorf("error = %v, want it to name the failing query", err)
	}
}

func TestApply_neverConnectsWhenScriptsHaveNoJobs(t *testing.T) {
	conn := &fakeConn{}
	report, err := Apply(context.Background(), conn.single(), []Script{
		{Name: "1.sql", Bytes: []byte("-- only a comment\n")},
	}, Params{}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(conn.queries) != 0 {
		t.Errorf("a DDL-only run should not query routine load: %q", conn.queries)
	}
	if len(report.Applied) != 0 {
		t.Errorf("applied = %q", report.Applied)
	}
}

func TestRunETL_keepsSectionOnOneConnectionInOrder(t *testing.T) {
	src := []byte(`
USE gw_uba;

-- ============================================================
-- 1. 会话
-- ============================================================
INSERT INTO sessions_fact SELECT 1 WHERE d = '{{.RunDate}}';

-- 2. 用户
-- 2.1 开启部分列更新
SET enable_unique_key_partial_update = true;
-- 2.2 写入
INSERT INTO users_dim SELECT 2 WHERE d = '{{.RunDate}}';
-- 2.3 关闭
SET enable_unique_key_partial_update = false;

-- 3. 聚合（不在默认白名单里）
INSERT INTO sessions_agg_daily SELECT 3;

-- 6. 校验
SELECT 'x', COUNT(*) FROM sessions_fact WHERE session_date = '{{.RunDate}}';
`)

	conn := &fakeConn{}
	report, err := RunETL(context.Background(), conn.single(), Script{Name: "06_etl.sql", Bytes: src},
		Params{"RunDate": "2026-06-28"}, ETLOptions{Sections: []string{"1", "2"}})
	if err != nil {
		t.Fatalf("RunETL: %v", err)
	}

	want := []string{
		"INSERT INTO sessions_fact SELECT 1 WHERE d = '2026-06-28'",
		"SET enable_unique_key_partial_update = true",
		"INSERT INTO users_dim SELECT 2 WHERE d = '2026-06-28'",
		"SET enable_unique_key_partial_update = false",
	}
	if strings.Join(conn.execed, "\n") != strings.Join(want, "\n") {
		t.Errorf("executed:\n%s\nwant:\n%s", strings.Join(conn.execed, "\n"), strings.Join(want, "\n"))
	}
	if report.RunDate != "2026-06-28" {
		t.Errorf("RunDate = %q", report.RunDate)
	}
	if len(report.Steps) != 4 {
		t.Fatalf("steps = %+v", report.Steps)
	}
	if report.Steps[1].Section != "2.1" || report.Steps[1].Title == "" {
		t.Errorf("step 1 keeps its sub-section identity: %+v", report.Steps[1])
	}
	if report.TotalRows() != 14 { // two INSERTs, none of the SETs report rows
		t.Errorf("TotalRows = %d, want 14", report.TotalRows())
	}

	// A missing parameter must fail loudly instead of shipping an empty date.
	if _, err := RunETL(context.Background(), (&fakeConn{}).single(), Script{Name: "06_etl.sql", Bytes: src},
		Params{}, ETLOptions{Sections: []string{"1"}}); err == nil ||
		!strings.Contains(err.Error(), "render 06_etl.sql") {
		t.Errorf("missing RunDate error = %v", err)
	}
}

func TestRunETL_dryRunAndGuards(t *testing.T) {
	etl := Script{Name: "06_etl.sql", Bytes: []byte("-- 1. a\nINSERT INTO t SELECT 1;\n")}

	conn := &fakeConn{}
	if _, err := RunETL(context.Background(), conn.single(), etl, Params{"RunDate": "2026-06-28"},
		ETLOptions{Sections: []string{"1"}, DryRun: true}); err != nil {
		t.Fatalf("RunETL dry run: %v", err)
	}
	if len(conn.execed) != 0 {
		t.Errorf("dry run executed %q", conn.execed)
	}

	// The verification section reads through Exec and would be discarded: refuse it.
	_, err := RunETL(context.Background(), (&fakeConn{}).single(), Script{Name: "06_etl.sql",
		Bytes: []byte("-- 1. a\nSELECT COUNT(*) FROM t;\n")}, Params{}, ETLOptions{Sections: []string{"1"}})
	if err == nil || !strings.Contains(err.Error(), "not runnable by the ETL runner") {
		t.Errorf("SELECT guard error = %v", err)
	}

	// Routine Load statements do not belong in an ETL script.
	_, err = RunETL(context.Background(), (&fakeConn{}).single(), Script{Name: "06_etl.sql", Bytes: []byte(
		"-- 1. a\nCREATE ROUTINE LOAD db.job ON t FROM KAFKA();\n")}, Params{}, ETLOptions{Sections: []string{"1"}})
	if err == nil || !strings.Contains(err.Error(), "routine load statements") {
		t.Errorf("routine load guard error = %v", err)
	}
}

func TestRender(t *testing.T) {
	src := []byte("\xEF\xBB\xBFSELECT '{{.A}}', '{{.B}}';\n")

	out, err := Render("x.sql", src, Params{"A": "1", "B": "2"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(out) != "SELECT '1', '2';\n" {
		t.Errorf("rendered = %q", out)
	}

	if _, err := Render("x.sql", src, Params{"A": "1"}); err == nil ||
		!strings.Contains(err.Error(), "render x.sql") {
		t.Errorf("missing key error = %v", err)
	}

	leftover, err := Render("y.sql", []byte("SELECT ${RUN_DATE};"), Params{})
	if err == nil || !strings.Contains(err.Error(), "was not substituted") {
		t.Errorf("unresolved placeholder error = %v (out=%q)", err, leftover)
	}

	if _, err := Render("z.sql", []byte("SELECT {{."), Params{}); err == nil ||
		!strings.Contains(err.Error(), "parse template z.sql") {
		t.Errorf("template parse error = %v", err)
	}
}

func TestRenderKeepsLineNumbers(t *testing.T) {
	// Templates that only substitute inline are what lets an error message address the file
	// on disk; a block template would break that promise.
	src := []byte("CREATE TABLE a (x INT);\nSET b = '{{.V}}';\n")
	stmts, err := Load(Script{Name: "f.sql", Bytes: src}, Params{"V": "1"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stmts[1].Line != 2 {
		t.Errorf("second statement on line %d, want 2", stmts[1].Line)
	}
}

func TestRenderLoadErrorsNameTheFile(t *testing.T) {
	_, err := Load(Script{Name: "f.sql", Bytes: []byte("SELECT 'unterminated;")}, Params{})
	if err == nil || !strings.Contains(err.Error(), "f.sql") || !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("Load error = %v, want file name and cause", err)
	}
}

func TestCheckRunDate(t *testing.T) {
	valid := []string{"2026-06-28", "2026-01-01", " 2026-12-31 "}
	for _, s := range valid {
		if err := CheckRunDate(s); err != nil {
			t.Errorf("CheckRunDate(%q) = %v", s, err)
		}
	}
	invalid := []string{"", "2026-6-28", "28-06-2026", "2026-06-28T00:00:00", "2026-13-01", "'2026-06-28'", "2026-06-28' OR 1=1 --"}
	for _, s := range invalid {
		if err := CheckRunDate(s); err == nil {
			t.Errorf("CheckRunDate(%q) accepted an invalid date", s)
		}
	}
}

func TestPreviousRunDate(t *testing.T) {
	tests := []struct{ in, want string }{
		{"2026-06-28", "2026-06-27"},
		{"2026-03-01", "2026-02-28"},
		{"2024-03-01", "2024-02-29"},
		{"2026-01-01", "2025-12-31"},
	}
	for _, tt := range tests {
		got, err := PreviousRunDate(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("PreviousRunDate(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	if _, err := PreviousRunDate("nope"); err == nil {
		t.Error("want an error for a non-date")
	}
}

func TestDefaultRunDate(t *testing.T) {
	noon := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	if got := DefaultRunDate(noon); got != "2026-06-27" {
		t.Errorf("DefaultRunDate = %q, want 2026-06-27", got)
	}
}

func TestBrokerList(t *testing.T) {
	got, err := BrokerList([]string{"kafka:9092", " kafka-2:9092 ", ""})
	if err != nil || got != "kafka:9092,kafka-2:9092" {
		t.Errorf("BrokerList = %q, %v", got, err)
	}

	for _, bad := range [][]string{nil, {}, {""}, {"kafka:9092\"; DROP TABLE x; --"}} {
		if _, err := BrokerList(bad); err == nil {
			t.Errorf("BrokerList(%q) accepted a bad list", bad)
		} else if !regexp.MustCompile(`kafka|endpoint|broker`).MatchString(err.Error()) {
			t.Errorf("BrokerList(%q) error = %v, want it to name the problem", bad, err)
		}
	}
}

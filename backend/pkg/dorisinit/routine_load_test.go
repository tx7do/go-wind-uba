package dorisinit

import (
	"context"
	"fmt"

	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	src := `
USE gw_uba;

-- 1. 行为事件流入任务
CREATE ROUTINE LOAD gw_uba.job_events_to_fact
ON events_fact
COLUMNS(event_id)
PROPERTIES ("jsonpaths" = "[\"$.eventId\"]", "max_batch_rows" = "200000")
FROM KAFKA (
    "kafka_broker_list" = "kafka:9092",
    "kafka_topic" = "uba_events_raw"
);

-- 2. 风险事件流入任务
CREATE ROUTINE LOAD job_risk_events_to_fact ON risk_events
PROPERTIES("format" = "json")
FROM KAFKA("kafka_topic" = "uba_risk_events");

STOP ROUTINE LOAD FOR gw_uba.job_events_to_fact;
DROP ROUTINE LOAD IF EXISTS gw_uba.job_events_to_fact;
RESUME ROUTINE LOAD FOR gw_uba.job_events_to_fact;
ALTER ROUTINE LOAD FOR gw_uba.job_events_to_fact PROPERTIES("max_batch_interval" = "16");
PAUSE ROUTINE LOAD FOR gw_uba.job_events_to_fact;
`
	stmts := mustStatements(t, src)
	plan, err := Classify(stmts)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if len(plan.Plain) != 1 || plan.Plain[0].Text != "USE gw_uba" {
		t.Errorf("plain = %q, want only the USE", texts(plan.Plain))
	}
	if len(plan.Creates) != 2 {
		t.Fatalf("creates = %d, want 2", len(plan.Creates))
	}
	want := []RoutineLoadJob{
		{Database: "gw_uba", Name: "job_events_to_fact", Table: "events_fact"},
		{Database: "", Name: "job_risk_events_to_fact", Table: "risk_events"},
	}
	for i, w := range want {
		got := plan.Creates[i]
		if got.Database != w.Database || got.Name != w.Name || got.Table != w.Table {
			t.Errorf("create %d = %+v, want %+v", i, got, w)
		}
	}
	if len(plan.Mutations) != 5 {
		t.Errorf("mutations = %d (%v), want 5", len(plan.Mutations), texts(plan.Mutations))
	}
}

func TestClassify_ignoresComments(t *testing.T) {
	// Detection reads the comment-stripped code, so a comment that quotes the lifecycle
	// statements cannot make a script look destructive.
	src := "-- STOP ROUTINE LOAD FOR x;\n/* DROP ROUTINE LOAD IF EXISTS y */\nCREATE TABLE t (x INT);\n"
	plan, err := Classify(mustStatements(t, src))
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(plan.Mutations) != 0 || len(plan.Creates) != 0 {
		t.Errorf("mutations=%v creates=%v, want neither", texts(plan.Mutations), plan.Creates)
	}
	if len(plan.Plain) != 1 {
		t.Fatalf("plain = %q, want the CREATE TABLE", texts(plan.Plain))
	}
}

func TestQualified(t *testing.T) {
	tests := []struct {
		job       RoutineLoadJob
		defaultDB string
		want      string
	}{
		{RoutineLoadJob{Database: "gw_uba", Name: "job_a"}, "other", "`gw_uba`.`job_a`"},
		{RoutineLoadJob{Name: "job_a"}, "gw_uba", "`gw_uba`.`job_a`"},
		{RoutineLoadJob{Name: "job_a"}, "", "`job_a`"},
		{RoutineLoadJob{Name: "job`a"}, "db", "`db`.`joba`"},
	}
	for _, tt := range tests {
		if got := tt.job.Qualified(tt.defaultDB); got != tt.want {
			t.Errorf("Qualified(%q) = %q, want %q", tt.defaultDB, got, tt.want)
		}
	}
}

func TestDecodeJobs(t *testing.T) {
	// A realistic 4.x header, abbreviated: columns this package does not know about must
	// still survive in Fields, and a version that drops one must not break decoding.
	rs := &fakeRows{
		columns: []string{"Id", "Name", "DbName", "TableName", "State", "Statistic", "Progress", "Lag", "ReasonOfStateChanged", "ErrorLogUrls", "NewInFutureVersion"},
		rows: [][]any{
			{int64(195002), "job_events_to_fact", "gw_uba", "events_fact", "PAUSED",
				`{"totalRows":10,"loadedRows":8,"errorRows":2,"runningTxns":[]}`, `{"0":"42"}`, `{"0":"7"}`,
				"Too many filtered rows", "be:8040/log", nil},
			{int64(195003), "job_risk_events_to_fact", "gw_uba", "risk_events", "RUNNING",
				"", "N/A", "N/A", "N/A", "N/A", "x"},
		},
	}

	jobs, err := DecodeJobs(rs)
	if err != nil {
		t.Fatalf("DecodeJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}

	first := jobs[0]
	if first.Name != "job_events_to_fact" || first.Database != "gw_uba" || first.State != "PAUSED" {
		t.Errorf("identity = %+v", first)
	}
	if got := first.Column("NewInFutureVersion"); got != "" {
		t.Errorf("NULL column = %q, want empty", got)
	}
	if got := first.Diagnostics(); !strings.Contains(got, "ReasonOfStateChanged=Too many filtered rows") {
		t.Errorf("Diagnostics() = %q", got)
	}
	if got := first.Diagnostics(); strings.Contains(got, "ErrorLogUrls=N/A") {
		t.Errorf("Diagnostics() should drop N/A values: %q", got)
	}
	if first.Progress() != `{"0":"42"}` || first.Lag() != `{"0":"7"}` {
		t.Errorf("progress/lag = %q / %q", first.Progress(), first.Lag())
	}

	stat, ok := DecodeStatistic(first.Column("Statistic"))
	if !ok {
		t.Fatalf("DecodeStatistic(%q) reported not ok", first.Column("Statistic"))
	}
	if stat.TotalRows != 10 || stat.LoadedRows != 8 || stat.ErrorRows != 2 {
		t.Errorf("statistic = %+v", stat)
	}
	if _, ok := DecodeStatistic(jobs[1].Column("Progress")); ok {
		t.Errorf("DecodeStatistic should refuse a non-JSON value")
	}
}

func TestDecodeJobs_errorsSurface(t *testing.T) {
	if _, err := DecodeJobs(&fakeRows{err: context.DeadlineExceeded}); err == nil {
		t.Error("want the row error to surface")
	}
	if _, err := DecodeJobs(&fakeRows{columnsErr: errBoom}); err == nil {
		t.Error("want the columns error to surface")
	}
	if _, err := DecodeJobs(&fakeRows{columns: []string{"Name"}, rows: [][]any{{}, {}}}); err == nil {
		t.Error("want the scan error to surface")
	}
}

func TestStateClassification(t *testing.T) {
	tests := []struct {
		in       string
		want     State
		live     bool
		terminal bool
	}{
		{"RUNNING", StateRunning, true, false},
		{"NEED_SCHEDULE", StateScheduling, true, false},
		{" paused ", StatePaused, false, false},
		{"STOPPED", StateStopped, false, true},
		{"CANCELLED", StateCancelled, false, true},
		{"FINISHED", StateFinished, false, true},
		{"SOMETHING_NEW", State("SOMETHING_NEW"), false, false},
	}
	for _, tt := range tests {
		got := ParseState(tt.in)
		if got != tt.want || got.Live() != tt.live || got.Terminal() != tt.terminal {
			t.Errorf("ParseState(%q) = %q (live=%v terminal=%v), want %q (live=%v terminal=%v)",
				tt.in, got, got.Live(), got.Terminal(), tt.want, tt.live, tt.terminal)
		}
	}
}

func TestDecide(t *testing.T) {
	creates := []RoutineLoadJob{
		{Name: "job_missing", Database: "gw_uba", Create: Statement{Text: "CREATE ROUTINE LOAD gw_uba.job_missing ON t"}},
		{Name: "job_running", Database: "gw_uba"},
		{Name: "job_paused", Database: "gw_uba"},
		{Name: "job_stopped", Database: "gw_uba"},
		{Name: "job_cancelled", Database: "gw_uba"},
		{Name: "job_scheduling", Database: "gw_uba"},
	}
	existing := []Job{
		{Name: "job_running", Database: "gw_uba", State: "RUNNING"},
		{Name: "job_paused", Database: "gw_uba", State: "PAUSED", Fields: map[string]string{"ReasonOfStateChanged": "max_error_number"}},
		{Name: "job_stopped", Database: "gw_uba", State: "STOPPED"},
		{Name: "job_cancelled", Database: "gw_uba", State: "CANCELLED"},
		{Name: "job_scheduling", Database: "gw_uba", State: "NEED_SCHEDULE"},
		{Name: "job_of_another_db", Database: "other", State: "PAUSED"},
	}

	got := Decide(creates, existing, EnsurePolicy{Database: "gw_uba", ResumePaused: true})
	want := []struct {
		name   string
		action Action
		prior  State
		detail string
	}{
		{"job_missing", ActionCreated, StateAbsent, ""},
		{"job_running", ActionAlreadyActive, StateRunning, ""},
		{"job_paused", ActionResumed, StatePaused, "ReasonOfStateChanged=max_error_number"},
		{"job_stopped", ActionNeedsHuman, StateStopped, ""},
		{"job_cancelled", ActionNeedsHuman, StateCancelled, ""},
		{"job_scheduling", ActionAlreadyActive, StateScheduling, ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d", len(got), len(want))
	}
	for i, w := range want {
		r := got[i]
		if r.Job.Name != w.name || r.Action != w.action || r.Prior != w.prior {
			t.Errorf("result %d = %+v, want %s/%s/%s", i, r, w.name, w.action, w.prior)
		}
		if r.Detail != w.detail {
			t.Errorf("result %d detail = %q, want %q", i, r.Detail, w.detail)
		}
	}

	// The statements ensure would send are the create itself and a qualified RESUME.
	if got[0].Statement.Text != "CREATE ROUTINE LOAD gw_uba.job_missing ON t" {
		t.Errorf("create statement = %q", got[0].Statement.Text)
	}
	if got[2].Statement.Text != "RESUME ROUTINE LOAD FOR `gw_uba`.`job_paused`" {
		t.Errorf("resume statement = %q", got[2].Statement.Text)
	}
	for _, r := range []Result{got[1], got[3], got[4], got[5]} {
		if r.Statement.Text != "" {
			t.Errorf("%s must not execute anything, got %q", r.Job.Name, r.Statement.Text)
		}
	}

	// A job that exists in another database must not be mistaken for ours.
	withForeign := Decide([]RoutineLoadJob{{Name: "job_of_another_db"}}, existing, EnsurePolicy{Database: "gw_uba"})
	if withForeign[0].Action != ActionCreated {
		t.Errorf("cross-database name collision: %+v", withForeign[0])
	}

	// Read-only mode never resumes, so a status pass cannot mutate a cluster.
	readOnly := Decide(creates, existing, EnsurePolicy{Database: "gw_uba"})
	if readOnly[2].Action != ActionNeedsHuman || !strings.HasPrefix(readOnly[2].Detail, "resume is disabled") {
		t.Errorf("resume-paused off: %+v", readOnly[2])
	}
}

// fakeRows implements Rows over an in-memory result set.
type fakeRows struct {
	columns    []string
	rows       [][]any
	at         int
	err        error
	columnsErr error
}

func (f *fakeRows) Columns() ([]string, error) {
	if f.columnsErr != nil {
		return nil, f.columnsErr
	}
	return f.columns, nil
}

func (f *fakeRows) Next() bool { return f.at < len(f.rows) }

func (f *fakeRows) Scan(dest ...any) error {
	row := f.rows[f.at]
	f.at++
	if len(row) != len(dest) {
		return &rowError{len(row), len(dest)}
	}
	if f.err != nil {
		return f.err
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *[]byte:
			if row[i] != nil {
				*target = []byte(fmt.Sprint(row[i]))
			}
		default:
			return &rowError{0, 0}
		}
	}
	return nil
}

func (f *fakeRows) Err() error { return f.err }

func (f *fakeRows) Close() error { return nil }

type rowError struct{ got, want int }

func (e *rowError) Error() string {
	return fmt.Sprintf("scan: %d values for %d columns", e.got, e.want)
}

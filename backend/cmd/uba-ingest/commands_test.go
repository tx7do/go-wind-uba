package main

import (
	"slices"
	"strings"
	"testing"

	"go-wind-uba/pkg/dorisinit"
)

// The status report is what an operator trusts to answer "is ingestion actually flowing", so
// the merge of declared-vs-reported has to be right in both directions.
func TestJobViewsCoversBothDirections(t *testing.T) {
	declared := []dorisinit.RoutineLoadJob{
		{Database: "gw_uba", Name: "job_events_to_fact", Table: "events_fact"},
		{Database: "gw_uba", Name: "job_risk_events_to_fact", Table: "risk_events"},
	}
	reported := []dorisinit.Job{
		newJob("gw_uba", "job_events_to_fact", "RUNNING", `{"totalRows":100,"loadedRows":99,"errorRows":1}`),
		newJob("gw_uba", "job_risk_events_to_fact", "PAUSED", `{"totalRows":10,"loadedRows":4,"errorRows":6}`),
		newJob("gw_uba", "job_legacy_import", "STOPPED", ""),
	}

	results := dorisinit.Decide(declared, reported, dorisinit.EnsurePolicy{Database: "gw_uba", ResumePaused: true, DryRun: true})
	views := jobViews(results, reported, "gw_uba")

	byJob := map[string]jobView{}
	for _, v := range views {
		byJob[v.Job] = v
	}

	if len(views) != 3 {
		t.Fatalf("got %d rows, want 3 (two declared + one undeclared): %+v", len(views), views)
	}

	running := byJob["gw_uba.job_events_to_fact"]
	if !running.Declared || !running.Live || running.Decision != string(dorisinit.ActionAlreadyActive) {
		t.Errorf("running job = %+v", running)
	}
	if !strings.Contains(running.Rows, "loaded 99 of 100, 1 errors") {
		t.Errorf("statistic not decoded: %q", running.Rows)
	}

	paused := byJob["gw_uba.job_risk_events_to_fact"]
	if paused.Live || paused.Decision != string(dorisinit.ActionResumed) {
		t.Errorf("paused job = %+v", paused)
	}

	legacy := byJob["gw_uba.job_legacy_import"]
	if legacy.Declared || legacy.Decision != "undeclared" {
		t.Errorf("undeclared job = %+v", legacy)
	}

	// A job the scripts declare but the cluster has never seen must not disappear from the
	// report: that is the failure mode this whole tool exists to catch.
	missing := dorisinit.Decide(declared, reported[:0], dorisinit.EnsurePolicy{Database: "gw_uba"})
	if got := jobViews(missing, nil, "gw_uba"); len(got) != 2 || got[0].State != "ABSENT" {
		t.Errorf("absent jobs = %+v", got)
	}
}

// An unqualified declaration resolves against the database the DSN points at, which is also
// how Doris reports it — so the two spellings must collapse to one row, not two.
func TestJobViewsMatchesAnUnqualifiedDeclaration(t *testing.T) {
	declared := []dorisinit.RoutineLoadJob{{Name: "job_events_to_fact", Table: "events_fact"}}
	reported := []dorisinit.Job{newJob("GW_UBA", "JOB_EVENTS_TO_FACT", "RUNNING", "")}

	results := dorisinit.Decide(declared, reported, dorisinit.EnsurePolicy{Database: "gw_uba"})
	views := jobViews(results, reported, "gw_uba")

	if len(views) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(views), views)
	}
	if views[0].State != "RUNNING" || views[0].Decision != string(dorisinit.ActionAlreadyActive) {
		t.Errorf("view = %+v", views[0])
	}
}

func TestSettingsEndpointNeverPrintsAPassword(t *testing.T) {
	st := &settings{dsn: "analyst:Sup3r-Secret@tcp(doris-fe:9030)/gw_uba?charset=utf8mb4", configPath: "configs"}

	got := st.endpoint()
	if strings.Contains(got, "Sup3r-Secret") {
		t.Errorf("the password leaked into a printable DSN: %s", got)
	}
	if !strings.Contains(got, "analyst@tcp(doris-fe:9030)/gw_uba") {
		t.Errorf("the DSN lost the part an operator needs: %s", got)
	}

	if !strings.Contains(st.describe(), "database gw_uba") {
		t.Errorf("describe() = %q", st.describe())
	}

	// An unparsable DSN must not be echoed either: it may be a typo of the same secret.
	broken := &settings{dsn: "not a dsn at all"}
	if got := broken.endpoint(); !strings.Contains(got, "unparsable") {
		t.Errorf("endpoint() = %q", got)
	}
}

func TestETLSectionsDefaultToTheIdempotentRollUps(t *testing.T) {
	// A scheduled run repeats a day whenever a container restarts inside its own hour, so
	// the default has to be the sections that replace rather than accumulate.
	if got := (&options{command: "etl"}).etlSections(); !slices.Equal(got, defaultETLSections) {
		t.Errorf("default sections = %v want %v", got, defaultETLSections)
	}
	for _, s := range defaultETLSections {
		if additiveETLSections[s] {
			t.Errorf("section %s is in the default but cannot be re-run for a day", s)
		}
		if additiveWarning([]string{s}) != "" {
			t.Errorf("section %s warned about despite being a default", s)
		}
	}

	if got := (&options{command: "etl", sections: stringList{"1", "3", "5"}}).etlSections(); !slices.Equal(got, []string{"1", "3", "5"}) {
		t.Errorf("sections = %v", got)
	}

	warning := additiveWarning([]string{"1", "3"})
	if !strings.Contains(warning, "sections 3") || !strings.Contains(warning, "sums") {
		t.Errorf("warning does not name the section and the consequence: %q", warning)
	}
	if additiveWarning([]string{"1", "2"}) != "" {
		t.Error("the idempotent sections should not warn")
	}
}

func TestETLLinesMarksAPlanRun(t *testing.T) {
	report := &dorisinit.ETLReport{
		RunDate: "2026-06-28",
		Steps: []dorisinit.ETLStep{
			{Section: "1", Title: "填充 sessions_fact", Line: 44, Text: "INSERT INTO sessions_fact (\n  session_id\n) SELECT 1"},
			{Section: "2", Title: "users_dim", Line: 95, Text: "SET enable_unique_key_partial_update = true", Rows: 0},
			{Section: "2", Title: "users_dim", Line: 98, Text: "INSERT INTO users_dim SELECT 1", Rows: 12},
		},
	}

	plan := strings.Join(etlLines(report, true), "\n")
	if !strings.Contains(plan, "run_date 2026-06-28 · sections 1, 2") {
		t.Errorf("plan header = %q", plan)
	}
	if strings.Count(plan, "plan ") != 3 {
		t.Errorf("a dry run must mark every step as a plan:\n%s", plan)
	}
	if !strings.Contains(plan, "SELECT 1  (12 rows)") {
		t.Errorf("row count missing:\n%s", plan)
	}

	ran := strings.Join(etlLines(report, false), "\n")
	if strings.Contains(ran, "plan ") || !strings.Contains(ran, "ok  1:44") {
		t.Errorf("real run lines:\n%s", ran)
	}
}

func TestAbbrevKeepsOneLinePerEntry(t *testing.T) {
	if got := abbrev("INSERT INTO  sessions_fact\n  (a, b)\nSELECT", 3); got != "INSERT INTO sessions_fact …" {
		t.Errorf("abbrev = %q", got)
	}
	if got := abbrev("", 4); got != "" {
		t.Errorf("abbrev of an empty string = %q", got)
	}
}

func newJob(db, name, state, statistic string) dorisinit.Job {
	return dorisinit.Job{
		Database: db,
		Name:     name,
		State:    state,
		Fields:   map[string]string{"DbName": db, "Name": name, "State": state, "Statistic": statistic},
	}
}

package doris

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"go-wind-uba/pkg/dorisinit"
)

// The scripts in this directory are the input to pkg/dorisinit. These tests are the guard
// rail that keeps them parseable: a byte-order mark before the leading "--" makes Doris
// reject the whole file, a CRLF leaks a stray "\r" into every statement, and an unrendered
// placeholder reaches the server as a syntax error that hides its own cause.

var legacyPlaceholder = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// renderedScripts are the files uba-ingest actually passes through the template engine. The
// hand-run cookbook files (query.sql, demo-data.sql) keep their own ${VAR} conventions and
// are only held to the encoding rules below.
func renderedScripts() map[string]bool {
	out := map[string]bool{}
	for _, s := range AllScripts() {
		out[s.Name] = true
	}
	return out
}

func TestShippedSQLIsCleanUTF8(t *testing.T) {
	sqlRoot := filepath.Join("..")
	rendered := renderedScripts()

	var checked int
	err := filepath.Walk(sqlRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".sql") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++

		if bytes.HasPrefix(src, []byte{0xEF, 0xBB, 0xBF}) {
			t.Errorf("%s: starts with a UTF-8 BOM, which breaks the first '--' comment", path)
		}
		if i := bytes.IndexByte(src, '\r'); i >= 0 {
			line := 1 + bytes.Count(src[:i], []byte{'\n'})
			t.Errorf("%s:%d: CR found; SQL scripts must be LF (see the root .gitattributes)", path, line)
		}
		if rendered[filepath.Base(path)] {
			if m := legacyPlaceholder.Find(src); m != nil {
				t.Errorf("%s: %s is not a supported placeholder; use {{.Name}} so uba-ingest can render it", path, m)
			}
		}
		if !utf8.Valid(src) {
			t.Errorf("%s: not valid UTF-8", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 10 {
		t.Fatalf("only %d .sql files checked from %s; the walk is broken", checked, sqlRoot)
	}
}

func TestInitScriptsRenderAndClassify(t *testing.T) {
	params := dorisinit.Params{"KafkaBrokerList": "kafka:9092"}

	var (
		jobs  []dorisinit.RoutineLoadJob
		empty []string
	)
	for _, s := range InitScripts() {
		stmts, err := dorisinit.Load(s, params)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		if len(stmts) == 0 {
			empty = append(empty, s.Name)
			continue
		}
		plan, err := dorisinit.Classify(stmts)
		if err != nil {
			t.Fatalf("%s: classify: %v", s.Name, err)
		}
		if len(plan.Mutations) > 0 {
			t.Errorf("%s declares routine load lifecycle statements %v; apply would refuse to run them",
				s.Name, stmtTexts(plan.Mutations))
		}
		jobs = append(jobs, plan.Creates...)
	}

	// 04_indexes.sql is comments-only today. It stays in the apply list on purpose, but if
	// it ever gains statements this test should be told about it rather than go stale.
	if want := []string{"04_indexes.sql"}; !slices.Equal(empty, want) {
		t.Errorf("scripts with no statements = %v, want %v", empty, want)
	}

	var names []string
	for _, j := range jobs {
		names = append(names, j.Database+"."+j.Name+"->"+j.Table)
	}
	want := []string{"gw_uba.job_events_to_fact->events_fact", "gw_uba.job_risk_events_to_fact->risk_events"}
	if !slices.Equal(names, want) {
		t.Errorf("routine load jobs = %v, want %v", names, want)
	}
	for _, j := range jobs {
		if !strings.Contains(j.Create.Text, `"kafka_broker_list" = "kafka:9092"`) {
			t.Errorf("job %s did not receive the broker list:\n%s", j.Name, j.Create.Text)
		}
	}
}

func TestEtlSectionsAreRunnableAndWellFormed(t *testing.T) {
	script := EtlScript()
	rendered, stmts, err := dorisinit.LoadRendered(script, dorisinit.Params{"RunDate": "2026-06-28"})
	if err != nil {
		t.Fatalf("%s: %v", script.Name, err)
	}

	picked, err := dorisinit.SelectSections(dorisinit.SplitSections(rendered, stmts), "1", "2")
	if err != nil {
		t.Fatalf("sections: %v", err)
	}

	var numbers, kinds []string
	for _, s := range picked {
		numbers = append(numbers, s.Number)
		for _, st := range s.Statements {
			kinds = append(kinds, st.FirstWord())
			if !strings.Contains(st.Text, "'2026-06-28'") && st.FirstWord() == "INSERT" {
				t.Errorf("section %s INSERT is not scoped to the run date:\n%s", s.Number, st.Text)
			}
		}
	}
	if want := []string{"1", "2", "2.1", "2.2", "2.3"}; !slices.Equal(numbers, want) {
		t.Errorf("sections = %v, want %v", numbers, want)
	}
	// Section 2 must keep its SET/INSERT/SET triple together, otherwise the partial update
	// flag would be set on a different connection from the INSERT that needs it.
	if want := []string{"INSERT", "SET", "INSERT", "SET"}; !slices.Equal(kinds, want) {
		t.Errorf("statement kinds = %v, want %v", kinds, want)
	}
}

func TestEtlSectionNumbersCoverTheFile(t *testing.T) {
	// The whole-file view, so a renumbering that turns a bullet into a header (or the
	// reverse) shows up here instead of silently changing which steps uba-ingest runs.
	script := EtlScript()
	rendered, stmts, err := dorisinit.LoadRendered(script, dorisinit.Params{"RunDate": "2026-06-28"})
	if err != nil {
		t.Fatal(err)
	}
	var numbers []string
	for _, s := range dorisinit.SplitSections(rendered, stmts) {
		numbers = append(numbers, s.Number)
	}
	if want := []string{"", "0", "1", "2", "2.1", "2.2", "2.3", "3", "4", "5", "6"}; !slices.Equal(numbers, want) {
		t.Errorf("section headers in %s = %v, want %v", script.Name, numbers, want)
	}
}

func stmtTexts(stmts []dorisinit.Statement) []string {
	out := make([]string, len(stmts))
	for i, s := range stmts {
		out[i] = strings.TrimSpace(s.Text)
	}
	return out
}

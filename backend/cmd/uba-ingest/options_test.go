package main

import (
	"flag"
	"slices"
	"strings"
	"testing"

	"go-wind-uba/pkg/dorisinit"
	"go-wind-uba/sql/doris"
)

func TestParseFlagsRoutesSubcommands(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   func(*testing.T, *options)
		errSub string
	}{
		{
			name: "apply with a dry run and a global config path",
			args: []string{"-c", "/app/configs", "apply", "--dry-run"},
			want: func(t *testing.T, o *options) {
				t.Helper()
				if o.command != "apply" || !o.dryRun {
					t.Errorf("command=%q dryRun=%v", o.command, o.dryRun)
				}
				if got := o.global.configPath(); got != "/app/configs" {
					t.Errorf("configPath=%q", got)
				}
				if !o.resumePaused {
					t.Error("apply should resume a paused job by default; that is the whole point of ensure")
				}
			},
		},
		{
			name: "apply keeps its own flags after the command",
			args: []string{"apply", "--resume-paused=false", "--script", "1_base_tables.sql"},
			want: func(t *testing.T, o *options) {
				t.Helper()
				if o.resumePaused {
					t.Error("resumePaused should be false")
				}
				if want := []string{"1_base_tables.sql"}; !slices.Equal([]string(o.scripts), want) {
					t.Errorf("scripts=%v want %v", o.scripts, want)
				}
			},
		},
		{
			name: "status json",
			args: []string{"status", "--json"},
			want: func(t *testing.T, o *options) {
				t.Helper()
				if o.command != "status" || !o.jsonOut {
					t.Errorf("command=%q json=%v", o.command, o.jsonOut)
				}
			},
		},
		{
			name: "etl accepts comma separated sections",
			args: []string{"etl", "--date", "2026-06-28", "--backfill", "3", "--sections", "1, 2"},
			want: func(t *testing.T, o *options) {
				t.Helper()
				if want := []string{"1", "2"}; !slices.Equal([]string(o.sections), want) {
					t.Errorf("sections=%v want %v", o.sections, want)
				}
				if o.backfill != 3 || o.date != "2026-06-28" {
					t.Errorf("backfill=%d date=%q", o.backfill, o.date)
				}
			},
		},
		{
			name:   "a flag before the command is global",
			args:   []string{"--sections", "1", "etl"},
			errSub: "flag provided but not defined",
		},
		{
			name:   "a dsn never comes off the command line",
			args:   []string{"--dsn", "root:secret@tcp(h:9030)/db", "status"},
			errSub: "flag provided but not defined",
		},
		{
			name:   "an unknown command is refused",
			args:   []string{"destroy"},
			errSub: `unknown command "destroy"`,
		},
		{
			name:   "a stray argument is refused",
			args:   []string{"apply", "extra"},
			errSub: "unexpected argument",
		},
		{
			name:   "no command at all",
			args:   nil,
			errSub: "no command given",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseFlags(tt.args)
			if tt.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("err=%v, want one containing %q", err, tt.errSub)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tt.want(t, opts)
		})
	}
}

func TestParseFlagsHelp(t *testing.T) {
	if _, err := parseFlags([]string{"-h"}); err != flag.ErrHelp {
		t.Errorf("err=%v, want flag.ErrHelp", err)
	}
}

func TestValidateFillsTheRunDate(t *testing.T) {
	opts := &options{command: "etl", backfill: 1}
	if err := opts.validate(); err != nil {
		t.Fatal(err)
	}
	if err := dorisinit.CheckRunDate(opts.date); err != nil {
		t.Errorf("an omitted --date should default to a well-formed one, got %q: %v", opts.date, err)
	}

	explicit := &options{command: "etl", date: "2026-06-28", backfill: 1}
	if err := explicit.validate(); err != nil {
		t.Fatal(err)
	}
	if explicit.date != "2026-06-28" {
		t.Errorf("validate overwrote --date with %q", explicit.date)
	}
}

func TestValidateRejectsUnusableFlags(t *testing.T) {
	tests := []struct {
		name   string
		opts   *options
		errSub string
	}{
		{
			name:   "etl rejects a date that is not a calendar date",
			opts:   &options{command: "etl", date: "2026-06-28 or 1=1", backfill: 1},
			errSub: "run date",
		},
		{
			name:   "etl rejects a backfill of zero",
			opts:   &options{command: "etl", date: "2026-06-28", backfill: 0},
			errSub: "--backfill",
		},
		{
			name:   "apply rejects a dry run that allows mutations",
			opts:   &options{command: "apply", dryRun: true, allowRoutineLoadMutations: true},
			errSub: "allow-routine-load-mutations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.validate()
			if err == nil || !strings.Contains(err.Error(), tt.errSub) {
				t.Fatalf("err=%v, want one containing %q", err, tt.errSub)
			}
		})
	}
}

// A backfill recomputes whole days independently, so the order only decides what an operator
// sees first; what matters is that the window is the requested length and never repeats a day.
func TestETLDatesWalkBackwards(t *testing.T) {
	opts := &options{command: "etl", date: "2026-06-28", backfill: 3}
	dates, err := opts.etlDates()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2026-06-28", "2026-06-27", "2026-06-26"}; !slices.Equal(dates, want) {
		t.Errorf("dates=%v, want %v", dates, want)
	}
}

func TestSelectedScripts(t *testing.T) {
	all := doris.InitScripts()

	t.Run("no filter keeps the apply order", func(t *testing.T) {
		got, err := (&options{}).selectedScripts(all)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(scriptNames(got), scriptNames(all)) {
			t.Errorf("got %v, want %v", scriptNames(got), scriptNames(all))
		}
	})

	t.Run("a path is matched by its base name", func(t *testing.T) {
		opts := &options{scripts: []string{"backend/sql/doris/03_aggregate_tables.sql"}}
		got, err := opts.selectedScripts(all)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "03_aggregate_tables.sql" {
			t.Fatalf("got %v", scriptNames(got))
		}
	})

	t.Run("an unknown name lists the real ones", func(t *testing.T) {
		_, err := (&options{scripts: []string{"07_nope.sql"}}).selectedScripts(all)
		if err == nil {
			t.Fatal("want an error")
		}
		for _, name := range scriptNames(all) {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error should list %q so the operator can retry: %v", name, err)
			}
		}
	})
}

// The broker list is only resolved for scripts that reference it, which is what lets a
// deployment create its tables before kafka addresses are known.
func TestBuildParamsIsLazyAboutBrokers(t *testing.T) {
	st := &settings{configPath: "configs"}

	params, err := st.buildParams([]dorisinit.Script{{Name: "no_params.sql", Bytes: []byte("SELECT 1;")}}, "")
	if err != nil {
		t.Fatalf("a script with no placeholders should apply without kafka config: %v", err)
	}
	if len(params) != 0 {
		t.Errorf("params=%v, want none", params)
	}

	if _, err := st.buildParams(doris.InitScripts(), ""); err == nil {
		t.Error("02_kafka_tables.sql references the broker list, so an unconfigured deployment must fail loudly")
	}

	st.kafkaOverride = "kafka-1:9092, kafka-2:9092"
	params, err = st.buildParams(doris.InitScripts(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := params["KafkaBrokerList"]; got != "kafka-1:9092,kafka-2:9092" {
		t.Errorf("KafkaBrokerList=%v", got)
	}
}

// The ETL script is parameterised by date, and a missing or malformed one would either fail in
// the parser or interpolate SQL into a quoted literal.
func TestBuildParamsRequiresTheRunDate(t *testing.T) {
	st := &settings{configPath: "configs"}
	script := []dorisinit.Script{doris.EtlScript()}

	if _, err := st.buildParams(script, ""); err == nil {
		t.Error("want an error when the date is missing")
	}
	if _, err := st.buildParams(script, "yesterday"); err == nil {
		t.Error("want an error for a non-calendar date")
	}
	params, err := st.buildParams(script, "2026-06-28")
	if err != nil {
		t.Fatal(err)
	}
	if params["RunDate"] != "2026-06-28" {
		t.Errorf("RunDate=%v", params["RunDate"])
	}
}

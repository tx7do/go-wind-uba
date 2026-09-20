package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-wind-uba/pkg/dorisinit"
	"go-wind-uba/sql/doris"
)

// defaultETLSections are the blocks of 06_etl.sql a run executes when --sections is not given:
// §1 sessions_fact and §2 users_dim. Both are UNIQUE KEY tables, so re-running a day replaces
// the rows it wrote — the property a scheduled run depends on, since a restart repeats a night.
// §3-§5 stay opt-in for two reasons: they write AGGREGATE tables (see additiveETLSections), and
// nothing reads what they produce — the analytics models query events_fact and sessions_fact
// directly, as the segment headers in internal/data/doris/analytics_repo.go record. Their inputs
// (§4 user_tags, §5 path_features) are maintained by the admin-side services, not by the ETL.
// Section 6 is the verification SELECT, which the ETL runner would refuse (it accepts INSERT and
// SET only) and which `status` answers better, since it reports the counters Doris keeps.
var defaultETLSections = []string{"1", "2"}

// additiveETLSections write AGGREGATE KEY tables: a second run for one day sums its result
// into the first instead of replacing it, so they cannot be re-run the way §1-§2 can.
var additiveETLSections = map[string]bool{"3": true, "4": true, "5": true}

// selectedScripts filters a script set by --script, accepting a bare name or a path to one,
// and rejects a name that matches nothing so a typo cannot quietly apply no scripts.
func (o *options) selectedScripts(all []dorisinit.Script) ([]dorisinit.Script, error) {
	if len(o.scripts) == 0 {
		return all, nil
	}
	out := make([]dorisinit.Script, 0, len(o.scripts))
	for _, want := range o.scripts {
		found := false
		for _, s := range all {
			if s.Name == want || filepath.Base(s.Name) == filepath.Base(want) {
				out = append(out, s)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown script %q: embedded scripts are %s",
				want, strings.Join(scriptNames(all), ", "))
		}
	}
	return out, nil
}

func scriptNames(scripts []dorisinit.Script) []string {
	out := make([]string, 0, len(scripts))
	for _, s := range scripts {
		out = append(out, s.Name)
	}
	return out
}

// settings resolves the configs, tolerating a config path that does not exist: UBA_DORIS_DSN
// and UBA_KAFKA_BROKERS can carry everything a run needs, and `render` needs neither.
func (o *options) settings() (*settings, error) {
	st, err := loadSettings(o.global.configPath(), o.kafkaBrokers)
	switch {
	case err == nil:
		return st, nil
	case errors.Is(err, os.ErrNotExist):
		note("%v — reading credentials from the environment instead", err)
		return st, nil
	default:
		return nil, &exitError{exitUsage, err}
	}
}

func (o *options) runApply(ctx context.Context) error {
	st, err := o.settings()
	if err != nil {
		return err
	}
	scripts, err := o.selectedScripts(doris.InitScripts())
	if err != nil {
		return &exitError{exitUsage, err}
	}
	params, err := st.buildParams(scripts, "")
	if err != nil {
		return &exitError{exitUsage, err}
	}

	src, close, err := o.connectToDoris(ctx, st)
	if err != nil {
		return o.connectErr(st, err)
	}
	defer close()

	note("apply on %s", st.describe())
	if filtered := scriptNames(scripts); len(scripts) < len(doris.InitScripts()) {
		note("scripts: %s", strings.Join(filtered, " "))
	}

	report, err := dorisinit.Apply(ctx, src, scripts, params, dorisinit.ApplyOptions{
		Policy: dorisinit.EnsurePolicy{
			Database:     st.database(),
			ResumePaused: o.resumePaused,
			DryRun:       o.dryRun,
		},
		AllowRoutineLoadMutations: o.allowRoutineLoadMutations,
	})
	applyReport(report)
	if err != nil {
		return &exitError{exitFailed, err}
	}
	if n := len(report.NeedsHuman()); n > 0 {
		return &exitError{exitNeedsHuman, fmt.Errorf(
			"%d routine load job(s) need a human. Recreating a terminal job would replay the topic, so this tool stopped short; see the lines above", n)}
	}
	return nil
}

func (o *options) runStatus(ctx context.Context) error {
	st, err := o.settings()
	if err != nil {
		return err
	}

	scripts := doris.InitScripts()
	params, err := st.buildParams(scripts, "")
	if err != nil {
		// The declared jobs are read out of the statement heads, so a placeholder broker
		// list still yields the right names — and status sends nothing but its SHOW.
		note("%v; reporting job names only", err)
		params = dorisinit.Params{"KafkaBrokerList": "not-configured"}
	}
	declared, err := declaredJobs(scripts, params)
	if err != nil {
		return &exitError{exitUsage, err}
	}

	src, close, err := st.connect(ctx)
	if err != nil {
		return o.connectErr(st, err)
	}
	defer close()

	conn, err := src.Conn(ctx)
	if err != nil {
		return &exitError{exitFailed, fmt.Errorf("acquire connection: %w", err)}
	}
	defer func() { _ = conn.Close() }()

	jobs, err := dorisinit.ShowJobs(ctx, conn)
	if err != nil {
		return &exitError{exitFailed, err}
	}

	// ResumePaused mirrors what a default `apply` would do, because DECISION answers "what
	// would apply do about this". DryRun is what keeps status itself read-only: it never
	// sends anything but its SHOW, so an operator can run it while deciding.
	policy := dorisinit.EnsurePolicy{Database: st.database(), ResumePaused: true, DryRun: true}
	results := dorisinit.Decide(declared, jobs, policy)

	views := jobViews(results, jobs, st.database())
	if o.jsonOut {
		if err := writeJSON(views); err != nil {
			return &exitError{exitFailed, err}
		}
	} else {
		printJobTable(views)
	}

	var notes, reasons []string
	unhealthy := 0
	for _, v := range views {
		switch {
		case !v.Declared:
			notes = append(notes, fmt.Sprintf("%s is %s, but no embedded script declares it: apply will leave it alone", v.Job, v.State))
		case !v.Live:
			unhealthy++
			reasons = append(reasons, fmt.Sprintf("%s is %s; apply would %s", v.Job, v.State, decisionPhrase(v.Decision)))
		}
	}
	printLines(notes)
	printLines(reasons)

	if unhealthy > 0 && !o.exitZero {
		return &exitError{exitFailed, fmt.Errorf("%d declared routine load job(s) are not consuming", unhealthy)}
	}
	return nil
}

// jobView is one row of the status report: a job Doris listed, or one the scripts declare and
// Doris has never heard of.
type jobView struct {
	Job      string `json:"job"`
	State    string `json:"state"`
	Declared bool   `json:"declared"`
	Live     bool   `json:"live"`
	Decision string `json:"decision"`
	Rows     string `json:"rows,omitempty"`
	Lag      string `json:"lag,omitempty"`
	Detail   string `json:"diagnostics,omitempty"`
}

// decisionPhrase turns an ensure action into the sentence "apply would <phrase>", because the
// bare action names ("created", "needs-human") read as facts about the past, not as plans.
func decisionPhrase(decision string) string {
	if phrase, ok := decisionPhrases[decision]; ok {
		return phrase
	}
	return decision
}

var decisionPhrases = map[string]string{
	string(dorisinit.ActionCreated):       "create it",
	string(dorisinit.ActionResumed):       "resume it",
	string(dorisinit.ActionAlreadyActive): "leave it alone",
	string(dorisinit.ActionNeedsHuman):    "leave it to a human — restarting a terminal job replays the topic",
	string(dorisinit.ActionFailed):        "fail on it",
	"undeclared":                          "leave it alone",
}

// jobViews merges the two views Ensure needs: what the scripts declare, matched against what
// the cluster reports. A declared job missing from the cluster is reported as ABSENT, which
// is the case "SHOW ALL ROUTINE LOAD" cannot express on its own.
func jobViews(results []dorisinit.Result, jobs []dorisinit.Job, database string) []jobView {
	byName := make(map[string]dorisinit.Job, len(jobs))
	for _, j := range jobs {
		byName[dorisinit.JobIdentity(database, j.Database, j.Name)] = j
	}

	views := make([]jobView, 0, len(results)+len(jobs))
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		key := dorisinit.JobIdentity(database, r.Job.Database, r.Job.Name)
		seen[key] = struct{}{}

		v := jobView{
			Job:      displayJob(database, r.Job.Database, r.Job.Name),
			Declared: true,
			Decision: string(r.Action),
			Detail:   r.Detail,
			State:    "ABSENT",
		}
		if j, ok := byName[key]; ok {
			v.State = j.State
			v.Live = dorisinit.ParseState(j.State).Live()
			v.Rows, v.Lag = counters(j)
		}
		views = append(views, v)
	}

	for _, j := range jobs {
		if _, ok := seen[dorisinit.JobIdentity(database, j.Database, j.Name)]; ok {
			continue
		}
		rows, lag := counters(j)
		views = append(views, jobView{
			Job:      displayJob(database, j.Database, j.Name),
			State:    j.State,
			Live:     dorisinit.ParseState(j.State).Live(),
			Decision: "undeclared",
			Rows:     rows,
			Lag:      lag,
			Detail:   j.Diagnostics(),
		})
	}
	return views
}

// counters formats the Statistic and Lag columns. Both are JSON whose key set grows between
// Doris releases, so they are lifted best effort and never fatal.
func counters(j dorisinit.Job) (rows, lag string) {
	if s, ok := dorisinit.DecodeStatistic(j.Column("Statistic")); ok {
		rows = fmt.Sprintf("loaded %d of %d, %d errors", s.LoadedRows, s.TotalRows, s.ErrorRows)
	}
	return rows, strings.TrimSpace(j.Lag())
}

func displayJob(defaultDB, db, name string) string {
	if db == "" {
		db = defaultDB
	}
	if db == "" {
		return name
	}
	return db + "." + name
}

// declaredJobs reads the Routine Load jobs the scripts declare out of their CREATE statements.
func declaredJobs(scripts []dorisinit.Script, params dorisinit.Params) ([]dorisinit.RoutineLoadJob, error) {
	var out []dorisinit.RoutineLoadJob
	for _, s := range scripts {
		stmts, err := dorisinit.Load(s, params)
		if err != nil {
			return nil, err
		}
		plan, err := dorisinit.Classify(stmts)
		if err != nil {
			return nil, err
		}
		out = append(out, plan.Creates...)
	}
	return out, nil
}

func (o *options) runETL(ctx context.Context) error {
	st, err := o.settings()
	if err != nil {
		return err
	}
	sections := o.etlSections()
	if o.loop {
		return o.runETLLoop(ctx, st, sections)
	}

	src, close, err := st.connect(ctx)
	if err != nil {
		return o.connectErr(st, err)
	}
	defer close()

	dates, err := o.etlDates()
	if err != nil {
		return &exitError{exitUsage, err}
	}

	what := "etl run"
	if o.dryRun {
		what = "etl plan"
	}
	note("%s for %s on %s", what, strings.Join(dates, ", "), st.describe())

	for _, date := range dates {
		if err := o.runETLDay(ctx, src, st, sections, date); err != nil {
			return err
		}
	}
	return nil
}

// etlSections resolves which numbered blocks to run, and warns out loud when the selection
// includes a roll-up that cannot be repeated safely.
func (o *options) etlSections() []string {
	if len(o.sections) == 0 {
		return defaultETLSections
	}
	if warning := additiveWarning(o.sections); warning != "" {
		note("%s", warning)
	}
	return []string(o.sections)
}

// additiveWarning names the selected sections whose tables sum a second run into what is
// already there, so an operator passing them knows a repeat is a double count. Empty when the
// selection is the idempotent one, which is what the default is made of.
func additiveWarning(sections []string) string {
	var additive []string
	for _, s := range sections {
		if additiveETLSections[s] {
			additive = append(additive, s)
		}
	}
	if len(additive) == 0 {
		return ""
	}
	return fmt.Sprintf("sections %s write AGGREGATE tables: running them twice for one day sums"+
		" the second result into the first instead of replacing it, so compute each day once"+
		" (--sections %s restricts the run to the idempotent roll-ups)",
		strings.Join(additive, ", "), strings.Join(defaultETLSections, ","))
}

// runETLDay computes one (date, sections) pair and prints each statement as it lands.
func (o *options) runETLDay(ctx context.Context, src dorisinit.Source, st *settings, sections []string, date string) error {
	script := doris.EtlScript()
	params, err := st.buildParams([]dorisinit.Script{script}, date)
	if err != nil {
		return &exitError{exitUsage, err}
	}
	report, err := dorisinit.RunETL(ctx, src, script, params, dorisinit.ETLOptions{
		Sections: sections,
		DryRun:   o.dryRun,
	})
	if report != nil {
		printLines(etlLines(report, o.dryRun))
		if !o.dryRun {
			note("run_date %s: %d statement(s), %d row(s) affected", report.RunDate, len(report.Steps), report.TotalRows())
		}
	}
	if err != nil {
		return &exitError{exitFailed, err}
	}
	return nil
}

// etlRetryDelay is how long the loop waits before recomputing a day it failed on, and
// etlMaxAttempts how many tries that day gets.
const (
	etlRetryDelay  = 10 * time.Minute
	etlMaxAttempts = 6
)

// runETLLoop is the scheduler for a deployment that has no host cron: the compose stack runs
// `uba-ingest etl --loop` as a service, and each tick computes the day that just ended. It
// connects per attempt rather than holding a pool idle for 24 hours, so a Doris restart during
// the day is not something the loop has to notice.
func (o *options) runETLLoop(ctx context.Context, st *settings, sections []string) error {
	at, err := parseAtTime(o.at)
	if err != nil {
		return &exitError{exitUsage, err}
	}
	sched := &scheduler{at: at, delay: etlRetryDelay, maxAttempts: etlMaxAttempts}
	wake := sched.next(time.Now())
	note("etl loop on %s — every day at %s, next run %s", st.describe(), o.at, wake.Format(time.RFC3339))

	for {
		if !sleepUntil(ctx, wake) {
			note("etl loop stopped")
			return nil
		}
		date := dorisinit.DefaultRunDate(time.Now())
		runErr := o.attemptETLDay(ctx, st, sections, date)
		if ctx.Err() != nil {
			// The shutdown signal arrived during the run. Whatever Doris already accepted
			// stands on its own, and leaving on a clean exit is what a stop should look like.
			note("etl loop stopped during run_date %s", date)
			return nil
		}
		if runErr != nil {
			note("etl run_date %s failed: %v", date, runErr)
		} else {
			note("etl run_date %s done", date)
		}

		var what string
		wake, what = sched.afterAttempt(time.Now(), runErr == nil)
		note("%s", what)
	}
}

// attemptETLDay connects, computes one day, and hangs up. The connection is per attempt rather
// than per loop so the 24 hours in between hold no state that can go stale.
func (o *options) attemptETLDay(ctx context.Context, st *settings, sections []string, date string) error {
	src, close, err := st.connect(ctx)
	if err != nil {
		return err
	}
	defer close()
	return o.runETLDay(ctx, src, st, sections, date)
}

// sleepUntil waits for the instant, reporting false if the context ended first. The timer is
// stopped on the way out, so a loop cancelled at shutdown does not leave one armed.
func sleepUntil(ctx context.Context, until time.Time) bool {
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// etlDates walks backwards from --date, newest first: a backfill should leave the most recent
// day computed last only if the days were independent, and they are — each section filters on
// its own date — so the run reports progress in the order an operator reads it.
func (o *options) etlDates() ([]string, error) {
	dates := make([]string, 0, o.backfill)
	date := o.date
	for i := 0; i < o.backfill; i++ {
		dates = append(dates, date)
		prev, err := dorisinit.PreviousRunDate(date)
		if err != nil {
			return nil, err
		}
		date = prev
	}
	return dates, nil
}

func etlLines(report *dorisinit.ETLReport, plan bool) []string {
	ran := "ok"
	if plan {
		ran = "plan"
	}
	out := make([]string, 0, len(report.Steps)+1)
	out = append(out, fmt.Sprintf("run_date %s · sections %s", report.RunDate, strings.Join(sectionsOf(report), ", ")))
	for _, s := range report.Steps {
		line := fmt.Sprintf("  %s  %s:%d  %s  %s", ran, s.Section, s.Line, s.Title, abbrev(s.Text, 8))
		if s.Rows > 0 {
			line += fmt.Sprintf("  (%d rows)", s.Rows)
		}
		out = append(out, line)
	}
	return out
}

func sectionsOf(report *dorisinit.ETLReport) []string {
	var out []string
	for _, s := range report.Steps {
		if len(out) == 0 || out[len(out)-1] != s.Section {
			out = append(out, s.Section)
		}
	}
	return out
}

func (o *options) runRender() error {
	all := doris.AllScripts()
	if len(o.scripts) == 0 {
		all = doris.InitScripts()
	}
	st, err := o.settings()
	if err != nil {
		return err
	}
	scripts, err := o.selectedScripts(all)
	if err != nil {
		return &exitError{exitUsage, err}
	}
	params, err := st.buildParams(scripts, o.date)
	if err != nil {
		return &exitError{exitUsage, err}
	}

	for i, s := range scripts {
		src, err := dorisinit.Render(s.Name, s.Bytes, params)
		if err != nil {
			return &exitError{exitFailed, err}
		}
		if i > 0 {
			fmt.Printf("\n-- ============================================================\n-- >>> %s\n-- ============================================================\n\n", s.Name)
		}
		fmt.Print(string(src))
		if !strings.HasSuffix(string(src), "\n") {
			fmt.Println()
		}
	}
	return nil
}

// applyReport prints what an apply decided, guarding the nil report Apply returns when it
// could not even get a connection.
func applyReport(report *dorisinit.Report) {
	if report == nil {
		return
	}
	printLines(report.Summary())
}

// connectErr separates "nothing to connect to" — a config problem, exit 3 — from a cluster
// that is up but unreachable, which is the ordinary failure exit 1.
func (o *options) connectErr(st *settings, err error) error {
	if st.dsn == "" {
		return &exitError{exitUsage, err}
	}
	return &exitError{exitFailed, err}
}

// ingestRetryDelay is how long `--wait` sleeps between two attempts to reach Doris.
const ingestRetryDelay = 5 * time.Second

// connectToDoris opens the pool, retrying the handshake until --wait runs out. Only the
// handshake is retried: a FE that is still starting is the ordinary sight in `compose up`, and
// every statement apply sends is idempotent, so waiting is always safe where quitting would
// leave the pipeline half-built. A statement that fails later is a real error, not a race.
func (o *options) connectToDoris(ctx context.Context, st *settings) (dorisinit.Source, func(), error) {
	deadline := time.Now().Add(o.wait)
	for {
		src, closeFn, err := st.connect(ctx)
		if err == nil {
			return src, closeFn, nil
		}
		next := time.Now().Add(ingestRetryDelay)
		if o.wait <= 0 || next.After(deadline) {
			if o.wait > 0 {
				note("gave up on %s after %s", st.endpoint(), o.wait)
			}
			return nil, nil, err
		}
		note("waiting for doris at %s (retry at %s): %v", st.endpoint(), next.Format(time.RFC3339), err)
		if !sleepUntil(ctx, next) {
			return nil, nil, context.Cause(ctx)
		}
	}
}

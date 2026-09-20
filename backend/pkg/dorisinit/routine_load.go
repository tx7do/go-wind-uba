package dorisinit

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Routine Load has no "CREATE … IF NOT EXISTS" in Doris, and a job that was STOPped or
// CANCELled is in a terminal state that cannot be resumed — recreating it is the only way
// forward, and that silently discards the stored Kafka offsets, replaying the topic from
// the configured default (OFFSET_BEGINNING in this repo). So the ensure step below never
// destroys a job: it creates what is missing, resumes what is paused, and asks for a human
// when a job has reached a terminal state.

// createRoutineLoad matches the head of a CREATE ROUTINE LOAD statement:
// "CREATE ROUTINE LOAD [db.]job ON table …".
var createRoutineLoad = regexp.MustCompile(`(?i)^CREATE\s+ROUTINE\s+LOAD\s+([A-Za-z0-9_$]+)(?:\s*\.\s*([A-Za-z0-9_$]+))?\s+ON\s+([^\s(]+)`)

// mutateRoutineLoad matches the lifecycle statements this package refuses to run as part of
// an apply: they are what turns an idempotent provisioning step into data loss.
var mutateRoutineLoad = regexp.MustCompile(`(?i)^(STOP|PAUSE|RESUME|DROP|ALTER)\s+ROUTINE\s+LOAD\b`)

// RoutineLoadJob is one CREATE ROUTINE LOAD found in a script.
type RoutineLoadJob struct {
	// Database is the qualifier as written; "" means the connection's current database.
	Database string
	Name     string
	Table    string

	// Create is the statement to run when the job does not exist yet.
	Create Statement
}

// Qualified renders the job name as Doris expects it in RESUME statements.
func (j RoutineLoadJob) Qualified(defaultDB string) string {
	db := j.Database
	if db == "" {
		db = defaultDB
	}
	if db == "" {
		return quoteIdent(j.Name)
	}
	return quoteIdent(db) + "." + quoteIdent(j.Name)
}

// quoteIdent wraps a Doris identifier in backticks, rejecting the one character that could
// break out of them.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "") + "`"
}

// Plan is the classification of a script's statements.
type Plan struct {
	// Plain holds every statement that is not a Routine Load lifecycle statement, in
	// source order.
	Plain []Statement
	// Creates holds the Routine Load jobs the script declares.
	Creates []RoutineLoadJob
	// Mutations holds STOP/PAUSE/RESUME/DROP/ALTER ROUTINE LOAD statements. Apply refuses
	// to run them: they were the cause of the replay-on-restart this package fixes.
	Mutations []Statement
}

// Classify splits statements into plain DDL/DML, Routine Load creations and Routine Load
// lifecycle mutations. Detection reads Statement.Code, so a comment that merely mentions
// "CREATE ROUTINE LOAD" cannot confuse it.
func Classify(stmts []Statement) (*Plan, error) {
	plan := &Plan{}
	for _, s := range stmts {
		switch {
		case createRoutineLoad.MatchString(s.Code):
			m := createRoutineLoad.FindStringSubmatch(s.Code)
			job := RoutineLoadJob{Create: s, Table: m[3]}
			// "CREATE ROUTINE LOAD a.b" qualifies the job with a database; the
			// single-token form leaves the database to the session.
			if m[2] != "" {
				job.Database, job.Name = m[1], m[2]
			} else {
				job.Name = m[1]
			}
			plan.Creates = append(plan.Creates, job)

		case mutateRoutineLoad.MatchString(s.Code):
			plan.Mutations = append(plan.Mutations, s)

		default:
			plan.Plain = append(plan.Plain, s)
		}
	}
	return plan, nil
}

// Runnable returns the statements of a script that Apply should send, in source order.
//
// Routine Load creations are never included: they are deferred to the ensure step so the
// tables they load into already exist. Lifecycle mutations are included only when the caller
// explicitly allows them — a STOP is asynchronous on the FE side, so a create that follows
// it in the same run may still see the old job and report "already used", which is why the
// legacy stop-and-drop pattern stays opt-in.
func (p *Plan) Runnable(allowMutations bool) []Statement {
	if !allowMutations || len(p.Mutations) == 0 {
		return p.Plain
	}
	out := make([]Statement, 0, len(p.Plain)+len(p.Mutations))
	out = append(out, p.Plain...)
	out = append(out, p.Mutations...)
	slices.SortStableFunc(out, func(a, b Statement) int { return a.Line - b.Line })
	return out
}

// Job is one row of "SHOW ALL ROUTINE LOAD".
//
// The column list differs across Doris versions (4.x returns 24 columns, and not every one
// is documented), so every column is kept verbatim in Fields and only the handful this
// package reasons about are lifted into named fields.
type Job struct {
	Name     string
	Database string
	State    string

	// Fields maps the column name Doris reported to its stringified value. Columns whose
	// payload is JSON (Statistic, Progress, Lag, JobDetail) keep their raw text.
	Fields map[string]string
}

// Column returns a column as Doris reported it, or "".
func (j Job) Column(name string) string { return j.Fields[name] }

// Diagnostics assembles the human-readable explanation for a paused or stopped job.
func (j Job) Diagnostics() string {
	var parts []string
	for _, col := range []string{"ReasonOfStateChanged", "FirstErrorMsg", "OtherMsg", "ErrorLogUrls"} {
		if v := j.Column(col); v != "" && v != "N/A" {
			parts = append(parts, col+"="+v)
		}
	}
	return strings.Join(parts, "; ")
}

// Lag reports the consumer lag as JSON text ("{…}") if Doris exposed it.
func (j Job) Lag() string { return j.Column("Lag") }

// Progress reports the per-partition offsets as JSON text if Doris exposed them.
func (j Job) Progress() string { return j.Column("Progress") }

// DecodeJobs turns a SHOW ROUTINE LOAD result set into Jobs. Values are scanned into byte
// slices because Doris mixes strings, numbers and JSON blobs in one row; NULL becomes "".
func DecodeJobs(rs Rows) ([]Job, error) {
	columns, err := rs.Columns()
	if err != nil {
		return nil, fmt.Errorf("read routine load columns: %w", err)
	}

	var jobs []Job
	for rs.Next() {
		raw := make([][]byte, len(columns))
		dest := make([]any, len(columns))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rs.Scan(dest...); err != nil {
			return nil, fmt.Errorf("read routine load row: %w", err)
		}

		j := Job{Fields: make(map[string]string, len(columns))}
		for i, name := range columns {
			v := string(raw[i])
			if v == "NULL" {
				// Doris reports some absent values as the literal string.
				v = ""
			}
			j.Fields[name] = v
			switch strings.ToLower(name) {
			case "name":
				j.Name = v
			case "dbname":
				j.Database = v
			case "state":
				j.State = v
			}
		}
		jobs = append(jobs, j)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("list routine load: %w", err)
	}
	return jobs, nil
}

// showAllRoutineLoad lists every job in the current database, including the terminal ones
// that plain "SHOW ROUTINE LOAD" hides — distinguishing "never created" from "stopped by a
// human" is what makes the ensure step safe.
const showAllRoutineLoad = "SHOW ALL ROUTINE LOAD"

// ShowJobs lists the Routine Load jobs of the database the connection is bound to.
func ShowJobs(ctx context.Context, conn Conn) ([]Job, error) {
	rows, err := conn.QueryContext(ctx, showAllRoutineLoad)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", showAllRoutineLoad, err)
	}
	defer func() { _ = rows.Close() }()
	return DecodeJobs(rows)
}

// State is a Routine Load job state. Doris writes the state as one of NEED_SCHEDULE,
// RUNNING, PAUSED, STOPPED, CANCELLED (and FINISHED for a job that consumed the topic end).
type State string

const (
	StateAbsent     State = ""
	StateScheduling State = "NEED_SCHEDULE"
	StateRunning    State = "RUNNING"
	StatePaused     State = "PAUSED"
	StateStopped    State = "STOPPED"
	StateCancelled  State = "CANCELLED"
	StateFinished   State = "FINISHED"
)

// ParseState normalises the State column. An unrecognised value is returned upper-cased so
// it still shows up in the report instead of being mistaken for a known state.
func ParseState(s string) State {
	return State(strings.ToUpper(strings.TrimSpace(s)))
}

// Terminal reports whether only a human (or a new job name) can move the job forward.
func (s State) Terminal() bool {
	switch s {
	case StateStopped, StateCancelled, StateFinished:
		return true
	}
	return false
}

// Live reports whether the job is already doing its work.
func (s State) Live() bool {
	return s == StateRunning || s == StateScheduling
}

// Action is what Ensure decided to do about one declared job.
type Action string

const (
	// ActionCreated means the job was missing, so the script's CREATE ran (or would run
	// under DryRun).
	ActionCreated Action = "created"
	// ActionResumed means the job was PAUSED, so RESUME ran (or would run under DryRun).
	ActionResumed Action = "resumed"
	// ActionAlreadyActive means the job is running or awaiting scheduling; nothing to do.
	ActionAlreadyActive Action = "already-running"
	// ActionNeedsHuman means the job exists in a state ensure must not touch: terminal
	// (STOPPED/CANCELLED/FINISHED), or PAUSED while ResumePaused is off.
	ActionNeedsHuman Action = "needs-human"
	// ActionFailed marks a decided action whose statement returned an error.
	ActionFailed Action = "failed"
)

// Result records one declared job's outcome.
type Result struct {
	Job    RoutineLoadJob
	Action Action
	// Prior is the state observed before the action, StateAbsent when the job is new.
	Prior State
	// Detail carries the pause reason for a job that needs attention, or the failure.
	Detail string
	// Statement is what ensure sent (or would send); empty when no action was needed.
	Statement Statement
}

// EnsurePolicy tunes the ensure step.
type EnsurePolicy struct {
	// Database qualifies the job name in RESUME statements when the script did not
	// qualify it.
	Database string
	// ResumePaused issues "RESUME ROUTINE LOAD" for a job found PAUSED. Turning it off
	// makes ensure read-only, which is what the status subcommand wants.
	ResumePaused bool
	// DryRun decides but does not execute, so an operator can see the plan first.
	DryRun bool
}

// Decide maps each declared job against the jobs Doris reported. It is pure, which is what
// lets the whole policy — create, resume, leave alone — be tested without a database.
func Decide(creates []RoutineLoadJob, existing []Job, policy EnsurePolicy) []Result {
	byName := make(map[string]Job, len(existing))
	for _, j := range existing {
		byName[JobIdentity(policy.Database, j.Database, j.Name)] = j
	}

	results := make([]Result, 0, len(creates))
	for _, declared := range creates {
		r := Result{Job: declared}

		job, ok := byName[JobIdentity(policy.Database, declared.Database, declared.Name)]
		switch {
		case !ok:
			r.Prior = StateAbsent
			r.Action = ActionCreated
			r.Statement = declared.Create

		case ParseState(job.State).Live():
			r.Prior = ParseState(job.State)
			r.Action = ActionAlreadyActive
			r.Detail = job.Diagnostics()

		case ParseState(job.State) == StatePaused && policy.ResumePaused:
			r.Prior = StatePaused
			r.Action = ActionResumed
			r.Detail = job.Diagnostics()
			r.Statement = Statement{Text: "RESUME ROUTINE LOAD FOR " + declared.Qualified(policy.Database)}

		default:
			r.Prior = ParseState(job.State)
			r.Action = ActionNeedsHuman
			r.Detail = job.Diagnostics()
			if r.Prior == StatePaused {
				r.Detail = strings.TrimSpace("resume is disabled; " + r.Detail)
			}
		}
		results = append(results, r)
	}
	return results
}

// EnsureJobs creates the missing jobs and resumes the paused ones. It never stops, drops or
// recreates a job: a terminal job is reported for a human instead, because recreating it
// would lose the Kafka offsets and replay the topic.
//
// Every result is returned even when one job fails, and the aggregated failure is returned
// as the error so a caller can print the whole table before exiting non-zero.
func EnsureJobs(ctx context.Context, conn Conn, creates []RoutineLoadJob, policy EnsurePolicy) ([]Result, error) {
	existing, err := ShowJobs(ctx, conn)
	if err != nil {
		return nil, err
	}
	results := Decide(creates, existing, policy)
	if policy.DryRun {
		return results, nil
	}

	var failures []string
	for i, r := range results {
		if r.Statement.Text == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, r.Statement.Text); err != nil {
			results[i].Action = ActionFailed
			results[i].Detail = err.Error()
			failures = append(failures, fmt.Sprintf("%s %s: %v", r.Action, r.Job.Name, err))
		}
	}
	if len(failures) > 0 {
		return results, fmt.Errorf("%d of %d routine load jobs failed: %s",
			len(failures), len(creates), strings.Join(failures, "; "))
	}
	return results, nil
}

// JobIdentity normalises a job identity: an unqualified declaration refers to the
// connection's default database, which is also what Doris reports in DbName. Job and database
// names are case-insensitive in Doris, so both are folded before comparison. Callers outside
// this package — the status report, chiefly — need the same key to tell "declared but absent"
// apart from "running but undeclared".
func JobIdentity(defaultDB, declaredDB, name string) string {
	db := strings.ToLower(declaredDB)
	if db == "" {
		db = strings.ToLower(defaultDB)
	}
	return db + "." + strings.ToLower(name)
}

// Statistic decodes the Statistic column of SHOW ALL ROUTINE LOAD. The column is a JSON
// blob whose key set grows between Doris releases, so it is decoded key by key on a best
// effort basis and every raw entry is kept in Extra.
type Statistic struct {
	ReceivedBytes        int64
	TotalRows            int64
	LoadedRows           int64
	ErrorRows            int64
	ErrorRowsAfterResume int64
	UnselectedRows       int64
	CommittedTasks       int64
	AbortedTasks         int64

	// Extra is the untouched key set, so a column Doris added later is still printable.
	Extra map[string]json.RawMessage
}

// DecodeStatistic parses the Statistic column. An empty or unparsable value yields ok=false
// rather than an error, because the column is missing on some Doris builds.
func DecodeStatistic(raw string) (Statistic, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") {
		return Statistic{}, false
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return Statistic{}, false
	}

	s := Statistic{Extra: all}
	s.ReceivedBytes, _ = statInt(all, "receivedBytes")
	s.TotalRows, _ = statInt(all, "totalRows")
	s.LoadedRows, _ = statInt(all, "loadedRows")
	s.ErrorRows, _ = statInt(all, "errorRows")
	s.ErrorRowsAfterResume, _ = statInt(all, "errorRowsAfterResumed")
	s.UnselectedRows, _ = statInt(all, "unselectedRows")
	s.CommittedTasks, _ = statInt(all, "committedTaskNum")
	s.AbortedTasks, _ = statInt(all, "abortedTaskNum")
	return s, true
}

// statInt reads a numeric key, tolerating a value that changed type upstream.
func statInt(m map[string]json.RawMessage, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(v, &f); err != nil {
		return 0, false
	}
	return int64(f), true
}

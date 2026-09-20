package dorisinit

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

// Rows is the result-set surface this package reads. *sql.Rows satisfies it.
type Rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// Conn is the single-connection surface this package works through. Every statement of one
// unit (a script, an ETL section) runs on the same connection on purpose: "USE gw_uba" and
// "SET enable_unique_key_partial_update" are session-scoped in Doris, so sending them on a
// pooled connection and the statement they govern on another would silently lose the
// setting.
type Conn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	Close() error
}

// Source hands out connections.
type Source interface {
	Conn(ctx context.Context) (Conn, error)
}

// SourceFunc adapts a function to Source.
type SourceFunc func(ctx context.Context) (Conn, error)

// Conn implements Source.
func (f SourceFunc) Conn(ctx context.Context) (Conn, error) { return f(ctx) }

// WrapDB adapts a *sql.DB — including the one a go-crud Doris client keeps behind DB().DB —
// to Source.
func WrapDB(db *sql.DB) Source {
	return SourceFunc(func(ctx context.Context) (Conn, error) {
		conn, err := db.Conn(ctx)
		if err != nil {
			return nil, err
		}
		return sqlConn{conn}, nil
	})
}

// sqlConn narrows *sql.Conn to Conn: the interface returns Rows, while database/sql returns
// the concrete *sql.Rows.
type sqlConn struct{ conn *sql.Conn }

func (c sqlConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.conn.ExecContext(ctx, query, args...)
}

func (c sqlConn) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return c.conn.QueryContext(ctx, query, args...)
}

func (c sqlConn) Close() error { return c.conn.Close() }

// Script is a provisioning script identified by name, so errors point at a file rather than
// a blob.
type Script struct {
	Name  string
	Bytes []byte
}

// Params supplies the values a script template needs, such as the Kafka broker list and the
// ETL run date.
type Params map[string]any

// unresolvedPlaceholder catches the ${VAR} style the ETL script used before it was
// templatized. Shipping one to Doris would fail with a parser error that hides the real
// cause, so rendering refuses to leave it behind.
var unresolvedPlaceholder = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// Render strips a UTF-8 BOM and expands the Go template in src. A missing parameter is an
// error rather than an empty string: an empty broker list or run date would produce a job
// that quietly ingests nothing.
//
// Parameters are only substituted inline — the shipped templates never add or remove a
// line, so a Statement's Line still addresses the file on disk.
func Render(name string, src []byte, params Params) ([]byte, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(StripBOM(src)))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, params); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	rendered := out.Bytes()
	if left := unresolvedPlaceholder.Find(rendered); left != nil {
		return nil, fmt.Errorf("render %s: %s was not substituted; pass the matching parameter", name, left)
	}
	return rendered, nil
}

// Load renders a script and splits it into statements.
func Load(s Script, params Params) ([]Statement, error) {
	_, stmts, err := LoadRendered(s, params)
	return stmts, err
}

// LoadRendered returns the rendered script alongside its statements, so line-numbered
// post-processing (section grouping) works on exactly the text that was split.
func LoadRendered(s Script, params Params) ([]byte, []Statement, error) {
	src, err := Render(s.Name, s.Bytes, params)
	if err != nil {
		return nil, nil, err
	}
	stmts, err := SplitStatements(src)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", s.Name, err)
	}
	return src, stmts, nil
}

// ApplyOptions tunes Apply.
type ApplyOptions struct {
	Policy EnsurePolicy

	// AllowRoutineLoadMutations runs STOP/DROP/RESUME statements found in a script.
	// They are refused by default: re-creating a Routine Load job discards its Kafka
	// offsets, so a script that drops and re-adds its jobs replays the whole topic on
	// every deploy.
	AllowRoutineLoadMutations bool
}

// AppliedStatement records one statement Apply sent — or would send under DryRun.
type AppliedStatement struct {
	Script string
	Line   int
	Text   string
}

// Report is the outcome of an apply run. It is returned even when the run fails, so the
// operator sees how far it got.
type Report struct {
	Applied []AppliedStatement
	Results []Result

	// Plan marks a DryRun report, whose statements were decided but not executed.
	Plan bool
}

// NeedsHuman lists the jobs ensure could not fix on its own.
func (r *Report) NeedsHuman() []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Action == ActionNeedsHuman || res.Action == ActionFailed {
			out = append(out, res)
		}
	}
	return out
}

// Summary renders a one-line-per-decision description of the run.
func (r *Report) Summary() []string {
	ran, planned := "ok", "plan"
	if r.Plan {
		ran = planned
	}

	out := make([]string, 0, len(r.Applied)+len(r.Results))
	for _, a := range r.Applied {
		out = append(out, fmt.Sprintf("%s:%d %s  %s", a.Script, a.Line, ran, firstWords(a.Text, 4)))
	}
	for _, res := range r.Results {
		action := string(res.Action)
		if r.Plan {
			action = "would " + action
		}
		line := fmt.Sprintf("routine load %s: %s", res.Job.Name, action)
		if res.Prior != StateAbsent {
			line += fmt.Sprintf(" (was %s)", res.Prior)
		}
		if res.Detail != "" {
			line += " — " + res.Detail
		}
		out = append(out, line)
	}
	return out
}

// Apply renders each script, runs its plain statements in order and then ensures the
// Routine Load jobs it declares. The whole run happens on one connection taken from src.
//
// Ensure never destroys a job, so a repeated apply converges on the declared set without
// replaying the topic.
func Apply(ctx context.Context, src Source, scripts []Script, params Params, opts ApplyOptions) (*Report, error) {
	conn, err := src.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	report := &Report{Plan: opts.Policy.DryRun}
	var creates []RoutineLoadJob

	for _, s := range scripts {
		stmts, err := Load(s, params)
		if err != nil {
			return report, err
		}
		plan, err := Classify(stmts)
		if err != nil {
			return report, err
		}
		if len(plan.Mutations) > 0 && !opts.AllowRoutineLoadMutations {
			lines := make([]string, 0, len(plan.Mutations))
			for _, m := range plan.Mutations {
				lines = append(lines, fmt.Sprintf("line %d: %s", m.Line, firstWords(m.Text, 3)))
			}
			return report, fmt.Errorf("%s mutates routine load jobs (%s). Stopping or dropping a job discards its stored kafka offsets, "+
				"so the next create replays the topic; this tool only ever creates or resumes. Edit the script, or pass "+
				"--allow-routine-load-mutations if you really mean it.", s.Name, strings.Join(lines, "; "))
		}

		for _, stmt := range plan.Runnable(opts.AllowRoutineLoadMutations) {
			if !opts.Policy.DryRun {
				if _, err := conn.ExecContext(ctx, stmt.Text); err != nil {
					return report, fmt.Errorf("%s:%d: %w\n  statement: %s", s.Name, stmt.Line, err, firstWords(stmt.Text, 12))
				}
			}
			report.Applied = append(report.Applied, AppliedStatement{Script: s.Name, Line: stmt.Line, Text: stmt.Text})
		}
		creates = append(creates, plan.Creates...)
	}

	if len(creates) == 0 {
		return report, nil
	}
	results, err := EnsureJobs(ctx, conn, creates, opts.Policy)
	report.Results = append(report.Results, results...)
	if err != nil {
		return report, err
	}
	return report, nil
}

// ETLStep records one executed ETL statement.
type ETLStep struct {
	Section string
	Title   string
	Line    int
	Text    string
	Rows    int64
}

// ETLReport is the outcome of an ETL run.
type ETLReport struct {
	RunDate string
	Steps   []ETLStep
}

// TotalRows sums the rows reported by the INSERT statements.
func (r *ETLReport) TotalRows() int64 {
	var n int64
	for _, s := range r.Steps {
		n += s.Rows
	}
	return n
}

// etlAllowedWords limits what the ETL runner will send: a section either writes rows or
// sets a session variable. The verification SELECT at the end of 06_etl.sql sits in its own
// section and is therefore not selected by the default allowlist.
var etlAllowedWords = []string{"INSERT", "SET"}

// ETLOptions tunes RunETL.
type ETLOptions struct {
	// Sections are the top-level section numbers to run, e.g. {"1", "2"}.
	Sections []string
	// DryRun selects and reports the statements without sending them.
	DryRun bool
}

// RunETL renders an ETL script, keeps only the requested numbered sections and runs them on
// one connection in file order, so a section's "SET … = true" still governs the INSERT that
// follows it.
func RunETL(ctx context.Context, src Source, script Script, params Params, opts ETLOptions) (*ETLReport, error) {
	rendered, stmts, err := LoadRendered(script, params)
	if err != nil {
		return nil, err
	}

	picked, err := SelectSections(SplitSections(rendered, stmts), opts.Sections...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", script.Name, err)
	}

	conn, err := src.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	report := &ETLReport{RunDate: paramString(params, "RunDate")}
	for _, s := range picked {
		plan, err := Classify(s.Statements)
		if err != nil {
			return report, err
		}
		if len(plan.Creates) > 0 || len(plan.Mutations) > 0 {
			return report, fmt.Errorf("%s section %s contains routine load statements, which do not belong in an ETL script",
				script.Name, s.Number)
		}
		for _, stmt := range plan.Plain {
			if !slices.Contains(etlAllowedWords, stmt.FirstWord()) {
				return report, fmt.Errorf("%s:%d section %s: %s is not runnable by the ETL runner, which only accepts %s",
					script.Name, stmt.Line, s.Number, stmt.FirstWord(), strings.Join(etlAllowedWords, " / "))
			}
			step := ETLStep{Section: s.Number, Title: s.Title, Line: stmt.Line, Text: stmt.Text}
			if opts.DryRun {
				report.Steps = append(report.Steps, step)
				continue
			}
			res, err := conn.ExecContext(ctx, stmt.Text)
			if err != nil {
				return report, fmt.Errorf("%s:%d section %s: %w\n  statement: %s",
					script.Name, stmt.Line, s.Number, err, firstWords(stmt.Text, 12))
			}
			if n, err := res.RowsAffected(); err == nil && n > 0 {
				step.Rows = n
			}
			report.Steps = append(report.Steps, step)
		}
	}
	return report, nil
}

func paramString(params Params, key string) string {
	if v, ok := params[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

// firstWords truncates a statement for log lines, keeping it on one line.
func firstWords(text string, n int) string {
	fields := strings.Fields(strings.ReplaceAll(text, "\n", " "))
	if len(fields) <= n {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:n], " ") + " …"
}

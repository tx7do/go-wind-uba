// Command uba-ingest provisions and operates the Doris side of the UBA event pipeline.
//
// Ingestion itself is done by Doris: a Routine Load job is a consumer that the FE schedules
// and supervises, so this tool does not move events. It applies the schema, creates the jobs
// that read the kafka topics, schedules the daily roll-ups, and reports whether the pipeline
// is actually healthy — the three things nothing in the stack did before, which is why
// reported events used to pile up in kafka unseen.
//
// Credentials are read from the service configs (or the environment), never from the command
// line, and the DSN is redacted wherever it is printed.
//
// Usage:
//
//	uba-ingest -c /app/configs apply --dry-run
//	uba-ingest -c /app/configs apply
//	uba-ingest -c /app/configs status --json
//	uba-ingest -c /app/configs etl --date 2026-06-28 --backfill 7
//	uba-ingest -c /app/configs etl --loop --at 02:00
//	uba-ingest -c /app/configs render --script 02_kafka_tables.sql
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

func init() {
	// The config loader logs every file it reads, at Debug, through kratos' global logger —
	// which writes to stdout. Redirect it to stderr and keep only warnings and errors, so
	// `render > jobs.sql` and `status --json | jq` get exactly what they ask for.
	kratoslog.SetLogger(kratoslog.NewFilter(
		kratoslog.NewStdLogger(os.Stderr),
		kratoslog.FilterLevel(kratoslog.LevelWarn),
	))
}

// exitError carries a decided exit status instead of letting main collapse every failure into
// the same code: a job that needs a human is a different outcome from a bad flag.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }

// Unwrap keeps the underlying error reachable with errors.Is, so wrapping a failure in a
// decided exit code does not hide what failed.
func (e *exitError) Unwrap() error { return e.err }

const (
	exitOK         = 0
	exitFailed     = 1
	exitNeedsHuman = 2
	exitUsage      = 3
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			usage(os.Stdout)
			return exitOK
		}
		fmt.Fprintf(os.Stderr, "uba-ingest: %v\n", err)
		usage(os.Stderr)
		return exitUsage
	}

	if err := opts.run(ctx); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			fmt.Fprintf(os.Stderr, "uba-ingest: %v\n", ee.err)
			return ee.code
		}
		fmt.Fprintf(os.Stderr, "uba-ingest: %v\n", err)
		return exitFailed
	}
	return exitOK
}

func usage(w *os.File) {
	fmt.Fprint(w, `uba-ingest — provision and operate the Doris ingestion pipeline

Usage:
  uba-ingest [flags] <command> [command flags]

Flags:
  -c, --configs <dir|file>   service configs to read data.doris / data.kafka from;
                             repeatable, the last one wins       (default "configs",
                             env UBA_CONFIGS as a path list)

Commands:
  apply      create the database schema and ensure the Routine Load jobs
             --dry-run                     report the plan, change nothing (still
                                           reads the cluster: a plan needs the
                                           current job states)
             --resume-paused=false         do not auto-resume a PAUSED job
             --script <name>               apply only this script (repeatable)
             --allow-routine-load-mutations
                                           run STOP/DROP statements found in a script;
                                           they discard kafka offsets
             --wait 5m                     keep trying to reach Doris for this long
  status     list the Routine Load jobs with state, lag and error counters
             --json                        machine-readable output
             --exit-zero                   always exit 0
  etl        run the daily roll-ups from 06_etl.sql
             --date YYYY-MM-DD             the day to compute (default: yesterday)
             --backfill N                  also recompute the N-1 earlier days, newest first
             --sections 1,2                which numbered sections to run (default: 1,2 — the
                                           idempotent ones; 3,4,5 sum into what is already there)
             --dry-run                     print the statements, run none
             --loop                        stay up and run at --at every day instead of once now
             --at HH:MM                    with --loop: the local time to run at  (default "02:00")
  render     print a script after template rendering, connecting to nothing
             --script <name>               which script (default: every init script)
             --date / --kafka-brokers      template parameters

Exit codes:
  0   the work is done and, for status, every declared job is consuming
  1   a statement or connection failed, or status found a declared job that is not
      running — 1 is what a cron or monitoring check should alert on
  2   a job is in a state only a human may move on from (STOPPED, CANCELLED, FINISHED)
  3   bad usage, or a configuration that cannot be resolved

Environment:
  UBA_DORIS_DSN      overrides data.doris.dsn. There is deliberately no --dsn: a flag
                     value is visible in ps and in the shell history.
  UBA_KAFKA_BROKERS  overrides data.kafka.endpoints, comma-separated
`)
}

func parseFlags(args []string) (*options, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command given")
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		return nil, flag.ErrHelp
	}

	fs := flag.NewFlagSet("uba-ingest", flag.ContinueOnError)
	// The flag package would echo its own error and the usage text; main prints both once.
	fs.SetOutput(io.Discard)

	common := globalFlags{}
	fs.Func("c", "config file or directory (repeatable)", common.addConfig)
	fs.Func("configs", "config file or directory (repeatable)", common.addConfig)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	rest := fs.Args()
	if len(rest) == 0 {
		return nil, fmt.Errorf("no command given")
	}

	cmd := rest[0]
	switch cmd {
	case "apply", "status", "etl", "render":
	default:
		if strings.HasPrefix(cmd, "-") {
			return nil, fmt.Errorf("unknown flag %q: flags come before the command", cmd)
		}
		return nil, fmt.Errorf("unknown command %q", cmd)
	}

	opts := &options{global: common, command: cmd}
	cmdFS := flag.NewFlagSet(cmd, flag.ContinueOnError)
	cmdFS.SetOutput(io.Discard)
	opts.bind(cmdFS)
	if err := cmdFS.Parse(rest[1:]); err != nil {
		return nil, err
	}
	if cmdFS.NArg() > 0 {
		return nil, fmt.Errorf("%s: unexpected argument %q", cmd, cmdFS.Arg(0))
	}
	return opts, nil
}

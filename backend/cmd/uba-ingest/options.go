package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-wind-uba/pkg/dorisinit"
)

// defaultConfigPath is the directory the service images mount their configs in, so
// `uba-ingest -c /app/configs …` and a bare `uba-ingest …` inside the container agree.
const defaultConfigPath = "configs"

// Environment overrides. Secrets arrive through these or the config files — never a command
// line argument, which would leak them into `ps` and the shell history.
const (
	envConfigs      = "UBA_CONFIGS"
	envDorisDSN     = "UBA_DORIS_DSN"
	envKafkaBrokers = "UBA_KAFKA_BROKERS"
)

// globalFlags are accepted before the subcommand, because they say which cluster the
// subcommand talks to rather than what to do to it.
type globalFlags struct {
	configs []string
}

func (g *globalFlags) addConfig(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("empty config path")
	}
	g.configs = append(g.configs, v)
	return nil
}

// configPath resolves the config location: the last -c/--configs argument, else the last
// entry of UBA_CONFIGS, else "configs".
func (g *globalFlags) configPath() string {
	if n := len(g.configs); n > 0 {
		return g.configs[n-1]
	}
	if parts := filepath.SplitList(os.Getenv(envConfigs)); len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return defaultConfigPath
}

// options is one parsed invocation: the subcommand plus its own flags.
type options struct {
	global  globalFlags
	command string

	// apply, etl
	dryRun bool

	// apply
	resumePaused              bool
	allowRoutineLoadMutations bool
	scripts                   stringList

	// apply: how long to wait for a cluster that is still starting
	wait time.Duration

	// status
	jsonOut  bool
	exitZero bool

	// etl, render
	date string

	// etl
	sections stringList
	backfill int
	loop     bool
	at       string

	// render
	kafkaBrokers string
}

func (o *options) bind(fs *flag.FlagSet) {
	switch o.command {
	case "apply":
		fs.BoolVar(&o.dryRun, "dry-run", false, "print the plan without sending anything")
		fs.BoolVar(&o.resumePaused, "resume-paused", true, "resume a job found PAUSED")
		fs.Var(&o.scripts, "script", "apply only this script (repeatable)")
		fs.BoolVar(&o.allowRoutineLoadMutations, "allow-routine-load-mutations", false,
			"run STOP/DROP ROUTINE LOAD statements found in a script (replays the topic)")
		fs.DurationVar(&o.wait, "wait", 0, "keep trying to reach Doris for this long, e.g. 5m")

	case "status":
		fs.BoolVar(&o.jsonOut, "json", false, "emit JSON instead of a table")
		fs.BoolVar(&o.exitZero, "exit-zero", false, "always exit 0, even when a job is unhealthy")

	case "etl":
		fs.StringVar(&o.date, "date", "", "the day to compute (default: yesterday)")
		fs.IntVar(&o.backfill, "backfill", 1, "also recompute the N-1 earlier days, newest first")
		fs.Var(&o.sections, "sections", "numbered sections to run (default: 1,2, the idempotent roll-ups)")
		fs.BoolVar(&o.dryRun, "dry-run", false, "print the statements without running them")
		fs.BoolVar(&o.loop, "loop", false, "stay up and run once a day at --at instead of once now")
		fs.StringVar(&o.at, "at", defaultETLAtTime, "with --loop: the local HH:MM to run at")

	case "render":
		fs.Var(&o.scripts, "script", "script to render (repeatable; default: every init script)")
		fs.StringVar(&o.date, "date", "", "value for the RunDate parameter")
		fs.StringVar(&o.kafkaBrokers, "kafka-brokers", "", "value for the KafkaBrokerList parameter")
	}
}

// validate rejects flag combinations that would silently do the wrong thing.
func (o *options) validate() error {
	switch o.command {
	case "apply":
		if o.dryRun && o.allowRoutineLoadMutations {
			return fmt.Errorf("--dry-run cannot be combined with --allow-routine-load-mutations: a dry run sends nothing, so there is nothing for the flag to allow")
		}
	case "etl":
		if o.loop {
			// The loop takes its run date from the clock on every tick; pinning one would
			// make it recompute the same day each night, which is a silent double-count for
			// the aggregate sections.
			if o.date != "" {
				return fmt.Errorf("--loop computes the day that just ended on every tick, so it cannot take --date %q: drop one of them", o.date)
			}
		} else {
			if o.date == "" {
				o.date = dorisinit.DefaultRunDate(time.Now())
			}
			if err := dorisinit.CheckRunDate(o.date); err != nil {
				return err
			}
		}
		if o.backfill < 1 {
			return fmt.Errorf("--backfill must be at least 1 (1 runs only the given date)")
		}
	}
	return nil
}

func (o *options) run(ctx context.Context) error {
	if err := o.validate(); err != nil {
		return &exitError{exitUsage, err}
	}

	switch o.command {
	case "apply":
		return o.runApply(ctx)
	case "status":
		return o.runStatus(ctx)
	case "etl":
		return o.runETL(ctx)
	case "render":
		return o.runRender()
	}
	return &exitError{exitUsage, fmt.Errorf("unknown command %q", o.command)}
}

// stringList collects a repeatable flag whose values may also be comma-separated, so both
// "--sections 1,2" and "--sections 1 --sections 2" work.
type stringList []string

func (s *stringList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

func (s stringList) String() string { return strings.Join(s, ",") }

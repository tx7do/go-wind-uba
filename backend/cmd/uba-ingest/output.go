package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// note writes the operator-facing chatter to stderr, so stdout stays pipeable:
// `uba-ingest render > /tmp/jobs.sql` and `uba-ingest status --json | jq` both work.
func note(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "uba-ingest: "+format+"\n", args...)
}

func printLines(lines []string) {
	for _, l := range lines {
		fmt.Println(l)
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printJobTable renders the status report. Every column comes straight from Doris except
// DECISION, which is what `apply` would do about that job.
func printJobTable(views []jobView) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "JOB\tSTATE\tLIVE\tDECISION\tROWS\tLAG\tDIAGNOSTICS")
	for _, v := range views {
		live := "-"
		if v.Live {
			live = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			v.Job, orDash(v.State), live, v.Decision, orDash(abbrev(v.Rows, 8)), orDash(abbrev(v.Lag, 6)), abbrev(v.Detail, 8))
	}
	if err := w.Flush(); err != nil {
		note("write status table: %v", err)
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// abbrev collapses whitespace and keeps the first n words, so one line per statement or
// diagnostic stays readable in a terminal.
func abbrev(text string, n int) string {
	fields := strings.Fields(strings.ReplaceAll(text, "\n", " "))
	if len(fields) <= n {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:n], " ") + " …"
}

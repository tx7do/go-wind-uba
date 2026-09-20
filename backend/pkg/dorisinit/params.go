package dorisinit

import (
	"fmt"
	"strings"
	"time"
)

// RunDateFormat is the layout a RunDate parameter must use. The ETL script interpolates the
// value inside SQL quotes, so anything other than a bare calendar date would either break
// the parser or smuggle SQL into the statement.
const RunDateFormat = "2006-01-02"

// CheckRunDate validates a run date before it reaches a template.
func CheckRunDate(s string) error {
	t, err := time.Parse(RunDateFormat, strings.TrimSpace(s))
	if err != nil || t.Format(RunDateFormat) != strings.TrimSpace(s) {
		return fmt.Errorf("run date %q is not a %s date (for example 2026-06-28)", s, RunDateFormat)
	}
	return nil
}

// PreviousRunDate returns the day before the given date, which is how a daily ETL run walks
// backwards through a backfill window.
func PreviousRunDate(s string) (string, error) {
	t, err := time.Parse(RunDateFormat, s)
	if err != nil {
		return "", CheckRunDate(s)
	}
	return t.AddDate(0, 0, -1).Format(RunDateFormat), nil
}

// DefaultRunDate is the date a scheduled run should process: yesterday in the server's own
// timezone, because an event-day aggregate must not be computed against a day that is still
// being written to.
func DefaultRunDate(now time.Time) string {
	return now.AddDate(0, 0, -1).Format(RunDateFormat)
}

// BrokerList renders kafka endpoint addresses in the comma-separated form Doris'
// "kafka_broker_list" property expects.
func BrokerList(endpoints []string) (string, error) {
	var out []string
	for _, e := range endpoints {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		// A broker list is interpolated into a quoted SQL literal; a quote or a
		// newline in it would end the literal early.
		if strings.ContainsAny(e, "\"'`\n\r;\\") {
			return "", fmt.Errorf("kafka endpoint %q contains a character that cannot appear in kafka_broker_list", e)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return "", fmt.Errorf("no kafka broker endpoints configured")
	}
	return strings.Join(out, ","), nil
}

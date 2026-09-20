package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// defaultETLAtTime is when the nightly roll-up runs. Two in the morning is far enough past
// midnight that the day being computed has stopped receiving events, and early enough that the
// tables are refreshed before anyone reads them during the business day.
const defaultETLAtTime = "02:00"

// atTime is a parsed wall-clock time of day. It stays in the location it is used with rather
// than being converted, because the roll-up is meant to run at a time a human recognises in
// the container's TZ, not at a fixed instant across a DST change.
type atTime struct {
	hour, minute int
}

// parseAtTime reads a 24-hour HH:MM. It is strict about the shape rather than forgiving typos
// like "2am": a schedule that is quietly misread runs the roll-up at the wrong hour, and the
// wrong hour is exactly the kind of misconfiguration nobody notices until a dashboard is stale.
func parseAtTime(at string) (atTime, error) {
	parts := strings.Split(strings.TrimSpace(at), ":")
	if len(parts) != 2 {
		return atTime{}, fmt.Errorf("--at %q is not a 24-hour HH:MM time, for example 02:00", at)
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return atTime{}, fmt.Errorf("--at %q has an hour outside 00-23", at)
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return atTime{}, fmt.Errorf("--at %q has a minute outside 00-59", at)
	}
	return atTime{hour: hour, minute: minute}, nil
}

// next returns the first occurrence of the time of day strictly after `from`. Strictly, so a
// run that finishes inside its own minute is not immediately scheduled again.
func (a atTime) next(from time.Time) time.Time {
	today := time.Date(from.Year(), from.Month(), from.Day(), a.hour, a.minute, 0, 0, from.Location())
	if !today.After(from) {
		today = today.AddDate(0, 0, 1)
	}
	return today
}

// scheduler turns the wall-clock schedule into wake-up instants, and keeps track of how many
// attempts the day being served has spent.
type scheduler struct {
	at          atTime
	delay       time.Duration
	maxAttempts int

	spent int
}

// next is the coming slot, which is where the loop starts.
func (s *scheduler) next(now time.Time) time.Time { return s.at.next(now) }

// afterAttempt records an outcome and returns when to wake plus the line to log. A failed day
// is retried every delay until it has spent its attempts; after that the loop waits for the
// following slot instead of chasing a stale day, because tonight's numbers are the ones
// somebody reads in the morning and a database unreachable for an hour needs a human.
func (s *scheduler) afterAttempt(now time.Time, ranOK bool) (time.Time, string) {
	slot := s.at.next(now)
	if ranOK {
		s.spent = 0
		return slot, "next etl run at " + slot.Format(time.RFC3339)
	}
	s.spent++
	retry := now.Add(s.delay)
	switch {
	case s.spent >= s.maxAttempts:
		spent := s.spent
		s.spent = 0
		return slot, fmt.Sprintf("the day stayed unreachable over %d attempts; recomputing it takes"+
			" `etl --date <that day>`. Next run at %s", spent, slot.Format(time.RFC3339))
	case !retry.Before(slot):
		s.spent = 0
		return slot, "next etl run at " + slot.Format(time.RFC3339)
	default:
		return retry, fmt.Sprintf("retrying in %s at %s (attempt %d of %d)",
			s.delay, retry.Format(time.RFC3339), s.spent+1, s.maxAttempts)
	}
}

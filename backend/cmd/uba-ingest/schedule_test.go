package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseAtTime(t *testing.T) {
	for _, tc := range []struct {
		at   string
		hour int
		min  int
	}{
		{at: "02:00", hour: 2, min: 0},
		{at: "00:00", hour: 0, min: 0},
		{at: "23:59", hour: 23, min: 59},
		{at: " 7:05 ", hour: 7, min: 5},
	} {
		got, err := parseAtTime(tc.at)
		if err != nil {
			t.Fatalf("parseAtTime(%q) = %v", tc.at, err)
		}
		if got.hour != tc.hour || got.minute != tc.min {
			t.Errorf("parseAtTime(%q) = %02d:%02d want %02d:%02d", tc.at, got.hour, got.minute, tc.hour, tc.min)
		}
	}
}

func TestParseAtTimeRejectsUnusableTimes(t *testing.T) {
	// "15:00" is 3pm, not 15 minutes past midnight, and a misread schedule is the kind of
	// mistake that surfaces a day later as a stale dashboard.
	for _, at := range []string{"", "0200", "2am", "24:00", "-1:00", "02:60", "02:00:30", "ab:cd"} {
		if _, err := parseAtTime(at); err == nil {
			t.Errorf("parseAtTime(%q) accepted it", at)
		}
	}
}

func TestNextRunRollsOverToTomorrow(t *testing.T) {
	noon := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	at, err := parseAtTime("02:00")
	if err != nil {
		t.Fatal(err)
	}

	got := at.next(noon)
	if want := time.Date(2026, 6, 29, 2, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("next(12:00) = %s want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}

	// Before the hour, the same day is still coming.
	got = at.next(noon.Add(-11 * time.Hour))
	if want := time.Date(2026, 6, 28, 2, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("next(01:00) = %s want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestNextRunIsStrictlyAfterTheInstant(t *testing.T) {
	// A run that finishes inside its own minute must not schedule itself again immediately.
	at, err := parseAtTime("02:00")
	if err != nil {
		t.Fatal(err)
	}
	exactly := time.Date(2026, 6, 28, 2, 0, 0, 0, time.UTC)
	if got := at.next(exactly); !got.Equal(exactly.AddDate(0, 0, 1)) {
		t.Errorf("next(02:00) = %s want the next day", got.Format(time.RFC3339))
	}
	if got := at.next(exactly.Add(-time.Second)); !got.Equal(exactly) {
		t.Errorf("next(01:59:59) = %s want 02:00 the same day", got.Format(time.RFC3339))
	}
}

func TestNextRunKeepsTheLocalWallClock(t *testing.T) {
	// The schedule is a time a human recognises in the container's TZ, so a tick computed at
	// 01:00 there fires at 02:00 there — not at 02:00 UTC.
	loc := time.FixedZone("CST", 8*60*60)
	from := time.Date(2026, 6, 28, 1, 0, 0, 0, loc)
	at, err := parseAtTime("02:00")
	if err != nil {
		t.Fatal(err)
	}

	got := at.next(from)
	if got.Format("15:04") != "02:00" {
		t.Errorf("next = %s want the clock to read 02:00", got.Format(time.RFC3339))
	}
	if got.Location() != loc {
		t.Error("next changed location")
	}
}

func TestSchedulerRetriesAFailedDay(t *testing.T) {
	// 02:00 has just been served and failed; the next slot is tomorrow's.
	at, err := parseAtTime("02:00")
	if err != nil {
		t.Fatal(err)
	}
	s := &scheduler{at: at, delay: time.Hour, maxAttempts: 3}

	failed := time.Date(2026, 6, 28, 2, 0, 0, 0, time.UTC)
	wake, log := s.afterAttempt(failed, false)
	if want := failed.Add(time.Hour); !wake.Equal(want) {
		t.Errorf("wake = %s want %s", wake.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if !strings.Contains(log, "attempt 2 of 3") {
		t.Errorf("the log should say which attempt this was: %q", log)
	}

	// Attempt 3 of 3 is the last one the day gets; the next failure moves on to tomorrow.
	wake, _ = s.afterAttempt(failed.Add(2*time.Hour), false)
	if want := failed.Add(3 * time.Hour); !wake.Equal(want) {
		t.Errorf("wake = %s want %s", wake.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	gaveUpAt := failed.Add(3 * time.Hour)
	wake, log = s.afterAttempt(gaveUpAt, false)
	if want := time.Date(2026, 6, 29, 2, 0, 0, 0, time.UTC); !wake.Equal(want) {
		t.Errorf("wake = %s want the next slot %s", wake.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if !strings.Contains(log, "3 attempts") || !strings.Contains(log, "--date") {
		t.Errorf("giving up should say how hard it tried and what to do instead: %q", log)
	}

	// A day that comes good resets the budget, so a night of failures does not shorten the
	// next night's.
	if _, log := s.afterAttempt(time.Date(2026, 6, 29, 2, 0, 0, 0, time.UTC), true); !strings.Contains(log, "next etl run") {
		t.Errorf("success log = %q", log)
	}
	_, log = s.afterAttempt(time.Date(2026, 6, 30, 2, 0, 0, 0, time.UTC), false)
	if !strings.Contains(log, "attempt 2 of 3") {
		t.Errorf("attempts were not counted afresh: %q", log)
	}
}

func TestSchedulerDoesNotRetryPastTheNextSlot(t *testing.T) {
	// A run that overran its day — the delay would land after tomorrow's slot — waits for the
	// slot instead of stacking a retry on top of it.
	at, err := parseAtTime("02:00")
	if err != nil {
		t.Fatal(err)
	}
	s := &scheduler{at: at, delay: 24 * time.Hour, maxAttempts: 6}

	now := time.Date(2026, 6, 28, 2, 0, 0, 0, time.UTC)
	wake, log := s.afterAttempt(now, false)
	if want := now.AddDate(0, 0, 1); !wake.Equal(want) {
		t.Errorf("wake = %s want %s", wake.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if !strings.Contains(log, "next etl run") {
		t.Errorf("log = %q", log)
	}
}

func TestSleepUntilReturnsWhenTheInstantArrives(t *testing.T) {
	if !sleepUntil(context.Background(), time.Now().Add(time.Millisecond)) {
		t.Error("sleepUntil reported a timeout as a cancellation")
	}
	if !sleepUntil(context.Background(), time.Now().Add(-time.Hour)) {
		t.Error("sleepUntil did not fire for an instant in the past")
	}
}

func TestSleepUntilEndsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// An hour out: if the context were ignored this would take an hour rather than return.
	start := time.Now()
	if sleepUntil(ctx, time.Now().Add(time.Hour)) {
		t.Error("sleepUntil reported a cancelled wait as a completed one")
	}
	if time.Since(start) > time.Second {
		t.Errorf("sleepUntil ignored the cancellation for %s", time.Since(start))
	}
}

func TestLoopFlagsAreOnlyForTheLoop(t *testing.T) {
	o := &options{command: "etl", loop: true, date: "2026-06-28", at: defaultETLAtTime, backfill: 1}
	if err := o.validate(); err == nil {
		t.Fatal("a pinned --date was accepted with --loop: it would recompute one day every night")
	}

	// Without --date the loop derives its own, and validate must not fill one in for it.
	o = &options{command: "etl", loop: true, at: defaultETLAtTime, backfill: 1}
	if err := o.validate(); err != nil {
		t.Fatalf("validate() = %v", err)
	}
	if o.date != "" {
		t.Errorf("date = %q, want it left for the ticks to compute", o.date)
	}
}

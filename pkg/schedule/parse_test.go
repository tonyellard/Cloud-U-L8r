// SPDX-License-Identifier: Apache-2.0
package schedule

import (
	"testing"
	"time"
)

func TestParseRate_Valid(t *testing.T) {
	tests := []struct {
		expr  string
		value int
		unit  string
	}{
		{"rate(1 minute)", 1, "minute"},
		{"rate(5 minutes)", 5, "minutes"},
		{"rate(1 hour)", 1, "hour"},
		{"rate(12 hours)", 12, "hours"},
		{"rate(1 day)", 1, "day"},
		{"rate(30 days)", 30, "days"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ps, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ps.Type != ScheduleTypeRate {
				t.Errorf("expected ScheduleTypeRate, got %d", ps.Type)
			}
			if ps.RateValue != tt.value {
				t.Errorf("expected value %d, got %d", tt.value, ps.RateValue)
			}
			if ps.RateUnit != tt.unit {
				t.Errorf("expected unit %s, got %s", tt.unit, ps.RateUnit)
			}
		})
	}
}

func TestParseRate_Invalid(t *testing.T) {
	tests := []struct {
		expr string
		desc string
	}{
		{"rate(0 minutes)", "zero value"},
		{"rate(-1 minutes)", "negative value"},
		{"rate(2 minute)", "plural value with singular unit"},
		{"rate(1 minutes)", "singular value with plural unit"},
		{"rate(1 second)", "invalid unit"},
		{"rate(1)", "missing unit"},
		{"rate()", "empty"},
		{"rate(abc minutes)", "non-numeric value"},
		{"rate(1 hour extra)", "extra field"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if err == nil {
				t.Fatalf("expected error for %s", tt.expr)
			}
		})
	}
}

func TestParseCron_Valid(t *testing.T) {
	tests := []struct {
		expr string
		desc string
	}{
		{"cron(0 12 * * ? 2025)", "every day at noon in 2025"},
		{"cron(15 10 ? * MON-FRI *)", "weekdays at 10:15"},
		{"cron(0/15 * * * ? *)", "every 15 minutes"},
		{"cron(0 8 1 * ? *)", "first of every month at 8am"},
		{"cron(30 9 ? * 2 *)", "every Monday at 9:30"},
		{"cron(0 0 1 JAN ? *)", "Jan 1st midnight"},
		{"cron(0,30 * * * ? *)", "on the hour and half-hour"},
		{"cron(0 9-17 ? * MON-FRI *)", "business hours weekdays"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ps, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ps.Type != ScheduleTypeCron {
				t.Errorf("expected ScheduleTypeCron, got %d", ps.Type)
			}
			if ps.Cron == nil {
				t.Fatal("expected cron fields to be set")
			}
		})
	}
}

func TestParseCron_Invalid(t *testing.T) {
	tests := []struct {
		expr string
		desc string
	}{
		{"cron(0 12 * * * *)", "both dom and dow are *"},
		{"cron(0 12 ? * ? *)", "both dom and dow are ?"},
		{"cron(0 12 * * ?)", "only 5 fields"},
		{"cron(60 12 * * ? *)", "minute out of range"},
		{"cron(0 25 * * ? *)", "hour out of range"},
		{"cron(0 12 32 * ? *)", "dom out of range"},
		{"cron(0 12 * 13 ? *)", "month out of range"},
		{"cron(0 12 ? * 8 *)", "dow out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if err == nil {
				t.Fatalf("expected error for %s", tt.expr)
			}
		})
	}
}

func TestParseAt_Valid(t *testing.T) {
	ps, err := Parse("at(2025-12-25T10:30:00)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps.Type != ScheduleTypeAt {
		t.Errorf("expected ScheduleTypeAt, got %d", ps.Type)
	}
	expected := time.Date(2025, 12, 25, 10, 30, 0, 0, time.UTC)
	if !ps.AtTime.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ps.AtTime)
	}
}

func TestParseAt_Invalid(t *testing.T) {
	tests := []struct {
		expr string
		desc string
	}{
		{"at()", "empty"},
		{"at(2025-13-01T00:00:00)", "invalid month"},
		{"at(not-a-date)", "not a date"},
		{"at(2025-12-25)", "missing time"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if err == nil {
				t.Fatalf("expected error for %s", tt.expr)
			}
		})
	}
}

func TestNextFireTime_At(t *testing.T) {
	ps, err := Parse("at(2025-12-25T10:30:00)")
	if err != nil {
		t.Fatal(err)
	}

	// Before the target time: should return the target time
	before := time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(before)
	expected := time.Date(2025, 12, 25, 10, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}

	// After the target time: should return zero (won't fire again)
	after := time.Date(2025, 12, 26, 0, 0, 0, 0, time.UTC)
	next = ps.NextFireTime(after)
	if !next.IsZero() {
		t.Errorf("expected zero time, got %v", next)
	}

	// Exactly at the target time: should return zero (not after)
	next = ps.NextFireTime(expected)
	if !next.IsZero() {
		t.Errorf("expected zero time when at exact time, got %v", next)
	}
}

func TestParseInvalidPrefix(t *testing.T) {
	_, err := Parse("schedule(1 minute)")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestRateDuration(t *testing.T) {
	tests := []struct {
		expr     string
		expected time.Duration
	}{
		{"rate(1 minute)", time.Minute},
		{"rate(5 minutes)", 5 * time.Minute},
		{"rate(1 hour)", time.Hour},
		{"rate(2 hours)", 2 * time.Hour},
		{"rate(1 day)", 24 * time.Hour},
		{"rate(7 days)", 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ps, err := Parse(tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if ps.RateDuration() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, ps.RateDuration())
			}
		})
	}
}

func TestNextFireTime_Rate(t *testing.T) {
	ps, err := Parse("rate(5 minutes)")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := base.Add(5 * time.Minute)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronBasic(t *testing.T) {
	// Every day at noon
	ps, err := Parse("cron(0 12 * * ? *)")
	if err != nil {
		t.Fatal(err)
	}

	// After 11:30 → should fire at 12:00 same day
	base := time.Date(2025, 6, 15, 11, 30, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}

	// After 12:00 → should fire at 12:00 next day
	base = time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	next = ps.NextFireTime(base)
	expected = time.Date(2025, 6, 16, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronDayOfWeek(t *testing.T) {
	// Every Monday at 9:00 (DOW 2 = MON in AWS cron)
	ps, err := Parse("cron(0 9 ? * 2 *)")
	if err != nil {
		t.Fatal(err)
	}

	// Sunday June 15, 2025 → next Monday is June 16
	base := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2025, 6, 16, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronEvery15Min(t *testing.T) {
	ps, err := Parse("cron(0/15 * * * ? *)")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2025, 6, 15, 10, 7, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2025, 6, 15, 10, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronSpecificYear(t *testing.T) {
	// Only fires in 2030
	ps, err := Parse("cron(0 0 1 * ? 2030)")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronMonthNames(t *testing.T) {
	ps, err := Parse("cron(0 0 1 JAN ? *)")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextFireTime_CronDayNames(t *testing.T) {
	ps, err := Parse("cron(0 9 ? * MON-FRI *)")
	if err != nil {
		t.Fatal(err)
	}

	// Saturday June 14, 2025 → next weekday is Monday June 16
	base := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)
	next := ps.NextFireTime(base)
	expected := time.Date(2025, 6, 16, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

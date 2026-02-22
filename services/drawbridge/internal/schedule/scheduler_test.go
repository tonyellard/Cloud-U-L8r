// SPDX-License-Identifier: Apache-2.0
package schedule

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/tonyellard/drawbridge/internal/delivery"
	"github.com/tonyellard/drawbridge/internal/model"
	"github.com/tonyellard/drawbridge/internal/store"
)

func TestScheduler_TickFiresRateRule(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	st := store.NewStore("us-east-1", "000000000000")
	del := delivery.NewDeliverer(logger, "http://localhost:9320", "http://localhost:9330")

	// Create a scheduled rule
	_, err := st.PutRule(model.PutRuleRequest{
		Name:               "test-rate-rule",
		ScheduleExpression: "rate(1 minute)",
		State:              "ENABLED",
	})
	if err != nil {
		t.Fatal(err)
	}

	sched := NewScheduler(logger, st, del, "us-east-1", "000000000000")

	// tick should fire the rate rule (first time = immediate fire)
	sched.tick()

	sched.mu.Lock()
	_, fired := sched.lastFired["arn:aws:events:us-east-1:000000000000:rule/test-rate-rule"]
	sched.mu.Unlock()

	if !fired {
		t.Error("expected rate rule to fire on first tick")
	}
}

func TestScheduler_DisabledRuleNotFired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	st := store.NewStore("us-east-1", "000000000000")
	del := delivery.NewDeliverer(logger, "http://localhost:9320", "http://localhost:9330")

	_, err := st.PutRule(model.PutRuleRequest{
		Name:               "disabled-rule",
		ScheduleExpression: "rate(1 minute)",
		State:              "DISABLED",
	})
	if err != nil {
		t.Fatal(err)
	}

	sched := NewScheduler(logger, st, del, "us-east-1", "000000000000")
	sched.tick()

	sched.mu.Lock()
	_, fired := sched.lastFired["arn:aws:events:us-east-1:000000000000:rule/disabled-rule"]
	sched.mu.Unlock()

	if fired {
		t.Error("disabled rule should not fire")
	}
}

func TestScheduler_RateRuleNotFiredTooSoon(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	st := store.NewStore("us-east-1", "000000000000")
	del := delivery.NewDeliverer(logger, "http://localhost:9320", "http://localhost:9330")

	_, err := st.PutRule(model.PutRuleRequest{
		Name:               "5min-rule",
		ScheduleExpression: "rate(5 minutes)",
		State:              "ENABLED",
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := "arn:aws:events:us-east-1:000000000000:rule/5min-rule"
	sched := NewScheduler(logger, st, del, "us-east-1", "000000000000")

	// Simulate last fired 1 minute ago
	sched.mu.Lock()
	sched.lastFired[arn] = time.Now().UTC().Add(-1 * time.Minute)
	fireCount := 0
	sched.mu.Unlock()

	prevLast := sched.lastFired[arn]
	sched.tick()

	sched.mu.Lock()
	newLast := sched.lastFired[arn]
	sched.mu.Unlock()

	if !newLast.Equal(prevLast) {
		fireCount++
	}

	if fireCount > 0 {
		t.Error("5-minute rate rule should not fire after only 1 minute")
	}
}

func TestScheduler_InvalidExpressionSkipped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	st := store.NewStore("us-east-1", "000000000000")
	del := delivery.NewDeliverer(logger, "http://localhost:9320", "http://localhost:9330")

	// PutRule accepts the store-level validation (non-empty), but the scheduler
	// should skip rules with unparseable expressions. We simulate by using
	// a valid store entry but invalid expression won't get past Parse.
	_, err := st.PutRule(model.PutRuleRequest{
		Name:               "bad-schedule",
		ScheduleExpression: "rate(0 seconds)",
		State:              "ENABLED",
	})
	if err != nil {
		t.Fatal(err)
	}

	sched := NewScheduler(logger, st, del, "us-east-1", "000000000000")

	// Should not panic
	sched.tick()

	sched.mu.Lock()
	_, fired := sched.lastFired["arn:aws:events:us-east-1:000000000000:rule/bad-schedule"]
	sched.mu.Unlock()

	if fired {
		t.Error("rule with invalid expression should not fire")
	}
}

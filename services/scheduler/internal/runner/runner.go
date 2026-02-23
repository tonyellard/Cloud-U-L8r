// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tonyellard/cloud-u-l8r/pkg/schedule"
	"github.com/tonyellard/scheduler/internal/delivery"
	"github.com/tonyellard/scheduler/internal/model"
	"github.com/tonyellard/scheduler/internal/store"
)

// Runner executes schedules on a 1-minute tick.
type Runner struct {
	logger    *slog.Logger
	store     *store.Store
	deliverer *delivery.Deliverer

	mu        sync.Mutex
	lastFired map[string]time.Time // keyed by schedule ARN
}

// NewRunner creates a new schedule runner.
func NewRunner(logger *slog.Logger, st *store.Store, del *delivery.Deliverer) *Runner {
	return &Runner{
		logger:    logger,
		store:     st,
		deliverer: del,
		lastFired: make(map[string]time.Time),
	}
}

// Start runs the scheduler loop, ticking every minute. It blocks until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Fire once immediately on start
	r.tick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

func (r *Runner) tick() {
	schedules := r.store.EnabledSchedules()
	now := time.Now().UTC()

	for _, sched := range schedules {
		r.processSchedule(sched, now)
	}
}

func (r *Runner) processSchedule(sched *model.Schedule, now time.Time) {
	// Respect StartDate
	if sched.StartDate != nil && now.Before(*sched.StartDate) {
		return
	}
	// Respect EndDate
	if sched.EndDate != nil && now.After(*sched.EndDate) {
		return
	}

	parsed, err := schedule.Parse(sched.ScheduleExpression)
	if err != nil {
		r.logger.Warn("invalid schedule expression",
			"schedule", sched.Name,
			"expression", sched.ScheduleExpression,
			"error", err)
		return
	}

	r.mu.Lock()
	last, hasLast := r.lastFired[sched.Arn]
	r.mu.Unlock()

	var shouldFire bool
	if !hasLast {
		if parsed.Type == schedule.ScheduleTypeRate {
			shouldFire = true
		} else if parsed.Type == schedule.ScheduleTypeAt {
			shouldFire = parsed.AtTime.Before(now) || parsed.AtTime.Equal(now)
		} else {
			// Cron: check if current minute matches
			nextFire := parsed.NextFireTime(now.Add(-2 * time.Minute))
			shouldFire = !nextFire.IsZero() && !nextFire.After(now)
		}
	} else {
		nextFire := parsed.NextFireTime(last)
		shouldFire = !nextFire.IsZero() && !nextFire.After(now)
	}

	if shouldFire {
		r.mu.Lock()
		r.lastFired[sched.Arn] = now
		r.mu.Unlock()

		r.fireSchedule(sched, now)

		// Handle at() schedules: if ActionAfterCompletion is DELETE, remove schedule
		if parsed.Type == schedule.ScheduleTypeAt {
			if sched.ActionAfterCompletion == "DELETE" {
				if err := r.store.DeleteSchedule(sched.Name, sched.GroupName); err != nil {
					r.logger.Error("failed to auto-delete at() schedule",
						"schedule", sched.Name,
						"error", err)
				} else {
					r.logger.Info("at() schedule auto-deleted after completion",
						"schedule", sched.Name)
				}
			}
		}
	}
}

func (r *Runner) fireSchedule(sched *model.Schedule, now time.Time) {
	if sched.Target == nil {
		r.logger.Warn("schedule has no target", "schedule", sched.Name)
		return
	}

	// Use Target.Input as payload, or default scheduled event envelope
	payload := sched.Target.Input
	if payload == "" {
		payload = `{"source":"aws.scheduler","detail-type":"Scheduled Event","detail":{}}`
	}

	r.logger.Info("schedule fired",
		"schedule", sched.Name,
		"group", sched.GroupName,
		"expression", sched.ScheduleExpression,
		"target", sched.Target.Arn)

	if err := r.deliverer.Deliver(payload, *sched.Target); err != nil {
		r.logger.Error("schedule target delivery failed",
			"schedule", sched.Name,
			"target", sched.Target.Arn,
			"error", err)
	}
}

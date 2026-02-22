// SPDX-License-Identifier: Apache-2.0
package schedule

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tonyellard/cloud-u-l8r/pkg/schedule"
	"github.com/tonyellard/drawbridge/internal/delivery"
	"github.com/tonyellard/drawbridge/internal/model"
	"github.com/tonyellard/drawbridge/internal/store"
)

// Scheduler runs a background ticker that fires schedule-based rules.
type Scheduler struct {
	logger    *slog.Logger
	store     *store.Store
	deliverer *delivery.Deliverer
	region    string
	accountID string

	mu        sync.Mutex
	lastFired map[string]time.Time // keyed by rule ARN
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(logger *slog.Logger, st *store.Store, del *delivery.Deliverer, region, accountID string) *Scheduler {
	return &Scheduler{
		logger:    logger,
		store:     st,
		deliverer: del,
		region:    region,
		accountID: accountID,
		lastFired: make(map[string]time.Time),
	}
}

// Start runs the scheduler loop, ticking every minute. It blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Fire once immediately on start
	s.tick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	rules := s.store.EnabledScheduleRules()
	now := time.Now().UTC()

	for _, rt := range rules {
		parsed, err := schedule.Parse(rt.Rule.ScheduleExpression)
		if err != nil {
			s.logger.Warn("invalid schedule expression on rule",
				"rule", rt.Rule.Name,
				"expression", rt.Rule.ScheduleExpression,
				"error", err)
			continue
		}

		s.mu.Lock()
		last, hasLast := s.lastFired[rt.Rule.Arn]
		s.mu.Unlock()

		var shouldFire bool
		if !hasLast {
			// Never fired: rate rules fire immediately, cron rules check schedule
			if parsed.Type == schedule.ScheduleTypeRate {
				shouldFire = true
			} else {
				// For cron, check if the current minute matches
				nextFire := parsed.NextFireTime(now.Add(-2 * time.Minute))
				shouldFire = !nextFire.IsZero() && !nextFire.After(now)
			}
		} else {
			nextFire := parsed.NextFireTime(last)
			shouldFire = !nextFire.IsZero() && !nextFire.After(now)
		}

		if shouldFire {
			s.mu.Lock()
			s.lastFired[rt.Rule.Arn] = now
			s.mu.Unlock()

			s.fireRule(rt, now)
		}
	}
}

func (s *Scheduler) fireRule(rt store.RuleWithTargets, now time.Time) {
	eventID := uuid.New().String()

	fullEvent := map[string]interface{}{
		"version":     "0",
		"id":          eventID,
		"source":      "aws.events",
		"account":     s.accountID,
		"time":        now.Format(time.RFC3339),
		"region":      s.region,
		"resources":   []string{rt.Rule.Arn},
		"detail-type": "Scheduled Event",
		"detail":      map[string]interface{}{},
	}

	eventJSON, _ := json.Marshal(fullEvent)

	s.logger.Info("scheduled rule fired",
		"rule", rt.Rule.Name,
		"event_id", eventID,
		"targets", len(rt.Targets))

	for _, target := range rt.Targets {
		go func(t model.Target) {
			if err := s.deliverer.Deliver(string(eventJSON), t); err != nil {
				s.logger.Error("scheduled target delivery failed",
					"rule", rt.Rule.Name,
					"target", t.Id,
					"error", err)
			}
		}(target)
	}
}

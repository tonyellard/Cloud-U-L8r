// SPDX-License-Identifier: Apache-2.0
package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tonyellard/scheduler/internal/model"
)

// Store is a thread-safe in-memory store for EventBridge Scheduler resources.
type Store struct {
	mu        sync.RWMutex
	groups    map[string]*model.ScheduleGroup                // keyed by group name
	schedules map[string]map[string]*model.Schedule          // keyed by groupName → scheduleName
	region    string
	accountID string
}

// NewStore creates a new store with a "default" schedule group.
func NewStore(region, accountID string) *Store {
	s := &Store{
		groups:    make(map[string]*model.ScheduleGroup),
		schedules: make(map[string]map[string]*model.Schedule),
		region:    region,
		accountID: accountID,
	}
	now := time.Now().UTC()
	s.groups["default"] = &model.ScheduleGroup{
		Name:                 "default",
		Arn:                  s.groupArn("default"),
		State:                "ACTIVE",
		CreationDate:         now,
		LastModificationDate: now,
	}
	s.schedules["default"] = make(map[string]*model.Schedule)
	return s
}

func (s *Store) groupArn(name string) string {
	return fmt.Sprintf("arn:aws:scheduler:%s:%s:schedule-group/%s", s.region, s.accountID, name)
}

func (s *Store) scheduleArn(group, name string) string {
	return fmt.Sprintf("arn:aws:scheduler:%s:%s:schedule/%s/%s", s.region, s.accountID, group, name)
}

func (s *Store) resolveGroup(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

// --- Schedule Group operations ---

func (s *Store) CreateScheduleGroup(req model.CreateScheduleGroupRequest) (model.CreateScheduleGroupResponse, error) {
	if req.Name == "" {
		return model.CreateScheduleGroupResponse{}, fmt.Errorf("ValidationException: Name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[req.Name]; exists {
		return model.CreateScheduleGroupResponse{}, fmt.Errorf("ConflictException: Schedule group %s already exists", req.Name)
	}

	arn := s.groupArn(req.Name)
	now := time.Now().UTC()
	s.groups[req.Name] = &model.ScheduleGroup{
		Name:                 req.Name,
		Arn:                  arn,
		State:                "ACTIVE",
		CreationDate:         now,
		LastModificationDate: now,
	}
	s.schedules[req.Name] = make(map[string]*model.Schedule)

	return model.CreateScheduleGroupResponse{ScheduleGroupArn: arn}, nil
}

func (s *Store) GetScheduleGroup(name string) (model.ScheduleGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, exists := s.groups[name]
	if !exists {
		return model.ScheduleGroup{}, fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", name)
	}
	return *g, nil
}

func (s *Store) DeleteScheduleGroup(name string) error {
	if name == "" || name == "default" {
		return fmt.Errorf("ValidationException: Cannot delete the default schedule group")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[name]; !exists {
		return fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", name)
	}

	delete(s.groups, name)
	delete(s.schedules, name)
	return nil
}

func (s *Store) ListScheduleGroups(req model.ListScheduleGroupsRequest) model.ListScheduleGroupsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make([]model.ScheduleGroup, 0, len(s.groups))
	for _, g := range s.groups {
		if req.NamePrefix != "" && !strings.HasPrefix(g.Name, req.NamePrefix) {
			continue
		}
		groups = append(groups, *g)
	}

	return model.ListScheduleGroupsResponse{ScheduleGroups: groups}
}

// --- Schedule operations ---

func (s *Store) CreateSchedule(req model.CreateScheduleRequest) (model.CreateScheduleResponse, error) {
	if req.Name == "" {
		return model.CreateScheduleResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.ScheduleExpression == "" {
		return model.CreateScheduleResponse{}, fmt.Errorf("ValidationException: ScheduleExpression is required")
	}

	group := s.resolveGroup(req.GroupName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[group]; !exists {
		return model.CreateScheduleResponse{}, fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", group)
	}

	if _, exists := s.schedules[group][req.Name]; exists {
		return model.CreateScheduleResponse{}, fmt.Errorf("ConflictException: Schedule %s already exists in group %s", req.Name, group)
	}

	arn := s.scheduleArn(group, req.Name)
	state := req.State
	if state == "" {
		state = "ENABLED"
	}
	now := time.Now().UTC()

	sched := &model.Schedule{
		Name:                       req.Name,
		Arn:                        arn,
		GroupName:                  group,
		ScheduleExpression:         req.ScheduleExpression,
		ScheduleExpressionTimezone: req.ScheduleExpressionTimezone,
		State:                      state,
		Description:                req.Description,
		FlexibleTimeWindow:         req.FlexibleTimeWindow,
		Target:                     req.Target,
		StartDate:                  req.StartDate,
		EndDate:                    req.EndDate,
		ActionAfterCompletion:      req.ActionAfterCompletion,
		CreationDate:               now,
		LastModificationDate:       now,
	}

	s.schedules[group][req.Name] = sched

	return model.CreateScheduleResponse{ScheduleArn: arn}, nil
}

func (s *Store) GetSchedule(name, group string) (model.Schedule, error) {
	group = s.resolveGroup(group)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.groups[group]; !exists {
		return model.Schedule{}, fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", group)
	}

	sched, exists := s.schedules[group][name]
	if !exists {
		return model.Schedule{}, fmt.Errorf("ResourceNotFoundException: Schedule %s does not exist in group %s", name, group)
	}

	return *sched, nil
}

func (s *Store) UpdateSchedule(req model.UpdateScheduleRequest) (model.UpdateScheduleResponse, error) {
	if req.Name == "" {
		return model.UpdateScheduleResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.ScheduleExpression == "" {
		return model.UpdateScheduleResponse{}, fmt.Errorf("ValidationException: ScheduleExpression is required")
	}

	group := s.resolveGroup(req.GroupName)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[group]; !exists {
		return model.UpdateScheduleResponse{}, fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", group)
	}

	sched, exists := s.schedules[group][req.Name]
	if !exists {
		return model.UpdateScheduleResponse{}, fmt.Errorf("ResourceNotFoundException: Schedule %s does not exist in group %s", req.Name, group)
	}

	sched.ScheduleExpression = req.ScheduleExpression
	sched.ScheduleExpressionTimezone = req.ScheduleExpressionTimezone
	if req.State != "" {
		sched.State = req.State
	}
	sched.Description = req.Description
	sched.FlexibleTimeWindow = req.FlexibleTimeWindow
	sched.Target = req.Target
	sched.StartDate = req.StartDate
	sched.EndDate = req.EndDate
	sched.ActionAfterCompletion = req.ActionAfterCompletion
	sched.LastModificationDate = time.Now().UTC()

	return model.UpdateScheduleResponse{ScheduleArn: sched.Arn}, nil
}

func (s *Store) DeleteSchedule(name, group string) error {
	group = s.resolveGroup(group)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[group]; !exists {
		return fmt.Errorf("ResourceNotFoundException: Schedule group %s does not exist", group)
	}

	if _, exists := s.schedules[group][name]; !exists {
		return fmt.Errorf("ResourceNotFoundException: Schedule %s does not exist in group %s", name, group)
	}

	delete(s.schedules[group], name)
	return nil
}

func (s *Store) ListSchedules(req model.ListSchedulesRequest) model.ListSchedulesResponse {
	group := s.resolveGroup(req.GroupName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	groupScheds := s.schedules[group]
	if groupScheds == nil {
		return model.ListSchedulesResponse{Schedules: []model.ScheduleSummary{}}
	}

	summaries := make([]model.ScheduleSummary, 0, len(groupScheds))
	for _, sched := range groupScheds {
		if req.NamePrefix != "" && !strings.HasPrefix(sched.Name, req.NamePrefix) {
			continue
		}
		if req.State != "" && sched.State != req.State {
			continue
		}
		targetArn := ""
		if sched.Target != nil {
			targetArn = sched.Target.Arn
		}
		summaries = append(summaries, model.ScheduleSummary{
			Name:               sched.Name,
			Arn:                sched.Arn,
			GroupName:          sched.GroupName,
			ScheduleExpression: sched.ScheduleExpression,
			State:              sched.State,
			CreationDate:       sched.CreationDate,
			TargetArn:          targetArn,
		})
	}

	return model.ListSchedulesResponse{Schedules: summaries}
}

// --- Query helpers ---

// EnabledSchedules returns all schedules with state ENABLED.
func (s *Store) EnabledSchedules() []*model.Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.Schedule
	for _, groupScheds := range s.schedules {
		for _, sched := range groupScheds {
			if sched.State == "ENABLED" {
				cp := *sched
				result = append(result, &cp)
			}
		}
	}
	return result
}

// --- Admin helpers ---

func (s *Store) Summary() model.AdminSummaryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, groupScheds := range s.schedules {
		total += len(groupScheds)
	}

	return model.AdminSummaryResponse{
		Service:        "scheduler",
		ScheduleGroups: len(s.groups),
		Schedules:      total,
	}
}

func (s *Store) GroupDetails() []model.AdminGroupDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()

	details := make([]model.AdminGroupDetail, 0, len(s.groups))
	for _, g := range s.groups {
		gd := model.AdminGroupDetail{
			Name:      g.Name,
			Arn:       g.Arn,
			State:     g.State,
			Schedules: make([]model.AdminScheduleDetail, 0),
		}
		if groupScheds, ok := s.schedules[g.Name]; ok {
			for _, sched := range groupScheds {
				targetArn := ""
				if sched.Target != nil {
					targetArn = sched.Target.Arn
				}
				gd.Schedules = append(gd.Schedules, model.AdminScheduleDetail{
					Name:               sched.Name,
					Arn:                sched.Arn,
					State:              sched.State,
					ScheduleExpression: sched.ScheduleExpression,
					TargetArn:          targetArn,
					Description:        sched.Description,
				})
			}
		}
		details = append(details, gd)
	}
	return details
}

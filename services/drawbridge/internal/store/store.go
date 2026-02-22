// SPDX-License-Identifier: Apache-2.0
package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tonyellard/drawbridge/internal/model"
)

// busRecord holds an event bus and its rules.
type busRecord struct {
	bus   model.EventBus
	rules map[string]*ruleRecord // keyed by rule name
}

// ruleRecord holds a rule and its targets.
type ruleRecord struct {
	rule    model.Rule
	targets map[string]model.Target // keyed by target ID
}

// Store is a thread-safe in-memory store for EventBridge resources.
type Store struct {
	mu        sync.RWMutex
	buses     map[string]*busRecord // keyed by bus name
	region    string
	accountID string
}

// NewStore creates a new store with a "default" event bus.
func NewStore(region, accountID string) *Store {
	s := &Store{
		buses:     make(map[string]*busRecord),
		region:    region,
		accountID: accountID,
	}
	s.buses["default"] = &busRecord{
		bus: model.EventBus{
			Name:      "default",
			Arn:       fmt.Sprintf("arn:aws:events:%s:%s:event-bus/default", region, accountID),
			State:     "ACTIVE",
			CreatedAt: time.Now().UTC(),
		},
		rules: make(map[string]*ruleRecord),
	}
	return s
}

func (s *Store) busArn(name string) string {
	return fmt.Sprintf("arn:aws:events:%s:%s:event-bus/%s", s.region, s.accountID, name)
}

func (s *Store) ruleArn(busName, ruleName string) string {
	if busName == "" || busName == "default" {
		return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s", s.region, s.accountID, ruleName)
	}
	return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s/%s", s.region, s.accountID, busName, ruleName)
}

func (s *Store) resolveBusName(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

// --- Event Bus operations ---

func (s *Store) CreateEventBus(req model.CreateEventBusRequest) (model.CreateEventBusResponse, error) {
	if req.Name == "" {
		return model.CreateEventBusResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.Name == "default" {
		return model.CreateEventBusResponse{}, fmt.Errorf("ValidationException: Cannot create event bus named 'default'")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.buses[req.Name]; exists {
		return model.CreateEventBusResponse{}, fmt.Errorf("ResourceAlreadyExistsException: Event bus %s already exists", req.Name)
	}

	arn := s.busArn(req.Name)
	s.buses[req.Name] = &busRecord{
		bus: model.EventBus{
			Name:      req.Name,
			Arn:       arn,
			State:     "ACTIVE",
			CreatedAt: time.Now().UTC(),
		},
		rules: make(map[string]*ruleRecord),
	}

	return model.CreateEventBusResponse{EventBusArn: arn}, nil
}

func (s *Store) DescribeEventBus(name string) (model.DescribeEventBusResponse, error) {
	name = s.resolveBusName(name)

	s.mu.RLock()
	defer s.mu.RUnlock()

	br, exists := s.buses[name]
	if !exists {
		return model.DescribeEventBusResponse{}, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", name)
	}

	return model.DescribeEventBusResponse{
		Name: br.bus.Name,
		Arn:  br.bus.Arn,
	}, nil
}

func (s *Store) ListEventBuses(req model.ListEventBusesRequest) model.ListEventBusesResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buses := make([]model.EventBus, 0, len(s.buses))
	for _, br := range s.buses {
		if req.NamePrefix != "" && !strings.HasPrefix(br.bus.Name, req.NamePrefix) {
			continue
		}
		eb := br.bus
		eb.CreationTime = float64(eb.CreatedAt.UnixMilli()) / 1000.0
		buses = append(buses, eb)
	}

	return model.ListEventBusesResponse{EventBuses: buses}
}

func (s *Store) DeleteEventBus(name string) error {
	if name == "" || name == "default" {
		return fmt.Errorf("ValidationException: Cannot delete the default event bus")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.buses[name]; !exists {
		return fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", name)
	}

	delete(s.buses, name)
	return nil
}

// --- Rule operations ---

func (s *Store) PutRule(req model.PutRuleRequest) (model.PutRuleResponse, error) {
	if req.Name == "" {
		return model.PutRuleResponse{}, fmt.Errorf("ValidationException: Name is required")
	}
	if req.EventPattern == "" && req.ScheduleExpression == "" {
		return model.PutRuleResponse{}, fmt.Errorf("ValidationException: Either EventPattern or ScheduleExpression is required")
	}

	busName := s.resolveBusName(req.EventBusName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return model.PutRuleResponse{}, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	arn := s.ruleArn(busName, req.Name)
	state := req.State
	if state == "" {
		state = "ENABLED"
	}

	if existing, ok := br.rules[req.Name]; ok {
		// Update existing rule, preserve targets
		existing.rule.EventPattern = req.EventPattern
		existing.rule.ScheduleExpression = req.ScheduleExpression
		existing.rule.Description = req.Description
		existing.rule.State = state
		existing.rule.Arn = arn
		return model.PutRuleResponse{RuleArn: arn}, nil
	}

	br.rules[req.Name] = &ruleRecord{
		rule: model.Rule{
			Name:               req.Name,
			Arn:                arn,
			EventBusName:       busName,
			EventPattern:       req.EventPattern,
			ScheduleExpression: req.ScheduleExpression,
			State:              state,
			Description:        req.Description,
		},
		targets: make(map[string]model.Target),
	}

	return model.PutRuleResponse{RuleArn: arn}, nil
}

func (s *Store) DescribeRule(name, busName string) (model.Rule, error) {
	busName = s.resolveBusName(busName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	br, exists := s.buses[busName]
	if !exists {
		return model.Rule{}, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[name]
	if !exists {
		return model.Rule{}, fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", name, busName)
	}

	return rr.rule, nil
}

func (s *Store) ListRules(req model.ListRulesRequest) model.ListRulesResponse {
	busName := s.resolveBusName(req.EventBusName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	br, exists := s.buses[busName]
	if !exists {
		return model.ListRulesResponse{Rules: []model.Rule{}}
	}

	rules := make([]model.Rule, 0, len(br.rules))
	for _, rr := range br.rules {
		if req.NamePrefix != "" && !strings.HasPrefix(rr.rule.Name, req.NamePrefix) {
			continue
		}
		rules = append(rules, rr.rule)
	}

	return model.ListRulesResponse{Rules: rules}
}

func (s *Store) DeleteRule(name, busName string) error {
	busName = s.resolveBusName(busName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[name]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", name, busName)
	}

	if len(rr.targets) > 0 {
		return fmt.Errorf("ValidationException: Rule %s has targets. Remove targets before deleting the rule", name)
	}

	delete(br.rules, name)
	return nil
}

func (s *Store) EnableRule(name, busName string) error {
	busName = s.resolveBusName(busName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[name]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", name, busName)
	}

	rr.rule.State = "ENABLED"
	return nil
}

func (s *Store) DisableRule(name, busName string) error {
	busName = s.resolveBusName(busName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[name]
	if !exists {
		return fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", name, busName)
	}

	rr.rule.State = "DISABLED"
	return nil
}

// --- Target operations ---

func (s *Store) PutTargets(req model.PutTargetsRequest) (model.PutTargetsResponse, error) {
	if req.Rule == "" {
		return model.PutTargetsResponse{}, fmt.Errorf("ValidationException: Rule is required")
	}

	busName := s.resolveBusName(req.EventBusName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return model.PutTargetsResponse{}, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[req.Rule]
	if !exists {
		return model.PutTargetsResponse{}, fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", req.Rule, busName)
	}

	resp := model.PutTargetsResponse{}
	for _, t := range req.Targets {
		if t.Id == "" || t.Arn == "" {
			resp.FailedEntryCount++
			resp.FailedEntries = append(resp.FailedEntries, model.PutTargetsResultEntry{
				TargetId:     t.Id,
				ErrorCode:    "ValidationException",
				ErrorMessage: "Id and Arn are required",
			})
			continue
		}

		// Enforce max 5 targets per rule
		if _, updating := rr.targets[t.Id]; !updating && len(rr.targets) >= 5 {
			resp.FailedEntryCount++
			resp.FailedEntries = append(resp.FailedEntries, model.PutTargetsResultEntry{
				TargetId:     t.Id,
				ErrorCode:    "LimitExceededException",
				ErrorMessage: "Maximum number of targets per rule exceeded",
			})
			continue
		}

		rr.targets[t.Id] = t
	}

	return resp, nil
}

func (s *Store) ListTargetsByRule(ruleName, busName string) ([]model.Target, error) {
	busName = s.resolveBusName(busName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	br, exists := s.buses[busName]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[ruleName]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", ruleName, busName)
	}

	targets := make([]model.Target, 0, len(rr.targets))
	for _, t := range rr.targets {
		targets = append(targets, t)
	}

	return targets, nil
}

func (s *Store) RemoveTargets(req model.RemoveTargetsRequest) (model.RemoveTargetsResponse, error) {
	if req.Rule == "" {
		return model.RemoveTargetsResponse{}, fmt.Errorf("ValidationException: Rule is required")
	}

	busName := s.resolveBusName(req.EventBusName)

	s.mu.Lock()
	defer s.mu.Unlock()

	br, exists := s.buses[busName]
	if !exists {
		return model.RemoveTargetsResponse{}, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	rr, exists := br.rules[req.Rule]
	if !exists {
		return model.RemoveTargetsResponse{}, fmt.Errorf("ResourceNotFoundException: Rule %s does not exist on event bus %s", req.Rule, busName)
	}

	resp := model.RemoveTargetsResponse{}
	for _, id := range req.Ids {
		if _, exists := rr.targets[id]; !exists {
			resp.FailedEntryCount++
			resp.FailedEntries = append(resp.FailedEntries, model.RemoveTargetsResultEntry{
				TargetId:     id,
				ErrorCode:    "ResourceNotFoundException",
				ErrorMessage: fmt.Sprintf("Target %s not found", id),
			})
			continue
		}
		delete(rr.targets, id)
	}

	return resp, nil
}

// --- Query helpers for PutEvents and admin ---

// EnabledRulesForBus returns all enabled rules and their targets for a given bus.
type RuleWithTargets struct {
	Rule    model.Rule
	Targets []model.Target
}

func (s *Store) EnabledRulesForBus(busName string) ([]RuleWithTargets, error) {
	busName = s.resolveBusName(busName)

	s.mu.RLock()
	defer s.mu.RUnlock()

	br, exists := s.buses[busName]
	if !exists {
		return nil, fmt.Errorf("ResourceNotFoundException: Event bus %s does not exist", busName)
	}

	var result []RuleWithTargets
	for _, rr := range br.rules {
		if rr.rule.State != "ENABLED" {
			continue
		}
		if rr.rule.EventPattern == "" {
			continue
		}
		targets := make([]model.Target, 0, len(rr.targets))
		for _, t := range rr.targets {
			targets = append(targets, t)
		}
		result = append(result, RuleWithTargets{Rule: rr.rule, Targets: targets})
	}

	return result, nil
}

// --- Admin helpers ---

func (s *Store) Summary() model.AdminSummaryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rules, targets int
	for _, br := range s.buses {
		rules += len(br.rules)
		for _, rr := range br.rules {
			targets += len(rr.targets)
		}
	}

	return model.AdminSummaryResponse{
		Service:    "drawbridge",
		EventBuses: len(s.buses),
		Rules:      rules,
		Targets:    targets,
	}
}

func (s *Store) BusDetails() []model.AdminBusDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()

	details := make([]model.AdminBusDetail, 0, len(s.buses))
	for _, br := range s.buses {
		bd := model.AdminBusDetail{
			Name:  br.bus.Name,
			Arn:   br.bus.Arn,
			Rules: make([]model.AdminRuleDetail, 0, len(br.rules)),
		}
		for _, rr := range br.rules {
			targets := make([]model.Target, 0, len(rr.targets))
			for _, t := range rr.targets {
				targets = append(targets, t)
			}
			rd := model.AdminRuleDetail{
				Name:         rr.rule.Name,
				State:        rr.rule.State,
				EventPattern: rr.rule.EventPattern,
				TargetCount:  len(rr.targets),
				Targets:      targets,
			}
			bd.Rules = append(bd.Rules, rd)
			bd.RuleCount++
			bd.TargetCount += len(rr.targets)
		}
		details = append(details, bd)
	}

	return details
}

func (s *Store) ExportState() model.AdminExportResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := model.AdminExportResponse{
		EventBuses: make([]model.AdminBusExport, 0, len(s.buses)),
	}
	for _, br := range s.buses {
		be := model.AdminBusExport{
			Name:  br.bus.Name,
			Rules: make([]model.AdminRuleExport, 0, len(br.rules)),
		}
		for _, rr := range br.rules {
			targets := make([]model.Target, 0, len(rr.targets))
			for _, t := range rr.targets {
				targets = append(targets, t)
			}
			re := model.AdminRuleExport{
				Name:               rr.rule.Name,
				EventPattern:       rr.rule.EventPattern,
				ScheduleExpression: rr.rule.ScheduleExpression,
				State:              rr.rule.State,
				Description:        rr.rule.Description,
				Targets:            targets,
			}
			be.Rules = append(be.Rules, re)
		}
		resp.EventBuses = append(resp.EventBuses, be)
	}

	return resp
}

func (s *Store) ImportState(req model.AdminImportRequest) model.AdminImportResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	imported := 0
	for _, be := range req.EventBuses {
		name := be.Name
		if name == "" {
			continue
		}

		br, exists := s.buses[name]
		if !exists {
			br = &busRecord{
				bus: model.EventBus{
					Name:      name,
					Arn:       s.busArn(name),
					State:     "ACTIVE",
					CreatedAt: time.Now().UTC(),
				},
				rules: make(map[string]*ruleRecord),
			}
			s.buses[name] = br
		}

		for _, re := range be.Rules {
			if re.Name == "" {
				continue
			}
			state := re.State
			if state == "" {
				state = "ENABLED"
			}
			rr := &ruleRecord{
				rule: model.Rule{
					Name:               re.Name,
					Arn:                s.ruleArn(name, re.Name),
					EventBusName:       name,
					EventPattern:       re.EventPattern,
					ScheduleExpression: re.ScheduleExpression,
					State:              state,
					Description:        re.Description,
				},
				targets: make(map[string]model.Target),
			}
			for _, t := range re.Targets {
				if t.Id != "" {
					rr.targets[t.Id] = t
				}
			}
			br.rules[re.Name] = rr
			imported++
		}
	}

	return model.AdminImportResponse{Imported: imported}
}

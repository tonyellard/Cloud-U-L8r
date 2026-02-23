// SPDX-License-Identifier: Apache-2.0
package store

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tonyellard/pipes/internal/model"
)

// Store is an in-memory store for EventBridge Pipes.
type Store struct {
	mu        sync.RWMutex
	pipes     map[string]*model.Pipe
	tags      map[string]map[string]string // keyed by ARN
	region    string
	accountID string
}

// New creates a new pipes store.
func New(region, accountID string) *Store {
	return &Store{
		pipes:     make(map[string]*model.Pipe),
		tags:      make(map[string]map[string]string),
		region:    region,
		accountID: accountID,
	}
}

func (s *Store) makeARN(name string) string {
	return fmt.Sprintf("arn:aws:pipes:%s:%s:pipe/%s", s.region, s.accountID, name)
}

// CreatePipe creates a new pipe. Returns ConflictException if name exists.
func (s *Store) CreatePipe(name string, req model.CreatePipeRequest) (*model.Pipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pipes[name]; exists {
		return nil, fmt.Errorf("ConflictException: Pipe %s already exists", name)
	}

	now := time.Now().UTC()
	desired := req.DesiredState
	if desired == "" {
		desired = "RUNNING"
	}

	p := &model.Pipe{
		Name:                 name,
		Arn:                  s.makeARN(name),
		Description:          req.Description,
		Source:               req.Source,
		Target:               req.Target,
		RoleArn:              req.RoleArn,
		DesiredState:         desired,
		CurrentState:         desired,
		Enrichment:           req.Enrichment,
		EnrichmentParameters: req.EnrichmentParameters,
		CreationTime:         now,
		LastModifiedTime:     now,
	}

	// Handle SourceParameters — hoist FilterCriteria if nested
	if req.SourceParameters != nil {
		p.SourceParameters = req.SourceParameters
		if req.SourceParameters.FilterCriteria != nil {
			p.FilterCriteria = req.SourceParameters.FilterCriteria
		}
	}
	if p.SourceParameters == nil {
		p.SourceParameters = &model.SourceParameters{}
	}

	if req.TargetParameters != nil {
		p.TargetParameters = req.TargetParameters
	}
	if p.TargetParameters == nil {
		p.TargetParameters = &model.TargetParameters{}
	}

	if req.Tags != nil {
		p.Tags = req.Tags
		s.tags[p.Arn] = copyTags(req.Tags)
	}

	s.pipes[name] = p
	return p, nil
}

// DescribePipe returns a pipe by name.
func (s *Store) DescribePipe(name string) (*model.Pipe, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.pipes[name]
	if !exists {
		return nil, fmt.Errorf("NotFoundException: Pipe %s does not exist", name)
	}
	return p, nil
}

// UpdatePipe updates an existing pipe.
func (s *Store) UpdatePipe(name string, req model.UpdatePipeRequest) (*model.Pipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pipes[name]
	if !exists {
		return nil, fmt.Errorf("NotFoundException: Pipe %s does not exist", name)
	}

	if req.Description != "" {
		p.Description = req.Description
	}
	if req.DesiredState != "" {
		p.DesiredState = req.DesiredState
		p.CurrentState = req.DesiredState
	}
	if req.RoleArn != "" {
		p.RoleArn = req.RoleArn
	}
	if req.Enrichment != "" {
		p.Enrichment = req.Enrichment
	}
	if req.EnrichmentParameters != nil {
		p.EnrichmentParameters = req.EnrichmentParameters
	}
	if req.Target != "" {
		p.Target = req.Target
	}
	if req.SourceParameters != nil {
		p.SourceParameters = req.SourceParameters
		if req.SourceParameters.FilterCriteria != nil {
			p.FilterCriteria = req.SourceParameters.FilterCriteria
		}
	}
	if req.TargetParameters != nil {
		p.TargetParameters = req.TargetParameters
	}

	p.LastModifiedTime = time.Now().UTC()
	return p, nil
}

// DeletePipe removes a pipe by name.
func (s *Store) DeletePipe(name string) (*model.Pipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pipes[name]
	if !exists {
		return nil, fmt.Errorf("NotFoundException: Pipe %s does not exist", name)
	}

	delete(s.pipes, name)
	delete(s.tags, p.Arn)
	return p, nil
}

// ListPipes returns pipes with optional filtering.
func (s *Store) ListPipes(namePrefix, sourcePrefix, targetPrefix, currentState, desiredState string, limit int) []model.PipeSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []model.PipeSummary
	for _, p := range s.pipes {
		if namePrefix != "" && !strings.HasPrefix(p.Name, namePrefix) {
			continue
		}
		if sourcePrefix != "" && !strings.HasPrefix(p.Source, sourcePrefix) {
			continue
		}
		if targetPrefix != "" && !strings.HasPrefix(p.Target, targetPrefix) {
			continue
		}
		if currentState != "" && p.CurrentState != currentState {
			continue
		}
		if desiredState != "" && p.DesiredState != desiredState {
			continue
		}
		result = append(result, model.PipeSummary{
			Arn:              p.Arn,
			CreationTime:     float64(p.CreationTime.Unix()),
			CurrentState:     p.CurrentState,
			DesiredState:     p.DesiredState,
			Enrichment:       p.Enrichment,
			LastModifiedTime: float64(p.LastModifiedTime.Unix()),
			Name:             p.Name,
			Source:           p.Source,
			Target:           p.Target,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// StartPipe sets a pipe to RUNNING state.
func (s *Store) StartPipe(name string) (*model.Pipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pipes[name]
	if !exists {
		return nil, fmt.Errorf("NotFoundException: Pipe %s does not exist", name)
	}

	p.DesiredState = "RUNNING"
	p.CurrentState = "RUNNING"
	p.LastModifiedTime = time.Now().UTC()
	return p, nil
}

// StopPipe sets a pipe to STOPPED state.
func (s *Store) StopPipe(name string) (*model.Pipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, exists := s.pipes[name]
	if !exists {
		return nil, fmt.Errorf("NotFoundException: Pipe %s does not exist", name)
	}

	p.DesiredState = "STOPPED"
	p.CurrentState = "STOPPED"
	p.LastModifiedTime = time.Now().UTC()
	return p, nil
}

// TagResource adds tags to a resource by ARN.
func (s *Store) TagResource(arn string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify the ARN belongs to a known pipe
	found := false
	for _, p := range s.pipes {
		if p.Arn == arn {
			if p.Tags == nil {
				p.Tags = make(map[string]string)
			}
			for k, v := range tags {
				p.Tags[k] = v
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("NotFoundException: Resource %s does not exist", arn)
	}

	if s.tags[arn] == nil {
		s.tags[arn] = make(map[string]string)
	}
	for k, v := range tags {
		s.tags[arn][k] = v
	}
	return nil
}

// UntagResource removes tags from a resource by ARN.
func (s *Store) UntagResource(arn string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for _, p := range s.pipes {
		if p.Arn == arn {
			for _, k := range tagKeys {
				delete(p.Tags, k)
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("NotFoundException: Resource %s does not exist", arn)
	}

	if existing := s.tags[arn]; existing != nil {
		for _, k := range tagKeys {
			delete(existing, k)
		}
	}
	return nil
}

// ListTagsForResource returns tags for a resource ARN.
func (s *Store) ListTagsForResource(arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	found := false
	for _, p := range s.pipes {
		if p.Arn == arn {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("NotFoundException: Resource %s does not exist", arn)
	}

	tags := s.tags[arn]
	if tags == nil {
		return map[string]string{}, nil
	}
	return copyTags(tags), nil
}

// RunningPipes returns all pipes with CurrentState == "RUNNING".
func (s *Store) RunningPipes() []*model.Pipe {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.Pipe
	for _, p := range s.pipes {
		if p.CurrentState == "RUNNING" {
			result = append(result, p)
		}
	}
	return result
}

// Summary returns aggregate stats for admin.
func (s *Store) Summary() model.AdminSummaryResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	running := 0
	for _, p := range s.pipes {
		if p.CurrentState == "RUNNING" {
			running++
		}
	}
	return model.AdminSummaryResponse{
		Service:      "pipes",
		Pipes:        len(s.pipes),
		RunningPipes: running,
	}
}

// PipeDetails returns detailed info for all pipes (admin).
func (s *Store) PipeDetails() []model.AdminPipeDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []model.AdminPipeDetail
	for _, p := range s.pipes {
		filterCount := 0
		if p.FilterCriteria != nil {
			filterCount = len(p.FilterCriteria.Filters)
		}
		result = append(result, model.AdminPipeDetail{
			Name:             p.Name,
			Arn:              p.Arn,
			CurrentState:     p.CurrentState,
			DesiredState:     p.DesiredState,
			Source:           p.Source,
			Target:           p.Target,
			Enrichment:       p.Enrichment,
			Description:      p.Description,
			FilterCount:      filterCount,
			CreationTime:     p.CreationTime.Format(time.RFC3339),
			LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func copyTags(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

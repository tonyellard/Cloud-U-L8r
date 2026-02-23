// SPDX-License-Identifier: Apache-2.0
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tonyellard/cloud-u-l8r/pkg/activity"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
	"github.com/tonyellard/pipes/internal/delivery"
	"github.com/tonyellard/pipes/internal/model"
	"github.com/tonyellard/pipes/internal/poller"
	"github.com/tonyellard/pipes/internal/store"
)

var validName = regexp.MustCompile(`^[\.\-_A-Za-z0-9]+$`)

// Config holds service configuration from environment variables.
type Config struct {
	Region              string
	AccountID           string
	SQSEndpoint         string
	SNSEndpoint         string
	EventBridgeEndpoint string
}

// Server holds dependencies for the HTTP handlers.
type Server struct {
	logger      *slog.Logger
	store       *store.Store
	deliverer   *delivery.Deliverer
	poller      *poller.Poller
	activityLog *activity.Logger
}

// NewRouter creates the HTTP handler with all routes.
func NewRouter(logger *slog.Logger, cfg Config) http.Handler {
	st := store.New(cfg.Region, cfg.AccountID)
	d := delivery.NewDeliverer(logger, cfg.SQSEndpoint, cfg.SNSEndpoint, cfg.EventBridgeEndpoint)
	p := poller.New(logger, st, d, cfg.SQSEndpoint)

	actLog := activity.NewLogger(
		activity.WithMaxSize(50),
		activity.WithExcludeFunc(func(e activity.Entry) bool {
			return strings.HasPrefix(e.Path, "/admin/") || e.Path == "/health"
		}),
	)

	srv := &Server{
		logger:      logger,
		store:       st,
		deliverer:   d,
		poller:      p,
		activityLog: actLog,
	}

	go p.Start(context.Background())

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/health", health.Handler("pipes"))

	// Admin API
	mux.HandleFunc("/admin/api/summary", srv.handleAdminSummary)
	mux.HandleFunc("/admin/api/resources", srv.handleAdminResources)
	mux.HandleFunc("/admin/api/activity", srv.handleAdminActivity)

	// Tag operations — /tags/{arn...}
	mux.HandleFunc("/tags/", srv.handleTags)

	// Pipes REST API
	mux.HandleFunc("/v1/pipes", srv.handleListPipes)
	mux.HandleFunc("/v1/pipes/", srv.handlePipeRoute)

	return mux
}

// handlePipeRoute dispatches /v1/pipes/{name}[/action] routes.
func (s *Server) handlePipeRoute(w http.ResponseWriter, r *http.Request) {
	// Parse: /v1/pipes/{name} or /v1/pipes/{name}/start or /v1/pipes/{name}/stop
	path := strings.TrimPrefix(r.URL.Path, "/v1/pipes/")
	if path == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "Pipe name is required")
		return
	}

	parts := strings.SplitN(path, "/", 2)
	name := parts[0]

	// Record activity
	defer func() {
		action := fmt.Sprintf("%s /v1/pipes/%s", r.Method, name)
		if len(parts) == 2 {
			action = fmt.Sprintf("%s /v1/pipes/%s/%s", r.Method, name, parts[1])
		}
		s.activityLog.Record(activity.Entry{
			Timestamp: time.Now().UTC(),
			Method:    r.Method,
			Path:      r.URL.Path,
			Action:    action,
		})
	}()

	if len(parts) == 2 {
		switch parts[1] {
		case "start":
			if r.Method != http.MethodPost {
				s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Use POST")
				return
			}
			s.handleStartPipe(w, r, name)
			return
		case "stop":
			if r.Method != http.MethodPost {
				s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Use POST")
				return
			}
			s.handleStopPipe(w, r, name)
			return
		default:
			s.writeError(w, http.StatusNotFound, "NotFoundException", "Unknown action")
			return
		}
	}

	switch r.Method {
	case http.MethodPost:
		s.handleCreatePipe(w, r, name)
	case http.MethodGet:
		s.handleDescribePipe(w, r, name)
	case http.MethodPut:
		s.handleUpdatePipe(w, r, name)
	case http.MethodDelete:
		s.handleDeletePipe(w, r, name)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Unsupported method")
	}
}

// ── Pipe CRUD handlers ──

func (s *Server) handleCreatePipe(w http.ResponseWriter, r *http.Request, name string) {
	if len(name) < 1 || len(name) > 64 || !validName.MatchString(name) {
		s.writeError(w, http.StatusBadRequest, "ValidationException",
			"Name must be 1-64 characters matching [.\\-_A-Za-z0-9]+")
		return
	}

	var req model.CreatePipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "Invalid request body")
		return
	}

	if req.Source == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "Source is required")
		return
	}
	if req.Target == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "Target is required")
		return
	}
	if req.RoleArn == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "RoleArn is required")
		return
	}

	p, err := s.store.CreatePipe(name, req)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.CreatePipeResponse{
		Arn:              p.Arn,
		CreationTime:     p.CreationTime.Format(time.RFC3339),
		CurrentState:     p.CurrentState,
		DesiredState:     p.DesiredState,
		LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		Name:             p.Name,
	})
}

func (s *Server) handleDescribePipe(w http.ResponseWriter, _ *http.Request, name string) {
	p, err := s.store.DescribePipe(name)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	resp := model.DescribePipeResponse{
		Arn:                  p.Arn,
		CreationTime:         p.CreationTime.Format(time.RFC3339),
		CurrentState:         p.CurrentState,
		Description:          p.Description,
		DesiredState:         p.DesiredState,
		Enrichment:           p.Enrichment,
		EnrichmentParameters: p.EnrichmentParameters,
		LastModifiedTime:     p.LastModifiedTime.Format(time.RFC3339),
		Name:                 p.Name,
		RoleArn:              p.RoleArn,
		Source:               p.Source,
		SourceParameters:     p.SourceParameters,
		Tags:                 p.Tags,
		Target:               p.Target,
		TargetParameters:     p.TargetParameters,
	}

	// Ensure non-nil for Terraform compatibility
	if resp.SourceParameters == nil {
		resp.SourceParameters = &model.SourceParameters{}
	}
	if resp.TargetParameters == nil {
		resp.TargetParameters = &model.TargetParameters{}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdatePipe(w http.ResponseWriter, r *http.Request, name string) {
	var req model.UpdatePipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "Invalid request body")
		return
	}

	if req.RoleArn == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "RoleArn is required")
		return
	}

	p, err := s.store.UpdatePipe(name, req)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.UpdatePipeResponse{
		Arn:              p.Arn,
		CreationTime:     p.CreationTime.Format(time.RFC3339),
		CurrentState:     p.CurrentState,
		DesiredState:     p.DesiredState,
		LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		Name:             p.Name,
	})
}

func (s *Server) handleDeletePipe(w http.ResponseWriter, _ *http.Request, name string) {
	p, err := s.store.DeletePipe(name)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.DeletePipeResponse{
		Arn:              p.Arn,
		CreationTime:     p.CreationTime.Format(time.RFC3339),
		CurrentState:     p.CurrentState,
		DesiredState:     p.DesiredState,
		LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		Name:             p.Name,
	})
}

func (s *Server) handleListPipes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Use GET")
		return
	}

	q := r.URL.Query()
	limit := 0
	if ls := q.Get("Limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 {
			limit = n
		}
	}

	pipes := s.store.ListPipes(
		q.Get("NamePrefix"),
		q.Get("SourcePrefix"),
		q.Get("TargetPrefix"),
		q.Get("CurrentState"),
		q.Get("DesiredState"),
		limit,
	)

	if pipes == nil {
		pipes = []model.PipeSummary{}
	}

	writeJSON(w, http.StatusOK, model.ListPipesResponse{
		Pipes: pipes,
	})
}

func (s *Server) handleStartPipe(w http.ResponseWriter, _ *http.Request, name string) {
	p, err := s.store.StartPipe(name)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.UpdatePipeResponse{
		Arn:              p.Arn,
		CreationTime:     p.CreationTime.Format(time.RFC3339),
		CurrentState:     p.CurrentState,
		DesiredState:     p.DesiredState,
		LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		Name:             p.Name,
	})
}

func (s *Server) handleStopPipe(w http.ResponseWriter, _ *http.Request, name string) {
	p, err := s.store.StopPipe(name)
	if err != nil {
		s.mapAndWriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.UpdatePipeResponse{
		Arn:              p.Arn,
		CreationTime:     p.CreationTime.Format(time.RFC3339),
		CurrentState:     p.CurrentState,
		DesiredState:     p.DesiredState,
		LastModifiedTime: p.LastModifiedTime.Format(time.RFC3339),
		Name:             p.Name,
	})
}

// ── Tag handlers ──

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	arn := strings.TrimPrefix(r.URL.Path, "/tags/")
	if arn == "" {
		s.writeError(w, http.StatusBadRequest, "ValidationException", "ARN is required")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var body struct {
			Tags map[string]string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeError(w, http.StatusBadRequest, "ValidationException", "Invalid request body")
			return
		}
		if err := s.store.TagResource(arn, body.Tags); err != nil {
			s.mapAndWriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{})

	case http.MethodGet:
		tags, err := s.store.ListTagsForResource(arn)
		if err != nil {
			s.mapAndWriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"tags": tags})

	case http.MethodDelete:
		tagKeys := r.URL.Query()["tagKeys"]
		if len(tagKeys) == 0 {
			s.writeError(w, http.StatusBadRequest, "ValidationException", "tagKeys is required")
			return
		}
		if err := s.store.UntagResource(arn, tagKeys); err != nil {
			s.mapAndWriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Unsupported method")
	}
}

// ── Admin handlers ──

func (s *Server) handleAdminSummary(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Summary())
}

func (s *Server) handleAdminResources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.PipeDetails())
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := 50
	if ms := r.URL.Query().Get("maxResults"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil && n > 0 {
			maxResults = n
		}
	}
	nextToken := r.URL.Query().Get("nextToken")

	entries, newNext, err := s.activityLog.List(maxResults, nextToken)
	if err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"activity":  entries,
		"nextToken": newNext,
	})
}

// ── Helpers ──

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, errType, message string) {
	awserrors.WriteJSON(w, status, errType, message)
}

func (s *Server) mapAndWriteError(w http.ResponseWriter, err error) {
	msg := err.Error()

	switch {
	case strings.HasPrefix(msg, "ConflictException:"):
		detail := strings.TrimPrefix(msg, "ConflictException:")
		awserrors.WriteJSON(w, http.StatusConflict, "ConflictException", strings.TrimSpace(detail))
	case strings.HasPrefix(msg, "NotFoundException:"):
		detail := strings.TrimPrefix(msg, "NotFoundException:")
		awserrors.WriteJSON(w, http.StatusNotFound, "NotFoundException", strings.TrimSpace(detail))
	case strings.HasPrefix(msg, "ValidationException:"):
		detail := strings.TrimPrefix(msg, "ValidationException:")
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", strings.TrimSpace(detail))
	default:
		awserrors.WriteJSON(w, http.StatusInternalServerError, "InternalServiceError", msg)
	}
}


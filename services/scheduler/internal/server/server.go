// SPDX-License-Identifier: Apache-2.0
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/tonyellard/cloud-u-l8r/pkg/activity"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
	"github.com/tonyellard/cloud-u-l8r/pkg/schedule"
	"github.com/tonyellard/scheduler/internal/delivery"
	"github.com/tonyellard/scheduler/internal/model"
	"github.com/tonyellard/scheduler/internal/runner"
	"github.com/tonyellard/scheduler/internal/store"
)

// Config holds service configuration from environment variables.
type Config struct {
	Region      string
	AccountID   string
	SQSEndpoint string
	SNSEndpoint string
}

// Server holds the scheduler state.
type Server struct {
	logger      *slog.Logger
	store       *store.Store
	activityLog *activity.Logger
	runner      *runner.Runner
}

// NewRouter creates an http.Handler for the scheduler service.
func NewRouter(logger *slog.Logger, cfg Config) http.Handler {
	st := store.NewStore(cfg.Region, cfg.AccountID)
	del := delivery.NewDeliverer(logger, cfg.SQSEndpoint, cfg.SNSEndpoint)
	srv := &Server{
		logger: logger,
		store:  st,
		activityLog: activity.NewLogger(activity.WithExcludeFunc(func(e activity.Entry) bool {
			return strings.HasPrefix(e.Path, "/admin/") || e.Path == "/health"
		})),
		runner: runner.NewRunner(logger, st, del),
	}

	go srv.runner.Start(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Handler("scheduler"))

	// Admin API
	mux.HandleFunc("/admin/api/summary", srv.handleAdminSummary)
	mux.HandleFunc("/admin/api/resources", srv.handleAdminResources)
	mux.HandleFunc("/admin/api/activity", srv.handleAdminActivity)

	// AWS Scheduler JSON API
	mux.HandleFunc("/", srv.handleAWSJSON)

	return mux
}

// --- AWS JSON Protocol handler ---

func (s *Server) handleAWSJSON(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		errType, errDetail := parseJSONError(recorder.body.Bytes())
		s.activityLog.Record(activity.Entry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Action:     target,
			StatusCode: recorder.status,
			ErrorType:  errType,
			Detail:     errDetail,
		})
	}()

	if r.Method != http.MethodPost {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if target == "" {
		awserrors.WriteJSON(recorder, http.StatusBadRequest, "ValidationException", "X-Amz-Target header is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		awserrors.WriteJSON(recorder, http.StatusBadRequest, "ValidationException", "failed to read request body")
		return
	}

	switch target {
	// Schedule Groups
	case "AWSScheduler.CreateScheduleGroup":
		s.handleCreateScheduleGroup(recorder, body)
	case "AWSScheduler.GetScheduleGroup":
		s.handleGetScheduleGroup(recorder, body)
	case "AWSScheduler.DeleteScheduleGroup":
		s.handleDeleteScheduleGroup(recorder, body)
	case "AWSScheduler.ListScheduleGroups":
		s.handleListScheduleGroups(recorder, body)
	// Schedules
	case "AWSScheduler.CreateSchedule":
		s.handleCreateSchedule(recorder, body)
	case "AWSScheduler.GetSchedule":
		s.handleGetSchedule(recorder, body)
	case "AWSScheduler.UpdateSchedule":
		s.handleUpdateSchedule(recorder, body)
	case "AWSScheduler.DeleteSchedule":
		s.handleDeleteSchedule(recorder, body)
	case "AWSScheduler.ListSchedules":
		s.handleListSchedules(recorder, body)
	default:
		awserrors.WriteJSON(recorder, http.StatusBadRequest, "ValidationException", "unsupported target: "+target)
	}
}

// --- Schedule Group handlers ---

func (s *Server) handleCreateScheduleGroup(w http.ResponseWriter, body []byte) {
	var req model.CreateScheduleGroupRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.CreateScheduleGroup(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetScheduleGroup(w http.ResponseWriter, body []byte) {
	var req model.GetScheduleGroupRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.GetScheduleGroup(req.Name)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteScheduleGroup(w http.ResponseWriter, body []byte) {
	var req model.DeleteScheduleGroupRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.DeleteScheduleGroup(req.Name); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (s *Server) handleListScheduleGroups(w http.ResponseWriter, body []byte) {
	var req model.ListScheduleGroupsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res := s.store.ListScheduleGroups(req)
	writeJSON(w, http.StatusOK, res)
}

// --- Schedule handlers ---

func (s *Server) handleCreateSchedule(w http.ResponseWriter, body []byte) {
	var req model.CreateScheduleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if req.ScheduleExpression != "" {
		if _, err := schedule.Parse(req.ScheduleExpression); err != nil {
			awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException",
				"invalid ScheduleExpression: "+err.Error())
			return
		}
	}
	res, err := s.store.CreateSchedule(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetSchedule(w http.ResponseWriter, body []byte) {
	var req model.GetScheduleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.GetSchedule(req.Name, req.GroupName)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, body []byte) {
	var req model.UpdateScheduleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if req.ScheduleExpression != "" {
		if _, err := schedule.Parse(req.ScheduleExpression); err != nil {
			awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException",
				"invalid ScheduleExpression: "+err.Error())
			return
		}
	}
	res, err := s.store.UpdateSchedule(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, body []byte) {
	var req model.DeleteScheduleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.DeleteSchedule(req.Name, req.GroupName); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (s *Server) handleListSchedules(w http.ResponseWriter, body []byte) {
	var req model.ListSchedulesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res := s.store.ListSchedules(req)
	writeJSON(w, http.StatusOK, res)
}

// --- Admin handlers ---

func (s *Server) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.store.Summary())
}

func (s *Server) handleAdminResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.store.GroupDetails())
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	maxResults := 50
	if maxStr := r.URL.Query().Get("maxResults"); maxStr != "" {
		if v, err := strconv.Atoi(maxStr); err == nil && v > 0 {
			maxResults = v
		}
	}

	entries, token, err := s.activityLog.List(maxResults, r.URL.Query().Get("nextToken"))
	if err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"activity":  entries,
		"nextToken": token,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func mapErrorToAWS(w http.ResponseWriter, err error) {
	if err == nil {
		awserrors.WriteJSON(w, http.StatusInternalServerError, "InternalFailure", "unknown error")
		return
	}
	msg := err.Error()
	typeName := "InternalFailure"
	status := http.StatusInternalServerError

	if strings.Contains(msg, ":") {
		parts := strings.SplitN(msg, ":", 2)
		typeName = strings.TrimSpace(parts[0])
		msg = strings.TrimSpace(parts[1])
	}

	switch typeName {
	case "ValidationException", "ConflictException":
		status = http.StatusBadRequest
	case "ResourceNotFoundException":
		status = http.StatusNotFound
	}

	awserrors.WriteJSON(w, status, typeName, msg)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	_, _ = s.body.Write(b)
	return s.ResponseWriter.Write(b)
}

func parseJSONError(body []byte) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	var payload struct {
		Type    string `json:"__type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	return payload.Type, payload.Message
}

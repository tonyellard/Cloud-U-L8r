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
	"time"

	"github.com/google/uuid"
	"github.com/tonyellard/cloud-u-l8r/pkg/activity"
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
	"github.com/tonyellard/cloud-u-l8r/pkg/schedule"
	"github.com/tonyellard/drawbridge/internal/delivery"
	"github.com/tonyellard/drawbridge/internal/matching"
	"github.com/tonyellard/drawbridge/internal/model"
	dbschedule "github.com/tonyellard/drawbridge/internal/schedule"
	"github.com/tonyellard/drawbridge/internal/store"
)

// Config holds service configuration from environment variables.
type Config struct {
	Region      string
	AccountID   string
	SQSEndpoint string
	SNSEndpoint string
}

// Server holds the drawbridge state.
type Server struct {
	logger      *slog.Logger
	store       *store.Store
	deliverer   *delivery.Deliverer
	activityLog *activity.Logger
	scheduler   *dbschedule.Scheduler
}

// NewRouter creates an http.Handler for the drawbridge service.
func NewRouter(logger *slog.Logger, cfg Config) http.Handler {
	st := store.NewStore(cfg.Region, cfg.AccountID)
	del := delivery.NewDeliverer(logger, cfg.SQSEndpoint, cfg.SNSEndpoint)
	srv := &Server{
		logger:    logger,
		store:     st,
		deliverer: del,
		activityLog: activity.NewLogger(activity.WithExcludeFunc(func(e activity.Entry) bool {
			return strings.HasPrefix(e.Path, "/admin/") || e.Path == "/health"
		})),
		scheduler: dbschedule.NewScheduler(logger, st, del, cfg.Region, cfg.AccountID),
	}

	go srv.scheduler.Start(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Handler("drawbridge"))

	// Admin API
	mux.HandleFunc("/admin/api/summary", srv.handleAdminSummary)
	mux.HandleFunc("/admin/api/resources", srv.handleAdminResources)
	mux.HandleFunc("/admin/api/activity", srv.handleAdminActivity)
	mux.HandleFunc("/admin/api/export", srv.handleAdminExport)
	mux.HandleFunc("/admin/api/import", srv.handleAdminImport)

	// AWS EventBridge JSON API
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
	// Event Buses
	case "AWSEvents.CreateEventBus":
		s.handleCreateEventBus(recorder, body)
	case "AWSEvents.DescribeEventBus":
		s.handleDescribeEventBus(recorder, body)
	case "AWSEvents.ListEventBuses":
		s.handleListEventBuses(recorder, body)
	case "AWSEvents.DeleteEventBus":
		s.handleDeleteEventBus(recorder, body)
	// Rules
	case "AWSEvents.PutRule":
		s.handlePutRule(recorder, body)
	case "AWSEvents.DescribeRule":
		s.handleDescribeRule(recorder, body)
	case "AWSEvents.ListRules":
		s.handleListRules(recorder, body)
	case "AWSEvents.DeleteRule":
		s.handleDeleteRule(recorder, body)
	case "AWSEvents.EnableRule":
		s.handleEnableRule(recorder, body)
	case "AWSEvents.DisableRule":
		s.handleDisableRule(recorder, body)
	// Targets
	case "AWSEvents.PutTargets":
		s.handlePutTargets(recorder, body)
	case "AWSEvents.ListTargetsByRule":
		s.handleListTargetsByRule(recorder, body)
	case "AWSEvents.RemoveTargets":
		s.handleRemoveTargets(recorder, body)
	// Events
	case "AWSEvents.PutEvents":
		s.handlePutEvents(recorder, body)
	case "AWSEvents.TestEventPattern":
		s.handleTestEventPattern(recorder, body)
	// Tags (stubs)
	case "AWSEvents.ListTagsForResource":
		writeJSON(recorder, http.StatusOK, map[string]interface{}{"Tags": []interface{}{}})
	case "AWSEvents.TagResource":
		writeJSON(recorder, http.StatusOK, map[string]interface{}{})
	case "AWSEvents.UntagResource":
		writeJSON(recorder, http.StatusOK, map[string]interface{}{})
	default:
		awserrors.WriteJSON(recorder, http.StatusBadRequest, "ValidationException", "unsupported target: "+target)
	}
}

// --- Event Bus handlers ---

func (s *Server) handleCreateEventBus(w http.ResponseWriter, body []byte) {
	var req model.CreateEventBusRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.CreateEventBus(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDescribeEventBus(w http.ResponseWriter, body []byte) {
	var req model.DescribeEventBusRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.DescribeEventBus(req.Name)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListEventBuses(w http.ResponseWriter, body []byte) {
	var req model.ListEventBusesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res := s.store.ListEventBuses(req)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteEventBus(w http.ResponseWriter, body []byte) {
	var req model.DeleteEventBusRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.DeleteEventBus(req.Name); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// --- Rule handlers ---

func (s *Server) handlePutRule(w http.ResponseWriter, body []byte) {
	var req model.PutRuleRequest
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
	res, err := s.store.PutRule(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDescribeRule(w http.ResponseWriter, body []byte) {
	var req model.DescribeRuleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.DescribeRule(req.Name, req.EventBusName)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListRules(w http.ResponseWriter, body []byte) {
	var req model.ListRulesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res := s.store.ListRules(req)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, body []byte) {
	var req model.DeleteRuleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.DeleteRule(req.Name, req.EventBusName); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (s *Server) handleEnableRule(w http.ResponseWriter, body []byte) {
	var req model.EnableRuleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.EnableRule(req.Name, req.EventBusName); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (s *Server) handleDisableRule(w http.ResponseWriter, body []byte) {
	var req model.DisableRuleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	if err := s.store.DisableRule(req.Name, req.EventBusName); err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// --- Target handlers ---

func (s *Server) handlePutTargets(w http.ResponseWriter, body []byte) {
	var req model.PutTargetsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.PutTargets(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListTargetsByRule(w http.ResponseWriter, body []byte) {
	var req model.ListTargetsByRuleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	targets, err := s.store.ListTargetsByRule(req.Rule, req.EventBusName)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.ListTargetsByRuleResponse{Targets: targets})
}

func (s *Server) handleRemoveTargets(w http.ResponseWriter, body []byte) {
	var req model.RemoveTargetsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}
	res, err := s.store.RemoveTargets(req)
	if err != nil {
		mapErrorToAWS(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// --- Event handlers ---

func (s *Server) handlePutEvents(w http.ResponseWriter, body []byte) {
	var req model.PutEventsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	resp := model.PutEventsResponse{
		Entries: make([]model.PutEventsResultEntry, 0, len(req.Entries)),
	}

	for _, entry := range req.Entries {
		eventID := uuid.New().String()
		busName := entry.EventBusName
		if busName == "" {
			busName = "default"
		}

		// Build the full event envelope
		eventTime := time.Now().UTC()
		if entry.Time != nil {
			eventTime = time.Unix(int64(*entry.Time), 0).UTC()
		}

		fullEvent := map[string]interface{}{
			"version":     "0",
			"id":          eventID,
			"source":      entry.Source,
			"account":     "000000000000",
			"time":        eventTime.Format(time.RFC3339),
			"region":      "us-east-1",
			"resources":   entry.Resources,
			"detail-type": entry.DetailType,
		}

		// Parse detail as raw JSON
		if entry.Detail != "" {
			var detail interface{}
			if err := json.Unmarshal([]byte(entry.Detail), &detail); err == nil {
				fullEvent["detail"] = detail
			} else {
				fullEvent["detail"] = entry.Detail
			}
		} else {
			fullEvent["detail"] = map[string]interface{}{}
		}

		eventJSON, _ := json.Marshal(fullEvent)

		// Get enabled rules for this bus and evaluate patterns
		rules, err := s.store.EnabledRulesForBus(busName)
		if err != nil {
			resp.FailedEntryCount++
			resp.Entries = append(resp.Entries, model.PutEventsResultEntry{
				ErrorCode:    "InternalError",
				ErrorMessage: err.Error(),
			})
			continue
		}

		matchCount := 0
		for _, rt := range rules {
			matched, err := matching.Match(rt.Rule.EventPattern, string(eventJSON))
			if err != nil {
				s.logger.Warn("pattern match error",
					"rule", rt.Rule.Name,
					"error", err)
				continue
			}
			if !matched {
				continue
			}
			matchCount++

			// Deliver to all targets asynchronously
			for _, target := range rt.Targets {
				go func(t model.Target) {
					if err := s.deliverer.Deliver(string(eventJSON), t); err != nil {
						s.logger.Error("target delivery failed",
							"rule", rt.Rule.Name,
							"target", t.Id,
							"error", err)
					}
				}(target)
			}
		}

		s.logger.Info("event processed",
			"event_id", eventID,
			"source", entry.Source,
			"detail_type", entry.DetailType,
			"bus", busName,
			"rules_matched", matchCount)

		resp.Entries = append(resp.Entries, model.PutEventsResultEntry{
			EventId: eventID,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTestEventPattern(w http.ResponseWriter, body []byte) {
	var req model.TestEventPatternRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	if req.EventPattern == "" || req.Event == "" {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "EventPattern and Event are required")
		return
	}

	result, err := matching.Match(req.EventPattern, req.Event)
	if err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "InvalidEventPatternException", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.TestEventPatternResponse{Result: result})
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
	writeJSON(w, http.StatusOK, s.store.BusDetails())
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

func (s *Server) handleAdminExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.store.ExportState())
}

func (s *Server) handleAdminImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "failed to read request body")
		return
	}

	var req model.AdminImportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		awserrors.WriteJSON(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res := s.store.ImportState(req)
	writeJSON(w, http.StatusOK, res)
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
	case "ValidationException", "ResourceAlreadyExistsException", "InvalidEventPatternException",
		"LimitExceededException":
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


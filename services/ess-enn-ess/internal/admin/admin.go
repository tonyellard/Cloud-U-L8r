package admin

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tonyellard/ess-enn-ess/internal/activity"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/subscription"
	"github.com/tonyellard/ess-enn-ess/internal/topic"
)

// GetAdminRouteHandlers returns all admin dashboard route handlers
// This allows the admin routes to be registered on the SNS server's mux
func GetAdminRouteHandlers(cfg *config.Config, logger *slog.Logger, topicStore *topic.Store, subscriptionStore *subscription.Store, activityLogger *activity.Logger) (http.HandlerFunc, map[string]http.HandlerFunc) {
	// Create a temporary admin server just to get its methods
	s := &Server{
		config:            cfg,
		logger:            logger,
		topicStore:        topicStore,
		subscriptionStore: subscriptionStore,
		activityLogger:    activityLogger,
	}

	// Return the dashboard handler and a map of API handlers
	dashboardHandler := s.handleDashboard

	apiHandlers := map[string]http.HandlerFunc{
		"/api/topics":               s.handleTopics,
		"/api/topics/delete":        s.handleDeleteTopic,
		"/api/subscriptions":        s.handleSubscriptions,
		"/api/subscriptions/delete": s.handleDeleteSubscription,
		"/api/sqs/queues":           s.handleSQSQueues,
		"/api/activities":           s.handleGetActivities,
		"/api/stats":                s.handleGetStats,
		"/api/export":               s.handleExport,
		"/api/import":               s.handleImport,
		"/api/activities-stream":    s.handleActivityStream,
	}

	return dashboardHandler, apiHandlers
}

// Server represents the admin dashboard server
type Server struct {
	config            *config.Config
	logger            *slog.Logger
	topicStore        *topic.Store
	subscriptionStore *subscription.Store
	activityLogger    *activity.Logger
	httpServer        *http.Server
	mux               *http.ServeMux
}

// NewServer creates a new admin server
func NewServer(cfg *config.Config, logger *slog.Logger, topicStore *topic.Store, subscriptionStore *subscription.Store, activityLogger *activity.Logger) *Server {
	s := &Server{
		config:            cfg,
		logger:            logger,
		topicStore:        topicStore,
		subscriptionStore: subscriptionStore,
		activityLogger:    activityLogger,
		mux:               http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// registerRoutes registers all admin routes
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/topics", s.handleTopics)
	s.mux.HandleFunc("/api/topics/delete", s.handleDeleteTopic)
	s.mux.HandleFunc("/api/subscriptions", s.handleSubscriptions)
	s.mux.HandleFunc("/api/subscriptions/delete", s.handleDeleteSubscription)
	s.mux.HandleFunc("/api/activities", s.handleGetActivities)
	s.mux.HandleFunc("/api/stats", s.handleGetStats)
	s.mux.HandleFunc("/api/export", s.handleExport)
	s.mux.HandleFunc("/api/import", s.handleImport)
	s.mux.HandleFunc("/api/activities-stream", s.handleActivityStream)
	s.mux.HandleFunc("/", s.handleDashboard)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

// handleDashboard serves the admin dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

// handleTopics handles GET (list) and POST (create) for topics
func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		topics := s.topicStore.ListTopics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(topics)

	case http.MethodPost:
		var req struct {
			Name       string            `json:"name"`
			Attributes map[string]string `json:"attributes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "Topic name is required", http.StatusBadRequest)
			return
		}

		topic, err := s.topicStore.CreateTopic(req.Name, req.Attributes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("Topic created via admin", "name", req.Name, "arn", topic.TopicArn)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(topic)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDeleteTopic handles DELETE for topics
func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TopicArn string `json:"topic_arn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TopicArn == "" {
		http.Error(w, "Topic ARN is required", http.StatusBadRequest)
		return
	}

	err := s.topicStore.DeleteTopic(req.TopicArn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("Topic deleted via admin", "arn", req.TopicArn)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Topic deleted"})
}

// handleSubscriptions handles GET (list) and POST (create) for subscriptions
func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		topicArn := r.URL.Query().Get("topic")

		var subs []*subscription.Subscription
		if topicArn != "" {
			subs = s.subscriptionStore.GetByTopic(topicArn)
		} else {
			subs = s.subscriptionStore.ListAll()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subs)

	case http.MethodPost:
		var req struct {
			TopicArn    string `json:"topic_arn"`
			Protocol    string `json:"protocol"`
			Endpoint    string `json:"endpoint"`
			AutoConfirm bool   `json:"auto_confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TopicArn == "" || req.Protocol == "" || req.Endpoint == "" {
			http.Error(w, "topic_arn, protocol, and endpoint are required", http.StatusBadRequest)
			return
		}

		// Validate protocol
		protocol := subscription.Protocol(req.Protocol)
		if protocol != subscription.ProtocolHTTP &&
			protocol != subscription.ProtocolSQS {
			http.Error(w, "Invalid protocol. Must be http or sqs", http.StatusBadRequest)
			return
		}

		sub, err := s.subscriptionStore.Create(req.TopicArn, protocol, req.Endpoint, req.AutoConfirm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("Subscription created via admin", "topic", req.TopicArn, "protocol", req.Protocol, "endpoint", req.Endpoint)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sub)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDeleteSubscription handles DELETE for subscriptions
func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SubscriptionArn string `json:"subscription_arn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SubscriptionArn == "" {
		http.Error(w, "Subscription ARN is required", http.StatusBadRequest)
		return
	}

	err := s.subscriptionStore.Delete(req.SubscriptionArn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("Subscription deleted via admin", "arn", req.SubscriptionArn)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Subscription deleted"})
}

// handleGetActivities returns activity log entries as JSON
func (s *Server) handleGetActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topicArn := r.URL.Query().Get("topic")
	eventType := r.URL.Query().Get("event")
	status := r.URL.Query().Get("status")
	limit := 100

	entries := s.activityLogger.GetEntries()
	filtered := make([]*activity.Entry, 0)
	for _, entry := range entries {
		if topicArn != "" && entry.TopicArn != topicArn {
			continue
		}
		if eventType != "" && string(entry.EventType) != eventType {
			continue
		}
		if status != "" && string(entry.Status) != status {
			continue
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// handleGetStats returns overall statistics
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topics := s.topicStore.ListTopics()
	subscriptions := s.subscriptionStore.ListAll()
	entries := s.activityLogger.GetEntries()

	confirmedSubs := 0
	pendingSubs := 0
	for _, sub := range subscriptions {
		if sub.Status == subscription.StatusConfirmed {
			confirmedSubs++
		} else if sub.Status == subscription.StatusPending {
			pendingSubs++
		}
	}

	// Count activities by type
	publishCount := 0
	deliveryCount := 0
	deliverySuccessCount := 0
	deliveryFailCount := 0

	for _, entry := range entries {
		if entry.EventType == activity.EventTypePublish {
			publishCount++
		}
		if entry.EventType == activity.EventTypeDelivery {
			deliveryCount++
			if entry.Status == activity.StatusSuccess {
				deliverySuccessCount++
			} else if entry.Status == activity.StatusFailed {
				deliveryFailCount++
			}
		}
	}

	stats := map[string]interface{}{
		"topics": map[string]interface{}{
			"total": len(topics),
		},
		"subscriptions": map[string]interface{}{
			"total":     len(subscriptions),
			"confirmed": confirmedSubs,
			"pending":   pendingSubs,
		},
		"messages": map[string]interface{}{
			"published": publishCount,
			"delivered": deliverySuccessCount,
			"failed":    deliveryFailCount,
		},
		"events": map[string]interface{}{
			"total": len(entries),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleExport exports current configuration and state as YAML
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=sns-export.yaml")

	// Create export structure with config and state
	export := map[string]interface{}{
		"config":        s.config,
		"topics":        s.topicStore.ListTopics(),
		"subscriptions": s.subscriptionStore.ListAll(),
	}

	// Marshal to YAML
	data, err := yaml.Marshal(export)
	if err != nil {
		s.logger.Error("failed to marshal export", "error", err)
		http.Error(w, "Failed to marshal export", http.StatusInternalServerError)
		return
	}

	w.Write(data)
	s.logger.Info("SNS configuration and state exported")
}

// handleImport imports configuration from YAML (placeholder)
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","message":"Import feature coming soon"}`)
	s.logger.Info("Import endpoint called (not yet implemented)")
}

// handleActivityStream handles activity stream (placeholder)
func (s *Server) handleActivityStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprintf(w, "data: {\"status\":\"connected\"}\n\n")
	s.logger.Debug("Activity stream connected")
}

// handleSQSQueues lists available SQS queues
func (s *Server) handleSQSQueues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Call SQS ListQueues API
	sqsEndpoint := s.config.SQS.Endpoint
	resp, err := http.Post(sqsEndpoint, "application/x-www-form-urlencoded", strings.NewReader("Action=ListQueues"))
	if err != nil {
		s.logger.Error("failed to call SQS ListQueues", "error", err)
		http.Error(w, fmt.Sprintf("Failed to list SQS queues: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Parse SQS XML response
	type ListQueuesResult struct {
		QueueUrls []string `xml:"QueueUrl"`
	}
	type ListQueuesResponse struct {
		Result ListQueuesResult `xml:"ListQueuesResult"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("failed to read SQS response", "error", err)
		http.Error(w, "Failed to read SQS response", http.StatusInternalServerError)
		return
	}

	var result ListQueuesResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		s.logger.Warn("failed to parse SQS response", "error", err)
		// Return empty list instead of error for graceful degradation
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{})
		return
	}

	// Convert to JSON response
	queues := make([]map[string]string, 0)
	for _, queueUrl := range result.Result.QueueUrls {
		queues = append(queues, map[string]string{
			"url":  queueUrl,
			"name": extractQueueName(queueUrl),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queues)
}

// extractQueueName extracts the queue name from a queue URL
func extractQueueName(queueUrl string) string {
	// Queue URL format: http://localhost:9320/queue-name
	parts := strings.Split(queueUrl, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return queueUrl
}

// Start starts the admin HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.AdminPort),
		Handler:      s.mux,
		ReadTimeout:  time.Duration(s.config.Server.TimeoutSec) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.TimeoutSec) * time.Second,
	}

	s.logger.Info("Admin dashboard starting", "address", s.httpServer.Addr, "url", fmt.Sprintf("http://localhost:%d", s.config.Server.AdminPort))
	return s.httpServer.ListenAndServe()
}

// Stop stops the admin HTTP server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

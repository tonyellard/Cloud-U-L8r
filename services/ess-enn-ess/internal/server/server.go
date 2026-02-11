package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tonyellard/ess-enn-ess/internal/activity"
	"github.com/tonyellard/ess-enn-ess/internal/config"
	"github.com/tonyellard/ess-enn-ess/internal/delivery"
	"github.com/tonyellard/ess-enn-ess/internal/subscription"
	"github.com/tonyellard/ess-enn-ess/internal/topic"
)

// Server represents the SNS API server
type Server struct {
	config            *config.Config
	logger            *slog.Logger
	topicStore        *topic.Store
	subscriptionStore *subscription.Store
	activityLogger    *activity.Logger
	deliverer         *delivery.Deliverer
	httpServer        *http.Server
	mux               *http.ServeMux
}

// NewServer creates a new SNS server
func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	activityLogger := activity.NewLogger(cfg.Storage.ActivityLogSize, logger)
	s := &Server{
		config:            cfg,
		logger:            logger,
		topicStore:        topic.NewStore(cfg.AWS.AccountId, cfg.AWS.Region),
		subscriptionStore: subscription.NewStore(cfg.AWS.AccountId, cfg.AWS.Region, activityLogger),
		activityLogger:    activityLogger,
		deliverer:         delivery.NewDeliverer(logger, activityLogger, cfg),
		mux:               http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// registerRoutes registers all SNS API routes
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/", s.handleSNSRequest)
	s.mux.HandleFunc("/health", s.handleHealth)
}

// RegisterAdminRoutes registers the admin dashboard routes on the same server
func (s *Server) RegisterAdminRoutes(dashboardHandler http.HandlerFunc, apiHandlers map[string]http.HandlerFunc) {
	s.mux.HandleFunc("/admin", dashboardHandler)
	for path, handler := range apiHandlers {
		s.mux.HandleFunc(path, handler)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

// handleSNSRequest handles SNS API requests
func (s *Server) handleSNSRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	action := r.FormValue("Action")
	start := time.Now()

	s.logger.Debug("SNS request", "action", action)

	switch action {
	case "CreateTopic":
		s.handleCreateTopic(w, r, start)
	case "DeleteTopic":
		s.handleDeleteTopic(w, r, start)
	case "ListTopics":
		s.handleListTopics(w, r, start)
	case "GetTopicAttributes":
		s.handleGetTopicAttributes(w, r, start)
	case "SetTopicAttributes":
		s.handleSetTopicAttributes(w, r, start)
	case "Subscribe":
		s.handleSubscribe(w, r, start)
	case "Unsubscribe":
		s.handleUnsubscribe(w, r, start)
	case "ListSubscriptionsByTopic":
		s.handleListSubscriptionsByTopic(w, r, start)
	case "GetSubscriptionAttributes":
		s.handleGetSubscriptionAttributes(w, r, start)
	case "SetSubscriptionAttributes":
		s.handleSetSubscriptionAttributes(w, r, start)
	case "Publish":
		s.handlePublish(w, r, start)
	default:
		s.activityLogger.Log(activity.EventType("unknown_action"), "", activity.StatusFailed, map[string]interface{}{"action": action})
		http.Error(w, fmt.Sprintf("Unknown action: %s", action), http.StatusBadRequest)
	}
}

// handleCreateTopic creates a new topic
func (s *Server) handleCreateTopic(w http.ResponseWriter, r *http.Request, start time.Time) {
	name := r.FormValue("Name")
	if name == "" {
		s.activityLogger.LogError(activity.EventTypeCreateTopic, "", "", "topic name is required", nil)
		http.Error(w, "Topic name is required", http.StatusBadRequest)
		return
	}

	newTopic, err := s.topicStore.CreateTopic(name, make(map[string]string))
	if err != nil {
		s.activityLogger.LogError(activity.EventTypeCreateTopic, "", "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypeCreateTopic, newTopic.TopicArn, activity.StatusSuccess, duration, nil)

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintf(w, "<?xml version=\"1.0\"?><CreateTopicResponse xmlns=\"http://sns.amazonaws.com/doc/2010-03-31/\"><CreateTopicResult><TopicArn>%s</TopicArn></CreateTopicResult><ResponseMetadata><RequestId>%s</RequestId></ResponseMetadata></CreateTopicResponse>", newTopic.TopicArn, generateRequestId())
}

// handleDeleteTopic deletes a topic
func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	if topicArn == "" {
		s.activityLogger.LogError(activity.EventTypeDeleteTopic, "", "", "topic arn is required", nil)
		http.Error(w, "Topic ARN is required", http.StatusBadRequest)
		return
	}

	if err := s.topicStore.DeleteTopic(topicArn); err != nil {
		s.activityLogger.LogError(activity.EventTypeDeleteTopic, topicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypeDeleteTopic, topicArn, activity.StatusSuccess, duration, nil)

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintf(w, "<?xml version=\"1.0\"?><DeleteTopicResponse xmlns=\"http://sns.amazonaws.com/doc/2010-03-31/\"><ResponseMetadata><RequestId>%s</RequestId></ResponseMetadata></DeleteTopicResponse>", generateRequestId())
}

// handleListTopics lists all topics
func (s *Server) handleListTopics(w http.ResponseWriter, r *http.Request, start time.Time) {
	topics := s.topicStore.ListTopics()
	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypeListTopics, "", activity.StatusSuccess, duration, map[string]interface{}{"count": len(topics)})

	topicsXML := ""
	for _, t := range topics {
		topicsXML += fmt.Sprintf("<member><TopicArn>%s</TopicArn></member>", t.TopicArn)
	}

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintf(w, "<?xml version=\"1.0\"?><ListTopicsResponse xmlns=\"http://sns.amazonaws.com/doc/2010-03-31/\"><ListTopicsResult><Topics>%s</Topics></ListTopicsResult><ResponseMetadata><RequestId>%s</RequestId></ResponseMetadata></ListTopicsResponse>", topicsXML, generateRequestId())
}

// handleGetTopicAttributes gets topic attributes
func (s *Server) handleGetTopicAttributes(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	if topicArn == "" {
		s.activityLogger.LogError(activity.EventTypeGetAttributes, "", "", "topic arn is required", nil)
		return
	}

	_, err := s.topicStore.GetAttributes(topicArn)
	if err != nil {
		s.activityLogger.LogError(activity.EventTypeGetAttributes, topicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypeGetAttributes, topicArn, activity.StatusSuccess, duration, nil)

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprint(w, "<?xml version=\"1.0\"?><GetTopicAttributesResponse xmlns=\"http://sns.amazonaws.com/doc/2010-03-31/\"><GetTopicAttributesResult><Attributes></Attributes></GetTopicAttributesResult><ResponseMetadata></ResponseMetadata></GetTopicAttributesResponse>")
}

// handleSetTopicAttributes sets a topic attribute
func (s *Server) handleSetTopicAttributes(w http.ResponseWriter, r *http.Request, start time.Time) {
	topicArn := r.FormValue("TopicArn")
	attrName := r.FormValue("AttributeName")
	attrValue := r.FormValue("AttributeValue")

	if topicArn == "" || attrName == "" {
		s.activityLogger.LogError(activity.EventTypeSetAttributes, topicArn, "", "topic arn and attribute name are required", nil)
		http.Error(w, "Topic ARN and attribute name are required", http.StatusBadRequest)
		return
	}

	if err := s.topicStore.SetAttribute(topicArn, attrName, attrValue); err != nil {
		s.activityLogger.LogError(activity.EventTypeSetAttributes, topicArn, "", err.Error(), nil)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	duration := time.Since(start)
	s.activityLogger.LogWithDuration(activity.EventTypeSetAttributes, topicArn, activity.StatusSuccess, duration, map[string]interface{}{
		"attribute_name":  attrName,
		"attribute_value": attrValue,
	})

	w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintf(w, "<?xml version=\"1.0\"?><SetTopicAttributesResponse xmlns=\"http://sns.amazonaws.com/doc/2010-03-31/\"><ResponseMetadata><RequestId>%s</RequestId></ResponseMetadata></SetTopicAttributesResponse>", generateRequestId())
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.APIPort),
		Handler:      s.mux,
		ReadTimeout:  time.Duration(s.config.Server.TimeoutSec) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.TimeoutSec) * time.Second,
	}

	s.logger.Info("SNS API server starting", "address", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// GetActivityLogger returns the activity logger
func (s *Server) GetActivityLogger() *activity.Logger {
	return s.activityLogger
}

// GetTopicStore returns the topic store
func (s *Server) GetTopicStore() *topic.Store {
	return s.topicStore
}

// GetSubscriptionStore returns the subscription store
func (s *Server) GetSubscriptionStore() *subscription.Store {
	return s.subscriptionStore
}

// generateRequestId generates a request ID
func generateRequestId() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

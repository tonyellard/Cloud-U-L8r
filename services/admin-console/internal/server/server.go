package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	logger *slog.Logger
	client *http.Client
}

type QueueAdminResponse struct {
	Queues []QueueAdmin `json:"queues"`
}

type QueueAdmin struct {
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	VisibleCount    int               `json:"visible_count"`
	NotVisibleCount int               `json:"not_visible_count"`
	DelayedCount    int               `json:"delayed_count"`
	FifoQueue       bool              `json:"fifo_queue"`
	RedrivePolicy   *json.RawMessage  `json:"redrive_policy"`
	Messages        []json.RawMessage `json:"messages"`
}

type QueueView struct {
	QueueName       string            `json:"queue_name"`
	QueueURL        string            `json:"queue_url"`
	VisibleCount    int               `json:"visible_count"`
	NotVisibleCount int               `json:"not_visible_count"`
	DelayedCount    int               `json:"delayed_count"`
	IsFIFO          bool              `json:"is_fifo"`
	HasDLQ          bool              `json:"has_dlq"`
	IsDLQ           bool              `json:"is_dlq"`
	Messages        []json.RawMessage `json:"messages"`
	QueueID         string            `json:"queue_id"`
}

type QueueViewResponse struct {
	Service string      `json:"service"`
	Queues  []QueueView `json:"queues"`
}

type QueuePeekResponse struct {
	QueueID   string            `json:"queue_id"`
	QueueName string            `json:"queue_name"`
	QueueURL  string            `json:"queue_url"`
	Messages  []json.RawMessage `json:"messages"`
}

type CreateQueueRequest struct {
	QueueName                 string `json:"queue_name"`
	IsFIFO                    bool   `json:"is_fifo"`
	ContentBasedDeduplication bool   `json:"content_based_deduplication"`
	CreateDLQ                 bool   `json:"create_dlq"`
	DLQMaxReceiveCount        int    `json:"dlq_max_receive_count"`
	VisibilityTimeout         int    `json:"visibility_timeout"`
	MessageRetentionPeriod    int    `json:"message_retention_period"`
	MaximumMessageSize        int    `json:"maximum_message_size"`
	DelaySeconds              int    `json:"delay_seconds"`
	ReceiveMessageWaitTime    int    `json:"receive_message_wait_time_seconds"`
}

type SendMessageRequest struct {
	QueueURL               string `json:"queue_url"`
	MessageBody            string `json:"message_body"`
	MessageGroupID         string `json:"message_group_id"`
	MessageDeduplicationID string `json:"message_deduplication_id"`
	DelaySeconds           int    `json:"delay_seconds"`
}

type QueueActionRequest struct {
	QueueURL string `json:"queue_url"`
}

type UpdateQueueAttributesRequest struct {
	QueueURL                      string `json:"queue_url"`
	VisibilityTimeout             int    `json:"visibility_timeout"`
	MessageRetentionPeriod        int    `json:"message_retention_period"`
	MaximumMessageSize            int    `json:"maximum_message_size"`
	DelaySeconds                  int    `json:"delay_seconds"`
	ReceiveMessageWaitTimeSeconds int    `json:"receive_message_wait_time_seconds"`
}

type QueueRedriveRequest struct {
	QueueURL                 string `json:"queue_url"`
	DestinationQueueURL      string `json:"destination_queue_url"`
	MaxMessagesPerSecondHint int    `json:"max_messages_per_second"`
}

type QueueAttributesResponse struct {
	QueueID     string            `json:"queue_id"`
	QueueName   string            `json:"queue_name"`
	QueueURL    string            `json:"queue_url"`
	Attributes  map[string]string `json:"attributes"`
	FetchedAt   time.Time         `json:"fetched_at"`
	IsFIFO      bool              `json:"is_fifo"`
	HasDLQ      bool              `json:"has_dlq"`
	IsDLQ       bool              `json:"is_dlq"`
	RedriveFrom string            `json:"redrive_from,omitempty"`
}

type TopicView struct {
	TopicARN          string    `json:"topic_arn"`
	DisplayName       string    `json:"display_name"`
	FIFOTopic         bool      `json:"fifo_topic"`
	SubscriptionCount int       `json:"subscription_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type SubscriptionView struct {
	SubscriptionARN string    `json:"subscription_arn"`
	TopicARN        string    `json:"topic_arn"`
	Protocol        string    `json:"protocol"`
	Endpoint        string    `json:"endpoint"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type PubSubStateResponse struct {
	Service       string             `json:"service"`
	Topics        []TopicView        `json:"topics"`
	Subscriptions []SubscriptionView `json:"subscriptions"`
	Stats         struct {
		Topics        int `json:"topics"`
		Subscriptions int `json:"subscriptions"`
	} `json:"stats"`
}

type CreateTopicRequest struct {
	Name string `json:"name"`
}

type DeleteTopicRequest struct {
	TopicARN string `json:"topic_arn"`
}

type CreateSubscriptionRequest struct {
	TopicARN    string `json:"topic_arn"`
	Protocol    string `json:"protocol"`
	Endpoint    string `json:"endpoint"`
	AutoConfirm bool   `json:"auto_confirm"`
}

type DeleteSubscriptionRequest struct {
	SubscriptionARN string `json:"subscription_arn"`
}

type PublishTopicMessageRequest struct {
	TopicARN string `json:"topic_arn"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
}

type DashboardSummary struct {
	Services  []DashboardService `json:"services"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type DashboardService struct {
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Stats  []DashboardStat `json:"stats"`
}

type DashboardStat struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

func NewRouter(logger *slog.Logger) http.Handler {
	srv := &Server{
		logger: logger,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	r := chi.NewRouter()
	r.Get("/health", srv.handleHealth)
	r.Get("/api/dashboard/summary", srv.handleDashboardSummary)
	r.Get("/api/services/ess-queue-ess/queues", srv.handleQueueList)
	r.Get("/api/services/ess-queue-ess/queues/{queueID}/messages/peek", srv.handleQueuePeek)
	r.Get("/api/services/ess-queue-ess/queues/{queueID}/attributes", srv.handleQueueAttributes)
	r.Get("/api/services/ess-enn-ess/state", srv.handlePubSubState)
	r.Get("/api/services/{service}/config/export", srv.handleServiceConfigExport)
	r.Post("/api/services/ess-queue-ess/actions/create-queue", srv.handleCreateQueue)
	r.Post("/api/services/ess-queue-ess/actions/send-message", srv.handleSendMessage)
	r.Post("/api/services/ess-queue-ess/actions/update-attributes", srv.handleUpdateQueueAttributes)
	r.Post("/api/services/ess-queue-ess/actions/purge-queue", srv.handlePurgeQueue)
	r.Post("/api/services/ess-queue-ess/actions/delete-queue", srv.handleDeleteQueue)
	r.Post("/api/services/ess-queue-ess/actions/start-redrive", srv.handleStartRedrive)
	r.Post("/api/services/ess-enn-ess/actions/create-topic", srv.handleCreateTopic)
	r.Post("/api/services/ess-enn-ess/actions/delete-topic", srv.handleDeleteTopic)
	r.Post("/api/services/ess-enn-ess/actions/create-subscription", srv.handleCreateSubscription)
	r.Post("/api/services/ess-enn-ess/actions/delete-subscription", srv.handleDeleteSubscription)
	r.Post("/api/services/ess-enn-ess/actions/publish", srv.handlePublishTopicMessage)
	r.Get("/api/events", srv.handleEvents)

	fs := http.FileServer(http.Dir("./web"))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/index.html")
	})
	r.Handle("/*", fs)

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "admin-console"})
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.buildDashboardSummary())
}

func (s *Server) handleServiceConfigExport(w http.ResponseWriter, r *http.Request) {
	service := strings.TrimSpace(chi.URLParam(r, "service"))

	var upstreamURL string
	var fallbackFilename string
	switch service {
	case "ess-queue-ess":
		upstreamURL = "http://ess-queue-ess:9320/admin/api/config/export"
		fallbackFilename = "ess-queue-ess.config.yaml"
	case "ess-enn-ess":
		upstreamURL = "http://ess-enn-ess:9330/api/export"
		fallbackFilename = "sns-export.yaml"
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported service"))
		return
	}

	resp, err := s.client.Get(upstreamURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("failed to fetch export: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		writeError(w, http.StatusBadGateway, fmt.Errorf("export failed for %s (%d): %s", service, resp.StatusCode, message))
		return
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentDisposition := strings.TrimSpace(resp.Header.Get("Content-Disposition"))
	if contentDisposition == "" {
		contentDisposition = fmt.Sprintf("attachment; filename=%s", fallbackFilename)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition)
	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Error("failed to proxy export response", "service", service, "error", err)
	}
}

func (s *Server) handleQueueList(w http.ResponseWriter, _ *http.Request) {
	queues, err := s.fetchQueues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, QueueViewResponse{Service: "ess-queue-ess", Queues: queues})
}

func (s *Server) handlePubSubState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.fetchPubSubState()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	var req CreateTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/topics", map[string]any{
		"name": strings.TrimSpace(req.Name),
	}, nil); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "topic_name": strings.TrimSpace(req.Name)})
}

func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	var req DeleteTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/topics/delete", map[string]any{
		"topic_arn": strings.TrimSpace(req.TopicARN),
	}, nil); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("protocol is required"))
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("endpoint is required"))
		return
	}

	if protocol != "http" && protocol != "ess-queue-ess" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("protocol must be http or ess-queue-ess"))
		return
	}

	upstreamProtocol := protocol
	if protocol == "ess-queue-ess" {
		upstreamProtocol = "sqs"
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/subscriptions", map[string]any{
		"topic_arn":    strings.TrimSpace(req.TopicARN),
		"protocol":     upstreamProtocol,
		"endpoint":     strings.TrimSpace(req.Endpoint),
		"auto_confirm": req.AutoConfirm,
	}, nil); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	var req DeleteSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SubscriptionARN) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("subscription_arn is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/subscriptions/delete", map[string]any{
		"subscription_arn": strings.TrimSpace(req.SubscriptionARN),
	}, nil); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePublishTopicMessage(w http.ResponseWriter, r *http.Request) {
	var req PublishTopicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("message is required"))
		return
	}

	form := url.Values{}
	form.Set("Action", "Publish")
	form.Set("TopicArn", strings.TrimSpace(req.TopicARN))
	form.Set("Message", req.Message)
	if strings.TrimSpace(req.Subject) != "" {
		form.Set("Subject", strings.TrimSpace(req.Subject))
	}

	if err := s.callSNSAction(form); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleQueuePeek(w http.ResponseWriter, r *http.Request) {
	queueID := normalizeQueueIDParam(chi.URLParam(r, "queueID"))
	if strings.TrimSpace(queueID) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_id is required"))
		return
	}

	limit := 10
	if limitParam := r.URL.Query().Get("limit"); strings.TrimSpace(limitParam) != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil || parsedLimit < 1 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("limit must be a positive integer"))
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
	}

	queues, err := s.fetchQueues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	selected := findQueueByID(queues, queueID)
	if selected == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("queue not found"))
		return
	}

	messages := selected.Messages
	if len(messages) > limit {
		messages = messages[:limit]
	}

	writeJSON(w, http.StatusOK, QueuePeekResponse{
		QueueID:   selected.QueueID,
		QueueName: selected.QueueName,
		QueueURL:  selected.QueueURL,
		Messages:  messages,
	})
}

func (s *Server) handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	var req CreateQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueName) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_name is required"))
		return
	}

	queueName := strings.TrimSpace(req.QueueName)
	if req.IsFIFO && !strings.HasSuffix(queueName, ".fifo") {
		queueName += ".fifo"
	}

	visibilityTimeout := req.VisibilityTimeout
	if visibilityTimeout == 0 {
		visibilityTimeout = 30
	}
	messageRetention := req.MessageRetentionPeriod
	if messageRetention == 0 {
		messageRetention = 345600
	}
	maximumMessageSize := req.MaximumMessageSize
	if maximumMessageSize == 0 {
		maximumMessageSize = 262144
	}
	delaySeconds := req.DelaySeconds
	receiveWait := req.ReceiveMessageWaitTime

	if visibilityTimeout < 0 || messageRetention < 0 || maximumMessageSize <= 0 || delaySeconds < 0 || receiveWait < 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid create queue attribute values"))
		return
	}

	dlqMaxReceiveCount := req.DLQMaxReceiveCount
	if dlqMaxReceiveCount == 0 {
		dlqMaxReceiveCount = 3
	}
	if req.CreateDLQ && dlqMaxReceiveCount <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("dlq_max_receive_count must be greater than zero"))
		return
	}

	if req.CreateDLQ {
		dlqName := deriveDLQName(queueName)
		dlqForm := url.Values{}
		dlqForm.Set("Action", "CreateQueue")
		dlqForm.Set("QueueName", dlqName)
		dlqForm.Set("Attribute.1.Name", "VisibilityTimeout")
		dlqForm.Set("Attribute.1.Value", strconv.Itoa(visibilityTimeout))
		dlqForm.Set("Attribute.2.Name", "MessageRetentionPeriod")
		dlqForm.Set("Attribute.2.Value", strconv.Itoa(messageRetention))
		dlqForm.Set("Attribute.3.Name", "MaximumMessageSize")
		dlqForm.Set("Attribute.3.Value", strconv.Itoa(maximumMessageSize))
		dlqForm.Set("Attribute.4.Name", "DelaySeconds")
		dlqForm.Set("Attribute.4.Value", "0")
		dlqForm.Set("Attribute.5.Name", "ReceiveMessageWaitTimeSeconds")
		dlqForm.Set("Attribute.5.Value", strconv.Itoa(receiveWait))

		if req.IsFIFO {
			dlqForm.Set("Attribute.6.Name", "FifoQueue")
			dlqForm.Set("Attribute.6.Value", "true")
			if req.ContentBasedDeduplication {
				dlqForm.Set("Attribute.7.Name", "ContentBasedDeduplication")
				dlqForm.Set("Attribute.7.Value", "true")
			}
		}

		if err := s.callSQSAction(dlqForm); err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("failed to create DLQ: %w", err))
			return
		}
	}

	form := url.Values{}
	form.Set("Action", "CreateQueue")
	form.Set("QueueName", queueName)
	form.Set("Attribute.1.Name", "VisibilityTimeout")
	form.Set("Attribute.1.Value", strconv.Itoa(visibilityTimeout))
	form.Set("Attribute.2.Name", "MessageRetentionPeriod")
	form.Set("Attribute.2.Value", strconv.Itoa(messageRetention))
	form.Set("Attribute.3.Name", "MaximumMessageSize")
	form.Set("Attribute.3.Value", strconv.Itoa(maximumMessageSize))
	form.Set("Attribute.4.Name", "DelaySeconds")
	form.Set("Attribute.4.Value", strconv.Itoa(delaySeconds))
	form.Set("Attribute.5.Name", "ReceiveMessageWaitTimeSeconds")
	form.Set("Attribute.5.Value", strconv.Itoa(receiveWait))

	if req.IsFIFO {
		form.Set("Attribute.6.Name", "FifoQueue")
		form.Set("Attribute.6.Value", "true")
		if req.ContentBasedDeduplication {
			form.Set("Attribute.7.Name", "ContentBasedDeduplication")
			form.Set("Attribute.7.Value", "true")
		}
	}

	if req.CreateDLQ {
		dlqName := deriveDLQName(queueName)
		redrivePolicy := fmt.Sprintf(`{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:000000000000:%s","maxReceiveCount":%d}`, dlqName, dlqMaxReceiveCount)
		form.Set("Attribute.8.Name", "RedrivePolicy")
		form.Set("Attribute.8.Value", redrivePolicy)
	}

	if err := s.callSQSAction(form); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	response := map[string]any{"ok": true, "queue_name": queueName}
	if req.CreateDLQ {
		response["dlq_name"] = deriveDLQName(queueName)
		response["dlq_max_receive_count"] = dlqMaxReceiveCount
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}
	if strings.TrimSpace(req.MessageBody) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("message_body is required"))
		return
	}
	if strings.HasSuffix(req.QueueURL, ".fifo") && strings.TrimSpace(req.MessageGroupID) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("message_group_id is required for FIFO queues"))
		return
	}

	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", strings.TrimSpace(req.QueueURL))
	form.Set("MessageBody", req.MessageBody)
	if req.DelaySeconds > 0 {
		form.Set("DelaySeconds", fmt.Sprintf("%d", req.DelaySeconds))
	}
	if strings.TrimSpace(req.MessageGroupID) != "" {
		form.Set("MessageGroupId", strings.TrimSpace(req.MessageGroupID))
	}
	if strings.TrimSpace(req.MessageDeduplicationID) != "" {
		form.Set("MessageDeduplicationId", strings.TrimSpace(req.MessageDeduplicationID))
	}

	if err := s.callSQSAction(form); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePurgeQueue(w http.ResponseWriter, r *http.Request) {
	var req QueueActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	form := url.Values{}
	form.Set("Action", "PurgeQueue")
	form.Set("QueueUrl", strings.TrimSpace(req.QueueURL))

	if err := s.callSQSAction(form); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdateQueueAttributes(w http.ResponseWriter, r *http.Request) {
	var req UpdateQueueAttributesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	if strings.TrimSpace(req.QueueURL) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}
	if req.VisibilityTimeout < 0 || req.MessageRetentionPeriod < 0 || req.MaximumMessageSize <= 0 || req.DelaySeconds < 0 || req.ReceiveMessageWaitTimeSeconds < 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid attribute values"))
		return
	}

	payload := map[string]any{
		"QueueUrl": strings.TrimSpace(req.QueueURL),
		"Attributes": map[string]string{
			"VisibilityTimeout":             strconv.Itoa(req.VisibilityTimeout),
			"MessageRetentionPeriod":        strconv.Itoa(req.MessageRetentionPeriod),
			"MaximumMessageSize":            strconv.Itoa(req.MaximumMessageSize),
			"DelaySeconds":                  strconv.Itoa(req.DelaySeconds),
			"ReceiveMessageWaitTimeSeconds": strconv.Itoa(req.ReceiveMessageWaitTimeSeconds),
		},
	}

	if err := s.callSQSJSONAction("SetQueueAttributes", payload, nil); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	var req QueueActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	form := url.Values{}
	form.Set("Action", "DeleteQueue")
	form.Set("QueueUrl", strings.TrimSpace(req.QueueURL))

	if err := s.callSQSAction(form); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleQueueAttributes(w http.ResponseWriter, r *http.Request) {
	queueID := normalizeQueueIDParam(chi.URLParam(r, "queueID"))
	if strings.TrimSpace(queueID) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_id is required"))
		return
	}

	queues, err := s.fetchQueues()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	selected := findQueueByID(queues, queueID)
	if selected == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("queue not found"))
		return
	}

	var sqsResp struct {
		Attributes map[string]string `json:"Attributes"`
	}
	if err := s.callSQSJSONAction("GetQueueAttributes", map[string]any{
		"QueueUrl":       selected.QueueURL,
		"AttributeNames": []string{"All"},
	}, &sqsResp); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, QueueAttributesResponse{
		QueueID:    selected.QueueID,
		QueueName:  selected.QueueName,
		QueueURL:   selected.QueueURL,
		Attributes: sqsResp.Attributes,
		FetchedAt:  time.Now().UTC(),
		IsFIFO:     selected.IsFIFO,
		HasDLQ:     selected.HasDLQ,
		IsDLQ:      strings.Contains(strings.ToLower(selected.QueueName), "-dlq"),
	})
}

func (s *Server) handleStartRedrive(w http.ResponseWriter, r *http.Request) {
	var req QueueRedriveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	sourceArn, err := queueURLToARN(req.QueueURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	payload := map[string]any{"SourceArn": sourceArn}
	if strings.TrimSpace(req.DestinationQueueURL) != "" {
		destinationArn, destinationErr := queueURLToARN(req.DestinationQueueURL)
		if destinationErr != nil {
			writeError(w, http.StatusBadRequest, destinationErr)
			return
		}
		payload["DestinationArn"] = destinationArn
	}
	if req.MaxMessagesPerSecondHint > 0 {
		payload["MaxNumberOfMessagesPerSecond"] = req.MaxMessagesPerSecondHint
	}

	var sqsResp struct {
		TaskHandle string `json:"TaskHandle"`
	}
	if err := s.callSQSJSONAction("StartMessageMoveTask", payload, &sqsResp); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"task_handle": sqsResp.TaskHandle,
		"source_arn":  sourceArn,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "dashboard"
	}
	if view != "dashboard" && view != "ess-queue-ess" && view != "ess-enn-ess" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid view"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	ctx := r.Context()

	var lastPayload []byte
	s.sendEventForView(w, flusher, view, &lastPayload)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendEventForView(w, flusher, view, &lastPayload)
		}
	}
}

func (s *Server) sendEventForView(w http.ResponseWriter, flusher http.Flusher, view string, lastPayload *[]byte) {
	payload, err := s.payloadForView(view)
	if err != nil {
		s.logger.Error("event payload failure", "view", view, "error", err)
		w.Write([]byte(": keep-alive\n\n"))
		flusher.Flush()
		return
	}
	if bytes.Equal(payload, *lastPayload) {
		w.Write([]byte(": keep-alive\n\n"))
		flusher.Flush()
		return
	}

	w.Write([]byte("event: state\n"))
	w.Write([]byte("data: "))
	w.Write(payload)
	w.Write([]byte("\n\n"))
	flusher.Flush()
	*lastPayload = payload
}

func (s *Server) payloadForView(view string) ([]byte, error) {
	if view == "ess-queue-ess" {
		queues, err := s.fetchQueues()
		if err != nil {
			return nil, err
		}
		return json.Marshal(QueueViewResponse{Service: "ess-queue-ess", Queues: queues})
	}
	if view == "ess-enn-ess" {
		state, err := s.fetchPubSubState()
		if err != nil {
			return nil, err
		}
		return json.Marshal(state)
	}

	return json.Marshal(s.buildDashboardSummary())
}

func (s *Server) buildDashboardSummary() DashboardSummary {
	summary := DashboardSummary{UpdatedAt: time.Now().UTC()}
	services := make([]DashboardService, 0, 2)

	queueService := DashboardService{
		Name:   "ess-queue-ess",
		Status: s.checkService("http://ess-queue-ess:9320/health"),
		Stats: []DashboardStat{
			{Label: "Queues", Value: 0},
			{Label: "Visible", Value: 0},
			{Label: "In Flight", Value: 0},
			{Label: "Delayed", Value: 0},
		},
	}
	if queueService.Status == "online" {
		if queues, err := s.fetchQueues(); err == nil {
			queueService.Stats[0].Value = len(queues)
			for _, queue := range queues {
				queueService.Stats[1].Value += queue.VisibleCount
				queueService.Stats[2].Value += queue.NotVisibleCount
				queueService.Stats[3].Value += queue.DelayedCount
			}
		} else {
			s.logger.Warn("failed to fetch ess-queue-ess dashboard stats", "error", err)
		}
	}
	services = append(services, queueService)

	pubsubService := DashboardService{
		Name:   "ess-enn-ess",
		Status: s.checkService("http://ess-enn-ess:9330/health"),
		Stats: []DashboardStat{
			{Label: "Topics", Value: 0},
			{Label: "Subscriptions", Value: 0},
		},
	}
	if pubsubService.Status == "online" {
		if topicsTotal, subscriptionsTotal, err := s.fetchSNSAdminStats(); err == nil {
			pubsubService.Stats[0].Value = topicsTotal
			pubsubService.Stats[1].Value = subscriptionsTotal
		} else {
			s.logger.Warn("failed to fetch ess-enn-ess dashboard stats", "error", err)
		}
	}
	services = append(services, pubsubService)

	summary.Services = services
	return summary
}

func (s *Server) fetchSNSAdminStats() (int, int, error) {
	resp, err := s.client.Get("http://ess-enn-ess:9330/api/stats")
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("sns admin status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var stats struct {
		Topics struct {
			Total int `json:"total"`
		} `json:"topics"`
		Subscriptions struct {
			Total int `json:"total"`
		} `json:"subscriptions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return 0, 0, err
	}

	return stats.Topics.Total, stats.Subscriptions.Total, nil
}

func (s *Server) fetchPubSubState() (PubSubStateResponse, error) {
	topics, err := s.fetchTopics()
	if err != nil {
		return PubSubStateResponse{}, err
	}

	subscriptions, err := s.fetchSubscriptions()
	if err != nil {
		return PubSubStateResponse{}, err
	}

	state := PubSubStateResponse{
		Service:       "ess-enn-ess",
		Topics:        topics,
		Subscriptions: subscriptions,
	}
	state.Stats.Topics = len(topics)
	state.Stats.Subscriptions = len(subscriptions)
	return state, nil
}

func (s *Server) fetchTopics() ([]TopicView, error) {
	var topics []TopicView
	if err := s.callSNSAdminJSON(http.MethodGet, "/api/topics", nil, &topics); err != nil {
		return nil, err
	}
	return topics, nil
}

func (s *Server) fetchSubscriptions() ([]SubscriptionView, error) {
	var subscriptions []SubscriptionView
	if err := s.callSNSAdminJSON(http.MethodGet, "/api/subscriptions", nil, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

func (s *Server) fetchQueues() ([]QueueView, error) {
	resp, err := s.client.Get("http://ess-queue-ess:9320/admin/api/queues")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("queue admin status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var adminResp QueueAdminResponse
	if err := json.NewDecoder(resp.Body).Decode(&adminResp); err != nil {
		return nil, err
	}

	queues := make([]QueueView, 0, len(adminResp.Queues))
	for _, item := range adminResp.Queues {
		decodedURL, _ := url.QueryUnescape(item.URL)
		if decodedURL == "" {
			decodedURL = item.URL
		}
		queues = append(queues, QueueView{
			QueueName:       item.Name,
			QueueURL:        decodedURL,
			VisibleCount:    item.VisibleCount,
			NotVisibleCount: item.NotVisibleCount,
			DelayedCount:    item.DelayedCount,
			IsFIFO:          item.FifoQueue,
			HasDLQ:          item.RedrivePolicy != nil,
			IsDLQ:           strings.HasSuffix(item.Name, "-dlq") || strings.HasSuffix(item.Name, "-dlq.fifo"),
			Messages:        item.Messages,
			QueueID:         base64.StdEncoding.EncodeToString([]byte(decodedURL)),
		})
	}
	return queues, nil
}

func (s *Server) callSQSAction(form url.Values) error {
	resp, err := s.client.PostForm("http://ess-queue-ess:9320/", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("sqs action %q failed (%d): %s", form.Get("Action"), resp.StatusCode, message)
	}

	return nil
}

func (s *Server) callSNSAction(form url.Values) error {
	resp, err := s.client.PostForm("http://ess-enn-ess:9330/", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("sns action %q failed (%d): %s", form.Get("Action"), resp.StatusCode, message)
	}

	return nil
}

func (s *Server) callSNSAdminJSON(method string, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, "http://ess-enn-ess:9330"+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("sns admin request %s %s failed (%d): %s", method, path, resp.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (s *Server) callSQSJSONAction(action string, payload map[string]any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "http://ess-queue-ess:9320/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-amz-json-1.0")
	request.Header.Set("X-Amz-Target", "AmazonSQS."+action)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("sqs action %q failed (%d): %s", action, response.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func queueURLToARN(queueURL string) (string, error) {
	decoded, _ := url.QueryUnescape(strings.TrimSpace(queueURL))
	if decoded == "" {
		return "", fmt.Errorf("queue_url is required")
	}
	queueName := strings.TrimPrefix(decoded, "/")
	if queueName == "" {
		return "", fmt.Errorf("invalid queue_url")
	}
	if strings.Contains(queueName, "/") {
		parts := strings.Split(queueName, "/")
		queueName = parts[len(parts)-1]
	}
	return "arn:aws:sqs:us-east-1:000000000000:" + queueName, nil
}

func normalizeQueueIDParam(queueID string) string {
	trimmed := strings.TrimSpace(queueID)
	if trimmed == "" {
		return ""
	}
	if unescaped, err := url.PathUnescape(trimmed); err == nil && strings.TrimSpace(unescaped) != "" {
		return strings.TrimSpace(unescaped)
	}
	if unescaped, err := url.QueryUnescape(trimmed); err == nil && strings.TrimSpace(unescaped) != "" {
		return strings.TrimSpace(unescaped)
	}
	return trimmed
}

func findQueueByID(queues []QueueView, queueID string) *QueueView {
	for index := range queues {
		if queues[index].QueueID == queueID {
			return &queues[index]
		}
	}
	return nil
}

func deriveDLQName(queueName string) string {
	trimmed := strings.TrimSpace(queueName)
	if strings.HasSuffix(trimmed, ".fifo") {
		base := strings.TrimSuffix(trimmed, ".fifo")
		if strings.HasSuffix(base, "-dlq") {
			return base + ".fifo"
		}
		return base + "-dlq.fifo"
	}
	if strings.HasSuffix(trimmed, "-dlq") {
		return trimmed
	}
	return trimmed + "-dlq"
}

func (s *Server) checkService(healthURL string) string {
	resp, err := s.client.Get(healthURL)
	if err != nil {
		return "offline"
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "offline"
	}
	return "online"
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

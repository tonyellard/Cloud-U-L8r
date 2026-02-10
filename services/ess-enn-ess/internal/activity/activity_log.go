package activity

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// EventType represents the type of activity event
type EventType string

const (
	EventTypeCreateTopic   EventType = "create_topic"
	EventTypeDeleteTopic   EventType = "delete_topic"
	EventTypeListTopics    EventType = "list_topics"
	EventTypeSubscribe     EventType = "subscribe"
	EventTypeUnsubscribe   EventType = "unsubscribe"
	EventTypePublish       EventType = "publish"
	EventTypeDelivery      EventType = "delivery"
	EventTypeDeliveryError EventType = "delivery_error"
	EventTypeHTTPError     EventType = "http_error"
	EventTypeSQSError      EventType = "sqs_error"
	EventTypeQueueNotFound EventType = "queue_not_found"
	EventTypeGetAttributes EventType = "get_attributes"
	EventTypeSetAttributes EventType = "set_attributes"
)

// Status represents the status of an operation
type Status string

const (
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusRetrying Status = "retrying"
	StatusPending  Status = "pending"
)

// Entry represents a single activity log entry
type Entry struct {
	Id              string                 `json:"id"`
	Timestamp       time.Time              `json:"timestamp"`
	EventType       EventType              `json:"event_type"`
	TopicArn        string                 `json:"topic_arn,omitempty"`
	MessageId       string                 `json:"message_id,omitempty"`
	SubscriptionArn string                 `json:"subscription_arn,omitempty"`
	Protocol        string                 `json:"protocol,omitempty"`
	Endpoint        string                 `json:"endpoint,omitempty"`
	Status          Status                 `json:"status"`
	Details         map[string]interface{} `json:"details,omitempty"`
	Duration        time.Duration          `json:"duration_ms"`
	Error           string                 `json:"error,omitempty"`
}

// Logger manages the activity log
type Logger struct {
	mu          sync.RWMutex
	entries     []*Entry
	maxSize     int
	logger      *slog.Logger
	subscribers []chan *Entry
}

// NewLogger creates a new activity logger
func NewLogger(maxSize int, slogger *slog.Logger) *Logger {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &Logger{
		entries:     make([]*Entry, 0, maxSize),
		maxSize:     maxSize,
		logger:      slogger,
		subscribers: make([]chan *Entry, 0),
	}
}

// Log adds a new activity entry
func (l *Logger) Log(eventType EventType, topicArn string, status Status, details map[string]interface{}) *Entry {
	entry := &Entry{
		Id:        generateId(),
		Timestamp: time.Now(),
		EventType: eventType,
		TopicArn:  topicArn,
		Status:    status,
		Details:   details,
	}
	return l.logEntry(entry)
}

// LogWithDuration adds an activity entry with operation duration
func (l *Logger) LogWithDuration(eventType EventType, topicArn string, status Status, duration time.Duration, details map[string]interface{}) *Entry {
	entry := &Entry{
		Id:        generateId(),
		Timestamp: time.Now(),
		EventType: eventType,
		TopicArn:  topicArn,
		Status:    status,
		Duration:  duration,
		Details:   details,
	}
	return l.logEntry(entry)
}

// LogDelivery logs a message delivery attempt
func (l *Logger) LogDelivery(topicArn, messageId, subscriptionArn, protocol, endpoint string, status Status, duration time.Duration, errMsg string) *Entry {
	entry := &Entry{
		Id:              generateId(),
		Timestamp:       time.Now(),
		EventType:       EventTypeDelivery,
		TopicArn:        topicArn,
		MessageId:       messageId,
		SubscriptionArn: subscriptionArn,
		Protocol:        protocol,
		Endpoint:        endpoint,
		Status:          status,
		Duration:        duration,
		Error:           errMsg,
	}
	return l.logEntry(entry)
}

// LogError logs an error event
func (l *Logger) LogError(eventType EventType, topicArn, messageId string, errMsg string, details map[string]interface{}) *Entry {
	if details == nil {
		details = make(map[string]interface{})
	}
	entry := &Entry{
		Id:        generateId(),
		Timestamp: time.Now(),
		EventType: eventType,
		TopicArn:  topicArn,
		MessageId: messageId,
		Status:    StatusFailed,
		Error:     errMsg,
		Details:   details,
	}
	return l.logEntry(entry)
}

// logEntry adds an entry to the log and notifies subscribers
func (l *Logger) logEntry(entry *Entry) *Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Keep only the last maxSize entries
	if len(l.entries) >= l.maxSize {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)

	// Log to slog
	l.logger.Info("activity",
		"event", entry.EventType,
		"topic", entry.TopicArn,
		"status", entry.Status,
		"duration_ms", entry.Duration.Milliseconds(),
	)

	// Notify subscribers (non-blocking)
	go l.notifySubscribers(entry)

	return entry
}

// GetEntries returns all log entries
func (l *Logger) GetEntries() []*Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := make([]*Entry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// GetEntriesByTopic returns log entries for a specific topic
func (l *Logger) GetEntriesByTopic(topicArn string) []*Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []*Entry
	for _, entry := range l.entries {
		if entry.TopicArn == topicArn {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// GetEntriesByEventType returns log entries of a specific event type
func (l *Logger) GetEntriesByEventType(eventType EventType) []*Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []*Entry
	for _, entry := range l.entries {
		if entry.EventType == eventType {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// GetEntriesByStatus returns log entries with a specific status
func (l *Logger) GetEntriesByStatus(status Status) []*Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []*Entry
	for _, entry := range l.entries {
		if entry.Status == status {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// Subscribe returns a channel that receives new log entries
func (l *Logger) Subscribe() <-chan *Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	ch := make(chan *Entry, 10)
	l.subscribers = append(l.subscribers, ch)
	return ch
}

// notifySubscribers sends entry to all subscribers
func (l *Logger) notifySubscribers(entry *Entry) {
	l.mu.RLock()
	subscribers := make([]chan *Entry, len(l.subscribers))
	copy(subscribers, l.subscribers)
	l.mu.RUnlock()

	for _, ch := range subscribers {
		select {
		case ch <- entry:
		default:
			// Channel full, skip to avoid blocking
		}
	}
}

// generateId creates a unique ID for a log entry
func generateId() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// Clear removes all entries (useful for testing)
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]*Entry, 0, l.maxSize)
}

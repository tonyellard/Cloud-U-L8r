package message

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message represents an SNS message to be published
type Message struct {
	MessageId         string                 `json:"MessageId"`
	TopicArn          string                 `json:"TopicArn"`
	Subject           string                 `json:"Subject,omitempty"`
	Message           string                 `json:"Message"`
	Timestamp         time.Time              `json:"Timestamp"`
	MessageAttributes map[string]interface{} `json:"MessageAttributes,omitempty"`
}

// NewMessage creates a new message
func NewMessage(topicArn, subject, message string, attributes map[string]interface{}) *Message {
	return &Message{
		MessageId:         generateMessageId(),
		TopicArn:          topicArn,
		Subject:           subject,
		Message:           message,
		Timestamp:         time.Now(),
		MessageAttributes: attributes,
	}
}

// ToJSON converts the message to JSON format
func (m *Message) ToJSON() (string, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToSNSJSON converts to SNS notification JSON format
func (m *Message) ToSNSJSON() (string, error) {
	notification := map[string]interface{}{
		"Type":      "Notification",
		"MessageId": m.MessageId,
		"TopicArn":  m.TopicArn,
		"Subject":   m.Subject,
		"Message":   m.Message,
		"Timestamp": m.Timestamp.Format(time.RFC3339),
	}

	if len(m.MessageAttributes) > 0 {
		notification["MessageAttributes"] = m.MessageAttributes
	}

	bytes, err := json.Marshal(notification)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// generateMessageId generates a unique message ID
func generateMessageId() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

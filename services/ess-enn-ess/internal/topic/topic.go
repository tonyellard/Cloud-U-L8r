package topic

import (
	"fmt"
	"sync"
	"time"
)

// Topic represents an SNS topic
type Topic struct {
	TopicArn          string            `json:"topic_arn" yaml:"topicarn"`
	DisplayName       string            `json:"display_name" yaml:"displayname"`
	FifoTopic         bool              `json:"fifo_topic" yaml:"fifotopic"`
	ContentBased      bool              `json:"content_based" yaml:"contentbased"`
	KmsMasterKeyId    string            `json:"kms_master_key_id" yaml:"kmsmasterkeyid"`
	Attributes        map[string]string `json:"attributes" yaml:"attributes"`
	CreatedAt         time.Time         `json:"created_at" yaml:"createdat"`
	SubscriptionCount int               `json:"subscription_count" yaml:"subscriptioncount"`
}

// Store represents a thread-safe topic store
type Store struct {
	mu        sync.RWMutex
	topics    map[string]*Topic
	accountId string
	region    string
}

// NewStore creates a new topic store
func NewStore(accountId, region string) *Store {
	if accountId == "" {
		accountId = "123456789012"
	}
	if region == "" {
		region = "us-east-1"
	}
	return &Store{
		topics:    make(map[string]*Topic),
		accountId: accountId,
		region:    region,
	}
}

// CreateTopic creates a new topic or returns existing one
func (s *Store) CreateTopic(name string, attributes map[string]string) (*Topic, error) {
	if name == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	arn := fmt.Sprintf("arn:aws:sns:%s:%s:%s", s.region, s.accountId, name)

	// Return existing topic if it already exists
	if topic, exists := s.topics[arn]; exists {
		return topic, nil
	}

	topic := &Topic{
		TopicArn:     arn,
		DisplayName:  name,
		Attributes:   attributes,
		CreatedAt:    time.Now(),
		FifoTopic:    attributes["FifoTopic"] == "true",
		ContentBased: attributes["ContentBasedDeduplication"] == "true",
	}

	s.topics[arn] = topic
	return topic, nil
}

// GetTopic retrieves a topic by ARN
func (s *Store) GetTopic(arn string) (*Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topic, exists := s.topics[arn]
	if !exists {
		return nil, fmt.Errorf("topic not found: %s", arn)
	}
	return topic, nil
}

// DeleteTopic deletes a topic
func (s *Store) DeleteTopic(arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.topics[arn]; !exists {
		return fmt.Errorf("topic not found: %s", arn)
	}

	delete(s.topics, arn)
	return nil
}

// ListTopics returns all topics
func (s *Store) ListTopics() []*Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics := make([]*Topic, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}
	return topics
}

// SetAttribute sets a topic attribute
func (s *Store) SetAttribute(arn, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, exists := s.topics[arn]
	if !exists {
		return fmt.Errorf("topic not found: %s", arn)
	}

	if topic.Attributes == nil {
		topic.Attributes = make(map[string]string)
	}

	topic.Attributes[key] = value

	// Update topic-specific fields
	if key == "DisplayName" {
		topic.DisplayName = value
	}

	return nil
}

// GetAttribute retrieves a topic attribute
func (s *Store) GetAttribute(arn, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topic, exists := s.topics[arn]
	if !exists {
		return "", fmt.Errorf("topic not found: %s", arn)
	}

	if value, ok := topic.Attributes[key]; ok {
		return value, nil
	}
	return "", nil
}

// GetAttributes retrieves all attributes for a topic
func (s *Store) GetAttributes(arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topic, exists := s.topics[arn]
	if !exists {
		return nil, fmt.Errorf("topic not found: %s", arn)
	}

	attrs := make(map[string]string)
	if topic.Attributes != nil {
		for k, v := range topic.Attributes {
			attrs[k] = v
		}
	}
	return attrs, nil
}

// IncrementSubscriptionCount increments the subscription count for a topic
func (s *Store) IncrementSubscriptionCount(arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, exists := s.topics[arn]
	if !exists {
		return fmt.Errorf("topic not found: %s", arn)
	}

	topic.SubscriptionCount++
	return nil
}

// DecrementSubscriptionCount decrements the subscription count for a topic
func (s *Store) DecrementSubscriptionCount(arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	topic, exists := s.topics[arn]
	if !exists {
		return fmt.Errorf("topic not found: %s", arn)
	}

	if topic.SubscriptionCount > 0 {
		topic.SubscriptionCount--
	}
	return nil
}

// GetCount returns the total number of topics
func (s *Store) GetCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.topics)
}

// Restore restores a topic from exported data
func (s *Store) Restore(topic *Topic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topics[topic.TopicArn] = topic
}

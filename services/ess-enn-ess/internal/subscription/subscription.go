package subscription

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tonyellard/ess-enn-ess/internal/activity"
)

// Protocol represents a subscription protocol type
type Protocol string

const (
	ProtocolHTTP   Protocol = "http"
	ProtocolSQS    Protocol = "sqs"
	ProtocolEmail  Protocol = "email"
	ProtocolLambda Protocol = "lambda"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	StatusPending      SubscriptionStatus = "pending"
	StatusConfirmed    SubscriptionStatus = "confirmed"
	StatusUnsubscribed SubscriptionStatus = "unsubscribed"
)

// Subscription represents an SNS subscription
type Subscription struct {
	SubscriptionArn   string             `json:"subscription_arn" yaml:"subscriptionarn"`
	TopicArn          string             `json:"topic_arn" yaml:"topicarn"`
	Protocol          Protocol           `json:"protocol" yaml:"protocol"`
	Endpoint          string             `json:"endpoint" yaml:"endpoint"`
	Status            SubscriptionStatus `json:"status" yaml:"status"`
	ConfirmationToken string             `json:"-" yaml:"-"`
	CreatedAt         time.Time          `json:"created_at" yaml:"createdat"`
	Attributes        map[string]string  `json:"attributes" yaml:"attributes"`
}

// Store manages subscriptions in memory
type Store struct {
	mu                 sync.RWMutex
	subscriptions      map[string]*Subscription
	topicSubs          map[string][]*Subscription
	activityLogger     *activity.Logger
	accountId          string
	region             string
	nextSubscriptionId int64
}

// NewStore creates a new subscription store
func NewStore(accountId string, region string, activityLogger *activity.Logger) *Store {
	return &Store{
		subscriptions:      make(map[string]*Subscription),
		topicSubs:          make(map[string][]*Subscription),
		activityLogger:     activityLogger,
		accountId:          accountId,
		region:             region,
		nextSubscriptionId: 1,
	}
}

// Create adds a new subscription
func (s *Store) Create(topicArn string, protocol Protocol, endpoint string, autoConfirm bool) (*Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topicName := topicArn[strings.LastIndex(topicArn, ":")+1:]
	subArn := fmt.Sprintf("arn:aws:sns:%s:%s:%s:%d", s.region, s.accountId, topicName, s.nextSubscriptionId)
	s.nextSubscriptionId++

	confirmToken := fmt.Sprintf("token-%d-%d", time.Now().UnixNano(), s.nextSubscriptionId)
	status := StatusPending
	if autoConfirm {
		status = StatusConfirmed
	}

	sub := &Subscription{
		SubscriptionArn:   subArn,
		TopicArn:          topicArn,
		Protocol:          protocol,
		Endpoint:          endpoint,
		Status:            status,
		ConfirmationToken: confirmToken,
		CreatedAt:         time.Now().UTC(),
		Attributes:        make(map[string]string),
	}

	s.subscriptions[subArn] = sub
	s.topicSubs[topicArn] = append(s.topicSubs[topicArn], sub)

	return sub, nil
}

// GetByArn retrieves a subscription by its ARN
func (s *Store) GetByArn(subscriptionArn string) *Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subscriptions[subscriptionArn]
}

// GetByTopic retrieves all subscriptions for a topic
func (s *Store) GetByTopic(topicArn string) []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	subs := s.topicSubs[topicArn]
	result := make([]*Subscription, len(subs))
	copy(result, subs)
	return result
}

// Delete removes a subscription
func (s *Store) Delete(subscriptionArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, exists := s.subscriptions[subscriptionArn]
	if !exists {
		return fmt.Errorf("subscription not found: %s", subscriptionArn)
	}

	delete(s.subscriptions, subscriptionArn)

	subs := s.topicSubs[sub.TopicArn]
	for i, item := range subs {
		if item.SubscriptionArn == subscriptionArn {
			s.topicSubs[sub.TopicArn] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	return nil
}

// Confirm marks a subscription as confirmed
func (s *Store) Confirm(subscriptionArn string, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, exists := s.subscriptions[subscriptionArn]
	if !exists {
		return fmt.Errorf("subscription not found: %s", subscriptionArn)
	}

	if sub.ConfirmationToken != token {
		return fmt.Errorf("invalid confirmation token")
	}

	sub.Status = StatusConfirmed
	sub.ConfirmationToken = ""
	return nil
}

// ListAll returns all subscriptions
func (s *Store) ListAll() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Subscription, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		result = append(result, sub)
	}
	return result
}

// SetAttribute sets a subscription attribute
func (s *Store) SetAttribute(subscriptionArn string, key string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, exists := s.subscriptions[subscriptionArn]
	if !exists {
		return fmt.Errorf("subscription not found: %s", subscriptionArn)
	}

	sub.Attributes[key] = value
	return nil
}

// GetAttributes retrieves all attributes for a subscription
func (s *Store) GetAttributes(subscriptionArn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, exists := s.subscriptions[subscriptionArn]
	if !exists {
		return nil, fmt.Errorf("subscription not found: %s", subscriptionArn)
	}

	result := make(map[string]string)
	for k, v := range sub.Attributes {
		result[k] = v
	}
	return result, nil
}

// Restore restores a subscription from exported data
func (s *Store) Restore(sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[sub.SubscriptionArn] = sub
	s.topicSubs[sub.TopicArn] = append(s.topicSubs[sub.TopicArn], sub)
}

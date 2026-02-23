// SPDX-License-Identifier: Apache-2.0
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
	"github.com/tonyellard/cloud-u-l8r/pkg/awserrors"
	"github.com/tonyellard/cloud-u-l8r/pkg/health"
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

type TopicActivityEntry struct {
	ID              string         `json:"id"`
	Timestamp       time.Time      `json:"timestamp"`
	EventType       string         `json:"event_type"`
	TopicARN        string         `json:"topic_arn,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	SubscriptionARN string         `json:"subscription_arn,omitempty"`
	Protocol        string         `json:"protocol,omitempty"`
	Endpoint        string         `json:"endpoint,omitempty"`
	Status          string         `json:"status"`
	Details         map[string]any `json:"details,omitempty"`
	DurationMS      int64          `json:"duration_ms"`
	Error           string         `json:"error,omitempty"`
}

type EssThreeBucketSummary struct {
	Name        string `json:"name"`
	ObjectCount int    `json:"object_count"`
}

type EssThreeSummaryResponse struct {
	Service string                  `json:"service"`
	Buckets []EssThreeBucketSummary `json:"buckets"`
	Stats   struct {
		Buckets int `json:"buckets"`
		Objects int `json:"objects"`
	} `json:"stats"`
}

type EssThreeActivityEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Action     string    `json:"action,omitempty"`
	StatusCode int       `json:"statusCode"`
	ErrorType  string    `json:"errorType,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

type EssThreeActivityResponse struct {
	Activity  []EssThreeActivityEntry `json:"activity"`
	NextToken string                  `json:"nextToken,omitempty"`
}

type EssThreeObjectEntry struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag"`
	ContentType  string `json:"contentType"`
}

type EssThreeObjectListResponse struct {
	Bucket                string                `json:"bucket"`
	Prefix                string                `json:"prefix"`
	Objects               []EssThreeObjectEntry `json:"objects"`
	IsTruncated           bool                  `json:"isTruncated"`
	NextContinuationToken string                `json:"nextContinuationToken,omitempty"`
}

type CloudfauxntOriginOverview struct {
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	PathPatterns      []string `json:"path_patterns"`
	StripPrefix       string   `json:"strip_prefix,omitempty"`
	TargetPrefix      string   `json:"target_prefix,omitempty"`
	RequireSignature  bool     `json:"require_signature"`
	DefaultRootObject string   `json:"default_root_object,omitempty"`
}

type CloudfauxntSummaryResponse struct {
	Service string `json:"service"`
	Server  struct {
		Host              string `json:"host"`
		Port              int    `json:"port"`
		DefaultRootObject string `json:"default_root_object"`
	} `json:"server"`
	Signing struct {
		Enabled       bool   `json:"enabled"`
		KeyPairID     string `json:"key_pair_id,omitempty"`
		PublicKeyPath string `json:"public_key_path,omitempty"`
		TokenOptions  struct {
			ClockSkewSeconds        int  `json:"clock_skew_seconds"`
			DefaultURLTTLSeconds    int  `json:"default_url_ttl_seconds"`
			DefaultCookieTTLSeconds int  `json:"default_cookie_ttl_seconds"`
			AllowWildcardPatterns   bool `json:"allow_wildcard_patterns"`
		} `json:"token_options"`
	} `json:"signing"`
	Stats struct {
		Origins   int `json:"origins"`
		Behaviors int `json:"behaviors"`
	} `json:"stats"`
	Origins []CloudfauxntOriginOverview `json:"origins"`
}

type CloudfauxntSetSigningRequest struct {
	Enabled bool `json:"enabled"`
}

type CloudfauxntSigningTokenOptions struct {
	ClockSkewSeconds        int  `json:"clock_skew_seconds"`
	DefaultURLTTLSeconds    int  `json:"default_url_ttl_seconds"`
	DefaultCookieTTLSeconds int  `json:"default_cookie_ttl_seconds"`
	AllowWildcardPatterns   bool `json:"allow_wildcard_patterns"`
}

type CloudfauxntUpdateSigningConfigRequest struct {
	KeyPairID     string                         `json:"key_pair_id"`
	PublicKeyPath string                         `json:"public_key_path"`
	TokenOptions  CloudfauxntSigningTokenOptions `json:"token_options"`
}

type CloudfauxntOriginUpsertRequest struct {
	CurrentName       string   `json:"current_name,omitempty"`
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	PathPatterns      []string `json:"path_patterns"`
	StripPrefix       string   `json:"strip_prefix,omitempty"`
	TargetPrefix      string   `json:"target_prefix,omitempty"`
	RequireSignature  *bool    `json:"require_signature"`
	DefaultRootObject *string  `json:"default_root_object"`
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

type KayVeeSummaryResponse struct {
	Service        string `json:"service,omitempty"`
	Parameters     int    `json:"parameters"`
	SecretsTotal   int    `json:"secretsTotal"`
	SecretsActive  int    `json:"secretsActive"`
	SecretsDeleted int    `json:"secretsDeleted"`
}

type KayVeeActivityEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Action     string    `json:"action,omitempty"`
	StatusCode int       `json:"statusCode"`
	ErrorType  string    `json:"errorType,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

type KayVeeActivityResponse struct {
	Activity  []KayVeeActivityEntry `json:"activity"`
	NextToken string                `json:"nextToken,omitempty"`
}

// ServiceActivityResponse is a generic activity response type used by
// services that emit the standard pkg/activity.Entry format.
type ServiceActivityResponse struct {
	Activity  []EssThreeActivityEntry `json:"activity"`
	NextToken string                  `json:"nextToken,omitempty"`
}

type KayVeeParameter struct {
	Name             string    `json:"Name"`
	Type             string    `json:"Type"`
	Value            string    `json:"Value,omitempty"`
	Version          int64     `json:"Version"`
	ARN              string    `json:"ARN,omitempty"`
	LastModifiedDate time.Time `json:"LastModifiedDate,omitempty"`
}

type KayVeeParametersResponse struct {
	Parameters []KayVeeParameter `json:"parameters"`
	NextToken  string            `json:"nextToken,omitempty"`
}

type KayVeeAdminResourcesResponse struct {
	Parameters          []KayVeeParameter   `json:"parameters"`
	ParametersNextToken string              `json:"parametersNextToken,omitempty"`
	Secrets             []KayVeeSecretEntry `json:"secrets"`
	SecretsNextToken    string              `json:"secretsNextToken,omitempty"`
}

type KayVeePutParameterRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Overwrite bool   `json:"overwrite"`
}

type KayVeeDeleteParameterRequest struct {
	Name string `json:"name"`
}

type KayVeeLabelParameterRequest struct {
	Name             string `json:"name"`
	Label            string `json:"label"`
	ParameterVersion int64  `json:"parameter_version"`
}

type KayVeeSecretEntry struct {
	ARN             string     `json:"ARN"`
	Name            string     `json:"Name"`
	Description     string     `json:"Description,omitempty"`
	CreatedDate     time.Time  `json:"CreatedDate"`
	LastChangedDate time.Time  `json:"LastChangedDate"`
	DeletedDate     *time.Time `json:"DeletedDate,omitempty"`
}

type KayVeeSecretsResponse struct {
	Secrets   []KayVeeSecretEntry `json:"secrets"`
	NextToken string              `json:"nextToken,omitempty"`
}

type KayVeeCreateSecretRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	SecretString string `json:"secret_string"`
}

type KayVeeDeleteSecretRequest struct {
	SecretID string `json:"secret_id"`
}

type KayVeeRestoreSecretRequest struct {
	SecretID string `json:"secret_id"`
}

type KayVeePutSecretValueRequest struct {
	SecretID     string `json:"secret_id"`
	SecretString string `json:"secret_string,omitempty"`
	SecretBinary string `json:"secret_binary,omitempty"`
}

type KayVeeUpdateSecretRequest struct {
	SecretID     string `json:"secret_id"`
	Description  string `json:"description,omitempty"`
	SecretString string `json:"secret_string,omitempty"`
	SecretBinary string `json:"secret_binary,omitempty"`
}

type KayVeeUpdateSecretVersionStageRequest struct {
	SecretID            string `json:"secret_id"`
	VersionStage        string `json:"version_stage"`
	MoveToVersionID     string `json:"move_to_version_id"`
	RemoveFromVersionID string `json:"remove_from_version_id,omitempty"`
}

type KayVeeSecretValueResponse struct {
	ARN          string `json:"arn"`
	Name         string `json:"name"`
	VersionID    string `json:"version_id"`
	SecretString string `json:"secret_string,omitempty"`
	SecretBinary string `json:"secret_binary,omitempty"`
}

// --- Drawbridge (EventBridge) types ---

type DrawbridgeSummaryResponse struct {
	Service    string `json:"service"`
	EventBuses int    `json:"eventBuses"`
	Rules      int    `json:"rules"`
	Targets    int    `json:"targets"`
}

type DrawbridgeBusDetail struct {
	Name        string                  `json:"name"`
	Arn         string                  `json:"arn"`
	RuleCount   int                     `json:"ruleCount"`
	TargetCount int                     `json:"targetCount"`
	Rules       []DrawbridgeRuleDetail  `json:"rules"`
}

type DrawbridgeRuleDetail struct {
	Name               string                  `json:"name"`
	State              string                  `json:"state"`
	EventPattern       string                  `json:"eventPattern,omitempty"`
	ScheduleExpression string                  `json:"scheduleExpression,omitempty"`
	TargetCount        int                     `json:"targetCount"`
	Targets            []DrawbridgeTarget      `json:"targets"`
}

type DrawbridgeTarget struct {
	Id  string `json:"Id"`
	Arn string `json:"Arn"`
}

type DrawbridgeActivityResponse struct {
	Activity  []KayVeeActivityEntry `json:"activity"`
	NextToken string                `json:"nextToken,omitempty"`
}

type DrawbridgeCreateEventBusRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type DrawbridgeDeleteEventBusRequest struct {
	Name string `json:"name"`
}

type DrawbridgePutRuleRequest struct {
	Name               string `json:"name"`
	EventBusName       string `json:"event_bus_name,omitempty"`
	EventPattern       string `json:"event_pattern,omitempty"`
	ScheduleExpression string `json:"schedule_expression,omitempty"`
	State              string `json:"state,omitempty"`
	Description        string `json:"description,omitempty"`
}

type DrawbridgeDeleteRuleRequest struct {
	Name         string `json:"name"`
	EventBusName string `json:"event_bus_name,omitempty"`
}

type DrawbridgeToggleRuleRequest struct {
	Name         string `json:"name"`
	EventBusName string `json:"event_bus_name,omitempty"`
}

type DrawbridgePutTargetRequest struct {
	Rule         string `json:"rule"`
	EventBusName string `json:"event_bus_name,omitempty"`
	TargetID     string `json:"target_id"`
	TargetArn    string `json:"target_arn"`
	Input        string `json:"input,omitempty"`
}

type DrawbridgeRemoveTargetRequest struct {
	Rule         string `json:"rule"`
	EventBusName string `json:"event_bus_name,omitempty"`
	TargetID     string `json:"target_id"`
}

type DrawbridgePutEventRequest struct {
	Source       string `json:"source"`
	DetailType   string `json:"detail_type"`
	Detail       string `json:"detail"`
	EventBusName string `json:"event_bus_name,omitempty"`
}

type DrawbridgeTestEventPatternRequest struct {
	EventPattern string `json:"event_pattern"`
	Event        string `json:"event"`
}

// --- Scheduler types ---

type SchedulerSummaryResponse struct {
	Service        string `json:"service"`
	ScheduleGroups int    `json:"scheduleGroups"`
	Schedules      int    `json:"schedules"`
}

type SchedulerGroupDetail struct {
	Name      string                   `json:"name"`
	Arn       string                   `json:"arn"`
	State     string                   `json:"state"`
	Schedules []SchedulerScheduleDetail `json:"schedules"`
}

type SchedulerScheduleDetail struct {
	Name               string `json:"name"`
	Arn                string `json:"arn"`
	State              string `json:"state"`
	ScheduleExpression string `json:"scheduleExpression"`
	TargetArn          string `json:"targetArn,omitempty"`
	Description        string `json:"description,omitempty"`
}

type SchedulerActivityResponse struct {
	Activity  []KayVeeActivityEntry `json:"activity"`
	NextToken string                `json:"nextToken,omitempty"`
}

type SchedulerCreateGroupRequest struct {
	Name string `json:"name"`
}

type SchedulerDeleteGroupRequest struct {
	Name string `json:"name"`
}

type SchedulerCreateScheduleRequest struct {
	Name               string `json:"name"`
	GroupName          string `json:"group_name,omitempty"`
	ScheduleExpression string `json:"schedule_expression"`
	TargetArn          string `json:"target_arn,omitempty"`
	TargetInput        string `json:"target_input,omitempty"`
	State              string `json:"state,omitempty"`
	Description        string `json:"description,omitempty"`
	ActionAfterCompletion string `json:"action_after_completion,omitempty"`
}

type SchedulerDeleteScheduleRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name,omitempty"`
}

// --- Pipes types ---

type PipesSummaryResponse struct {
	Service      string `json:"service"`
	Pipes        int    `json:"pipes"`
	RunningPipes int    `json:"runningPipes"`
}

type PipesPipeDetail struct {
	Name             string `json:"name"`
	Arn              string `json:"arn"`
	CurrentState     string `json:"currentState"`
	DesiredState     string `json:"desiredState"`
	Source           string `json:"source"`
	Target           string `json:"target"`
	Enrichment       string `json:"enrichment,omitempty"`
	Description      string `json:"description,omitempty"`
	FilterCount      int    `json:"filterCount"`
	CreationTime     string `json:"creationTime"`
	LastModifiedTime string `json:"lastModifiedTime"`
}

type PipesActivityResponse struct {
	Activity  []KayVeeActivityEntry `json:"activity"`
	NextToken string                `json:"nextToken,omitempty"`
}

type PipesCreatePipeRequest struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	RoleArn     string `json:"role_arn,omitempty"`
	Description string `json:"description,omitempty"`
	Filter      string `json:"filter,omitempty"`
	Enrichment  string `json:"enrichment,omitempty"`
	State       string `json:"state,omitempty"`
}

type PipesUpdatePipeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Target      string `json:"target,omitempty"`
	RoleArn     string `json:"role_arn,omitempty"`
	State       string `json:"state,omitempty"`
}

type PipesDeletePipeRequest struct {
	Name string `json:"name"`
}

type PipesStartStopPipeRequest struct {
	Name string `json:"name"`
}

func NewRouter(logger *slog.Logger) http.Handler {
	srv := &Server{
		logger: logger,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	r := chi.NewRouter()
	r.Get("/health", health.Handler("admin-console"))
	r.Get("/api/dashboard/summary", srv.handleDashboardSummary)
	r.Get("/api/services/ess-queue-ess/queues", srv.handleQueueList)
	r.Get("/api/services/ess-queue-ess/queues/{queueID}/messages/peek", srv.handleQueuePeek)
	r.Get("/api/services/ess-queue-ess/queues/{queueID}/attributes", srv.handleQueueAttributes)
	r.Get("/api/services/ess-enn-ess/state", srv.handlePubSubState)
	r.Get("/api/services/ess-enn-ess/topics/{topicARN}/activities", srv.handleTopicActivities)
	r.Get("/api/services/essthree/summary", srv.handleEssThreeSummary)
	r.Get("/api/services/essthree/buckets/{bucket}/objects", srv.handleEssThreeListObjects)
	r.Post("/api/services/essthree/actions/create-bucket", srv.handleEssThreeCreateBucket)
	r.Post("/api/services/essthree/actions/delete-bucket", srv.handleEssThreeDeleteBucket)
	r.Post("/api/services/essthree/actions/delete-object", srv.handleEssThreeDeleteObject)
	r.Post("/api/services/essthree/actions/purge-bucket", srv.handleEssThreePurgeBucket)
	r.Get("/api/services/essthree/activity", srv.handleEssThreeActivity)
	r.Get("/api/services/ess-queue-ess/activity", srv.handleEssQueueEssActivity)
	r.Get("/api/services/cloudfauxnt/summary", srv.handleCloudfauxntSummary)
	r.Get("/api/services/cloudfauxnt/activity", srv.handleCloudfauxntActivity)
	r.Post("/api/services/cloudfauxnt/actions/set-signing", srv.handleCloudfauxntSetSigning)
	r.Post("/api/services/cloudfauxnt/actions/update-signing-config", srv.handleCloudfauxntUpdateSigningConfig)
	r.Post("/api/services/cloudfauxnt/actions/create-origin", srv.handleCloudfauxntCreateOrigin)
	r.Post("/api/services/cloudfauxnt/actions/update-origin", srv.handleCloudfauxntUpdateOrigin)
	r.Get("/api/services/kay-vee/summary", srv.handleKayVeeSummary)
	r.Get("/api/services/kay-vee/activity", srv.handleKayVeeActivity)
	r.Get("/api/services/kay-vee/export", srv.handleKayVeeExport)
	r.Get("/api/services/kay-vee/parameters/by-path", srv.handleKayVeeParametersByPath)
	r.Get("/api/services/kay-vee/secrets", srv.handleKayVeeSecrets)
	r.Get("/api/services/kay-vee/secrets/value", srv.handleKayVeeSecretValue)
	r.Post("/api/services/kay-vee/actions/put-parameter", srv.handleKayVeePutParameter)
	r.Post("/api/services/kay-vee/actions/delete-parameter", srv.handleKayVeeDeleteParameter)
	r.Post("/api/services/kay-vee/actions/label-parameter-version", srv.handleKayVeeLabelParameterVersion)
	r.Post("/api/services/kay-vee/actions/create-secret", srv.handleKayVeeCreateSecret)
	r.Post("/api/services/kay-vee/actions/delete-secret", srv.handleKayVeeDeleteSecret)
	r.Post("/api/services/kay-vee/actions/restore-secret", srv.handleKayVeeRestoreSecret)
	r.Post("/api/services/kay-vee/actions/put-secret-value", srv.handleKayVeePutSecretValue)
	r.Post("/api/services/kay-vee/actions/update-secret", srv.handleKayVeeUpdateSecret)
	r.Post("/api/services/kay-vee/actions/update-secret-version-stage", srv.handleKayVeeUpdateSecretVersionStage)
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
	r.Get("/api/services/drawbridge/summary", srv.handleDrawbridgeSummary)
	r.Get("/api/services/drawbridge/resources", srv.handleDrawbridgeResources)
	r.Get("/api/services/drawbridge/activity", srv.handleDrawbridgeActivity)
	r.Post("/api/services/drawbridge/actions/create-event-bus", srv.handleDrawbridgeCreateEventBus)
	r.Post("/api/services/drawbridge/actions/delete-event-bus", srv.handleDrawbridgeDeleteEventBus)
	r.Post("/api/services/drawbridge/actions/put-rule", srv.handleDrawbridgePutRule)
	r.Post("/api/services/drawbridge/actions/delete-rule", srv.handleDrawbridgeDeleteRule)
	r.Post("/api/services/drawbridge/actions/enable-rule", srv.handleDrawbridgeEnableRule)
	r.Post("/api/services/drawbridge/actions/disable-rule", srv.handleDrawbridgeDisableRule)
	r.Post("/api/services/drawbridge/actions/put-target", srv.handleDrawbridgePutTarget)
	r.Post("/api/services/drawbridge/actions/remove-target", srv.handleDrawbridgeRemoveTarget)
	r.Post("/api/services/drawbridge/actions/put-event", srv.handleDrawbridgePutEvent)
	r.Post("/api/services/drawbridge/actions/test-event-pattern", srv.handleDrawbridgeTestEventPattern)
	r.Get("/api/services/scheduler/summary", srv.handleSchedulerSummary)
	r.Get("/api/services/scheduler/resources", srv.handleSchedulerResources)
	r.Get("/api/services/scheduler/activity", srv.handleSchedulerActivity)
	r.Post("/api/services/scheduler/actions/create-group", srv.handleSchedulerCreateGroup)
	r.Post("/api/services/scheduler/actions/delete-group", srv.handleSchedulerDeleteGroup)
	r.Post("/api/services/scheduler/actions/create-schedule", srv.handleSchedulerCreateSchedule)
	r.Post("/api/services/scheduler/actions/delete-schedule", srv.handleSchedulerDeleteSchedule)
	r.Get("/api/services/pipes/summary", srv.handlePipesSummary)
	r.Get("/api/services/pipes/resources", srv.handlePipesResources)
	r.Get("/api/services/pipes/activity", srv.handlePipesActivity)
	r.Post("/api/services/pipes/actions/create-pipe", srv.handlePipesCreatePipe)
	r.Post("/api/services/pipes/actions/update-pipe", srv.handlePipesUpdatePipe)
	r.Post("/api/services/pipes/actions/delete-pipe", srv.handlePipesDeletePipe)
	r.Post("/api/services/pipes/actions/start-pipe", srv.handlePipesStartPipe)
	r.Post("/api/services/pipes/actions/stop-pipe", srv.handlePipesStopPipe)
	r.Get("/api/events", srv.handleEvents)

	fs := http.FileServer(http.Dir("./web"))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/index.html")
	})
	r.Handle("/*", fs)

	return r
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
	case "kay-vee":
		upstreamURL = "http://kay-vee:9350/admin/api/export"
		fallbackFilename = "kay-vee-export.json"
	default:
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("unsupported service"))
		return
	}

	resp, err := s.client.Get(upstreamURL)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to fetch export: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("export failed for %s (%d): %s", service, resp.StatusCode, message))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, QueueViewResponse{Service: "ess-queue-ess", Queues: queues})
}

func (s *Server) handlePubSubState(w http.ResponseWriter, _ *http.Request) {
	state, err := s.fetchPubSubState()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleTopicActivities(w http.ResponseWriter, r *http.Request) {
	topicARN := strings.TrimSpace(normalizeQueueIDParam(chi.URLParam(r, "topicARN")))
	if topicARN == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}

	var activities []TopicActivityEntry
	path := "/api/activities?topic=" + url.QueryEscape(topicARN)
	if err := s.callSNSAdminJSON(http.MethodGet, path, nil, &activities); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"topic_arn":  topicARN,
		"activities": activities,
	})
}

func (s *Server) handleEssThreeSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchEssThreeSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleEssThreeListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
	maxKeys := strings.TrimSpace(r.URL.Query().Get("maxKeys"))
	continuationToken := strings.TrimSpace(r.URL.Query().Get("continuationToken"))

	objectList, err := s.fetchEssThreeObjectList(bucket, prefix, maxKeys, continuationToken)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, objectList)
}

func (s *Server) handleEssThreeCreateBucket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	// Use the S3 API (PUT /{bucket}/) so the operation is recorded in the activity log.
	s3URL := fmt.Sprintf("http://essthree:9300/%s/", url.PathEscape(req.Name))
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPut, s3URL, nil)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to create bucket: %w", err))
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode <= 299 {
		writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
		return
	}

	awserrors.WriteJSONGeneric(w, resp.StatusCode, fmt.Errorf("failed to create bucket %q (status %d)", req.Name, resp.StatusCode))
}

func (s *Server) handleEssThreeDeleteBucket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	// Use the S3 API (DELETE /{bucket}/) so the operation is recorded in the activity log.
	s3URL := fmt.Sprintf("http://essthree:9300/%s/", url.PathEscape(req.Name))
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, s3URL, nil)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to delete bucket: %w", err))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	// S3 returns XML errors; translate to a JSON error for the admin console.
	message := strings.TrimSpace(string(body))
	if strings.Contains(message, "BucketNotEmpty") {
		awserrors.WriteJSONGeneric(w, http.StatusConflict, fmt.Errorf("bucket is not empty"))
	} else if resp.StatusCode == http.StatusNotFound {
		awserrors.WriteJSONGeneric(w, http.StatusNotFound, fmt.Errorf("bucket does not exist"))
	} else {
		awserrors.WriteJSONGeneric(w, resp.StatusCode, fmt.Errorf("delete failed (status %d)", resp.StatusCode))
	}
}

func (s *Server) handleEssThreeDeleteObject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bucket string `json:"bucket"`
		Key    string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Bucket == "" || req.Key == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("bucket and key are required"))
		return
	}

	// Use the S3 API (DELETE /{bucket}/{key}) so the operation is recorded in the activity log.
	s3URL := fmt.Sprintf("http://essthree:9300/%s/%s",
		url.PathEscape(req.Bucket), req.Key)
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, s3URL, nil)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to delete object: %w", err))
		return
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	awserrors.WriteJSONGeneric(w, resp.StatusCode, fmt.Errorf("delete failed (status %d)", resp.StatusCode))
}

func (s *Server) handleEssThreePurgeBucket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	purgeURL := fmt.Sprintf("http://essthree:9300/admin/api/buckets/%s/purge", url.PathEscape(req.Name))
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, purgeURL, nil)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to purge bucket: %w", err))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (s *Server) handleEssThreeActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := strings.TrimSpace(r.URL.Query().Get("maxResults"))
	if maxResults == "" {
		maxResults = "25"
	}

	activityData, err := s.fetchEssThreeActivity(maxResults, strings.TrimSpace(r.URL.Query().Get("nextToken")))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, activityData)
}

func (s *Server) handleEssQueueEssActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := strings.TrimSpace(r.URL.Query().Get("maxResults"))
	if maxResults == "" {
		maxResults = "25"
	}

	data, err := s.fetchServiceActivity("http://ess-queue-ess:9320", "ess-queue-ess", maxResults, strings.TrimSpace(r.URL.Query().Get("nextToken")))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleCloudfauxntActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := strings.TrimSpace(r.URL.Query().Get("maxResults"))
	if maxResults == "" {
		maxResults = "25"
	}

	data, err := s.fetchServiceActivity("http://cloudfauxnt:9310", "cloudfauxnt", maxResults, strings.TrimSpace(r.URL.Query().Get("nextToken")))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleCloudfauxntSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchCloudfauxntSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCloudfauxntSetSigning(w http.ResponseWriter, r *http.Request) {
	var request CloudfauxntSetSigningRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	payload, err := json.Marshal(request)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusInternalServerError, err)
		return
	}

	resp, err := s.client.Post("http://cloudfauxnt:9310/admin/api/signing", "application/json", bytes.NewReader(payload))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to update cloudfauxnt signing: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("cloudfauxnt signing update failed (%d): %s", resp.StatusCode, message))
		return
	}

	summary, err := s.fetchCloudfauxntSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCloudfauxntUpdateSigningConfig(w http.ResponseWriter, r *http.Request) {
	var request CloudfauxntUpdateSigningConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	payload := map[string]any{
		"key_pair_id":     strings.TrimSpace(request.KeyPairID),
		"public_key_path": strings.TrimSpace(request.PublicKeyPath),
		"token_options": map[string]any{
			"clock_skew_seconds":         request.TokenOptions.ClockSkewSeconds,
			"default_url_ttl_seconds":    request.TokenOptions.DefaultURLTTLSeconds,
			"default_cookie_ttl_seconds": request.TokenOptions.DefaultCookieTTLSeconds,
			"allow_wildcard_patterns":    request.TokenOptions.AllowWildcardPatterns,
		},
	}

	if err := s.callCloudfauxntAdminJSON(http.MethodPut, "/admin/api/signing/config", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	summary, err := s.fetchCloudfauxntSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCloudfauxntCreateOrigin(w http.ResponseWriter, r *http.Request) {
	var req CloudfauxntOriginUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	payload := map[string]any{
		"name":                req.Name,
		"url":                 req.URL,
		"path_patterns":       req.PathPatterns,
		"strip_prefix":        req.StripPrefix,
		"target_prefix":       req.TargetPrefix,
		"require_signature":   req.RequireSignature,
		"default_root_object": req.DefaultRootObject,
	}

	if err := s.callCloudfauxntAdminJSON(http.MethodPost, "/admin/api/origins", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	summary, err := s.fetchCloudfauxntSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleCloudfauxntUpdateOrigin(w http.ResponseWriter, r *http.Request) {
	var req CloudfauxntOriginUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	currentName := strings.TrimSpace(req.CurrentName)
	if currentName == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("current_name is required"))
		return
	}

	payload := map[string]any{
		"name":                req.Name,
		"url":                 req.URL,
		"path_patterns":       req.PathPatterns,
		"strip_prefix":        req.StripPrefix,
		"target_prefix":       req.TargetPrefix,
		"require_signature":   req.RequireSignature,
		"default_root_object": req.DefaultRootObject,
	}

	path := "/admin/api/origins/" + url.PathEscape(currentName)
	if err := s.callCloudfauxntAdminJSON(http.MethodPut, path, payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	summary, err := s.fetchCloudfauxntSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleKayVeeSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchKayVeeSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleKayVeeActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := strings.TrimSpace(r.URL.Query().Get("maxResults"))
	if maxResults == "" {
		maxResults = "25"
	}

	activity, err := s.fetchKayVeeActivity(maxResults, strings.TrimSpace(r.URL.Query().Get("nextToken")))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, activity)
}

func (s *Server) handleKayVeeExport(w http.ResponseWriter, _ *http.Request) {
	resp, err := s.client.Get("http://kay-vee:9350/admin/api/export")
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to fetch kay-vee export: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("kay-vee export failed (%d): %s", resp.StatusCode, message))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=kay-vee-export.json")
	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Error("failed to proxy kay-vee export response", "error", err)
	}
}

func (s *Server) handleKayVeeParametersByPath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path = "/"
	}

	recursive := true
	if raw := strings.TrimSpace(r.URL.Query().Get("recursive")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("recursive must be a boolean"))
			return
		}
		recursive = parsed
	}

	withDecryption := false
	if raw := strings.TrimSpace(r.URL.Query().Get("withDecryption")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("withDecryption must be a boolean"))
			return
		}
		withDecryption = parsed
	}

	maxResults := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("maxResults")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("maxResults must be a non-negative integer"))
			return
		}
		if parsed > 10 {
			parsed = 10
		}
		maxResults = parsed
	}

	parameters, err := s.fetchKayVeeParametersByPath(
		path,
		recursive,
		withDecryption,
		maxResults,
		strings.TrimSpace(r.URL.Query().Get("nextToken")),
		strings.TrimSpace(r.URL.Query().Get("type")),
		strings.TrimSpace(r.URL.Query().Get("label")),
	)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, parameters)
}

func (s *Server) handleKayVeePutParameter(w http.ResponseWriter, r *http.Request) {
	var req KayVeePutParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	parameterName := normalizeKayVeeParameterName(req.Name)
	if parameterName == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if strings.TrimSpace(req.Value) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("value is required"))
		return
	}

	payload := map[string]any{
		"Name":      parameterName,
		"Type":      strings.TrimSpace(req.Type),
		"Value":     req.Value,
		"Overwrite": req.Overwrite,
	}
	if payload["Type"] == "" {
		payload["Type"] = "String"
	}

	if err := s.callKayVeeTarget("AmazonSSM.PutParameter", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": parameterName})
}

func (s *Server) handleKayVeeDeleteParameter(w http.ResponseWriter, r *http.Request) {
	var req KayVeeDeleteParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	parameterName := normalizeKayVeeParameterName(req.Name)
	if parameterName == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callKayVeeTarget("AmazonSSM.DeleteParameter", map[string]any{"Name": parameterName}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": parameterName})
}

func (s *Server) handleKayVeeLabelParameterVersion(w http.ResponseWriter, r *http.Request) {
	var req KayVeeLabelParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	parameterName := normalizeKayVeeParameterName(req.Name)
	if parameterName == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("label is required"))
		return
	}
	if req.ParameterVersion <= 0 {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("parameter_version must be > 0"))
		return
	}

	payload := map[string]any{
		"Name":             parameterName,
		"Labels":           []string{strings.TrimSpace(req.Label)},
		"ParameterVersion": req.ParameterVersion,
	}

	if err := s.callKayVeeTarget("AmazonSSM.LabelParameterVersion", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func normalizeKayVeeParameterName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func (s *Server) handleKayVeeSecrets(w http.ResponseWriter, r *http.Request) {
	maxResults := 25
	if raw := strings.TrimSpace(r.URL.Query().Get("maxResults")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("maxResults must be a non-negative integer"))
			return
		}
		maxResults = parsed
	}

	secrets, err := s.fetchKayVeeSecrets(maxResults, strings.TrimSpace(r.URL.Query().Get("nextToken")), strings.TrimSpace(r.URL.Query().Get("nameFilter")))
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, secrets)
}

func (s *Server) handleKayVeeSecretValue(w http.ResponseWriter, r *http.Request) {
	secretID := strings.TrimSpace(r.URL.Query().Get("secretId"))
	if secretID == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secretId is required"))
		return
	}

	payload := map[string]any{"SecretId": secretID}
	if versionStage := strings.TrimSpace(r.URL.Query().Get("versionStage")); versionStage != "" {
		payload["VersionStage"] = versionStage
	}

	var upstream struct {
		ARN          string  `json:"ARN"`
		Name         string  `json:"Name"`
		VersionID    string  `json:"VersionId"`
		SecretString *string `json:"SecretString,omitempty"`
		SecretBinary string  `json:"SecretBinary,omitempty"`
	}
	if err := s.callKayVeeTarget("secretsmanager.GetSecretValue", payload, &upstream); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	secretString := ""
	if upstream.SecretString != nil {
		secretString = *upstream.SecretString
	}

	writeJSON(w, http.StatusOK, KayVeeSecretValueResponse{
		ARN:          upstream.ARN,
		Name:         upstream.Name,
		VersionID:    upstream.VersionID,
		SecretString: secretString,
		SecretBinary: upstream.SecretBinary,
	})
}

func (s *Server) handleKayVeeCreateSecret(w http.ResponseWriter, r *http.Request) {
	var req KayVeeCreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if strings.TrimSpace(req.SecretString) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_string is required"))
		return
	}

	payload := map[string]any{
		"Name":         strings.TrimSpace(req.Name),
		"Description":  strings.TrimSpace(req.Description),
		"SecretString": req.SecretString,
	}

	if err := s.callKayVeeTarget("secretsmanager.CreateSecret", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": strings.TrimSpace(req.Name)})
}

func (s *Server) handleKayVeeDeleteSecret(w http.ResponseWriter, r *http.Request) {
	var req KayVeeDeleteSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SecretID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_id is required"))
		return
	}

	if err := s.callKayVeeTarget("secretsmanager.DeleteSecret", map[string]any{"SecretId": strings.TrimSpace(req.SecretID)}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKayVeeRestoreSecret(w http.ResponseWriter, r *http.Request) {
	var req KayVeeRestoreSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SecretID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_id is required"))
		return
	}

	if err := s.callKayVeeTarget("secretsmanager.RestoreSecret", map[string]any{"SecretId": strings.TrimSpace(req.SecretID)}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKayVeePutSecretValue(w http.ResponseWriter, r *http.Request) {
	var req KayVeePutSecretValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SecretID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_id is required"))
		return
	}
	if strings.TrimSpace(req.SecretString) == "" && strings.TrimSpace(req.SecretBinary) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_string or secret_binary is required"))
		return
	}

	payload := map[string]any{"SecretId": strings.TrimSpace(req.SecretID)}
	if strings.TrimSpace(req.SecretString) != "" {
		payload["SecretString"] = req.SecretString
	}
	if strings.TrimSpace(req.SecretBinary) != "" {
		payload["SecretBinary"] = req.SecretBinary
	}

	if err := s.callKayVeeTarget("secretsmanager.PutSecretValue", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKayVeeUpdateSecret(w http.ResponseWriter, r *http.Request) {
	var req KayVeeUpdateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SecretID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_id is required"))
		return
	}
	if strings.TrimSpace(req.Description) == "" && strings.TrimSpace(req.SecretString) == "" && strings.TrimSpace(req.SecretBinary) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("description, secret_string, or secret_binary is required"))
		return
	}

	payload := map[string]any{"SecretId": strings.TrimSpace(req.SecretID)}
	if strings.TrimSpace(req.Description) != "" {
		payload["Description"] = strings.TrimSpace(req.Description)
	}
	if strings.TrimSpace(req.SecretString) != "" {
		payload["SecretString"] = req.SecretString
	}
	if strings.TrimSpace(req.SecretBinary) != "" {
		payload["SecretBinary"] = req.SecretBinary
	}

	if err := s.callKayVeeTarget("secretsmanager.UpdateSecret", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKayVeeUpdateSecretVersionStage(w http.ResponseWriter, r *http.Request) {
	var req KayVeeUpdateSecretVersionStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SecretID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("secret_id is required"))
		return
	}
	if strings.TrimSpace(req.VersionStage) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("version_stage is required"))
		return
	}
	if strings.TrimSpace(req.MoveToVersionID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("move_to_version_id is required"))
		return
	}

	payload := map[string]any{
		"SecretId":        strings.TrimSpace(req.SecretID),
		"VersionStage":    strings.TrimSpace(req.VersionStage),
		"MoveToVersionId": strings.TrimSpace(req.MoveToVersionID),
	}
	if strings.TrimSpace(req.RemoveFromVersionID) != "" {
		payload["RemoveFromVersionId"] = strings.TrimSpace(req.RemoveFromVersionID)
	}

	if err := s.callKayVeeTarget("secretsmanager.UpdateSecretVersionStage", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Drawbridge (EventBridge) handlers ---

func (s *Server) handleDrawbridgeSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchDrawbridgeSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleDrawbridgeResources(w http.ResponseWriter, _ *http.Request) {
	resources, err := s.fetchDrawbridgeResources()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) handleDrawbridgeActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := r.URL.Query().Get("maxResults")
	if maxResults == "" {
		maxResults = "50"
	}
	nextToken := r.URL.Query().Get("nextToken")
	activity, err := s.fetchDrawbridgeActivity(maxResults, nextToken)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

func (s *Server) fetchDrawbridgeSummary() (DrawbridgeSummaryResponse, error) {
	resp, err := s.client.Get("http://drawbridge:9340/admin/api/summary")
	if err != nil {
		return DrawbridgeSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return DrawbridgeSummaryResponse{}, fmt.Errorf("drawbridge summary status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var summary DrawbridgeSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return DrawbridgeSummaryResponse{}, err
	}
	return summary, nil
}

func (s *Server) fetchDrawbridgeResources() ([]DrawbridgeBusDetail, error) {
	resp, err := s.client.Get("http://drawbridge:9340/admin/api/resources")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("drawbridge resources status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var resources []DrawbridgeBusDetail
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}
	return resources, nil
}

func (s *Server) fetchDrawbridgeActivity(maxResults, nextToken string) (DrawbridgeActivityResponse, error) {
	u := fmt.Sprintf("http://drawbridge:9340/admin/api/activity?maxResults=%s", url.QueryEscape(maxResults))
	if nextToken != "" {
		u += "&nextToken=" + url.QueryEscape(nextToken)
	}
	resp, err := s.client.Get(u)
	if err != nil {
		return DrawbridgeActivityResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return DrawbridgeActivityResponse{}, fmt.Errorf("drawbridge activity status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var activity DrawbridgeActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return DrawbridgeActivityResponse{}, err
	}
	return activity, nil
}

func (s *Server) handleDrawbridgeCreateEventBus(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeCreateEventBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["Description"] = desc
	}

	if err := s.callDrawbridgeTarget("AWSEvents.CreateEventBus", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handleDrawbridgeDeleteEventBus(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeDeleteEventBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if name == "default" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("cannot delete the default event bus"))
		return
	}

	if err := s.callDrawbridgeTarget("AWSEvents.DeleteEventBus", map[string]any{"Name": name}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgePutRule(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgePutRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}
	if ep := strings.TrimSpace(req.EventPattern); ep != "" {
		payload["EventPattern"] = ep
	}
	if se := strings.TrimSpace(req.ScheduleExpression); se != "" {
		payload["ScheduleExpression"] = se
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["Description"] = desc
	}
	state := strings.TrimSpace(req.State)
	if state == "" {
		state = "ENABLED"
	}
	payload["State"] = state

	if err := s.callDrawbridgeTarget("AWSEvents.PutRule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handleDrawbridgeDeleteRule(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeDeleteRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}

	if err := s.callDrawbridgeTarget("AWSEvents.DeleteRule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgeEnableRule(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeToggleRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}

	if err := s.callDrawbridgeTarget("AWSEvents.EnableRule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgeDisableRule(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeToggleRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}

	if err := s.callDrawbridgeTarget("AWSEvents.DisableRule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgePutTarget(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgePutTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	rule := strings.TrimSpace(req.Rule)
	if rule == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("rule is required"))
		return
	}
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("target_id is required"))
		return
	}
	targetArn := strings.TrimSpace(req.TargetArn)
	if targetArn == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("target_arn is required"))
		return
	}

	target := map[string]any{"Id": targetID, "Arn": targetArn}
	if input := strings.TrimSpace(req.Input); input != "" {
		target["Input"] = input
	}

	payload := map[string]any{
		"Rule":    rule,
		"Targets": []map[string]any{target},
	}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}

	var result struct {
		FailedEntryCount int `json:"FailedEntryCount"`
		FailedEntries    []struct {
			TargetId     string `json:"TargetId"`
			ErrorCode    string `json:"ErrorCode"`
			ErrorMessage string `json:"ErrorMessage"`
		} `json:"FailedEntries"`
	}
	if err := s.callDrawbridgeTarget("AWSEvents.PutTargets", payload, &result); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	if result.FailedEntryCount > 0 && len(result.FailedEntries) > 0 {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("%s: %s", result.FailedEntries[0].ErrorCode, result.FailedEntries[0].ErrorMessage))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgeRemoveTarget(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeRemoveTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	rule := strings.TrimSpace(req.Rule)
	if rule == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("rule is required"))
		return
	}
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("target_id is required"))
		return
	}

	payload := map[string]any{
		"Rule": rule,
		"Ids":  []string{targetID},
	}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		payload["EventBusName"] = bus
	}

	var result struct {
		FailedEntryCount int `json:"FailedEntryCount"`
		FailedEntries    []struct {
			TargetId     string `json:"TargetId"`
			ErrorCode    string `json:"ErrorCode"`
			ErrorMessage string `json:"ErrorMessage"`
		} `json:"FailedEntries"`
	}
	if err := s.callDrawbridgeTarget("AWSEvents.RemoveTargets", payload, &result); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	if result.FailedEntryCount > 0 && len(result.FailedEntries) > 0 {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("%s: %s", result.FailedEntries[0].ErrorCode, result.FailedEntries[0].ErrorMessage))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDrawbridgePutEvent(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgePutEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("source is required"))
		return
	}
	detailType := strings.TrimSpace(req.DetailType)
	if detailType == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("detail_type is required"))
		return
	}
	detail := strings.TrimSpace(req.Detail)
	if detail == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("detail is required"))
		return
	}

	entry := map[string]any{
		"Source":     source,
		"DetailType": detailType,
		"Detail":     detail,
	}
	if bus := strings.TrimSpace(req.EventBusName); bus != "" {
		entry["EventBusName"] = bus
	}

	payload := map[string]any{
		"Entries": []map[string]any{entry},
	}

	var result struct {
		FailedEntryCount int `json:"FailedEntryCount"`
		Entries          []struct {
			EventId      string `json:"EventId"`
			ErrorCode    string `json:"ErrorCode"`
			ErrorMessage string `json:"ErrorMessage"`
		} `json:"Entries"`
	}
	if err := s.callDrawbridgeTarget("AWSEvents.PutEvents", payload, &result); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	if result.FailedEntryCount > 0 && len(result.Entries) > 0 {
		entry := result.Entries[0]
		if entry.ErrorCode != "" {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("%s: %s", entry.ErrorCode, entry.ErrorMessage))
			return
		}
	}
	eventId := ""
	if len(result.Entries) > 0 {
		eventId = result.Entries[0].EventId
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event_id": eventId})
}

func (s *Server) handleDrawbridgeTestEventPattern(w http.ResponseWriter, r *http.Request) {
	var req DrawbridgeTestEventPatternRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	eventPattern := strings.TrimSpace(req.EventPattern)
	if eventPattern == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("event_pattern is required"))
		return
	}
	event := strings.TrimSpace(req.Event)
	if event == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("event is required"))
		return
	}

	var result struct {
		Result bool `json:"Result"`
	}
	payload := map[string]any{
		"EventPattern": eventPattern,
		"Event":        event,
	}
	if err := s.callDrawbridgeTarget("AWSEvents.TestEventPattern", payload, &result); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result.Result})
}

func (s *Server) handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	var req CreateTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/topics", map[string]any{
		"name": strings.TrimSpace(req.Name),
	}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "topic_name": strings.TrimSpace(req.Name)})
}

func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	var req DeleteTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/topics/delete", map[string]any{
		"topic_arn": strings.TrimSpace(req.TopicARN),
	}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("protocol is required"))
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("endpoint is required"))
		return
	}

	if protocol != "http" && protocol != "ess-queue-ess" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("protocol must be http or ess-queue-ess"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	var req DeleteSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.SubscriptionARN) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("subscription_arn is required"))
		return
	}

	if err := s.callSNSAdminJSON(http.MethodPost, "/api/subscriptions/delete", map[string]any{
		"subscription_arn": strings.TrimSpace(req.SubscriptionARN),
	}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePublishTopicMessage(w http.ResponseWriter, r *http.Request) {
	var req PublishTopicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.TopicARN) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("topic_arn is required"))
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("message is required"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleQueuePeek(w http.ResponseWriter, r *http.Request) {
	queueID := normalizeQueueIDParam(chi.URLParam(r, "queueID"))
	if strings.TrimSpace(queueID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_id is required"))
		return
	}

	limit := 10
	if limitParam := r.URL.Query().Get("limit"); strings.TrimSpace(limitParam) != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil || parsedLimit < 1 {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("limit must be a positive integer"))
			return
		}
		if parsedLimit > 100 {
			parsedLimit = 100
		}
		limit = parsedLimit
	}

	queues, err := s.fetchQueues()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	selected := findQueueByID(queues, queueID)
	if selected == nil {
		awserrors.WriteJSONGeneric(w, http.StatusNotFound, fmt.Errorf("queue not found"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueName) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_name is required"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid create queue attribute values"))
		return
	}

	dlqMaxReceiveCount := req.DLQMaxReceiveCount
	if dlqMaxReceiveCount == 0 {
		dlqMaxReceiveCount = 3
	}
	if req.CreateDLQ && dlqMaxReceiveCount <= 0 {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("dlq_max_receive_count must be greater than zero"))
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
			awserrors.WriteJSONGeneric(w, http.StatusBadGateway, fmt.Errorf("failed to create DLQ: %w", err))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
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
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}
	if strings.TrimSpace(req.MessageBody) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("message_body is required"))
		return
	}
	if strings.HasSuffix(req.QueueURL, ".fifo") && strings.TrimSpace(req.MessageGroupID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("message_group_id is required for FIFO queues"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePurgeQueue(w http.ResponseWriter, r *http.Request) {
	var req QueueActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	form := url.Values{}
	form.Set("Action", "PurgeQueue")
	form.Set("QueueUrl", strings.TrimSpace(req.QueueURL))

	if err := s.callSQSAction(form); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdateQueueAttributes(w http.ResponseWriter, r *http.Request) {
	var req UpdateQueueAttributesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	if strings.TrimSpace(req.QueueURL) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}
	if req.VisibilityTimeout < 0 || req.MessageRetentionPeriod < 0 || req.MaximumMessageSize <= 0 || req.DelaySeconds < 0 || req.ReceiveMessageWaitTimeSeconds < 0 {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid attribute values"))
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	var req QueueActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	form := url.Values{}
	form.Set("Action", "DeleteQueue")
	form.Set("QueueUrl", strings.TrimSpace(req.QueueURL))

	if err := s.callSQSAction(form); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleQueueAttributes(w http.ResponseWriter, r *http.Request) {
	queueID := normalizeQueueIDParam(chi.URLParam(r, "queueID"))
	if strings.TrimSpace(queueID) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_id is required"))
		return
	}

	queues, err := s.fetchQueues()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}

	selected := findQueueByID(queues, queueID)
	if selected == nil {
		awserrors.WriteJSONGeneric(w, http.StatusNotFound, fmt.Errorf("queue not found"))
		return
	}

	var sqsResp struct {
		Attributes map[string]string `json:"Attributes"`
	}
	if err := s.callSQSJSONAction("GetQueueAttributes", map[string]any{
		"QueueUrl":       selected.QueueURL,
		"AttributeNames": []string{"All"},
	}, &sqsResp); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
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
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if strings.TrimSpace(req.QueueURL) == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("queue_url is required"))
		return
	}

	sourceArn, err := queueURLToARN(req.QueueURL)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, err)
		return
	}

	payload := map[string]any{"SourceArn": sourceArn}
	if strings.TrimSpace(req.DestinationQueueURL) != "" {
		destinationArn, destinationErr := queueURLToARN(req.DestinationQueueURL)
		if destinationErr != nil {
			awserrors.WriteJSONGeneric(w, http.StatusBadRequest, destinationErr)
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
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
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
	if view != "dashboard" && view != "ess-queue-ess" && view != "ess-enn-ess" && view != "essthree" && view != "cloudfauxnt" && view != "kay-vee" && view != "drawbridge" && view != "scheduler" && view != "pipes" {
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
	if view == "essthree" {
		summary, err := s.fetchEssThreeSummary()
		if err != nil {
			return nil, err
		}
		activityData, _ := s.fetchEssThreeActivity("25", "")
		return json.Marshal(map[string]any{
			"service":           summary.Service,
			"buckets":           summary.Buckets,
			"stats":             summary.Stats,
			"activity":          activityData.Activity,
			"activityNextToken": activityData.NextToken,
		})
	}
	if view == "cloudfauxnt" {
		summary, err := s.fetchCloudfauxntSummary()
		if err != nil {
			return nil, err
		}
		return json.Marshal(summary)
	}
	if view == "kay-vee" {
		summary, err := s.fetchKayVeeSummary()
		if err != nil {
			return nil, err
		}
		activity, err := s.fetchKayVeeActivity("25", "")
		if err != nil {
			return nil, err
		}
		parameters, err := s.fetchKayVeeParametersByPath("/", true, false, 10, "", "", "")
		if err != nil {
			return nil, err
		}
		secrets, err := s.fetchKayVeeSecrets(25, "", "")
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"service":             "kay-vee",
			"summary":             summary,
			"activity":            activity.Activity,
			"nextToken":           activity.NextToken,
			"parameters":          parameters.Parameters,
			"parametersNextToken": parameters.NextToken,
			"secrets":             secrets.Secrets,
			"secretsNextToken":    secrets.NextToken,
		})
	}

	if view == "drawbridge" {
		summary, err := s.fetchDrawbridgeSummary()
		if err != nil {
			return nil, err
		}
		resources, _ := s.fetchDrawbridgeResources()
		activity, _ := s.fetchDrawbridgeActivity("25", "")
		return json.Marshal(map[string]any{
			"service":   "drawbridge",
			"summary":   summary,
			"resources": resources,
			"activity":  activity.Activity,
			"nextToken": activity.NextToken,
		})
	}

	if view == "scheduler" {
		summary, err := s.fetchSchedulerSummary()
		if err != nil {
			return nil, err
		}
		resources, _ := s.fetchSchedulerResources()
		activity, _ := s.fetchSchedulerActivity("25", "")
		return json.Marshal(map[string]any{
			"service":   "scheduler",
			"summary":   summary,
			"resources": resources,
			"activity":  activity.Activity,
			"nextToken": activity.NextToken,
		})
	}

	if view == "pipes" {
		summary, err := s.fetchPipesSummary()
		if err != nil {
			return nil, err
		}
		resources, _ := s.fetchPipesResources()
		activity, _ := s.fetchPipesActivity("25", "")
		return json.Marshal(map[string]any{
			"service":   "pipes",
			"summary":   summary,
			"resources": resources,
			"activity":  activity.Activity,
			"nextToken": activity.NextToken,
		})
	}

	return json.Marshal(s.buildDashboardSummary())
}

func (s *Server) buildDashboardSummary() DashboardSummary {
	summary := DashboardSummary{UpdatedAt: time.Now().UTC()}
	services := make([]DashboardService, 0, 5)

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

	storageService := DashboardService{
		Name:   "essthree",
		Status: s.checkService("http://essthree:9300/health"),
		Stats: []DashboardStat{
			{Label: "Buckets", Value: 0},
			{Label: "Objects", Value: 0},
		},
	}
	if storageService.Status == "online" {
		if summary, err := s.fetchEssThreeSummary(); err == nil {
			storageService.Stats[0].Value = summary.Stats.Buckets
			storageService.Stats[1].Value = summary.Stats.Objects
		} else {
			s.logger.Warn("failed to fetch essthree dashboard stats", "error", err)
		}
	}
	services = append(services, storageService)

	cdnService := DashboardService{
		Name:   "cloudfauxnt",
		Status: s.checkService("http://cloudfauxnt:9310/health"),
		Stats: []DashboardStat{
			{Label: "Origins", Value: 0},
			{Label: "Behaviors", Value: 0},
			{Label: "Signing", Value: 0},
		},
	}
	if cdnService.Status == "online" {
		if summary, err := s.fetchCloudfauxntSummary(); err == nil {
			cdnService.Stats[0].Value = summary.Stats.Origins
			cdnService.Stats[1].Value = summary.Stats.Behaviors
			if summary.Signing.Enabled {
				cdnService.Stats[2].Value = 1
			}
		} else {
			s.logger.Warn("failed to fetch cloudfauxnt dashboard stats", "error", err)
		}
	}
	services = append(services, cdnService)

	kayVeeService := DashboardService{
		Name:   "kay-vee",
		Status: s.checkService("http://kay-vee:9350/health"),
		Stats: []DashboardStat{
			{Label: "Parameters", Value: 0},
			{Label: "Secrets", Value: 0},
			{Label: "Deleted", Value: 0},
		},
	}
	if kayVeeService.Status == "online" {
		if summary, err := s.fetchKayVeeSummary(); err == nil {
			kayVeeService.Stats[0].Value = summary.Parameters
			kayVeeService.Stats[1].Value = summary.SecretsActive
			kayVeeService.Stats[2].Value = summary.SecretsDeleted
		} else {
			s.logger.Warn("failed to fetch kay-vee dashboard stats", "error", err)
		}
	}
	services = append(services, kayVeeService)

	drawbridgeService := DashboardService{
		Name:   "drawbridge",
		Status: s.checkService("http://drawbridge:9340/health"),
		Stats: []DashboardStat{
			{Label: "Event Buses", Value: 0},
			{Label: "Rules", Value: 0},
			{Label: "Targets", Value: 0},
		},
	}
	if drawbridgeService.Status == "online" {
		if dbSummary, err := s.fetchDrawbridgeSummary(); err == nil {
			drawbridgeService.Stats[0].Value = dbSummary.EventBuses
			drawbridgeService.Stats[1].Value = dbSummary.Rules
			drawbridgeService.Stats[2].Value = dbSummary.Targets
		} else {
			s.logger.Warn("failed to fetch drawbridge dashboard stats", "error", err)
		}
	}
	services = append(services, drawbridgeService)

	schedulerService := DashboardService{
		Name:   "scheduler",
		Status: s.checkService("http://scheduler:9342/health"),
		Stats: []DashboardStat{
			{Label: "Groups", Value: 0},
			{Label: "Schedules", Value: 0},
		},
	}
	if schedulerService.Status == "online" {
		if schSummary, err := s.fetchSchedulerSummary(); err == nil {
			schedulerService.Stats[0].Value = schSummary.ScheduleGroups
			schedulerService.Stats[1].Value = schSummary.Schedules
		} else {
			s.logger.Warn("failed to fetch scheduler dashboard stats", "error", err)
		}
	}
	services = append(services, schedulerService)

	pipesService := DashboardService{
		Name:   "pipes",
		Status: s.checkService("http://pipes:9344/health"),
		Stats: []DashboardStat{
			{Label: "Pipes", Value: 0},
			{Label: "Running", Value: 0},
		},
	}
	if pipesService.Status == "online" {
		if pipesSummary, err := s.fetchPipesSummary(); err == nil {
			pipesService.Stats[0].Value = pipesSummary.Pipes
			pipesService.Stats[1].Value = pipesSummary.RunningPipes
		} else {
			s.logger.Warn("failed to fetch pipes dashboard stats", "error", err)
		}
	}
	services = append(services, pipesService)

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

func (s *Server) fetchEssThreeSummary() (EssThreeSummaryResponse, error) {
	resp, err := s.client.Get("http://essthree:9300/admin/api/buckets")
	if err != nil {
		return EssThreeSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return EssThreeSummaryResponse{}, fmt.Errorf("essthree admin status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Buckets []EssThreeBucketSummary `json:"buckets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return EssThreeSummaryResponse{}, err
	}

	summary := EssThreeSummaryResponse{
		Service: "essthree",
		Buckets: payload.Buckets,
	}
	summary.Stats.Buckets = len(payload.Buckets)
	for _, bucket := range payload.Buckets {
		summary.Stats.Objects += bucket.ObjectCount
	}

	return summary, nil
}

func (s *Server) fetchEssThreeObjectList(bucket, prefix, maxKeys, continuationToken string) (EssThreeObjectListResponse, error) {
	objectURL := fmt.Sprintf("http://essthree:9300/admin/api/buckets/%s/objects", url.PathEscape(bucket))
	q := url.Values{}
	if prefix != "" {
		q.Set("prefix", prefix)
	}
	if maxKeys != "" {
		q.Set("maxKeys", maxKeys)
	}
	if continuationToken != "" {
		q.Set("continuationToken", continuationToken)
	}
	if len(q) > 0 {
		objectURL += "?" + q.Encode()
	}

	resp, err := s.client.Get(objectURL)
	if err != nil {
		return EssThreeObjectListResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return EssThreeObjectListResponse{}, fmt.Errorf("essthree object list status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var objectList EssThreeObjectListResponse
	if err := json.NewDecoder(resp.Body).Decode(&objectList); err != nil {
		return EssThreeObjectListResponse{}, err
	}
	return objectList, nil
}

func (s *Server) fetchEssThreeActivity(maxResults, nextToken string) (EssThreeActivityResponse, error) {
	activityURL := "http://essthree:9300/admin/api/activity?maxResults=" + url.QueryEscape(maxResults)
	if nextToken != "" {
		activityURL += "&nextToken=" + url.QueryEscape(nextToken)
	}

	resp, err := s.client.Get(activityURL)
	if err != nil {
		return EssThreeActivityResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return EssThreeActivityResponse{}, fmt.Errorf("essthree activity status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var activity EssThreeActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return EssThreeActivityResponse{}, err
	}
	return activity, nil
}

func (s *Server) fetchCloudfauxntSummary() (CloudfauxntSummaryResponse, error) {
	resp, err := s.client.Get("http://cloudfauxnt:9310/admin/api/overview")
	if err != nil {
		return CloudfauxntSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return CloudfauxntSummaryResponse{}, fmt.Errorf("cloudfauxnt admin status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var summary CloudfauxntSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return CloudfauxntSummaryResponse{}, err
	}

	if strings.TrimSpace(summary.Service) == "" {
		summary.Service = "cloudfauxnt"
	}

	return summary, nil
}

func (s *Server) callCloudfauxntAdminJSON(method, endpoint string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, "http://cloudfauxnt:9310"+endpoint, body)
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
		message, _ := io.ReadAll(resp.Body)
		trimmed := strings.TrimSpace(string(message))
		if trimmed == "" {
			trimmed = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("cloudfauxnt admin status %d: %s", resp.StatusCode, trimmed)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) fetchKayVeeSummary() (KayVeeSummaryResponse, error) {
	resp, err := s.client.Get("http://kay-vee:9350/admin/api/summary")
	if err != nil {
		return KayVeeSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return KayVeeSummaryResponse{}, fmt.Errorf("kay-vee admin summary status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var summary KayVeeSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return KayVeeSummaryResponse{}, err
	}

	if strings.TrimSpace(summary.Service) == "" {
		summary.Service = "kay-vee"
	}

	return summary, nil
}

func (s *Server) fetchServiceActivity(baseURL, serviceName, maxResults, nextToken string) (ServiceActivityResponse, error) {
	activityURL := baseURL + "/admin/api/activity?maxResults=" + url.QueryEscape(maxResults)
	if nextToken != "" {
		activityURL += "&nextToken=" + url.QueryEscape(nextToken)
	}

	resp, err := s.client.Get(activityURL)
	if err != nil {
		return ServiceActivityResponse{}, fmt.Errorf("failed to fetch %s activity: %w", serviceName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return ServiceActivityResponse{}, fmt.Errorf("%s admin activity status %d: %s", serviceName, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result ServiceActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ServiceActivityResponse{}, err
	}

	return result, nil
}

func (s *Server) fetchKayVeeActivity(maxResults, nextToken string) (KayVeeActivityResponse, error) {
	activityURL := "http://kay-vee:9350/admin/api/activity?maxResults=" + url.QueryEscape(maxResults)
	if nextToken != "" {
		activityURL += "&nextToken=" + url.QueryEscape(nextToken)
	}

	resp, err := s.client.Get(activityURL)
	if err != nil {
		return KayVeeActivityResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return KayVeeActivityResponse{}, fmt.Errorf("kay-vee admin activity status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var activity KayVeeActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return KayVeeActivityResponse{}, err
	}

	return activity, nil
}

func (s *Server) fetchKayVeeParametersByPath(path string, recursive, withDecryption bool, maxResults int, nextToken, typeFilter, labelFilter string) (KayVeeParametersResponse, error) {
	resources, err := s.fetchKayVeeAdminResources(
		path,
		recursive,
		withDecryption,
		maxResults,
		nextToken,
		typeFilter,
		labelFilter,
		0,
		"",
		"",
		true,
		false,
	)
	if err != nil {
		if !isKayVeeAdminResourcesUnavailable(err) {
			return KayVeeParametersResponse{}, err
		}
		return s.fetchKayVeeParametersByPathLegacy(path, recursive, withDecryption, maxResults, nextToken, typeFilter, labelFilter)
	}

	return KayVeeParametersResponse{Parameters: resources.Parameters, NextToken: resources.ParametersNextToken}, nil
}

func (s *Server) fetchKayVeeSecrets(maxResults int, nextToken, nameFilter string) (KayVeeSecretsResponse, error) {
	resources, err := s.fetchKayVeeAdminResources(
		"/",
		true,
		false,
		0,
		"",
		"",
		"",
		maxResults,
		nextToken,
		nameFilter,
		false,
		true,
	)
	if err != nil {
		if !isKayVeeAdminResourcesUnavailable(err) {
			return KayVeeSecretsResponse{}, err
		}
		return s.fetchKayVeeSecretsLegacy(maxResults, nextToken, nameFilter)
	}

	return KayVeeSecretsResponse{Secrets: resources.Secrets, NextToken: resources.SecretsNextToken}, nil
}

func (s *Server) fetchKayVeeParametersByPathLegacy(path string, recursive, withDecryption bool, maxResults int, nextToken, typeFilter, labelFilter string) (KayVeeParametersResponse, error) {
	payload := map[string]any{
		"Path":           path,
		"Recursive":      recursive,
		"WithDecryption": withDecryption,
		"MaxResults":     maxResults,
	}
	if strings.TrimSpace(nextToken) != "" {
		payload["NextToken"] = strings.TrimSpace(nextToken)
	}

	parameterFilters := make([]map[string]any, 0, 2)
	if strings.TrimSpace(typeFilter) != "" {
		parameterFilters = append(parameterFilters, map[string]any{"Key": "Type", "Option": "Equals", "Values": []string{strings.TrimSpace(typeFilter)}})
	}
	if strings.TrimSpace(labelFilter) != "" {
		parameterFilters = append(parameterFilters, map[string]any{"Key": "Label", "Option": "Equals", "Values": []string{strings.TrimSpace(labelFilter)}})
	}
	if len(parameterFilters) > 0 {
		payload["ParameterFilters"] = parameterFilters
	}

	var upstream struct {
		Parameters []KayVeeParameter `json:"Parameters"`
		NextToken  string            `json:"NextToken,omitempty"`
	}
	if err := s.callKayVeeTarget("AmazonSSM.GetParametersByPath", payload, &upstream); err != nil {
		return KayVeeParametersResponse{}, err
	}

	return KayVeeParametersResponse{Parameters: upstream.Parameters, NextToken: upstream.NextToken}, nil
}

func (s *Server) fetchKayVeeSecretsLegacy(maxResults int, nextToken, nameFilter string) (KayVeeSecretsResponse, error) {
	payload := map[string]any{"MaxResults": maxResults}
	if strings.TrimSpace(nextToken) != "" {
		payload["NextToken"] = strings.TrimSpace(nextToken)
	}
	if strings.TrimSpace(nameFilter) != "" {
		payload["Filters"] = []map[string]any{{"Key": "name", "Values": []string{strings.TrimSpace(nameFilter)}}}
	}

	var upstream struct {
		SecretList []KayVeeSecretEntry `json:"SecretList"`
		NextToken  string              `json:"NextToken,omitempty"`
	}
	if err := s.callKayVeeTarget("secretsmanager.ListSecrets", payload, &upstream); err != nil {
		return KayVeeSecretsResponse{}, err
	}

	return KayVeeSecretsResponse{Secrets: upstream.SecretList, NextToken: upstream.NextToken}, nil
}

func isKayVeeAdminResourcesUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "admin resources status 404") ||
		strings.Contains(message, "admin resources status 405") ||
		strings.Contains(message, "unsupported target")
}

func (s *Server) fetchKayVeeAdminResources(
	parameterPath string,
	recursive bool,
	withDecryption bool,
	parameterMaxResults int,
	parametersNextToken string,
	parameterType string,
	parameterLabel string,
	secretMaxResults int,
	secretsNextToken string,
	secretName string,
	includeParameters bool,
	includeSecrets bool,
) (KayVeeAdminResourcesResponse, error) {
	query := url.Values{}
	query.Set("includeParameters", strconv.FormatBool(includeParameters))
	query.Set("includeSecrets", strconv.FormatBool(includeSecrets))

	if strings.TrimSpace(parameterPath) != "" {
		query.Set("parameterPath", strings.TrimSpace(parameterPath))
	}
	query.Set("recursive", strconv.FormatBool(recursive))
	query.Set("withDecryption", strconv.FormatBool(withDecryption))

	if parameterMaxResults > 0 {
		query.Set("parameterMaxResults", strconv.Itoa(parameterMaxResults))
	}
	if strings.TrimSpace(parametersNextToken) != "" {
		query.Set("parametersNextToken", strings.TrimSpace(parametersNextToken))
	}
	if strings.TrimSpace(parameterType) != "" {
		query.Set("parameterType", strings.TrimSpace(parameterType))
	}
	if strings.TrimSpace(parameterLabel) != "" {
		query.Set("parameterLabel", strings.TrimSpace(parameterLabel))
	}

	if secretMaxResults > 0 {
		query.Set("secretMaxResults", strconv.Itoa(secretMaxResults))
	}
	if strings.TrimSpace(secretsNextToken) != "" {
		query.Set("secretsNextToken", strings.TrimSpace(secretsNextToken))
	}
	if strings.TrimSpace(secretName) != "" {
		query.Set("secretName", strings.TrimSpace(secretName))
	}

	requestURL := "http://kay-vee:9350/admin/api/resources"
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	resp, err := s.client.Get(requestURL)
	if err != nil {
		return KayVeeAdminResourcesResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return KayVeeAdminResourcesResponse{}, fmt.Errorf("kay-vee admin resources status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var resources KayVeeAdminResourcesResponse
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return KayVeeAdminResourcesResponse{}, err
	}

	return resources, nil
}

func (s *Server) callKayVeeTarget(targetName string, payload any, target any) error {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "http://kay-vee:9350/", bytes.NewReader(encodedPayload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-amz-json-1.1")
	request.Header.Set("X-Amz-Target", targetName)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		var awsErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(responseBody, &awsErr); err == nil && strings.TrimSpace(awsErr.Type) != "" {
			return fmt.Errorf("kay-vee target %s failed (%d): %s: %s", targetName, response.StatusCode, awsErr.Type, awsErr.Message)
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("kay-vee target %s failed (%d): %s", targetName, response.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// --- Scheduler handlers ---

func (s *Server) handleSchedulerSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchSchedulerSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleSchedulerResources(w http.ResponseWriter, _ *http.Request) {
	resources, err := s.fetchSchedulerResources()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) handleSchedulerActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := r.URL.Query().Get("maxResults")
	if maxResults == "" {
		maxResults = "50"
	}
	nextToken := r.URL.Query().Get("nextToken")
	activity, err := s.fetchSchedulerActivity(maxResults, nextToken)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

func (s *Server) fetchSchedulerSummary() (SchedulerSummaryResponse, error) {
	resp, err := s.client.Get("http://scheduler:9342/admin/api/summary")
	if err != nil {
		return SchedulerSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return SchedulerSummaryResponse{}, fmt.Errorf("scheduler summary status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var summary SchedulerSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return SchedulerSummaryResponse{}, err
	}
	return summary, nil
}

func (s *Server) fetchSchedulerResources() ([]SchedulerGroupDetail, error) {
	resp, err := s.client.Get("http://scheduler:9342/admin/api/resources")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scheduler resources status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var resources []SchedulerGroupDetail
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}
	return resources, nil
}

func (s *Server) fetchSchedulerActivity(maxResults, nextToken string) (SchedulerActivityResponse, error) {
	u := fmt.Sprintf("http://scheduler:9342/admin/api/activity?maxResults=%s", url.QueryEscape(maxResults))
	if nextToken != "" {
		u += "&nextToken=" + url.QueryEscape(nextToken)
	}
	resp, err := s.client.Get(u)
	if err != nil {
		return SchedulerActivityResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return SchedulerActivityResponse{}, fmt.Errorf("scheduler activity status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var activity SchedulerActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return SchedulerActivityResponse{}, err
	}
	return activity, nil
}

func (s *Server) handleSchedulerCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req SchedulerCreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callSchedulerTarget("AWSScheduler.CreateScheduleGroup", map[string]any{"Name": name}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handleSchedulerDeleteGroup(w http.ResponseWriter, r *http.Request) {
	var req SchedulerDeleteGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	if name == "default" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("cannot delete the default schedule group"))
		return
	}

	if err := s.callSchedulerTarget("AWSScheduler.DeleteScheduleGroup", map[string]any{"Name": name}, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSchedulerCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req SchedulerCreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	schedExpr := strings.TrimSpace(req.ScheduleExpression)
	if schedExpr == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("schedule_expression is required"))
		return
	}

	payload := map[string]any{
		"Name":               name,
		"ScheduleExpression": schedExpr,
	}
	if group := strings.TrimSpace(req.GroupName); group != "" {
		payload["GroupName"] = group
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["Description"] = desc
	}
	state := strings.TrimSpace(req.State)
	if state == "" {
		state = "ENABLED"
	}
	payload["State"] = state
	if aac := strings.TrimSpace(req.ActionAfterCompletion); aac != "" {
		payload["ActionAfterCompletion"] = aac
	}

	targetArn := strings.TrimSpace(req.TargetArn)
	if targetArn != "" {
		target := map[string]any{"Arn": targetArn}
		if input := strings.TrimSpace(req.TargetInput); input != "" {
			target["Input"] = input
		}
		payload["Target"] = target
	}

	if err := s.callSchedulerTarget("AWSScheduler.CreateSchedule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handleSchedulerDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	var req SchedulerDeleteScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	payload := map[string]any{"Name": name}
	if group := strings.TrimSpace(req.GroupName); group != "" {
		payload["GroupName"] = group
	}

	if err := s.callSchedulerTarget("AWSScheduler.DeleteSchedule", payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) callSchedulerTarget(targetName string, payload any, target any) error {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "http://scheduler:9342/", bytes.NewReader(encodedPayload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-amz-json-1.1")
	request.Header.Set("X-Amz-Target", targetName)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		var awsErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(responseBody, &awsErr); err == nil && strings.TrimSpace(awsErr.Type) != "" {
			return fmt.Errorf("%s: %s", awsErr.Type, awsErr.Message)
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("scheduler target %s failed (%d): %s", targetName, response.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (s *Server) callDrawbridgeTarget(targetName string, payload any, target any) error {
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, "http://drawbridge:9340/", bytes.NewReader(encodedPayload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-amz-json-1.1")
	request.Header.Set("X-Amz-Target", targetName)

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		var awsErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(responseBody, &awsErr); err == nil && strings.TrimSpace(awsErr.Type) != "" {
			return fmt.Errorf("%s: %s", awsErr.Type, awsErr.Message)
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("drawbridge target %s failed (%d): %s", targetName, response.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// --- Pipes handlers ---

func (s *Server) handlePipesSummary(w http.ResponseWriter, _ *http.Request) {
	summary, err := s.fetchPipesSummary()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handlePipesResources(w http.ResponseWriter, _ *http.Request) {
	resources, err := s.fetchPipesResources()
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) handlePipesActivity(w http.ResponseWriter, r *http.Request) {
	maxResults := r.URL.Query().Get("maxResults")
	if maxResults == "" {
		maxResults = "50"
	}
	nextToken := r.URL.Query().Get("nextToken")
	activity, err := s.fetchPipesActivity(maxResults, nextToken)
	if err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

func (s *Server) handlePipesCreatePipe(w http.ResponseWriter, r *http.Request) {
	var req PipesCreatePipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("source is required"))
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("target is required"))
		return
	}

	roleArn := strings.TrimSpace(req.RoleArn)
	if roleArn == "" {
		roleArn = "arn:aws:iam::000000000000:role/pipe-role"
	}

	payload := map[string]any{
		"Source":  source,
		"Target":  target,
		"RoleArn": roleArn,
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["Description"] = desc
	}
	if req.State == "STOPPED" {
		payload["DesiredState"] = "STOPPED"
	}
	if filter := strings.TrimSpace(req.Filter); filter != "" {
		payload["SourceParameters"] = map[string]any{
			"FilterCriteria": map[string]any{
				"Filters": []map[string]any{
					{"Pattern": filter},
				},
			},
		}
	}
	if enrich := strings.TrimSpace(req.Enrichment); enrich != "" {
		payload["Enrichment"] = enrich
	}

	if err := s.callPipesREST(http.MethodPost, "/v1/pipes/"+name, payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handlePipesUpdatePipe(w http.ResponseWriter, r *http.Request) {
	var req PipesUpdatePipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	roleArn := strings.TrimSpace(req.RoleArn)
	if roleArn == "" {
		roleArn = "arn:aws:iam::000000000000:role/pipe-role"
	}

	payload := map[string]any{
		"RoleArn": roleArn,
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		payload["Description"] = desc
	}
	if target := strings.TrimSpace(req.Target); target != "" {
		payload["Target"] = target
	}
	if req.State != "" {
		payload["DesiredState"] = req.State
	}

	if err := s.callPipesREST(http.MethodPut, "/v1/pipes/"+name, payload, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handlePipesDeletePipe(w http.ResponseWriter, r *http.Request) {
	var req PipesDeletePipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callPipesREST(http.MethodDelete, "/v1/pipes/"+name, nil, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handlePipesStartPipe(w http.ResponseWriter, r *http.Request) {
	var req PipesStartStopPipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callPipesREST(http.MethodPost, "/v1/pipes/"+name+"/start", nil, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) handlePipesStopPipe(w http.ResponseWriter, r *http.Request) {
	var req PipesStartStopPipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		awserrors.WriteJSONGeneric(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}

	if err := s.callPipesREST(http.MethodPost, "/v1/pipes/"+name+"/stop", nil, nil); err != nil {
		awserrors.WriteJSONGeneric(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name})
}

func (s *Server) fetchPipesSummary() (PipesSummaryResponse, error) {
	resp, err := s.client.Get("http://pipes:9344/admin/api/summary")
	if err != nil {
		return PipesSummaryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return PipesSummaryResponse{}, fmt.Errorf("pipes summary status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var summary PipesSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return PipesSummaryResponse{}, err
	}
	return summary, nil
}

func (s *Server) fetchPipesResources() ([]PipesPipeDetail, error) {
	resp, err := s.client.Get("http://pipes:9344/admin/api/resources")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pipes resources status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var resources []PipesPipeDetail
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}
	return resources, nil
}

func (s *Server) fetchPipesActivity(maxResults, nextToken string) (PipesActivityResponse, error) {
	u := fmt.Sprintf("http://pipes:9344/admin/api/activity?maxResults=%s", url.QueryEscape(maxResults))
	if nextToken != "" {
		u += "&nextToken=" + url.QueryEscape(nextToken)
	}
	resp, err := s.client.Get(u)
	if err != nil {
		return PipesActivityResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return PipesActivityResponse{}, fmt.Errorf("pipes activity status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var activity PipesActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return PipesActivityResponse{}, err
	}
	return activity, nil
}

func (s *Server) callPipesREST(method, path string, body any, target any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, "http://pipes:9344"+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		var awsErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(responseBody, &awsErr); err == nil && strings.TrimSpace(awsErr.Type) != "" {
			return fmt.Errorf("%s: %s", awsErr.Type, awsErr.Message)
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("pipes %s %s failed (%d): %s", method, path, response.StatusCode, message)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
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

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

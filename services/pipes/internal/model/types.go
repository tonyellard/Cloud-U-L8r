// SPDX-License-Identifier: Apache-2.0
package model

import "time"

// Pipe represents an EventBridge Pipe.
type Pipe struct {
	Name                 string            `json:"Name"`
	Arn                  string            `json:"Arn"`
	Description          string            `json:"Description,omitempty"`
	Source               string            `json:"Source"`
	Target               string            `json:"Target"`
	RoleArn              string            `json:"RoleArn"`
	DesiredState         string            `json:"DesiredState"`
	CurrentState         string            `json:"CurrentState"`
	SourceParameters     *SourceParameters `json:"SourceParameters,omitempty"`
	TargetParameters     *TargetParameters `json:"TargetParameters,omitempty"`
	FilterCriteria       *FilterCriteria   `json:"FilterCriteria,omitempty"`
	Enrichment           string            `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichParameters `json:"EnrichmentParameters,omitempty"`
	Tags                 map[string]string `json:"Tags,omitempty"`
	CreationTime         time.Time         `json:"CreationTime"`
	LastModifiedTime     time.Time         `json:"LastModifiedTime"`
}

// SourceParameters configures the pipe source.
type SourceParameters struct {
	FilterCriteria     *FilterCriteria     `json:"FilterCriteria,omitempty"`
	SqsQueueParameters *SqsQueueParameters `json:"SqsQueueParameters,omitempty"`
}

// SqsQueueParameters configures SQS source polling.
type SqsQueueParameters struct {
	BatchSize                      int `json:"BatchSize,omitempty"`
	MaximumBatchingWindowInSeconds int `json:"MaximumBatchingWindowInSeconds,omitempty"`
}

// TargetParameters configures the pipe target.
type TargetParameters struct {
	SqsQueueParameters          *SqsTargetParameters          `json:"SqsQueueParameters,omitempty"`
	EventBridgeEventBusParameters *EventBridgeEventBusParameters `json:"EventBridgeEventBusParameters,omitempty"`
	HttpParameters               *HttpParameters                `json:"HttpParameters,omitempty"`
	InputTemplate                string                         `json:"InputTemplate,omitempty"`
}

// SqsTargetParameters configures SQS target delivery.
type SqsTargetParameters struct {
	MessageGroupId         string `json:"MessageGroupId,omitempty"`
	MessageDeduplicationId string `json:"MessageDeduplicationId,omitempty"`
}

// EventBridgeEventBusParameters configures EventBridge target delivery.
type EventBridgeEventBusParameters struct {
	DetailType string   `json:"DetailType,omitempty"`
	Source     string   `json:"Source,omitempty"`
	Resources  []string `json:"Resources,omitempty"`
}

// HttpParameters configures HTTP target or enrichment delivery.
type HttpParameters struct {
	HeaderParameters      map[string]string `json:"HeaderParameters,omitempty"`
	PathParameterValues   []string          `json:"PathParameterValues,omitempty"`
	QueryStringParameters map[string]string `json:"QueryStringParameters,omitempty"`
}

// FilterCriteria holds event filters for the pipe.
type FilterCriteria struct {
	Filters []Filter `json:"Filters"`
}

// Filter holds a single event pattern.
type Filter struct {
	Pattern string `json:"Pattern"`
}

// EnrichParameters configures enrichment.
type EnrichParameters struct {
	HttpParameters *HttpParameters `json:"HttpParameters,omitempty"`
	InputTemplate  string          `json:"InputTemplate,omitempty"`
}

// ── REST API request/response types ──

// CreatePipeRequest is the body for POST /v1/pipes/{Name}.
type CreatePipeRequest struct {
	Description          string            `json:"Description,omitempty"`
	DesiredState         string            `json:"DesiredState,omitempty"`
	Enrichment           string            `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichParameters `json:"EnrichmentParameters,omitempty"`
	RoleArn              string            `json:"RoleArn"`
	Source               string            `json:"Source"`
	SourceParameters     *SourceParameters `json:"SourceParameters,omitempty"`
	Tags                 map[string]string `json:"Tags,omitempty"`
	Target               string            `json:"Target"`
	TargetParameters     *TargetParameters `json:"TargetParameters,omitempty"`
}

// CreatePipeResponse is returned from POST /v1/pipes/{Name}.
type CreatePipeResponse struct {
	Arn              string  `json:"Arn"`
	CreationTime     float64 `json:"CreationTime"`
	CurrentState     string  `json:"CurrentState"`
	DesiredState     string  `json:"DesiredState"`
	LastModifiedTime float64 `json:"LastModifiedTime"`
	Name             string  `json:"Name"`
}

// DescribePipeResponse is returned from GET /v1/pipes/{Name}.
type DescribePipeResponse struct {
	Arn                  string            `json:"Arn"`
	CreationTime         float64           `json:"CreationTime"`
	CurrentState         string            `json:"CurrentState"`
	Description          string            `json:"Description,omitempty"`
	DesiredState         string            `json:"DesiredState"`
	Enrichment           string            `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichParameters `json:"EnrichmentParameters,omitempty"`
	LastModifiedTime     float64           `json:"LastModifiedTime"`
	Name                 string            `json:"Name"`
	RoleArn              string            `json:"RoleArn"`
	Source               string            `json:"Source"`
	SourceParameters     *SourceParameters `json:"SourceParameters"`
	Tags                 map[string]string `json:"Tags,omitempty"`
	Target               string            `json:"Target"`
	TargetParameters     *TargetParameters `json:"TargetParameters"`
}

// UpdatePipeRequest is the body for PUT /v1/pipes/{Name}.
type UpdatePipeRequest struct {
	Description          string            `json:"Description,omitempty"`
	DesiredState         string            `json:"DesiredState,omitempty"`
	Enrichment           string            `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichParameters `json:"EnrichmentParameters,omitempty"`
	RoleArn              string            `json:"RoleArn"`
	SourceParameters     *SourceParameters `json:"SourceParameters,omitempty"`
	Target               string            `json:"Target,omitempty"`
	TargetParameters     *TargetParameters `json:"TargetParameters,omitempty"`
}

// UpdatePipeResponse is returned from PUT /v1/pipes/{Name}.
type UpdatePipeResponse struct {
	Arn              string  `json:"Arn"`
	CreationTime     float64 `json:"CreationTime"`
	CurrentState     string  `json:"CurrentState"`
	DesiredState     string  `json:"DesiredState"`
	LastModifiedTime float64 `json:"LastModifiedTime"`
	Name             string  `json:"Name"`
}

// DeletePipeResponse is returned from DELETE /v1/pipes/{Name}.
type DeletePipeResponse struct {
	Arn              string  `json:"Arn"`
	CreationTime     float64 `json:"CreationTime"`
	CurrentState     string  `json:"CurrentState"`
	DesiredState     string  `json:"DesiredState"`
	LastModifiedTime float64 `json:"LastModifiedTime"`
	Name             string  `json:"Name"`
}

// ListPipesResponse is returned from GET /v1/pipes.
type ListPipesResponse struct {
	NextToken string        `json:"NextToken,omitempty"`
	Pipes     []PipeSummary `json:"Pipes"`
}

// PipeSummary is used in list responses.
type PipeSummary struct {
	Arn              string  `json:"Arn"`
	CreationTime     float64 `json:"CreationTime"`
	CurrentState     string  `json:"CurrentState"`
	DesiredState     string  `json:"DesiredState"`
	Enrichment       string  `json:"Enrichment,omitempty"`
	LastModifiedTime float64 `json:"LastModifiedTime"`
	Name             string  `json:"Name"`
	Source           string  `json:"Source"`
	Target           string  `json:"Target"`
}

// ── Admin types ──

// AdminSummaryResponse is returned by /admin/api/summary.
type AdminSummaryResponse struct {
	Service      string `json:"service"`
	Pipes        int    `json:"pipes"`
	RunningPipes int    `json:"runningPipes"`
}

// AdminPipeDetail is returned by /admin/api/resources.
type AdminPipeDetail struct {
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

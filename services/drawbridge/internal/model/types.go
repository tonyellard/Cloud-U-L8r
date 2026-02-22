// SPDX-License-Identifier: Apache-2.0
package model

import "time"

// --- Event Bus ---

type EventBus struct {
	Name         string    `json:"Name"`
	Arn          string    `json:"Arn"`
	State        string    `json:"State,omitempty"`
	Description  string    `json:"Description,omitempty"`
	CreationTime float64   `json:"CreationTime,omitempty"`
	CreatedAt    time.Time `json:"-"`
}

type CreateEventBusRequest struct {
	Name        string `json:"Name"`
	Description string `json:"Description,omitempty"`
}

type CreateEventBusResponse struct {
	EventBusArn string `json:"EventBusArn"`
}

type DescribeEventBusRequest struct {
	Name string `json:"Name"`
}

type DescribeEventBusResponse struct {
	Name string `json:"Name"`
	Arn  string `json:"Arn"`
}

type ListEventBusesRequest struct {
	Limit     int    `json:"Limit,omitempty"`
	NextToken string `json:"NextToken,omitempty"`
	NamePrefix string `json:"NamePrefix,omitempty"`
}

type ListEventBusesResponse struct {
	EventBuses []EventBus `json:"EventBuses"`
	NextToken  string     `json:"NextToken,omitempty"`
}

type DeleteEventBusRequest struct {
	Name string `json:"Name"`
}

// --- Rule ---

type Rule struct {
	Name               string `json:"Name"`
	Arn                string `json:"Arn,omitempty"`
	EventBusName       string `json:"EventBusName,omitempty"`
	EventPattern       string `json:"EventPattern,omitempty"`
	ScheduleExpression string `json:"ScheduleExpression,omitempty"`
	State              string `json:"State,omitempty"`
	Description        string `json:"Description,omitempty"`
}

type PutRuleRequest struct {
	Name               string `json:"Name"`
	EventBusName       string `json:"EventBusName,omitempty"`
	EventPattern       string `json:"EventPattern,omitempty"`
	ScheduleExpression string `json:"ScheduleExpression,omitempty"`
	State              string `json:"State,omitempty"`
	Description        string `json:"Description,omitempty"`
}

type PutRuleResponse struct {
	RuleArn string `json:"RuleArn"`
}

type DescribeRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
}

type ListRulesRequest struct {
	EventBusName string `json:"EventBusName,omitempty"`
	Limit        int    `json:"Limit,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
	NamePrefix   string `json:"NamePrefix,omitempty"`
}

type ListRulesResponse struct {
	Rules     []Rule `json:"Rules"`
	NextToken string `json:"NextToken,omitempty"`
}

type DeleteRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
}

type EnableRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
}

type DisableRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
}

// --- Target ---

type Target struct {
	Id               string            `json:"Id"`
	Arn              string            `json:"Arn"`
	Input            string            `json:"Input,omitempty"`
	InputPath        string            `json:"InputPath,omitempty"`
	InputTransformer *InputTransformer `json:"InputTransformer,omitempty"`
	SqsParameters    *SqsParameters   `json:"SqsParameters,omitempty"`
}

type InputTransformer struct {
	InputPathsMap map[string]string `json:"InputPathsMap,omitempty"`
	InputTemplate string            `json:"InputTemplate"`
}

type SqsParameters struct {
	MessageGroupId string `json:"MessageGroupId,omitempty"`
}

type PutTargetsRequest struct {
	Rule         string   `json:"Rule"`
	EventBusName string   `json:"EventBusName,omitempty"`
	Targets      []Target `json:"Targets"`
}

type PutTargetsResponse struct {
	FailedEntryCount int                    `json:"FailedEntryCount"`
	FailedEntries    []PutTargetsResultEntry `json:"FailedEntries,omitempty"`
}

type PutTargetsResultEntry struct {
	TargetId  string `json:"TargetId"`
	ErrorCode string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

type ListTargetsByRuleRequest struct {
	Rule         string `json:"Rule"`
	EventBusName string `json:"EventBusName,omitempty"`
	Limit        int    `json:"Limit,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
}

type ListTargetsByRuleResponse struct {
	Targets   []Target `json:"Targets"`
	NextToken string   `json:"NextToken,omitempty"`
}

type RemoveTargetsRequest struct {
	Rule         string   `json:"Rule"`
	EventBusName string   `json:"EventBusName,omitempty"`
	Ids          []string `json:"Ids"`
}

type RemoveTargetsResponse struct {
	FailedEntryCount int                       `json:"FailedEntryCount"`
	FailedEntries    []RemoveTargetsResultEntry `json:"FailedEntries,omitempty"`
}

type RemoveTargetsResultEntry struct {
	TargetId     string `json:"TargetId"`
	ErrorCode    string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

// --- Events ---

type PutEventsRequest struct {
	Entries []PutEventsRequestEntry `json:"Entries"`
}

type PutEventsRequestEntry struct {
	Source       string   `json:"Source"`
	DetailType   string   `json:"DetailType"`
	Detail       string   `json:"Detail"`
	EventBusName string   `json:"EventBusName,omitempty"`
	Resources    []string `json:"Resources,omitempty"`
	Time         *float64 `json:"Time,omitempty"`
}

type PutEventsResponse struct {
	FailedEntryCount int                      `json:"FailedEntryCount"`
	Entries          []PutEventsResultEntry   `json:"Entries"`
}

type PutEventsResultEntry struct {
	EventId      string `json:"EventId,omitempty"`
	ErrorCode    string `json:"ErrorCode,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

type TestEventPatternRequest struct {
	EventPattern string `json:"EventPattern"`
	Event        string `json:"Event"`
}

type TestEventPatternResponse struct {
	Result bool `json:"Result"`
}

// --- Admin ---

type AdminSummaryResponse struct {
	Service    string `json:"service"`
	EventBuses int    `json:"eventBuses"`
	Rules      int    `json:"rules"`
	Targets    int    `json:"targets"`
}

type AdminBusDetail struct {
	Name        string           `json:"name"`
	Arn         string           `json:"arn"`
	RuleCount   int              `json:"ruleCount"`
	TargetCount int              `json:"targetCount"`
	Rules       []AdminRuleDetail `json:"rules"`
}

type AdminRuleDetail struct {
	Name               string   `json:"name"`
	State              string   `json:"state"`
	EventPattern       string   `json:"eventPattern,omitempty"`
	ScheduleExpression string   `json:"scheduleExpression,omitempty"`
	TargetCount        int      `json:"targetCount"`
	Targets            []Target `json:"targets"`
}

type AdminExportResponse struct {
	EventBuses []AdminBusExport `json:"eventBuses"`
}

type AdminBusExport struct {
	Name  string            `json:"name"`
	Rules []AdminRuleExport `json:"rules"`
}

type AdminRuleExport struct {
	Name               string   `json:"name"`
	EventPattern       string   `json:"eventPattern,omitempty"`
	ScheduleExpression string   `json:"scheduleExpression,omitempty"`
	State              string   `json:"state"`
	Description        string   `json:"description,omitempty"`
	Targets            []Target `json:"targets"`
}

type AdminImportRequest struct {
	EventBuses []AdminBusExport `json:"eventBuses"`
}

type AdminImportResponse struct {
	Imported int `json:"imported"`
}

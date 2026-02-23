// SPDX-License-Identifier: Apache-2.0
package model

import "time"

// ScheduleGroup represents a schedule group.
type ScheduleGroup struct {
	Name         string    `json:"Name"`
	Arn          string    `json:"Arn"`
	State        string    `json:"State"`
	CreationDate time.Time `json:"CreationDate"`
}

// Schedule represents a single schedule.
type Schedule struct {
	Name                       string             `json:"Name"`
	Arn                        string              `json:"Arn,omitempty"`
	GroupName                  string              `json:"GroupName,omitempty"`
	ScheduleExpression         string              `json:"ScheduleExpression"`
	ScheduleExpressionTimezone string              `json:"ScheduleExpressionTimezone,omitempty"`
	State                      string              `json:"State,omitempty"`
	Description                string              `json:"Description,omitempty"`
	FlexibleTimeWindow         *FlexibleTimeWindow `json:"FlexibleTimeWindow,omitempty"`
	Target                     *ScheduleTarget     `json:"Target,omitempty"`
	StartDate                  *time.Time          `json:"StartDate,omitempty"`
	EndDate                    *time.Time          `json:"EndDate,omitempty"`
	ActionAfterCompletion      string              `json:"ActionAfterCompletion,omitempty"`
	CreationDate               time.Time           `json:"CreationDate,omitempty"`
	LastModificationDate       time.Time           `json:"LastModificationDate,omitempty"`
}

// ScheduleTarget represents the target for a schedule.
type ScheduleTarget struct {
	Arn              string            `json:"Arn"`
	RoleArn          string            `json:"RoleArn,omitempty"`
	Input            string            `json:"Input,omitempty"`
	RetryPolicy      *RetryPolicy      `json:"RetryPolicy,omitempty"`
	DeadLetterConfig *DeadLetterConfig `json:"DeadLetterConfig,omitempty"`
	SqsParameters    *SqsParameters    `json:"SqsParameters,omitempty"`
}

// FlexibleTimeWindow configures scheduling flexibility.
type FlexibleTimeWindow struct {
	Mode                     string `json:"Mode"`
	MaximumWindowInMinutes   int    `json:"MaximumWindowInMinutes,omitempty"`
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
	MaximumRetryAttempts    int `json:"MaximumRetryAttempts,omitempty"`
	MaximumEventAgeInSeconds int `json:"MaximumEventAgeInSeconds,omitempty"`
}

// DeadLetterConfig configures the dead-letter queue.
type DeadLetterConfig struct {
	Arn string `json:"Arn,omitempty"`
}

// SqsParameters holds SQS-specific parameters.
type SqsParameters struct {
	MessageGroupId string `json:"MessageGroupId,omitempty"`
}

// --- Request/Response types ---

type CreateScheduleRequest struct {
	Name                       string              `json:"Name"`
	GroupName                  string              `json:"GroupName,omitempty"`
	ScheduleExpression         string              `json:"ScheduleExpression"`
	ScheduleExpressionTimezone string              `json:"ScheduleExpressionTimezone,omitempty"`
	State                      string              `json:"State,omitempty"`
	Description                string              `json:"Description,omitempty"`
	FlexibleTimeWindow         *FlexibleTimeWindow `json:"FlexibleTimeWindow,omitempty"`
	Target                     *ScheduleTarget     `json:"Target,omitempty"`
	StartDate                  *time.Time          `json:"StartDate,omitempty"`
	EndDate                    *time.Time          `json:"EndDate,omitempty"`
	ActionAfterCompletion      string              `json:"ActionAfterCompletion,omitempty"`
}

type CreateScheduleResponse struct {
	ScheduleArn string `json:"ScheduleArn"`
}

type GetScheduleRequest struct {
	Name      string `json:"Name"`
	GroupName string `json:"GroupName,omitempty"`
}

type UpdateScheduleRequest struct {
	Name                       string              `json:"Name"`
	GroupName                  string              `json:"GroupName,omitempty"`
	ScheduleExpression         string              `json:"ScheduleExpression"`
	ScheduleExpressionTimezone string              `json:"ScheduleExpressionTimezone,omitempty"`
	State                      string              `json:"State,omitempty"`
	Description                string              `json:"Description,omitempty"`
	FlexibleTimeWindow         *FlexibleTimeWindow `json:"FlexibleTimeWindow,omitempty"`
	Target                     *ScheduleTarget     `json:"Target,omitempty"`
	StartDate                  *time.Time          `json:"StartDate,omitempty"`
	EndDate                    *time.Time          `json:"EndDate,omitempty"`
	ActionAfterCompletion      string              `json:"ActionAfterCompletion,omitempty"`
}

type UpdateScheduleResponse struct {
	ScheduleArn string `json:"ScheduleArn"`
}

type DeleteScheduleRequest struct {
	Name      string `json:"Name"`
	GroupName string `json:"GroupName,omitempty"`
}

type ListSchedulesRequest struct {
	GroupName  string `json:"GroupName,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
	NamePrefix string `json:"NamePrefix,omitempty"`
	State      string `json:"State,omitempty"`
}

type ListSchedulesResponse struct {
	Schedules []ScheduleSummary `json:"Schedules"`
	NextToken string            `json:"NextToken,omitempty"`
}

type ScheduleSummary struct {
	Name               string    `json:"Name"`
	Arn                string    `json:"Arn"`
	GroupName          string    `json:"GroupName"`
	ScheduleExpression string    `json:"ScheduleExpression"`
	State              string    `json:"State"`
	CreationDate       time.Time `json:"CreationDate"`
	TargetArn          string    `json:"Target.Arn,omitempty"`
}

type CreateScheduleGroupRequest struct {
	Name string `json:"Name"`
}

type CreateScheduleGroupResponse struct {
	ScheduleGroupArn string `json:"ScheduleGroupArn"`
}

type GetScheduleGroupRequest struct {
	Name string `json:"Name"`
}

type DeleteScheduleGroupRequest struct {
	Name string `json:"Name"`
}

type ListScheduleGroupsRequest struct {
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
	NamePrefix string `json:"NamePrefix,omitempty"`
}

type ListScheduleGroupsResponse struct {
	ScheduleGroups []ScheduleGroup `json:"ScheduleGroups"`
	NextToken      string          `json:"NextToken,omitempty"`
}

// --- Admin types ---

type AdminSummaryResponse struct {
	Service        string `json:"service"`
	ScheduleGroups int    `json:"scheduleGroups"`
	Schedules      int    `json:"schedules"`
}

type AdminGroupDetail struct {
	Name      string                `json:"name"`
	Arn       string                `json:"arn"`
	State     string                `json:"state"`
	Schedules []AdminScheduleDetail `json:"schedules"`
}

type AdminScheduleDetail struct {
	Name               string `json:"name"`
	Arn                string `json:"arn"`
	State              string `json:"state"`
	ScheduleExpression string `json:"scheduleExpression"`
	TargetArn          string `json:"targetArn,omitempty"`
	Description        string `json:"description,omitempty"`
}

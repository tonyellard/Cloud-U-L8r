package model

import "time"

type PutParameterRequest struct {
	Name      string `json:"Name"`
	Type      string `json:"Type"`
	Value     string `json:"Value"`
	Overwrite bool   `json:"Overwrite"`
	Tier      string `json:"Tier,omitempty"`
}

type PutParameterResponse struct {
	Version int64  `json:"Version"`
	Tier    string `json:"Tier,omitempty"`
}

type GetParameterRequest struct {
	Name           string `json:"Name"`
	WithDecryption bool   `json:"WithDecryption"`
}

type GetParametersRequest struct {
	Names          []string `json:"Names"`
	WithDecryption bool     `json:"WithDecryption"`
}

type GetParametersByPathRequest struct {
	Path           string `json:"Path"`
	Recursive      bool   `json:"Recursive"`
	WithDecryption bool   `json:"WithDecryption"`
	MaxResults     int    `json:"MaxResults,omitempty"`
	NextToken      string `json:"NextToken,omitempty"`
}

type Parameter struct {
	Name             string    `json:"Name"`
	Type             string    `json:"Type"`
	Value            string    `json:"Value"`
	Version          int64     `json:"Version"`
	ARN              string    `json:"ARN,omitempty"`
	LastModifiedDate time.Time `json:"LastModifiedDate,omitempty"`
}

type GetParameterResponse struct {
	Parameter Parameter `json:"Parameter"`
}

type GetParametersResponse struct {
	Parameters        []Parameter `json:"Parameters"`
	InvalidParameters []string    `json:"InvalidParameters"`
}

type GetParametersByPathResponse struct {
	Parameters []Parameter `json:"Parameters"`
	NextToken  string      `json:"NextToken,omitempty"`
}

type DescribeParametersRequest struct {
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

type ParameterMetadata struct {
	Name             string    `json:"Name"`
	Type             string    `json:"Type"`
	Version          int64     `json:"Version"`
	ARN              string    `json:"ARN,omitempty"`
	LastModifiedDate time.Time `json:"LastModifiedDate,omitempty"`
}

type DescribeParametersResponse struct {
	Parameters []ParameterMetadata `json:"Parameters"`
	NextToken  string              `json:"NextToken,omitempty"`
}

type GetParameterHistoryRequest struct {
	Name           string `json:"Name"`
	WithDecryption bool   `json:"WithDecryption"`
	MaxResults     int    `json:"MaxResults,omitempty"`
	NextToken      string `json:"NextToken,omitempty"`
}

type GetParameterHistoryResponse struct {
	Parameters []Parameter `json:"Parameters"`
	NextToken  string      `json:"NextToken,omitempty"`
}

type DeleteParameterRequest struct {
	Name string `json:"Name"`
}

type DeleteParametersRequest struct {
	Names []string `json:"Names"`
}

type DeleteParameterResponse struct{}

type DeleteParametersResponse struct {
	DeletedParameters []string `json:"DeletedParameters"`
	InvalidParameters []string `json:"InvalidParameters"`
}

type LabelParameterVersionRequest struct {
	Name             string   `json:"Name"`
	Labels           []string `json:"Labels"`
	ParameterVersion int64    `json:"ParameterVersion"`
}

type LabelParameterVersionResponse struct {
	InvalidLabels    []string `json:"InvalidLabels"`
	ParameterVersion int64    `json:"ParameterVersion"`
}

type CreateSecretRequest struct {
	Name         string  `json:"Name"`
	Description  string  `json:"Description,omitempty"`
	SecretString *string `json:"SecretString,omitempty"`
	SecretBinary string  `json:"SecretBinary,omitempty"`
}

type CreateSecretResponse struct {
	ARN       string `json:"ARN"`
	Name      string `json:"Name"`
	VersionID string `json:"VersionId"`
}

type PutSecretValueRequest struct {
	SecretID     string   `json:"SecretId"`
	SecretString *string  `json:"SecretString,omitempty"`
	SecretBinary string   `json:"SecretBinary,omitempty"`
	VersionStage []string `json:"VersionStages,omitempty"`
}

type UpdateSecretRequest struct {
	SecretID     string  `json:"SecretId"`
	Description  string  `json:"Description,omitempty"`
	SecretString *string `json:"SecretString,omitempty"`
	SecretBinary string  `json:"SecretBinary,omitempty"`
}

type GetSecretValueRequest struct {
	SecretID     string `json:"SecretId"`
	VersionID    string `json:"VersionId,omitempty"`
	VersionStage string `json:"VersionStage,omitempty"`
}

type SecretValueResponse struct {
	ARN          string    `json:"ARN"`
	Name         string    `json:"Name"`
	VersionID    string    `json:"VersionId"`
	SecretString *string   `json:"SecretString,omitempty"`
	SecretBinary string    `json:"SecretBinary,omitempty"`
	VersionStage []string  `json:"VersionStages"`
	CreatedDate  time.Time `json:"CreatedDate"`
}

type DescribeSecretRequest struct {
	SecretID string `json:"SecretId"`
}

type SecretVersionStages struct {
	VersionID string   `json:"VersionId"`
	Stages    []string `json:"VersionStages"`
}

type DescribeSecretResponse struct {
	ARN                string                `json:"ARN"`
	Name               string                `json:"Name"`
	Description        string                `json:"Description,omitempty"`
	CreatedDate        time.Time             `json:"CreatedDate"`
	LastChangedDate    time.Time             `json:"LastChangedDate"`
	DeletedDate        *time.Time            `json:"DeletedDate,omitempty"`
	VersionIDsToStages []SecretVersionStages `json:"VersionIdsToStages,omitempty"`
}

type ListSecretsRequest struct {
	MaxResults int    `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

type SecretListEntry struct {
	ARN             string     `json:"ARN"`
	Name            string     `json:"Name"`
	Description     string     `json:"Description,omitempty"`
	CreatedDate     time.Time  `json:"CreatedDate"`
	LastChangedDate time.Time  `json:"LastChangedDate"`
	DeletedDate     *time.Time `json:"DeletedDate,omitempty"`
}

type ListSecretsResponse struct {
	SecretList []SecretListEntry `json:"SecretList"`
	NextToken  string            `json:"NextToken,omitempty"`
}

type DeleteSecretRequest struct {
	SecretID                   string `json:"SecretId"`
	RecoveryWindowInDays       int64  `json:"RecoveryWindowInDays,omitempty"`
	ForceDeleteWithoutRecovery bool   `json:"ForceDeleteWithoutRecovery,omitempty"`
}

type DeleteSecretResponse struct {
	ARN          string    `json:"ARN"`
	Name         string    `json:"Name"`
	DeletionDate time.Time `json:"DeletionDate"`
}

type RestoreSecretRequest struct {
	SecretID string `json:"SecretId"`
}

type RestoreSecretResponse struct {
	ARN  string `json:"ARN"`
	Name string `json:"Name"`
}

type UpdateSecretVersionStageRequest struct {
	SecretID            string `json:"SecretId"`
	VersionStage        string `json:"VersionStage"`
	MoveToVersionID     string `json:"MoveToVersionId"`
	RemoveFromVersionID string `json:"RemoveFromVersionId,omitempty"`
}

type UpdateSecretVersionStageResponse struct {
	ARN  string `json:"ARN"`
	Name string `json:"Name"`
}

type AdminSummaryResponse struct {
	Parameters     int `json:"parameters"`
	SecretsTotal   int `json:"secretsTotal"`
	SecretsActive  int `json:"secretsActive"`
	SecretsDeleted int `json:"secretsDeleted"`
}

type AWSJSONError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

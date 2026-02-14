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
	MaxResults       int                     `json:"MaxResults,omitempty"`
	NextToken        string                  `json:"NextToken,omitempty"`
	ParameterFilters []ParameterStringFilter `json:"ParameterFilters,omitempty"`
}

type ParameterStringFilter struct {
	Key    string   `json:"Key"`
	Option string   `json:"Option,omitempty"`
	Values []string `json:"Values"`
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
	Filters    []SecretFilter `json:"Filters,omitempty"`
}

type SecretFilter struct {
	Key    string   `json:"Key"`
	Values []string `json:"Values"`
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

type AdminActivityEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Target     string    `json:"target,omitempty"`
	StatusCode int       `json:"statusCode"`
	ErrorType  string    `json:"errorType,omitempty"`
}

type AdminActivityResponse struct {
	Activity  []AdminActivityEntry `json:"activity"`
	NextToken string               `json:"nextToken,omitempty"`
}

type ExportParameterVersion struct {
	Version   int64     `json:"version"`
	Value     string    `json:"value"`
	Tier      string    `json:"tier"`
	CreatedAt time.Time `json:"createdAt"`
}

type ExportParameterRecord struct {
	Name           string                   `json:"name"`
	Type           string                   `json:"type"`
	CurrentVersion int64                    `json:"currentVersion"`
	LastModifiedAt time.Time                `json:"lastModifiedAt"`
	Labels         map[string]int64         `json:"labels"`
	Versions       []ExportParameterVersion `json:"versions"`
}

type ExportSecretVersion struct {
	VersionID    string    `json:"versionId"`
	SecretString *string   `json:"secretString,omitempty"`
	SecretBinary string    `json:"secretBinary,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ExportSecretRecord struct {
	Name          string                `json:"name"`
	ARN           string                `json:"arn"`
	Description   string                `json:"description,omitempty"`
	DeletedAt     *time.Time            `json:"deletedAt,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
	LastChangedAt time.Time             `json:"lastChangedAt"`
	StageToID     map[string]string     `json:"stageToId"`
	VersionStages map[string][]string   `json:"versionStages"`
	Versions      []ExportSecretVersion `json:"versions"`
}

type AdminExportResponse struct {
	Region     string                  `json:"region"`
	AccountID  string                  `json:"accountId"`
	Parameters []ExportParameterRecord `json:"parameters"`
	Secrets    []ExportSecretRecord    `json:"secrets"`
}

type AdminImportRequest struct {
	Region     string                  `json:"region"`
	AccountID  string                  `json:"accountId"`
	Parameters []ExportParameterRecord `json:"parameters"`
	Secrets    []ExportSecretRecord    `json:"secrets"`
}

type AdminImportResponse struct {
	ImportedParameters int `json:"importedParameters"`
	ImportedSecrets    int `json:"importedSecrets"`
}

type AWSJSONError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

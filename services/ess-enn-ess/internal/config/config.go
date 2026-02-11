package config

import (
"fmt"
"os"

"gopkg.in/yaml.v3"
)

// Config represents the complete SNS emulator configuration
type Config struct {
Server       ServerConfig       `yaml:"server"`
Storage      StorageConfig      `yaml:"storage"`
ActivityLog  ActivityLogConfig  `yaml:"activity_log"`
SQS          SQSConfig          `yaml:"sqs"`
HTTP         HTTPConfig         `yaml:"http"`
Messages     MessagesConfig     `yaml:"messages"`
Admin        AdminConfig        `yaml:"admin"`
Telemetry    TelemetryConfig    `yaml:"telemetry"`
AWS          AWSConfig          `yaml:"aws"`
Developer    DeveloperConfig    `yaml:"developer"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
APIPort    int    `yaml:"api_port"`
AdminPort  int    `yaml:"admin_port"`
Host       string `yaml:"host"`
TimeoutSec int    `yaml:"timeout_seconds"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
Type              string `yaml:"type"`
ActivityLogSize   int    `yaml:"activity_log_size"`
PersistConfig     bool   `yaml:"persist_config"`
ConfigFile        string `yaml:"config_file"`
}

// ActivityLogConfig represents activity logging configuration
type ActivityLogConfig struct {
Enabled          bool   `yaml:"enabled"`
LogFile          string `yaml:"log_file"`
RetentionDays    int    `yaml:"retention_days"`
StreamToAdminUI  bool   `yaml:"stream_to_admin_ui"`
}

// SQSConfig represents SQS integration configuration
type SQSConfig struct {
Enabled           bool   `yaml:"enabled"`
Endpoint          string `yaml:"endpoint"`
Region            string `yaml:"region"`
OnQueueNotFound   string `yaml:"on_queue_not_found"`
MaxRetries        int    `yaml:"max_retries"`
RetryBackoffMs    int    `yaml:"retry_backoff_ms"`
}

// HTTPConfig represents HTTP configuration
type HTTPConfig struct {
Enabled         bool `yaml:"enabled"`
MaxRetries      int  `yaml:"max_retries"`
RetryBackoffMs  int  `yaml:"retry_backoff_ms"`
TimeoutSeconds  int  `yaml:"timeout_seconds"`
}

// MessagesConfig represents message configuration
type MessagesConfig struct {
MaxSize          int `yaml:"max_size"`
RetentionPeriod  int `yaml:"retention_period"`
}

// AdminConfig represents admin UI configuration
type AdminConfig struct {
Enabled bool `yaml:"enabled"`
Port    int  `yaml:"port"`
}

// TelemetryConfig represents telemetry configuration
type TelemetryConfig struct {
MetricsEnabled       bool `yaml:"metrics_enabled"`
TracingEnabled       bool `yaml:"tracing_enabled"`
ExportIntervalSeconds int `yaml:"export_interval_seconds"`
}

// AWSConfig represents AWS configuration
type AWSConfig struct {
AccountId string `yaml:"account_id"`
Region    string `yaml:"region"`
}

// DeveloperConfig represents developer-specific configuration
type DeveloperConfig struct {
NoAuth                  bool `yaml:"no_auth"`
VerboseErrors           bool `yaml:"verbose_errors"`
AutoConfirmSubscriptions bool `yaml:"auto_confirm_subscriptions"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filePath string) (*Config, error) {
data, err := os.ReadFile(filePath)
if err != nil {
return nil, fmt.Errorf("failed to read config file: %w", err)
}

cfg := &Config{}
if err := yaml.Unmarshal(data, cfg); err != nil {
return nil, fmt.Errorf("failed to parse config file: %w", err)
}

applyDefaults(cfg)
return cfg, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(cfg *Config, filePath string) error {
data, err := yaml.Marshal(cfg)
if err != nil {
return fmt.Errorf("failed to marshal config: %w", err)
}

if err := os.WriteFile(filePath, data, 0644); err != nil {
return fmt.Errorf("failed to write config file: %w", err)
}

return nil
}

// applyDefaults applies default values to configuration
func applyDefaults(cfg *Config) {
if cfg.Server.APIPort == 0 {
cfg.Server.APIPort = 9330
}
if cfg.Server.AdminPort == 0 {
cfg.Server.AdminPort = 9331
}
if cfg.Server.Host == "" {
cfg.Server.Host = "0.0.0.0"
}
if cfg.Server.TimeoutSec == 0 {
cfg.Server.TimeoutSec = 30
}

if cfg.Storage.Type == "" {
cfg.Storage.Type = "memory"
}
if cfg.Storage.ActivityLogSize == 0 {
cfg.Storage.ActivityLogSize = 10000
}
if cfg.Storage.ConfigFile == "" {
cfg.Storage.ConfigFile = "./data/sns_state.yaml"
}

if cfg.ActivityLog.LogFile == "" {
cfg.ActivityLog.LogFile = "./data/activity.log"
}
if cfg.ActivityLog.RetentionDays == 0 {
cfg.ActivityLog.RetentionDays = 7
}

if cfg.SQS.Endpoint == "" {
cfg.SQS.Endpoint = "http://ess-queue-ess:9320"
}
if cfg.SQS.Region == "" {
cfg.SQS.Region = "us-east-1"
}
if cfg.SQS.OnQueueNotFound == "" {
cfg.SQS.OnQueueNotFound = "log_error"
}
if cfg.SQS.MaxRetries == 0 {
cfg.SQS.MaxRetries = 3
}
if cfg.SQS.RetryBackoffMs == 0 {
cfg.SQS.RetryBackoffMs = 100
}

if cfg.HTTP.MaxRetries == 0 {
cfg.HTTP.MaxRetries = 3
}
if cfg.HTTP.RetryBackoffMs == 0 {
cfg.HTTP.RetryBackoffMs = 100
}
if cfg.HTTP.TimeoutSeconds == 0 {
cfg.HTTP.TimeoutSeconds = 10
}

if cfg.Messages.MaxSize == 0 {
cfg.Messages.MaxSize = 262144 // 256KB
}
if cfg.Messages.RetentionPeriod == 0 {
cfg.Messages.RetentionPeriod = 96 // 4 days
}

if cfg.Admin.Port == 0 {
cfg.Admin.Port = 9331
}

if cfg.Telemetry.ExportIntervalSeconds == 0 {
cfg.Telemetry.ExportIntervalSeconds = 60
}

if cfg.AWS.AccountId == "" {
cfg.AWS.AccountId = "123456789012"
}
if cfg.AWS.Region == "" {
cfg.AWS.Region = "us-east-1"
}

cfg.Developer.NoAuth = true
cfg.Developer.VerboseErrors = true
cfg.Developer.AutoConfirmSubscriptions = true
}

// Default returns a default configuration
func Default() *Config {
cfg := &Config{
Server: ServerConfig{
APIPort:    9330,
AdminPort:  9331,
Host:       "0.0.0.0",
TimeoutSec: 30,
},
Storage: StorageConfig{
Type:            "memory",
ActivityLogSize: 10000,
PersistConfig:   true,
ConfigFile:      "./data/sns_state.yaml",
},
ActivityLog: ActivityLogConfig{
Enabled:         true,
LogFile:         "./data/activity.log",
RetentionDays:   7,
StreamToAdminUI: true,
},
SQS: SQSConfig{
Enabled:         true,
Endpoint:        "http://ess-queue-ess:9320",
Region:          "us-east-1",
OnQueueNotFound: "log_error",
MaxRetries:      3,
RetryBackoffMs:  100,
},
HTTP: HTTPConfig{
Enabled:        true,
MaxRetries:     3,
RetryBackoffMs: 100,
TimeoutSeconds: 10,
},
Messages: MessagesConfig{
MaxSize:         262144,
RetentionPeriod: 96,
},
Admin: AdminConfig{
Enabled: true,
Port:    9331,
},
Telemetry: TelemetryConfig{
MetricsEnabled:       true,
TracingEnabled:       true,
ExportIntervalSeconds: 60,
},
AWS: AWSConfig{
AccountId: "123456789012",
Region:    "us-east-1",
},
Developer: DeveloperConfig{
NoAuth:                  true,
VerboseErrors:           true,
AutoConfirmSubscriptions: true,
},
}
return cfg
}
// LoadState loads topics and subscriptions from an exported config file
// Returns the raw data structures for topics and subscriptions
func LoadState(filePath string) ([]interface{}, []interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse the entire file as a generic map
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	var topics []interface{}
	var subscriptions []interface{}

	// Extract topics if present
	if topicsData, ok := raw["topics"].([]interface{}); ok {
		topics = topicsData
	}

	// Extract subscriptions if present
	if subsData, ok := raw["subscriptions"].([]interface{}); ok {
		subscriptions = subsData
	}

	return topics, subscriptions, nil
}
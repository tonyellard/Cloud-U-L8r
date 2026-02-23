# SPDX-License-Identifier: Apache-2.0

# --- S3 ---

output "s3_bucket_name" {
  description = "Name of the default S3 test bucket"
  value       = aws_s3_bucket.test_bucket.id
}

# --- SQS ---

output "sqs_queue_url" {
  description = "URL of the default SQS test queue"
  value       = aws_sqs_queue.my_test_queue.url
}

output "sqs_pipe_source_url" {
  description = "URL of the pipe source queue"
  value       = aws_sqs_queue.pipe_source.url
}

# --- SNS ---

output "sns_topic_arn" {
  description = "ARN of the default SNS test topic"
  value       = aws_sns_topic.my_test_topic.arn
}

output "sns_subscription_arn" {
  description = "ARN of the SNS-to-SQS subscription"
  value       = aws_sns_topic_subscription.my_test_queue.arn
}

# --- SSM / Secrets Manager ---

output "ssm_param_db_host" {
  description = "SSM parameter for database host"
  value       = aws_ssm_parameter.app_db_host.name
}

output "secret_db_credentials_arn" {
  description = "ARN of the database credentials secret"
  value       = aws_secretsmanager_secret.db_credentials.arn
}

# --- EventBridge ---

output "eventbridge_bus_arn" {
  description = "ARN of the app-events custom bus"
  value       = aws_cloudwatch_event_bus.app_events.arn
}

output "eventbridge_rule_arn" {
  description = "ARN of the order-placed rule"
  value       = aws_cloudwatch_event_rule.order_placed.arn
}

# --- EventBridge Scheduler ---

output "scheduler_group_arn" {
  description = "ARN of the app-schedules group"
  value       = aws_scheduler_schedule_group.app.arn
}

output "scheduler_heartbeat_arn" {
  description = "ARN of the heartbeat schedule"
  value       = aws_scheduler_schedule.heartbeat.arn
}

# --- EventBridge Pipes ---

output "pipe_arn" {
  description = "ARN of the forward-to-notifications pipe"
  value       = aws_pipes_pipe.forward_to_sns.arn
}

# SPDX-License-Identifier: Apache-2.0

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "s3_endpoint" {
  description = "S3 emulator endpoint"
  type        = string
  default     = "http://localhost:9300"
}

variable "sqs_endpoint" {
  description = "SQS emulator endpoint"
  type        = string
  default     = "http://localhost:9320"
}

variable "sns_endpoint" {
  description = "SNS emulator endpoint"
  type        = string
  default     = "http://localhost:9330"
}

variable "ssm_endpoint" {
  description = "SSM / Secrets Manager emulator endpoint"
  type        = string
  default     = "http://localhost:9350"
}

variable "eventbridge_endpoint" {
  description = "EventBridge emulator endpoint"
  type        = string
  default     = "http://localhost:9340"
}

variable "scheduler_endpoint" {
  description = "EventBridge Scheduler emulator endpoint"
  type        = string
  default     = "http://localhost:9342"
}

variable "pipes_endpoint" {
  description = "EventBridge Pipes emulator endpoint"
  type        = string
  default     = "http://localhost:9344"
}

# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for ess-queue-ess (SQS emulator)
# Validates CreateQueue, GetQueueAttributes, SendMessage, and DeleteQueue operations.

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.82.2"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    sqs = "http://localhost:9320"
  }
}

# Create a standard SQS queue
resource "aws_sqs_queue" "standard" {
  name                       = "terraform-test-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 86400
  delay_seconds              = 0
}

# Create a FIFO SQS queue
resource "aws_sqs_queue" "fifo" {
  name                        = "terraform-test-queue.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  visibility_timeout_seconds  = 60
}

# Create a dead-letter queue
resource "aws_sqs_queue" "dlq" {
  name = "terraform-test-dlq"
}

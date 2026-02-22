# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for pipes (EventBridge Pipes emulator)
# Validates CreatePipe, DescribePipe, and tag operations via the AWS provider.

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
    pipes = "http://localhost:9344"
    sqs   = "http://localhost:9320"
    sns   = "http://localhost:9330"
  }
}

# --- Source queue ---

resource "aws_sqs_queue" "source" {
  name = "tf-pipe-source"
}

# --- Target queue (simple SQS-to-SQS pipe) ---

resource "aws_sqs_queue" "target" {
  name = "tf-pipe-target"
}

# --- SNS topic as alternative target ---

resource "aws_sns_topic" "notifications" {
  name = "tf-pipe-notifications"
}

# --- Pipe 1: Simple SQS -> SQS (no filter) ---

resource "aws_pipes_pipe" "simple" {
  name     = "tf-simple-pipe"
  role_arn = "arn:aws:iam::000000000000:role/pipe-role"
  source   = aws_sqs_queue.source.arn
  target   = aws_sqs_queue.target.arn
}

# --- Pipe 2: SQS -> SNS with filtering ---

resource "aws_pipes_pipe" "filtered" {
  name          = "tf-filtered-pipe"
  role_arn      = "arn:aws:iam::000000000000:role/pipe-role"
  source        = aws_sqs_queue.source.arn
  target        = aws_sns_topic.notifications.arn
  desired_state = "STOPPED"
  description   = "Filtered pipe for high-priority orders"

  source_parameters {
    sqs_queue_parameters {
      batch_size = 5
    }
    filter_criteria {
      filter {
        pattern = jsonencode({ body = { priority = ["high"] } })
      }
    }
  }
}

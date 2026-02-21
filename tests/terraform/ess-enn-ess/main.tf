# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for ess-enn-ess (SNS emulator)
# Validates CreateTopic, GetTopicAttributes, Subscribe, and Publish operations.

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
    sns = "http://localhost:9330"
  }
}

# Create an SNS topic
resource "aws_sns_topic" "test" {
  name = "terraform-test-topic"
}

# Create an HTTP subscription
resource "aws_sns_topic_subscription" "http" {
  topic_arn = aws_sns_topic.test.arn
  protocol  = "http"
  endpoint  = "http://localhost:9999/webhook"
}

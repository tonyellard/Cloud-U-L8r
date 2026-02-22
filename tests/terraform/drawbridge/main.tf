# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for drawbridge (EventBridge emulator)
# Validates CreateEventBus, PutRule, PutTargets, and end-to-end event routing.

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
    eventbridge = "http://localhost:9340"
    sqs         = "http://localhost:9320"
  }
}

# --- Custom Event Bus ---

resource "aws_cloudwatch_event_bus" "custom" {
  name = "terraform-test-bus"
}

# --- Rule on the default bus: match a specific source ---

resource "aws_cloudwatch_event_rule" "order_placed" {
  name           = "capture-order-placed"
  description    = "Capture order.placed events from the orders service"
  event_bus_name = "default"

  event_pattern = jsonencode({
    source      = ["orders.service"]
    detail-type = ["OrderPlaced"]
  })
}

# --- SQS queue to receive matched events ---

resource "aws_sqs_queue" "order_events" {
  name = "terraform-order-events"
}

# --- Target: route matched events to SQS ---

resource "aws_cloudwatch_event_target" "to_sqs" {
  rule           = aws_cloudwatch_event_rule.order_placed.name
  event_bus_name = "default"
  target_id      = "order-events-queue"
  arn            = aws_sqs_queue.order_events.arn
}

# --- Rule on the custom bus: match by prefix ---

resource "aws_cloudwatch_event_rule" "audit" {
  name           = "audit-all-events"
  description    = "Capture all events on the custom bus"
  event_bus_name = aws_cloudwatch_event_bus.custom.name

  event_pattern = jsonencode({
    source = [{ "prefix" : "app." }]
  })
}

# --- Second SQS queue for audit ---

resource "aws_sqs_queue" "audit_events" {
  name = "terraform-audit-events"
}

# --- Target: route audit events to SQS ---

resource "aws_cloudwatch_event_target" "audit_to_sqs" {
  rule           = aws_cloudwatch_event_rule.audit.name
  event_bus_name = aws_cloudwatch_event_bus.custom.name
  target_id      = "audit-queue"
  arn            = aws_sqs_queue.audit_events.arn
}

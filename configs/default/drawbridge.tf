# SPDX-License-Identifier: Apache-2.0

# --- Custom Event Bus ---

resource "aws_cloudwatch_event_bus" "app_events" {
  name = "app-events"
}

# --- Rule on default bus: capture order events ---

resource "aws_cloudwatch_event_rule" "order_placed" {
  name           = "capture-order-placed"
  description    = "Route order.placed events to SQS"
  event_bus_name = "default"

  event_pattern = jsonencode({
    source      = ["orders.service"]
    detail-type = ["OrderPlaced"]
  })
}

# --- Target: route matched events to the test queue ---

resource "aws_cloudwatch_event_target" "orders_to_sqs" {
  rule           = aws_cloudwatch_event_rule.order_placed.name
  event_bus_name = "default"
  target_id      = "order-events-queue"
  arn            = aws_sqs_queue.my_test_queue.arn
}

# SPDX-License-Identifier: Apache-2.0

# --- Schedule Group ---

resource "aws_scheduler_schedule_group" "app" {
  name = "app-schedules"
}

# --- Recurring schedule: publish to SNS every 5 minutes ---

resource "aws_scheduler_schedule" "heartbeat" {
  name       = "heartbeat"
  group_name = aws_scheduler_schedule_group.app.name

  schedule_expression = "rate(5 minutes)"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_sns_topic.my_test_topic.arn
    role_arn = "arn:aws:iam::000000000000:role/scheduler-role"

    input = jsonencode({
      source      = "scheduler"
      detail-type = "Heartbeat"
      detail      = {}
    })
  }
}

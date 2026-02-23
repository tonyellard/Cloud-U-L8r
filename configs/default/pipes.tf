# SPDX-License-Identifier: Apache-2.0

# --- Dedicated source queue for the pipe ---

resource "aws_sqs_queue" "pipe_source" {
  name                       = "pipe-source"
  visibility_timeout_seconds = 30
}

# --- Pipe: SQS -> SNS (forward messages to the test topic) ---

resource "aws_pipes_pipe" "forward_to_sns" {
  name     = "forward-to-notifications"
  role_arn = "arn:aws:iam::000000000000:role/pipe-role"
  source   = aws_sqs_queue.pipe_source.arn
  target   = aws_sns_topic.my_test_topic.arn
}

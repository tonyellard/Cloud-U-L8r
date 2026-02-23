# SPDX-License-Identifier: Apache-2.0

resource "aws_sqs_queue" "my_test_queue" {
  name                       = "my-test-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600
  max_message_size           = 262144
}

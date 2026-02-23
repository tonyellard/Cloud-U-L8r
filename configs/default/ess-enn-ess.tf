# SPDX-License-Identifier: Apache-2.0

resource "aws_sns_topic" "my_test_topic" {
  name         = "my-test-topic"
  display_name = "test topic"
}

resource "aws_sns_topic_subscription" "my_test_queue" {
  topic_arn = aws_sns_topic.my_test_topic.arn
  protocol  = "sqs"
  endpoint  = "http://ess-queue-ess:9320/my-test-queue"
}

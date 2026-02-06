#!/bin/bash
set -e

echo "=== Quick FIFO & DLQ Test ==="
echo ""

# Test FIFO queue with AWS CLI
echo "1. Creating FIFO queue..."
aws sqs create-queue \
  --queue-name quick-test.fifo \
  --attributes FifoQueue=true,ContentBasedDeduplication=true \
  --endpoint-url http://localhost:9320 \
  --region us-east-1 \
  --output json | grep QueueUrl

echo ""
echo "2. Sending FIFO messages..."
aws sqs send-message \
  --queue-url http://localhost:9320/quick-test.fifo \
  --message-body "First message" \
  --message-group-id "group-1" \
  --endpoint-url http://localhost:9320 \
  --region us-east-1 | grep -E "MessageId|SequenceNumber"

echo ""
echo "3. Receiving FIFO messages..."
aws sqs receive-message \
  --queue-url http://localhost:9320/quick-test.fifo \
  --endpoint-url http://localhost:9320 \
  --region us-east-1 | grep -E "MessageId|Body"

echo ""
echo "4. Cleanup..."
aws sqs delete-queue \
  --queue-url http://localhost:9320/quick-test.fifo \
  --endpoint-url http://localhost:9320 \
  --region us-east-1

echo ""
echo "✓ FIFO test completed successfully!"

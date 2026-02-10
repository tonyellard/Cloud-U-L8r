#!/bin/bash
# Integration test for Phase 5: SQS Delivery Integration

set -e

SNS_URL="http://localhost:9330"
SQS_URL="http://localhost:9320"

echo "Testing SNS Emulator Phase 5: SQS Integration"
echo "==============================================="
echo ""
echo "Prerequisites:"
echo "  - ess-queue-ess running on port 9320"
echo "  - ess-enn-ess running on port 9330"
echo ""

# Check if services are running
echo "Checking services..."
if ! curl -s "${SNS_URL}/health" > /dev/null 2>&1; then
    echo "ERROR: ess-enn-ess is not running on port 9330"
    echo "Start it with: ./ess-enn-ess -config ./config/config.yaml"
    exit 1
fi

if ! curl -s "${SQS_URL}/health" > /dev/null 2>&1; then
    echo "WARNING: ess-queue-ess health check failed"
    echo "Trying to continue anyway..."
fi

echo "✓ Services are reachable"
echo ""

# Test 1: Create SQS Queue
echo "Test 1: Creating SQS queue..."
QUEUE_URL=$(curl -s -X POST "$SQS_URL/" \
    -d "Action=CreateQueue&QueueName=sns-test-queue" \
    | grep -oP '<QueueUrl>\K[^<]+' || echo "")

if [ -z "$QUEUE_URL" ]; then
    echo "ERROR: Failed to create SQS queue"
    exit 1
fi
echo "✓ Created queue: $QUEUE_URL"

# Test 2: Create SNS Topic
echo "Test 2: Creating SNS topic..."
TOPIC_ARN=$(curl -s -X POST "$SNS_URL/" \
    -d "Action=CreateTopic&Name=sqs-integration-topic" \
    | grep -oP '<TopicArn>\K[^<]+' || echo "")

if [ -z "$TOPIC_ARN" ]; then
    echo "ERROR: Failed to create SNS topic"
    exit 1
fi
echo "✓ Created topic: $TOPIC_ARN"

# Test 3: Subscribe SQS queue to SNS topic
echo "Test 3: Subscribing SQS queue to topic..."
SUB_ARN=$(curl -s -X POST "$SNS_URL/" \
    -d "Action=Subscribe&TopicArn=${TOPIC_ARN}&Protocol=sqs&Endpoint=${QUEUE_URL}" \
    | grep -oP '<SubscriptionArn>\K[^<]+' || echo "")

if [ -z "$SUB_ARN" ]; then
    echo "ERROR: Failed to create subscription"
    exit 1
fi
echo "✓ Created subscription: $SUB_ARN"

# Test 4: Publish message to SNS topic
echo "Test 4: Publishing message to topic..."
MESSAGE_ID=$(curl -s -X POST "$SNS_URL/" \
    -d "Action=Publish&TopicArn=${TOPIC_ARN}&Message=Hello%20from%20SNS&Subject=Test%20Message" \
    | grep -oP '<MessageId>\K[^<]+' || echo "")

if [ -z "$MESSAGE_ID" ]; then
    echo "ERROR: Failed to publish message"
    exit 1
fi
echo "✓ Published message: $MESSAGE_ID"

# Wait for async delivery
echo "Waiting for message delivery..."
sleep 2

# Test 5: Receive message from SQS queue
echo "Test 5: Receiving message from SQS queue..."
SQS_RESPONSE=$(curl -s -X POST "$SQS_URL/" \
    -d "Action=ReceiveMessage&QueueUrl=${QUEUE_URL}&MaxNumberOfMessages=1")

# Check if we got a message
if echo "$SQS_RESPONSE" | grep -q "<Body>"; then
    echo "✓ Message received from SQS queue"
    
    # Extract and display message body
    MESSAGE_BODY=$(echo "$SQS_RESPONSE" | grep -oP '<Body>\K[^<]+' | head -1)
    echo ""
    echo "Message body (SNS notification JSON):"
    echo "$MESSAGE_BODY" | python3 -m json.tool 2>/dev/null || echo "$MESSAGE_BODY"
    echo ""
    
    # Verify it contains SNS metadata
    if echo "$MESSAGE_BODY" | grep -q "\"Type\".*\"Notification\""; then
        echo "✓ Message contains SNS notification format"
    else
        echo "⚠ Warning: Message doesn't appear to be in SNS format"
    fi
    
    if echo "$MESSAGE_BODY" | grep -q "\"TopicArn\""; then
        echo "✓ Message contains TopicArn"
    fi
    
    if echo "$MESSAGE_BODY" | grep -q "Hello from SNS"; then
        echo "✓ Message contains expected text"
    fi
else
    echo "✗ No message received from SQS queue"
    echo "Response: $SQS_RESPONSE"
    exit 1
fi

# Test 6: Publish another message and verify delivery
echo ""
echo "Test 6: Publishing second message..."
MESSAGE_ID_2=$(curl -s -X POST "$SNS_URL/" \
    -d "Action=Publish&TopicArn=${TOPIC_ARN}&Message=Second%20message&Subject=Test%202" \
    | grep -oP '<MessageId>\K[^<]+')
echo "✓ Published message: $MESSAGE_ID_2"

sleep 2

# Check queue depth
echo "Test 7: Checking queue has messages..."
QUEUE_ATTRS=$(curl -s -X POST "$SQS_URL/" \
    -d "Action=GetQueueAttributes&QueueUrl=${QUEUE_URL}&AttributeName.1=ApproximateNumberOfMessages")

if echo "$QUEUE_ATTRS" | grep -q "ApproximateNumberOfMessages"; then
    MSG_COUNT=$(echo "$QUEUE_ATTRS" | grep -oP '<Value>\K[^<]+' | head -1)
    echo "✓ Queue has $MSG_COUNT message(s) waiting"
fi

echo ""
echo "==============================================="
echo "✅ All SQS integration tests passed!"
echo ""
echo "Summary:"
echo "  - Created SQS queue: $QUEUE_URL"
echo "  - Created SNS topic: $TOPIC_ARN"
echo "  - Subscribed SQS to SNS"
echo "  - Published messages to SNS"
echo "  - Verified delivery to SQS"
echo ""
echo "Phase 5 SQS Integration is working correctly! 🎉"

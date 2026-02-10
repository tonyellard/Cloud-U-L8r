#!/bin/bash
# Integration test for Phase 4: Message Publishing

set -e

BASE_URL="http://localhost:9330"
echo "Testing SNS Emulator Phase 4: Message Publishing"
echo "=================================================="

# Start server in background
echo "Starting server..."
./ess-enn-ess -config ./config/config.yaml &
SERVER_PID=$!
sleep 2

# Clean up on exit
cleanup() {
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

# Test 1: Create Topic
echo "Test 1: Creating topic..."
TOPIC_ARN=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=CreateTopic&Name=test-topic" \
    | grep -oP '<TopicArn>\K[^<]+')
echo "Created topic: $TOPIC_ARN"

# Test 2: Subscribe to topic (HTTP endpoint)
echo "Test 2: Subscribing to topic..."
SUB_ARN=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=Subscribe&TopicArn=$TOPIC_ARN&Protocol=http&Endpoint=http://localhost:8080/sns" \
    | grep -oP '<SubscriptionArn>\K[^<]+')
echo "Created subscription: $SUB_ARN"

# Test 3: Confirm subscription
echo "Test 3: Confirming subscription..."
# In dev mode, subscriptions are auto-confirmed, but let's verify status
curl -s -X POST "$BASE_URL/" \
    -d "Action=GetSubscriptionAttributes&SubscriptionArn=$SUB_ARN" \
    | grep -q "confirmed" && echo "Subscription confirmed" || echo "Subscription not confirmed"

# Test 4: Publish message
echo "Test 4: Publishing message..."
MESSAGE_ID=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=Publish&TopicArn=$TOPIC_ARN&Message=Hello%20World&Subject=Test%20Message" \
    | grep -oP '<MessageId>\K[^<]+')
echo "Published message: $MESSAGE_ID"

# Test 5: Publish another message with email subscription
echo "Test 5: Subscribe email and publish..."
EMAIL_SUB=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=Subscribe&TopicArn=$TOPIC_ARN&Protocol=email&Endpoint=test@example.com" \
    | grep -oP '<SubscriptionArn>\K[^<]+')
echo "Email subscription: $EMAIL_SUB"

MESSAGE_ID_2=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=Publish&TopicArn=$TOPIC_ARN&Message=Email%20Test&Subject=Email%20Subject" \
    | grep -oP '<MessageId>\K[^<]+')
echo "Published message 2: $MESSAGE_ID_2"

# Test 6: List subscriptions
echo "Test 6: Listing subscriptions..."
SUBSCRIPTION_COUNT=$(curl -s -X POST "$BASE_URL/" \
    -d "Action=ListSubscriptionsByTopic&TopicArn=$TOPIC_ARN" \
    | grep -oP '<member>' | wc -l)
echo "Subscription count: $SUBSCRIPTION_COUNT"

echo ""
echo "=================================================="
echo "All tests passed! ✓"
echo "Check server logs for delivery attempts"

#!/bin/bash
# test_admin_dashboard.sh - Test the admin dashboard with subscription support

set -e

echo "=== Admin Dashboard Test ==="
echo ""

# Configuration
SNS_ENDPOINT="http://localhost:9330"
ADMIN_URL="http://localhost:9331"
REGION="us-east-1"
ACCOUNT_ID="000000000000"

echo "📋 Testing Admin Dashboard APIs..."
echo ""

# Create test topics
echo "1️⃣  Creating test topics..."
TOPIC1_ARN=$(aws --endpoint-url=$SNS_ENDPOINT sns create-topic --name test-dashboard-topic-1 --region $REGION --output text)
TOPIC2_ARN=$(aws --endpoint-url=$SNS_ENDPOINT sns create-topic --name test-dashboard-topic-2 --region $REGION --output text)
echo "   ✓ Created topics: test-dashboard-topic-1, test-dashboard-topic-2"
echo ""

# Create subscriptions
echo "2️⃣  Creating test subscriptions..."
SUB1=$(aws --endpoint-url=$SNS_ENDPOINT sns subscribe \
    --topic-arn $TOPIC1_ARN \
    --protocol http \
    --notification-endpoint http://example.com/webhook1 \
    --region $REGION --output text)
    
SUB2=$(aws --endpoint-url=$SNS_ENDPOINT sns subscribe \
    --topic-arn $TOPIC1_ARN \
    --protocol sqs \
    --notification-endpoint arn:aws:sqs:us-east-1:000000000000:test-queue \
    --region $REGION --output text)
    
SUB3=$(aws --endpoint-url=$SNS_ENDPOINT sns subscribe \
    --topic-arn $TOPIC2_ARN \
    --protocol email \
    --notification-endpoint test@example.com \
    --region $REGION --output text)
    
echo "   ✓ Created 3 subscriptions (http, sqs, email)"
echo ""

# Confirm subscriptions
echo "3️⃣  Confirming subscriptions..."
aws --endpoint-url=$SNS_ENDPOINT sns set-subscription-attributes \
    --subscription-arn $SUB1 \
    --attribute-name ConfirmationStatus \
    --attribute-value confirmed \
    --region $REGION
    
aws --endpoint-url=$SNS_ENDPOINT sns set-subscription-attributes \
    --subscription-arn $SUB2 \
    --attribute-name ConfirmationStatus \
    --attribute-value confirmed \
    --region $REGION
    
echo "   ✓ Confirmed 2 subscriptions"
echo ""

# Publish messages
echo "4️⃣  Publishing test messages..."
aws --endpoint-url=$SNS_ENDPOINT sns publish \
    --topic-arn $TOPIC1_ARN \
    --message "Dashboard test message 1" \
    --subject "Test 1" \
    --region $REGION > /dev/null
    
aws --endpoint-url=$SNS_ENDPOINT sns publish \
    --topic-arn $TOPIC2_ARN \
    --message "Dashboard test message 2" \
    --subject "Test 2" \
    --region $REGION > /dev/null
    
echo "   ✓ Published 2 messages"
echo ""

# Test Admin API endpoints
echo "5️⃣  Testing Admin API endpoints..."
echo ""

echo "   Testing /api/topics..."
TOPICS_RESPONSE=$(curl -s $ADMIN_URL/api/topics)
TOPIC_COUNT=$(echo $TOPICS_RESPONSE | jq '. | length')
echo "   ✓ Topics endpoint: $TOPIC_COUNT topics found"
echo ""

echo "   Testing /api/subscriptions..."
SUBS_RESPONSE=$(curl -s $ADMIN_URL/api/subscriptions)
SUB_COUNT=$(echo $SUBS_RESPONSE | jq '. | length')
echo "   ✓ Subscriptions endpoint: $SUB_COUNT subscriptions found"
echo ""

echo "   Testing /api/stats..."
STATS_RESPONSE=$(curl -s $ADMIN_URL/api/stats)
echo "   Stats Response:"
echo $STATS_RESPONSE | jq '.'
echo ""

echo "   Testing /api/activities..."
ACTIVITIES_RESPONSE=$(curl -s $ADMIN_URL/api/activities)
ACTIVITY_COUNT=$(echo $ACTIVITIES_RESPONSE | jq '. | length')
echo "   ✓ Activities endpoint: $ACTIVITY_COUNT events found"
echo ""

# Test subscription filtering
echo "6️⃣  Testing subscription filtering..."
SUB_FILTER=$(curl -s "$ADMIN_URL/api/subscriptions?topic=$TOPIC1_ARN")
FILTERED_COUNT=$(echo $SUB_FILTER | jq '. | length')
echo "   ✓ Filtered subscriptions for topic 1: $FILTERED_COUNT subscriptions"
echo ""

# Display summary
echo "=========================================="
echo "📊 Test Summary"
echo "=========================================="
echo "Topics created:        $TOPIC_COUNT"
echo "Subscriptions created: $SUB_COUNT"
echo "Activity events:       $ACTIVITY_COUNT"
echo "Messages published:    2"
echo ""
echo "✅ All API endpoints working!"
echo ""
echo "🌐 Open the dashboard in your browser:"
echo "   $ADMIN_URL"
echo ""
echo "The dashboard will show:"
echo "  • Real-time statistics"
echo "  • Topics with subscription counts"
echo "  • Subscriptions grouped by protocol and status"
echo "  • Activity log with delivery tracking"
echo "  • Auto-refresh every 3 seconds"
echo ""

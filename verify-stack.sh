#!/bin/bash
# Verify Full Stack Deployment
# Tests that all four emulator services are running and responsive

set -e

echo "🔍 Full Stack Service Verification"
echo "===================================="
echo ""

cd "$(dirname "$0")"

echo "📋 Checking Docker containers..."
echo ""

# Check if containers are running
RUNNING=$(docker ps --filter "status=running" --quiet | wc -l)
TOTAL=$(docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | wc -l)

echo "Containers: $RUNNING running out of $TOTAL total"
echo ""

# Check each service
echo "🔗 Service Health Checks:"
echo ""

# S3/essthree
echo -n "S3 (essthree) on port 9300:     "
if curl -s http://localhost:9300/health > /dev/null 2>&1; then
    echo "✅ HEALTHY"
else
    echo "❌ UNAVAILABLE"
fi

# CloudFront/cloudfauxnt
echo -n "CloudFront (cloudfauxnt) on 9310: "
if curl -s http://localhost:9310/health > /dev/null 2>&1; then
    echo "✅ HEALTHY"
else
    echo "❌ UNAVAILABLE"
fi

# SQS/ess-queue-ess
echo -n "SQS (ess-queue-ess) on port 9320: "
if curl -s http://localhost:9320/health > /dev/null 2>&1; then
    echo "✅ HEALTHY"
else
    echo "❌ UNAVAILABLE"
fi

# SNS API/ess-enn-ess
echo -n "SNS API (ess-enn-ess) on 9330:    "
if curl -s http://localhost:9330/health > /dev/null 2>&1; then
    echo "✅ HEALTHY"
else
    echo "❌ UNAVAILABLE"
fi

# SNS Dashboard/ess-enn-ess admin
echo -n "SNS Dashboard on port 9331:       "
if curl -s http://localhost:9331/health > /dev/null 2>&1; then
    echo "✅ HEALTHY"
else
    echo "❌ UNAVAILABLE"
fi

echo ""
echo "📊 Container Details:"
echo ""
docker-compose ps 2>/dev/null || (echo "No containers running from docker-compose" && echo "")

echo ""
echo "🚀 If all services are healthy, you can access them at:"
echo "   S3:                 http://localhost:9300"
echo "   CloudFront:         http://localhost:9310"
echo "   SQS:                http://localhost:9320"
echo "   SNS API:            http://localhost:9330"
echo "   SNS Dashboard:      http://localhost:9331"
echo ""

# Count healthy services
HEALTHY=0
[ "$(curl -s http://localhost:9300/health 2>/dev/null)" != "" ] && ((HEALTHY++))
[ "$(curl -s http://localhost:9310/health 2>/dev/null)" != "" ] && ((HEALTHY++))
[ "$(curl -s http://localhost:9320/health 2>/dev/null)" != "" ] && ((HEALTHY++))
[ "$(curl -s http://localhost:9330/health 2>/dev/null)" != "" ] && ((HEALTHY++))
[ "$(curl -s http://localhost:9331/health 2>/dev/null)" != "" ] && ((HEALTHY++))

echo "Summary: $HEALTHY/5 services health check passed"
echo ""

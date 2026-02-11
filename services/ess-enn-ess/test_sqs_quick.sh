#!/bin/bash
# Quick test script for SNS-SQS integration
# This script starts both services and runs an integration test

set -e

echo "Starting SNS-SQS Integration Test Environment"
echo "=============================================="

# Check if we're in the right directory
if [ ! -f "./ess-enn-ess" ]; then
    echo "Building ess-enn-ess..."
    go build -o ess-enn-ess ./cmd/ess-enn-ess
fi

# Start ess-queue-ess if not running
if ! curl -s http://localhost:9320/health > /dev/null 2>&1; then
    echo "Starting ess-queue-ess..."
    if [ -f "../ess-queue-ess/ess-queue-ess" ]; then
        (cd ../ess-queue-ess && ./ess-queue-ess --config config.yaml > /tmp/ess-queue-ess.log 2>&1 &)
        echo $! > /tmp/ess-queue-ess.pid
        sleep 2
    else
        echo "ERROR: ess-queue-ess not found. Please build it first."
        exit 1
    fi
fi

# Start ess-enn-ess if not running
if ! curl -s http://localhost:9330/health > /dev/null 2>&1; then
    echo "Starting ess-enn-ess..."
    ./ess-enn-ess -config ../../config/ess-enn-ess.config.yaml > /tmp/ess-enn-ess.log 2>&1 &
    echo $! > /tmp/ess-enn-ess.pid
    sleep 2
fi

echo ""
echo "Services started!"
echo "  - ess-queue-ess: http://localhost:9320"
echo "  - ess-enn-ess:   http://localhost:9330"
echo "  - SNS Admin UI:  http://localhost:9331"
echo ""

# Run the integration test
echo "Running integration test..."
echo ""
./test_sqs_integration.sh

echo ""
echo "Test complete!"
echo ""
echo "To view logs:"
echo "  tail -f /tmp/ess-queue-ess.log"
echo "  tail -f /tmp/ess-enn-ess.log"
echo ""
echo "To stop services:"
echo "  kill \$(cat /tmp/ess-queue-ess.pid) 2>/dev/null"
echo "  kill \$(cat /tmp/ess-enn-ess.pid) 2>/dev/null"

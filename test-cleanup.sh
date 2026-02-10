#!/bin/bash
# Test cleanup and verify all containers are gone

echo "🧪 Testing Cleanup Procedure"
echo "=============================="
echo ""

cd "$(dirname "$0")"

echo "Step 1: Running make down..."
make down 2>&1 | tail -5

echo ""
echo "Step 2: Waiting 2 seconds for cleanup to complete..."
sleep 2

echo ""
echo "Step 3: Checking for remaining containers..."
REMAINING=$(docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three" --quiet | wc -l)

if [ "$REMAINING" -eq 0 ]; then
    echo "✅ SUCCESS: All containers cleaned up!"
    exit 0
else
    echo "❌ FAILED: $REMAINING containers still running:"
    docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three"
    
    echo ""
    echo "Attempting aggressive cleanup..."
    bash cleanup-stack.sh
    
    echo ""
    echo "Retrying verification..."
    REMAINING=$(docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three" --quiet | wc -l)
    if [ "$REMAINING" -eq 0 ]; then
        echo "✅ SUCCESS: All containers cleaned up!"
        exit 0
    else
        echo "❌ FAILED: Still $REMAINING containers remaining:"
        docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three"
        exit 1
    fi
fi

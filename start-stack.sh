#!/bin/bash
# Full Stack Docker Compose Startup Script
# This script properly starts all emulator services

set -e

cd "$(dirname "$0")"

echo "🧹 Cleaning up old docker-compose.yml files from subdirectories..."
mv -f services/essthree/docker-compose.yml services/essthree/docker-compose.yml.backup 2>/dev/null || true
mv -f services/ess-queue-ess/docker-compose.yml services/ess-queue-ess/docker-compose.yml.backup 2>/dev/null || true  
mv -f services/cloudfauxnt/docker-compose.yml services/cloudfauxnt/docker-compose.yml.backup 2>/dev/null || true

echo "🛑 Stopping any running containers..."
docker-compose down 2>/dev/null || true

echo ""
echo "🚀 Starting all emulator services from main docker-compose.yml..."
echo "   - essthree (S3) on port 9300"
echo "   - cloudfauxnt (CloudFront) on port 9310"
echo "   - ess-queue-ess (SQS) on port 9320"
echo "   - ess-enn-ess (SNS) on ports 9330-9331"
echo ""

docker-compose up -d

echo ""
sleep 3
echo "✅ Containers deployed. Checking status..."
docker-compose ps

echo ""
echo "🌐 Services available at:"
echo "   S3 (essthree):        http://localhost:9300"
echo "   CloudFront:           http://localhost:9310"
echo "   SQS (ess-queue-ess):  http://localhost:9320"
echo "   SNS API:              http://localhost:9330"
echo "   SNS Dashboard:        http://localhost:9331"

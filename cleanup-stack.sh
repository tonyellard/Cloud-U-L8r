#!/bin/bash
# Comprehensive Stack Cleanup and Shutdown
# Removes all emulator containers, networks, and volumes

set -e

echo "🧹 Comprehensive Stack Cleanup"
echo "=============================="
echo ""

cd "$(dirname "$0")"

echo "� Stopping docker-compose in all subdirectories first..."
for service_dir in services/*/; do
    if [[ -f "$service_dir/docker-compose.yml" ]]; then
        echo "  Stopping $service_dir..."
        cd "$service_dir"
        docker-compose down -v 2>/dev/null || true
        cd - > /dev/null
    fi
done
echo ""

echo "�📋 Current container status:"
docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three" --quiet || echo "(none found)"

echo ""
echo "🛑 Stopping and removing containers from main docker-compose..."
docker-compose down -v 2>/dev/null || true

echo ""
echo "🛑 Stopping all matching containers by filter..."
docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker stop 2>/dev/null || true

echo ""
echo "🧹 Removing stray containers by name..."
docker rm -f essthree 2>/dev/null || true
docker rm -f ess-three 2>/dev/null || true
docker rm -f cloudfauxnt 2>/dev/null || true
docker rm -f ess-queue-ess 2>/dev/null || true
docker rm -f ess-enn-ess 2>/dev/null || true

echo ""
echo "🧹 Removing any remaining emulator containers by ID..."
docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker rm -f 2>/dev/null || true

echo ""
echo "🧹 Removing stray networks and volumes..."
docker network rm cloud-u-l8r_shared-network 2>/dev/null || true

echo ""
echo "✅ Cleanup complete!"
echo ""
echo "Remaining containers:"
docker ps -a --filter "name=essthree\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|ess-three" || echo "(all cleaned up)"
echo ""


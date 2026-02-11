.PHONY: all build rebuild up down logs test clean status help

# ============================================================
# Cloud-U-L8r Stack Management
# ============================================================
# IMPORTANT: Always use 'make' commands to manage the stack.
# Never run 'docker compose' or 'docker' commands directly.
# This Makefile ensures proper build ordering and cleanup.
# ============================================================

# Default target
all: build

# Build all docker services (rebuild all images)
build:
	@echo "Building all services..."
	docker compose build --parallel 2

# Force a full rebuild (clean old images and rebuild)
rebuild: clean
	@echo "Full rebuild in progress..."
	docker compose build --no-cache --parallel 2
	@echo "✅ Full rebuild complete"

# Start services in detached mode (depends on build to ensure fresh images)
up: build
	@echo "Starting services..."
	docker compose up -d
	@echo "✅ All services started"

# Stop services and remove containers (including any stray containers)
down:
	@echo "Stopping services..."
	docker compose down -v
	@echo "Stopping any remaining emulator containers..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker stop 2>/dev/null || true
	@echo "Removing stray containers by name..."
	@docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess 2>/dev/null || true
	@echo "Removing any remaining emulator containers by ID..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker rm -f 2>/dev/null || true
	@echo "✅ All services stopped and cleaned up"

# View logs
logs:
	docker compose logs -f

# Show status of all containers
status:
	docker ps -a

# Run tests in all services
test:
	@echo "Running unit tests..."
	@cd services/essthree && go test ./... || true
	@cd services/cloudfauxnt && go test ./... || true
	@cd services/ess-queue-ess && go test ./... || true
	@echo ""
	@echo "Running integration tests..."
	@./tests/integration/test_cross_service.sh

# Clean up docker artifacts (containers, volumes, images)
clean:
	@echo "Cleaning up all Docker artifacts..."
	@docker compose down -v 2>/dev/null || true
	@echo "Removing build images..."
	@docker rmi cloud-u-l8r-essthree cloud-u-l8r-cloudfauxnt cloud-u-l8r-ess-queue-ess cloud-u-l8r-ess-enn-ess 2>/dev/null || true
	@echo "Stopping any remaining emulator containers..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker stop 2>/dev/null || true
	@echo "Removing stray containers by name..."
	@docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess 2>/dev/null || true
	@echo "Removing any remaining emulator containers by ID..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess" --quiet | xargs -r docker rm -f 2>/dev/null || true
	@echo "Removing stray volumes..."
	@docker volume rm cloud-u-l8r_shared-volume 2>/dev/null || true
	@echo "Removing shared network..."
	@docker network rm cloud-u-l8r_shared-network 2>/dev/null || true
	@echo "✅ Cleanup complete"

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build all Docker images (with cache)"
	@echo "  rebuild      - Clean and rebuild all Docker images (no cache, forces fresh build)"
	@echo "  up           - Build and start all services (automatically rebuilds if code changed)"
	@echo "  down         - Stop all services (removes stray containers)"
	@echo "  logs         - View logs from all services"
	@echo "  status       - Show status of all Docker containers"
	@echo "  test         - Run Go tests in all services"
	@echo "  clean        - Remove containers, volumes, networks, and images (full reset)"
	@echo ""
	@echo "Common workflows:"
	@echo "  make up              - Start fresh with latest code"
	@echo "  make down            - Stop everything cleanly"
	@echo "  make logs            - View container output"
	@echo "  make clean && make up - Full reset and restart"
	@echo "  make rebuild         - Force full rebuild (no cache)"

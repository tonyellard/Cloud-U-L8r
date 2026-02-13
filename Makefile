.PHONY: all build rebuild up down logs test clean clean-ports stop-service start-service restart-service status help

SERVICE ?=
SERVICE_GOAL := $(word 2,$(MAKECMDGOALS))
SERVICE_COMMANDS := stop-service start-service restart-service

ifneq (,$(filter $(firstword $(MAKECMDGOALS)),$(SERVICE_COMMANDS)))
ifneq ($(SERVICE_GOAL),)
SERVICE := $(SERVICE_GOAL)
$(SERVICE_GOAL):
	@:
endif
endif

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
	docker compose --parallel 2 build

# Force a full rebuild (clean old images and rebuild)
rebuild: clean
	@echo "Full rebuild in progress..."
	docker compose --no-cache --parallel 2 build
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
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|admin-console" --quiet | xargs -r docker stop 2>/dev/null || true
	@echo "Removing stray containers by name..."
	@docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess admin-console 2>/dev/null || true
	@echo "Removing any remaining emulator containers by ID..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|admin-console" --quiet | xargs -r docker rm -f 2>/dev/null || true
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
	@cd services/admin-console && go test ./... || true
	@echo ""
	@echo "Running integration tests..."
	@./tests/integration/test_cross_service.sh

# Clean up docker artifacts (containers, volumes, images)
clean:
	@echo "Cleaning up all Docker artifacts..."
	@docker compose down -v 2>/dev/null || true
	@echo "Removing build images..."
	@docker rmi cloud-u-l8r-essthree cloud-u-l8r-cloudfauxnt cloud-u-l8r-ess-queue-ess cloud-u-l8r-ess-enn-ess cloud-u-l8r-admin-console 2>/dev/null || true
	@echo "Stopping any remaining emulator containers..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|admin-console" --quiet | xargs -r docker stop 2>/dev/null || true
	@echo "Removing stray containers by name..."
	@docker rm -f essthree ess-three cloudfauxnt ess-queue-ess ess-enn-ess admin-console 2>/dev/null || true
	@echo "Removing any remaining emulator containers by ID..."
	@docker ps -a --filter "name=essthree\|ess-three\|cloudfauxnt\|ess-queue-ess\|ess-enn-ess\|admin-console" --quiet | xargs -r docker rm -f 2>/dev/null || true
	@echo "Removing stray volumes..."
	@docker volume rm cloud-u-l8r_shared-volume 2>/dev/null || true
	@echo "Removing shared network..."
	@docker network rm cloud-u-l8r_shared-network 2>/dev/null || true
	@echo "✅ Cleanup complete"

# Kill processes bound to service ports
clean-ports:
	@echo "Cleaning service ports with fuser..."
	@sudo fuser -k 9300/tcp 9310/tcp 9320/tcp 9330/tcp 9340/tcp 2>/dev/null || true
	@echo "✅ Service ports cleaned"

# Stop a single service by name
stop-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make stop-service SERVICE=<service> or make stop-service <service>"; \
		exit 1; \
	fi
	@echo "Stopping service: $(SERVICE)"
	@docker compose stop "$(SERVICE)"
	@echo "✅ Service stopped: $(SERVICE)"

# Start a single service by name
start-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make start-service SERVICE=<service> or make start-service <service>"; \
		exit 1; \
	fi
	@echo "Starting service: $(SERVICE)"
	@docker compose start "$(SERVICE)"
	@echo "✅ Service started: $(SERVICE)"

# Restart a single service by name
restart-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "Usage: make restart-service SERVICE=<service> or make restart-service <service>"; \
		exit 1; \
	fi
	@echo "Restarting service: $(SERVICE)"
	@docker compose restart "$(SERVICE)"
	@echo "✅ Service restarted: $(SERVICE)"

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
	@echo "  clean-ports  - Kill processes using service ports (9300, 9310, 9320, 9330, 9340)"
	@echo "  stop-service - Stop one service (use SERVICE=<name> or positional name)"
	@echo "  start-service - Start one service (use SERVICE=<name> or positional name)"
	@echo "  restart-service - Restart one service (use SERVICE=<name> or positional name)"
	@echo ""
	@echo "Common workflows:"
	@echo "  make up              - Start fresh with latest code"
	@echo "  make down            - Stop everything cleanly"
	@echo "  make logs            - View container output"
	@echo "  make clean && make up - Full reset and restart"
	@echo "  make rebuild         - Force full rebuild (no cache)"

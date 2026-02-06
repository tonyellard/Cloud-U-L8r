.PHONY: all build up down logs test clean help

# Default target
all: build

# Build all docker services
build:
	@echo "Building all services..."
	docker compose build

# Start services in detached mode
up:
	@echo "Starting services..."
	docker compose up -d

# Stop services and remove containers
down:
	@echo "Stopping services..."
	docker compose down

# View logs
logs:
	docker compose logs -f

# Run tests in all services
test:
	@echo "Running tests..."
	@cd services/essthree && go test ./... || true
	@cd services/cloudfauxnt && go test ./... || true
	@cd services/ess-queue-ess && go test ./... || true

# Clean up docker artifacts
clean:
	@echo "Cleaning up..."
	docker compose down -v --rmi local

# Show help
help:
	@echo "Available targets:"
	@echo "  build  - Build all Docker images"
	@echo "  up     - Start all services"
	@echo "  down   - Stop all services"
	@echo "  logs   - View logs from all services"
	@echo "  test   - Run Go tests in all services"
	@echo "  clean  - Remove containers, volumes, and images"

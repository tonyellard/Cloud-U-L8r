.PHONY: build run test test-coverage clean docker-build docker-run docker-stop docker-logs help

# Variables
BINARY_NAME=ess-three
DOCKER_IMAGE=ess-three:latest
DATA_DIR=./data
PORT=9300

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application binary
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) ./cmd/ess-three

run: ## Run the application locally
	@echo "Running $(BINARY_NAME)..."
	@go run ./cmd/ess-three --port=$(PORT) --data-dir=$(DATA_DIR)

test: ## Run unit tests
	@echo "Running unit tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -cover ./...

integration-test: ## Run integration tests
	@echo "Running integration tests..."
	@python3 test/integration_test.py

clean: ## Clean build artifacts and data
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(DATA_DIR)/
	@go clean

deps: ## Install and update dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run with Docker Compose
	@echo "Starting ess-three with Docker Compose..."
	@docker-compose up -d
	@echo "ess-three is running at http://localhost:$(PORT)"

docker-stop: ## Stop Docker Compose
	@echo "Stopping ess-three..."
	@docker-compose down

docker-logs: ## View Docker container logs
	@docker-compose logs -f

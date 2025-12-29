.PHONY: build run test clean docker-build docker-run help

# Variables
BINARY_NAME=api-doc-generator
DOCKER_IMAGE=api-doc-generator:latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	go build -o $(BINARY_NAME) ./cmd/server

run: ## Run the application locally
	go run ./cmd/server/main.go

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -f $(BINARY_NAME)
	rm -rf /tmp/repos/*

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run Docker container
	docker-compose up -d

docker-stop: ## Stop Docker container
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

deps: ## Download dependencies
	go mod download
	go mod tidy

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

fmt: ## Format code
	go fmt ./...

dev: ## Run in development mode with auto-reload (requires air)
	air

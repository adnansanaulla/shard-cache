.PHONY: proto build run-local test race bench lint fmt cover docker clean

# Build variables
BINARY_NAME=shard-cache
BUILD_DIR=build
DOCKER_IMAGE=shard-cache

# Go variables
GO=go
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)

# Default target
all: build

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@if command -v protoc >/dev/null 2>&1; then \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			proto/cache.proto; \
	else \
		echo "protoc not found, using pre-generated files"; \
	fi

# Build the application
build: proto
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# Run locally with 3 nodes
run-local: build
	@echo "Starting 3 local cache nodes..."
	@mkdir -p logs
	@$(BUILD_DIR)/$(BINARY_NAME) -grpc-port=8080 -http-port=8081 > logs/node1.log 2>&1 & echo $$! > logs/node1.pid
	@$(BUILD_DIR)/$(BINARY_NAME) -grpc-port=8082 -http-port=8083 > logs/node2.log 2>&1 & echo $$! > logs/node2.pid
	@$(BUILD_DIR)/$(BINARY_NAME) -grpc-port=8084 -http-port=8085 > logs/node3.log 2>&1 & echo $$! > logs/node3.pid
	@echo "Cache nodes started. PIDs saved in logs/ directory"
	@echo "Node 1: gRPC:8080, HTTP:8081"
	@echo "Node 2: gRPC:8082, HTTP:8083"
	@echo "Node 3: gRPC:8084, HTTP:8085"

# Stop local nodes
stop-local:
	@echo "Stopping local cache nodes..."
	@if [ -f logs/node1.pid ]; then kill $$(cat logs/node1.pid) 2>/dev/null || true; rm logs/node1.pid; fi
	@if [ -f logs/node2.pid ]; then kill $$(cat logs/node2.pid) 2>/dev/null || true; rm logs/node2.pid; fi
	@if [ -f logs/node3.pid ]; then kill $$(cat logs/node3.pid) 2>/dev/null || true; rm logs/node3.pid; fi
	@echo "Local nodes stopped"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run tests with race detection
race:
	@echo "Running tests with race detection..."
	$(GO) test -race -v ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@mkdir -p bench
	$(GO) test -bench=. -benchmem ./internal/cache/... > bench/cache_bench.txt
	$(GO) test -bench=. -benchmem ./internal/ring/... > bench/ring_bench.txt
	@echo "Benchmarks completed. Results in bench/ directory"

# Run load testing with vegeta
bench-load: run-local
	@echo "Running load tests with vegeta..."
	@mkdir -p bench
	@if command -v vegeta >/dev/null 2>&1; then \
		echo 'GET http://localhost:8081/health' | vegeta attack -duration=60s -rate=100 | vegeta report > bench/load_test.txt; \
	else \
		echo "vegeta not found, skipping load tests"; \
	fi
	@$(MAKE) stop-local

# Run linting
lint:
	@echo "Running linting..."
	$(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, using go vet only"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Run tests with coverage
cover:
	@echo "Running tests with coverage..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# Run with Docker Compose
docker-run:
	@echo "Starting with Docker Compose..."
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	@echo "Stopping Docker Compose..."
	docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf logs
	rm -f coverage.out coverage.html
	rm -rf bench/*.txt

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  proto      - Generate protobuf code"
	@echo "  build      - Build the application"
	@echo "  run-local  - Run 3 local cache nodes"
	@echo "  stop-local - Stop local cache nodes"
	@echo "  test       - Run tests"
	@echo "  race       - Run tests with race detection"
	@echo "  bench      - Run benchmarks"
	@echo "  bench-load - Run load tests with vegeta"
	@echo "  lint       - Run linting"
	@echo "  fmt        - Format code"
	@echo "  cover      - Run tests with coverage"
	@echo "  docker     - Build Docker image"
	@echo "  docker-run - Run with Docker Compose"
	@echo "  docker-stop- Stop Docker Compose"
	@echo "  clean      - Clean build artifacts"
	@echo "  deps       - Install dependencies"
	@echo "  help       - Show this help" 
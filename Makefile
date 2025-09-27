# CSV Search Application Makefile

.PHONY: help build run dev clean test lint format install-deps generate-sample

# Default target
help: ## Show this help message
	@echo "CSV Search Application"
	@echo "====================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development
dev: ## Start development servers (Go backend + Vite frontend)
	@echo "Starting development servers..."
	@echo "Backend will be available at: http://localhost:8080"
	@echo "Frontend will be available at: http://localhost:3000"
	@echo ""
	@echo "Press Ctrl+C to stop both servers"
	@trap 'kill %1; kill %2' INT; \
		go run cmd/server/main.go & \
		npm run dev & \
		wait

# Backend
run: ## Run the Go backend server
	@echo "Starting Go backend server..."
	go run cmd/server/main.go

run-prod: ## Run the Go backend server in production mode
	@echo "Starting Go backend server in production mode..."
	go run cmd/server/main.go -csv=data.csv -port=8080 -max-results=1000

# Frontend
dev-frontend: ## Start only the frontend development server
	@echo "Starting frontend development server..."
	npm run dev

build-frontend: ## Build the frontend for production
	@echo "Building frontend for production..."
	npm run build

# Dependencies
install-deps: ## Install all dependencies (Go + Node.js)
	@echo "Installing Go dependencies..."
	go mod tidy
	@echo "Installing Node.js dependencies..."
	npm install

# Code Quality
lint: ## Run linting for both Go and TypeScript
	@echo "Linting Go code..."
	golangci-lint run
	@echo "Linting TypeScript code..."
	npm run lint

format: ## Format code (Go + TypeScript)
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting TypeScript code..."
	npm run lint:fix

# Testing
test: ## Run tests
	@echo "Running Go tests..."
	go test -v ./...
	@echo "Running TypeScript type checking..."
	npm run type-check

# Build
build: build-frontend ## Build the entire application
	@echo "Building Go application..."
	go build -o bin/csv-search cmd/server/main.go
	@echo "Build complete! Binary available at: bin/csv-search"

# Cleanup
clean: ## Clean build artifacts and dependencies
	@echo "Cleaning build artifacts..."
	rm -rf dist/
	rm -rf bin/
	rm -rf node_modules/
	go clean
	@echo "Cleanup complete!"

# Sample Data
generate-sample: ## Generate a sample CSV file for testing
	@echo "Generating sample CSV file..."
	@python3 -c "import csv, random, string; \
		with open('data.csv', 'w', newline='') as f: \
			writer = csv.writer(f); \
			writer.writerow(['id', 'name', 'email', 'company', 'department', 'salary', 'location', 'phone']); \
			[writer.writerow([i, f'User {i}', f'user{i}@example.com', random.choice(['Acme Corp', 'Tech Solutions', 'Global Inc', 'StartupXYZ']), random.choice(['Engineering', 'Sales', 'Marketing', 'HR']), random.randint(30000, 150000), random.choice(['New York', 'San Francisco', 'London', 'Tokyo']), f'+1-555-{random.randint(100, 999)}-{random.randint(1000, 9999)}']) for i in range(1, 10001)]"
	@echo "Sample CSV file generated: data.csv (10,000 rows)"

# Docker (optional)
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t csv-search .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 -v $(PWD)/data.csv:/app/data.csv csv-search

# Development utilities
check-deps: ## Check if all dependencies are installed
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed. Please install Go 1.21+"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "Node.js is not installed. Please install Node.js 18+"; exit 1; }
	@command -v npm >/dev/null 2>&1 || { echo "npm is not installed. Please install npm"; exit 1; }
	@echo "All dependencies are installed!"

setup: check-deps install-deps generate-sample ## Complete setup for development
	@echo "Setup complete! You can now run 'make dev' to start development servers."

# Performance testing
benchmark: ## Run performance benchmarks
	@echo "Running performance benchmarks..."
	go test -bench=. -benchmem ./internal/search/

# Health check
health: ## Check if the application is running
	@echo "Checking application health..."
	@curl -s http://localhost:8080/api/health | jq . || echo "Application is not running or jq is not installed"

# Quick start
quick-start: setup dev ## Quick start: setup and run development servers
	@echo "Quick start complete!"

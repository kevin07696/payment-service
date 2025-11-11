.PHONY: help build test run docker-build docker-up docker-down docker-logs proto clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the server binary
	@echo "Building server..."
	@go build -o bin/payment-server ./cmd/server
	@echo "✓ Build complete: bin/payment-server"

test: ## Run all tests (unit + integration)
	@echo "Running all tests..."
	@go test ./... -v

test-unit: ## Run unit tests only (skip integration)
	@echo "Running unit tests..."
	@go test -short ./... -v

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	@go test -v ./tests/integration/... -tags=integration

test-cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test ./... -cover -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

test-integration-cover: ## Run integration tests with coverage
	@echo "Running integration tests with coverage..."
	@go test -cover -coverprofile=integration-coverage.out ./tests/integration/... -tags=integration
	@go tool cover -html=integration-coverage.out -o integration-coverage.html
	@echo "✓ Integration coverage report: integration-coverage.html"

run: ## Run the server locally
	@echo "Starting server..."
	@./bin/payment-server

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t payment-service:latest .
	@echo "✓ Docker image built: payment-service:latest"

docker-up: ## Start services with docker-compose
	@echo "Starting services..."
	@docker-compose up -d
	@echo "✓ Services started"
	@echo "  Payment Server (gRPC): localhost:8080"
	@echo "  Payment Server (HTTP/Cron): localhost:8081"
	@echo "  PostgreSQL: localhost:5432"

docker-down: ## Stop services
	@echo "Stopping services..."
	@docker-compose down
	@echo "✓ Services stopped"

docker-logs: ## View docker-compose logs
	@docker-compose logs -f

docker-rebuild: docker-down docker-build docker-up ## Rebuild and restart services

test-db-up: ## Start test database
	@echo "Starting test database..."
	@docker-compose -f docker-compose.test.yml up -d
	@echo "✓ Test database started on port 5434"

test-db-down: ## Stop test database
	@echo "Stopping test database..."
	@docker-compose -f docker-compose.test.yml down
	@echo "✓ Test database stopped"

test-db-logs: ## View test database logs
	@docker-compose -f docker-compose.test.yml logs -f

proto: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@protoc -I. -Iproto --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent/v1/agent.proto \
		proto/chargeback/v1/chargeback.proto \
		proto/payment_method/v1/payment_method.proto \
		proto/payment/v1/payment.proto \
		proto/subscription/v1/subscription.proto
	@echo "✓ Protobuf code generated"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

sqlc: ## Generate SQLC code
	@echo "Generating SQLC code..."
	@sqlc generate
	@echo "✓ SQLC code generated"

migrate-up: ## Run all pending migrations
	@echo "Running migrations..."
	@goose -dir internal/db/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=payment_service sslmode=disable" up
	@echo "✓ Migrations complete"

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	@goose -dir internal/db/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=payment_service sslmode=disable" down
	@echo "✓ Rollback complete"

migrate-status: ## Show migration status
	@goose -dir internal/db/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=payment_service sslmode=disable" status

migrate-create: ## Create new migration (usage: make migrate-create NAME=add_users_table)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=add_users_table"; \
		exit 1; \
	fi
	@goose -dir internal/db/migrations create $(NAME) sql
	@echo "✓ Migration created: internal/db/migrations/$(NAME).sql"

lint: ## Run linters
	@echo "Running linters..."
	@go vet ./...
	@echo "✓ Linting complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✓ Dependencies downloaded"

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	@go mod tidy
	@echo "✓ Modules tidied"

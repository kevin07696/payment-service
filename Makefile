.PHONY: help build test test-unit test-integration test-cover test-integration-cover run docker-build docker-down docker-logs docker-rebuild docker-up test-db-up test-db-down test-db-logs proto clean sqlc migrate-up migrate-down migrate-status migrate-create paycli lint deps tidy docs docs-validate docs-api docs-schema docs-sync-wiki podman-build podman-up podman-down podman-logs podman-rebuild test-db-podman-up test-db-podman-down test-db-podman-logs test-unit-podman test-integration-podman test-e2e-podman test-all-podman

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
	@echo "  Payment Server: localhost:8081 (ConnectRPC + REST)"
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

# ============================================================================
# Podman targets - Use podman-compose for container management
# ============================================================================

podman-build: ## Build container image with Podman
	@echo "Building container image with Podman..."
	@podman build -t payment-service:latest .
	@echo "✓ Container image built: payment-service:latest"

podman-up: ## Start services with podman-compose
	@echo "Starting services with Podman..."
	@podman-compose up -d
	@echo "✓ Services started"
	@echo "  Payment Server: localhost:8081 (ConnectRPC + REST)"
	@echo "  PostgreSQL: localhost:5432"

podman-down: ## Stop services with podman-compose
	@echo "Stopping services..."
	@podman-compose down
	@echo "✓ Services stopped"

podman-logs: ## View podman-compose logs
	@podman-compose logs -f

podman-rebuild: podman-down podman-build podman-up ## Rebuild and restart services with Podman

test-db-podman-up: ## Start test database with Podman
	@echo "Starting test database with Podman..."
	@podman-compose -f docker-compose.test.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if podman exec payment-postgres-test pg_isready -U postgres > /dev/null 2>&1; then \
			echo "✓ Test database ready on port 5434"; \
			exit 0; \
		fi; \
		echo "  Waiting... ($$i/10)"; \
		sleep 2; \
	done; \
	echo "✗ Timed out waiting for database"; \
	exit 1

test-db-podman-down: ## Stop test database with Podman
	@echo "Stopping test database..."
	@podman-compose -f docker-compose.test.yml down
	@echo "✓ Test database stopped"

test-db-podman-logs: ## View test database logs with Podman
	@podman-compose -f docker-compose.test.yml logs -f

# ============================================================================
# Test targets with Podman
# ============================================================================

test-unit-podman: ## Run unit tests (no containers needed)
	@echo "Running unit tests..."
	@go test -short ./... -v
	@echo "✓ Unit tests complete"

test-integration-podman: test-db-podman-up ## Run integration tests with Podman DB
	@echo "Running migrations on test database..."
	@goose -dir internal/db/migrations postgres "host=localhost port=5434 user=postgres password=postgres dbname=payment_service_test sslmode=disable" up
	@echo "Running integration tests..."
	@TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5434/payment_service_test?sslmode=disable" \
		go test -v ./tests/integration/... -tags=integration
	@echo "✓ Integration tests complete"

test-e2e-podman: podman-up ## Run E2E tests with Podman services
	@echo "Waiting for payment server to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		if curl -sf http://localhost:8081/cron/health > /dev/null 2>&1; then \
			echo "✓ Server ready"; \
			break; \
		fi; \
		echo "  Waiting for server... ($$i/15)"; \
		sleep 2; \
	done
	@echo "Running E2E tests..."
	@cd tests/e2e && npm test
	@echo "✓ E2E tests complete"

test-all-podman: test-unit-podman test-integration-podman test-e2e-podman ## Run all tests with Podman
	@echo ""
	@echo "============================================"
	@echo "✓ All tests complete (unit + integration + e2e)"
	@echo "============================================"

proto: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@protoc -I. -Iproto --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--connect-go_out=. --connect-go_opt=paths=source_relative \
		proto/merchant/v1/merchant.proto \
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

paycli: ## Run paycli for managing services and merchants
	@go run cmd/paycli/main.go $(ARGS)

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

# Documentation targets
docs: docs-api docs-schema ## Generate all documentation
	@echo "✓ All documentation generated"

docs-validate: ## Validate documentation (check for broken links, TODOs, required headers)
	@echo "Validating documentation..."
	@./scripts/validate_docs.sh

docs-api: ## Generate API documentation from proto files
	@echo "Generating API documentation from proto files..."
	@./scripts/generate_api_docs.sh

docs-schema: ## Generate database schema documentation
	@echo "Generating database schema documentation..."
	@./scripts/generate_schema_docs.sh

docs-sync-wiki: ## Sync documentation to GitHub wiki
	@echo "Syncing documentation to GitHub wiki..."
	@./scripts/sync_to_wiki.sh

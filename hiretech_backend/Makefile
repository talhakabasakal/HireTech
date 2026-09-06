.PHONY: build run test test-integration test-integration-postgres test-integration-redis test-integration-kafka test-ai-benchmark-validate production-readiness lint migrate migrate-down docker-up docker-down clean help

APP_NAME := masterfabric
BUILD_DIR := bin
MAIN_PATH := ./cmd/server
MIGRATION_DIR := internal/infrastructure/postgres/migrations
DB_DSN ?= postgres://masterfabric:masterfabric@localhost:5432/masterfabric?sslmode=disable

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

## run: Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_PATH)

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -race -count=1 ./...

## test-integration: Run the real PostgreSQL and Redis integration gates
test-integration: test-integration-postgres test-integration-redis

## test-integration-postgres: Run the PostgreSQL lifecycle integration test (requires an isolated DSN)
test-integration-postgres:
	@test -n "$(INTERVIEW_TEST_DSN)" || (echo "INTERVIEW_TEST_DSN is required; refusing to report a skipped integration test as success" >&2; exit 1)
	INTERVIEW_TEST_DSN="$(INTERVIEW_TEST_DSN)" go test -count=1 ./internal/infrastructure/postgres/interview -run '^TestInterviewLifecyclePostgres$$'

## test-integration-redis: Run the shared Redis rate-limit integration test
test-integration-redis:
	@test -n "$(REDIS_TEST_ADDR)" || (echo "REDIS_TEST_ADDR is required; refusing to report a skipped integration test as success" >&2; exit 1)
	REDIS_TEST_ADDR="$(REDIS_TEST_ADDR)" go test -count=1 ./internal/application/iam/usecase -run '^TestRedisRateLimiterIntegration$$'

## test-integration-kafka: Verify realtime fan-out reaches two instance-scoped consumer groups
test-integration-kafka:
	@test -n "$(KAFKA_TEST_BROKER)" || (echo "KAFKA_TEST_BROKER is required; refusing to report a skipped integration test as success" >&2; exit 1)
	KAFKA_TEST_BROKER="$(KAFKA_TEST_BROKER)" go test -count=1 ./internal/infrastructure/kafka -run '^TestKafkaRealtimeFanoutIntegration$$'

## test-ai-benchmark-validate: Verify frozen AI interviewer and evaluator test split manifests
test-ai-benchmark-validate:
	python3 ai/benchmarks/run_benchmark.py --role interviewer --dataset ai/training/data/curated/interviewer/test.jsonl --validate-only
	python3 ai/benchmarks/run_benchmark.py --role evaluator --dataset ai/training/data/curated/evaluator/test.jsonl --validate-only

## production-readiness: Assemble fail-closed evidence; does not deploy
production-readiness:
	@test -n "$(RELEASE_EVIDENCE)" || (echo "RELEASE_EVIDENCE is required; refusing to infer approvals" >&2; exit 1)
	python3 scripts/production_readiness.py --evidence "$(RELEASE_EVIDENCE)"

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## migrate: Run database migrations up
migrate:
	@echo "Running migrations..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" up

## migrate-down: Rollback last migration
migrate-down:
	@echo "Rolling back last migration..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" down

## migrate-status: Show migration status
migrate-status:
	@echo "Migration status..."
	goose -dir $(MIGRATION_DIR) postgres "$(DB_DSN)" status

## docker-up: Start Docker Compose services
docker-up:
	@echo "Starting Docker services..."
	docker compose -f deployments/docker-compose.yml up -d

## docker-down: Stop Docker Compose services
docker-down:
	@echo "Stopping Docker services..."
	docker compose -f deployments/docker-compose.yml down

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -f deployments/Dockerfile -t $(APP_NAME):latest .

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html tmp

## seed: Seed the database with sample data
seed:
	@echo "Seeding database..."
	go run ./scripts/seed.go

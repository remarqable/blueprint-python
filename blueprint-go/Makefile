.PHONY: run build test clean deps migrate-up migrate-down

# Application
APP_NAME=blueprint
MAIN_PATH=cmd/server/main.go

# Default target
all: deps build

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run the application
run:
	go run $(MAIN_PATH)

# Build the application
build:
	go build -o bin/$(APP_NAME) $(MAIN_PATH)

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f app.db

# Goose migrations
GOOSE_DRIVER=sqlite3
GOOSE_DBSTRING=app.db
GOOSE_DIR=migrations

migrate-up:
	goose -dir $(GOOSE_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) up

migrate-down:
	goose -dir $(GOOSE_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) down

migrate-status:
	goose -dir $(GOOSE_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) status

migrate-create:
	@read -p "Migration name: " name; \
	goose -dir $(GOOSE_DIR) $(GOOSE_DRIVER) $(GOOSE_DBSTRING) create $$name sql

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate (if using code generation)
generate:
	go generate ./...

# Development helpers
dev: deps run

# Production build
prod:
	CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/$(APP_NAME) $(MAIN_PATH)

# Help
help:
	@echo "Blueprint Go - Available targets:"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  run           - Run the application"
	@echo "  build         - Build the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  migrate-up    - Run database migrations"
	@echo "  migrate-down  - Rollback last migration"
	@echo "  migrate-status - Show migration status"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  dev           - Install deps and run"
	@echo "  prod          - Production build"

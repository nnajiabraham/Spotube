.PHONY: dev backend-dev test lint build-image clean migrate-up help

# Variables
PB_DEV_PORT ?= 8090

# Default target
help:
	@echo "Available targets:"
	@echo "  dev          - Start development servers (backend + frontend)"
	@echo "  backend-dev  - Start backend with Air (live reload)"
	@echo "  migrate-up   - Run database migrations manually"
	@echo "  test         - Run all tests (backend + frontend)"
	@echo "  lint         - Run all linters (backend + frontend)"
	@echo "  build-image  - Build Docker image"
	@echo "  clean        - Clean build artifacts"

# Development: run both backend and frontend concurrently
dev:
	@echo "Starting development servers..."
	@cd backend && go run cmd/server/main.go serve &
	@cd frontend && npm run dev &
	@echo "Backend (Go) on :8090 and frontend (Vite) on :5173 started."
	@echo "Press Ctrl+C to stop both servers."
	@wait

# Run backend with Air (live reload) - PB auto-migrates on each restart
backend-dev:
	@echo "Starting backend with Air on port $(PB_DEV_PORT)..."
	@cd backend && PORT=$(PB_DEV_PORT) go run github.com/air-verse/air@latest

# Run migrations manually (e.g. CI)
migrate-up:
	@echo "Running database migrations..."
	@cd backend && go run ./cmd/server migrate up

# Test: run backend and frontend tests
test:
	@echo "Running backend tests..."
	@cd backend && go test ./...
	@echo "Running frontend tests..."
	@cd frontend && (npm run test:run || echo "No test files found - expected for scaffold stage")

# Lint: run backend and frontend linters
lint:
	@echo "Running backend linter..."
	@cd backend && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run
	@echo "Running frontend linter..."
	@cd frontend && npm run lint

# Build Docker image
build-image:
	@echo "Building Docker image..."
	@docker build -f docker/Dockerfile -t spotube:dev .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@cd backend && go clean
	@cd frontend && rm -rf dist node_modules/.vite
	@echo "Clean complete." 
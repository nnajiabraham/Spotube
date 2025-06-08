.PHONY: dev backend-dev frontend-dev test test-backend test-frontend test-e2e lint build-image clean migrate-up help

# Variables
PB_DEV_PORT ?= 8090

# Default target
help:
	@echo "Available targets:"
	@echo "  dev             - Start development servers (backend + frontend)"
	@echo "  backend-dev     - Start backend with Air (live reload)"
	@echo "  frontend-dev    - Start frontend Vite server only"
	@echo "  migrate-up      - Run database migrations manually"
	@echo "  test            - Run all tests (backend + frontend)"
	@echo "  test-backend    - Run backend tests only"
	@echo "  test-frontend   - Run frontend unit tests only"
	@echo "  test-e2e        - Run frontend E2E tests (requires backend running)"
	@echo "  lint            - Run all linters (backend + frontend)"
	@echo "  build-image     - Build Docker image"
	@echo "  clean           - Clean build artifacts"

# Development: run both backend and frontend concurrently
dev:
	@echo "Starting development servers..."
	@cd backend && PORT=$(PB_DEV_PORT) go run github.com/air-verse/air@latest &
	@cd frontend && npm run dev &
	@echo "Backend (Go with Air) on :$(PB_DEV_PORT) and frontend (Vite) on :5173 started."
	@echo "Press Ctrl+C to stop both servers."
	@wait

# Run backend with Air (live reload) - PB auto-migrates on each restart
backend-dev:
	@echo "Starting backend with Air on port $(PB_DEV_PORT)..."
	@cd backend && PORT=$(PB_DEV_PORT) go run github.com/air-verse/air@latest

# Run frontend dev server only
frontend-dev:
	@echo "Starting frontend dev server on port 5173..."
	@cd frontend && npm run dev

# Run migrations manually (e.g. CI)
migrate-up:
	@echo "Running database migrations..."
	@cd backend && go run ./cmd/server migrate up

# Test: run backend and frontend tests
test: test-backend test-frontend
	@echo "All tests completed"

# Run backend tests only
test-backend:
	@echo "Running backend tests..."
	@cd backend && go test ./...

# Run frontend unit tests only
test-frontend:
	@echo "Running frontend unit tests..."
	@cd frontend && npm run test:run

# Run frontend E2E tests (requires backend running)
test-e2e:
	@echo "Running frontend E2E tests..."
	@echo "Note: Backend should be running on port 8090"
	@cd frontend && npm run test:e2e

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

kill-dev:
	@echo "Killing development servers..."
	@npx kill-port 8090 5173 5174 5175 5176
	@echo "Development servers killed."
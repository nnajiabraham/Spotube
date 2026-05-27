BACKEND_DIR := backend
DB_PATH := ./data/spotube.db
BINARY_NAME := spotube-server
DEV_LOGS := dev.logs

.PHONY: backend/install backend/dev backend/test backend/build backend/lint backend/clean
.PHONY: backend/db/create backend/db/up backend/db/down backend/db/gen

backend/install: ## Install backend dependencies
	@cd $(BACKEND_DIR) && go mod download

backend/dev: ## Start backend server (logs append to ../dev.logs; no terminal output)
	@printf '\n[backend] starting at %s\n' "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> ../$(DEV_LOGS)
	@$(MAKE) backend/db/up >> ../$(DEV_LOGS) 2>&1
	@$(MAKE) backend/db/gen >> ../$(DEV_LOGS) 2>&1
	@cd $(BACKEND_DIR) && go run ./cmd/server >> ../$(DEV_LOGS) 2>&1

backend/test: ## Run backend tests
	@cd $(BACKEND_DIR) && go test ./...

backend/build: ## Build backend binary
	@cd $(BACKEND_DIR) && go build -o ../dist/$(BINARY_NAME) ./cmd/server

backend/lint: ## Run backend linters
	@cd $(BACKEND_DIR) && go fmt ./...
	@cd $(BACKEND_DIR) && go vet ./...

backend/clean: ## Clean backend build artifacts
	@rm -rf dist/$(BINARY_NAME)

backend/db/create: ## Create new migration (usage: make backend/db/create NAME=xyz)
	@cd $(BACKEND_DIR) && go run github.com/pressly/goose/v3/cmd/goose@latest -dir ./migrations create $(NAME) sql

backend/db/up: ## Apply pending migrations
	@cd $(BACKEND_DIR) && mkdir -p $(dir $(DB_PATH)) && DB_PATH=$(DB_PATH) go run ./cmd/migrate up

backend/db/down: ## Roll back one migration
	@cd $(BACKEND_DIR) && DB_PATH=$(DB_PATH) go run ./cmd/migrate down

backend/db/gen: ## Run Jet codegen
	@$(MAKE) backend/db/up
	@cd $(BACKEND_DIR) && rm -rf internal/db && mkdir -p internal/db
	@cd $(BACKEND_DIR) && go run github.com/go-jet/jet/v2/cmd/jet@v2.13.0 -source=sqlite -dsn=$(DB_PATH) -path internal/db

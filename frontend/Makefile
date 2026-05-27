FRONTEND_DIR := frontend
DEV_LOGS := dev.logs

.PHONY: frontend/install frontend/dev frontend/test frontend/build frontend/lint frontend/type-check frontend/clean

frontend/install: ## Install frontend dependencies
	@cd $(FRONTEND_DIR) && npm install

frontend/dev: ## Start Vite dev server (logs append to ../dev.logs; no terminal output)
	@printf '\n[frontend] starting at %s\n' "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> ../$(DEV_LOGS)
	@cd $(FRONTEND_DIR) && npm run dev >> ../$(DEV_LOGS) 2>&1

frontend/test: ## Run frontend tests
	@cd $(FRONTEND_DIR) && npm run test

frontend/build: ## Build frontend for production
	@cd $(FRONTEND_DIR) && npm run build

frontend/lint: ## Run frontend lint checks
	@cd $(FRONTEND_DIR) && npm run lint

frontend/type-check: ## Run TypeScript type checking
	@cd $(FRONTEND_DIR) && npm run type-check

frontend/clean: ## Clean frontend build artifacts
	@rm -rf $(FRONTEND_DIR)/dist

FRONTEND_DIR := frontend
REPO_ROOT := ..
FRONTEND_PID := $(REPO_ROOT)/.frontend-dev.pid

.PHONY: frontend/install frontend/dev frontend/dev-bg frontend/dev-stop frontend/test frontend/build frontend/lint frontend/type-check frontend/clean

frontend/install: ## Install frontend dependencies
	@cd $(FRONTEND_DIR) && npm install

# Background Vite for "make dev" only — does not write to dev.log (use frontend/dev for Vite output).
frontend/dev-bg:
	@(cd $(FRONTEND_DIR) && npm run dev >/dev/null 2>&1 & echo $$! > $(FRONTEND_PID))

frontend/dev: ## Start Vite dev server (logs in this terminal only)
	@cd $(FRONTEND_DIR) && npm run dev

frontend/dev-stop: ## Stop background frontend started by frontend/dev-bg
	@if [ -f $(FRONTEND_PID) ]; then kill $$(cat $(FRONTEND_PID)) 2>/dev/null || true; rm -f $(FRONTEND_PID); fi

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

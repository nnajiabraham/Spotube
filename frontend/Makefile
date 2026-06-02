FRONTEND_DIR := frontend
REPO_ROOT := ..
DEV_LOGS := $(REPO_ROOT)/dev.logs
FRONTEND_PID := $(REPO_ROOT)/.frontend-dev.pid

.PHONY: frontend/install frontend/dev frontend/dev-bg frontend/dev-stop frontend/test frontend/build frontend/lint frontend/type-check frontend/clean

frontend/install: ## Install frontend dependencies
	@cd $(FRONTEND_DIR) && npm install

frontend/dev-bg: ## Start Vite in background (append logs to dev.logs only)
	@touch $(DEV_LOGS)
	@printf '\n[frontend] starting at %s\n' "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> $(DEV_LOGS)
	@(cd $(FRONTEND_DIR) && npm run dev >> $(DEV_LOGS) 2>&1 & echo $$! > $(FRONTEND_PID))
	@printf '[frontend] pid %s (logs: dev.logs)\n' "$$(cat $(FRONTEND_PID))" >> $(DEV_LOGS)

frontend/dev: frontend/dev-bg ## Start frontend and follow dev.logs (Vite stdout not on terminal)
	@tail -f $(DEV_LOGS)

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

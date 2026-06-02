.DEFAULT_GOAL := help

# Include project makefiles (order matters: backend before frontend for shared targets)
-include backend/Makefile
-include frontend/Makefile

.PHONY: help install dev dev-stop test lint build clean

help: ## Show all available commands
	@printf "Available commands:\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_\/\-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dependencies for both projects
	@$(MAKE) backend/install
	@$(MAKE) frontend/install

dev: ## Start both apps; tail backend dev.log only (run make frontend/dev in another terminal for Vite output)
	@touch dev.log
	@$(MAKE) backend/dev-bg frontend/dev-bg
	@printf '\n--- following backend logs in dev.log (frontend: http://localhost:5173; Vite logs → make frontend/dev) ---\n\n'
	@tail -f dev.log

dev-stop: ## Stop background dev servers
	@$(MAKE) backend/dev-stop frontend/dev-stop

test: ## Run tests for both projects
	@$(MAKE) backend/test
	@$(MAKE) frontend/test

lint: ## Run linters for both projects
	@$(MAKE) backend/lint
	@$(MAKE) frontend/lint

build: ## Build both projects
	@$(MAKE) backend/build
	@$(MAKE) frontend/build

clean: ## Clean build artifacts for both projects
	@$(MAKE) backend/clean
	@$(MAKE) frontend/clean
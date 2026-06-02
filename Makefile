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

dev: ## Start backend+frontend in background, then follow dev.logs (no server stdout on terminal)
	@touch dev.logs
	@$(MAKE) backend/dev-bg frontend/dev-bg
	@tail -f dev.logs

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
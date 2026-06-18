# StatShed — common developer / operator tasks. Run `make help` for the list.
# Recipe lines use `>` (RECIPEPREFIX) instead of tabs.

COMPOSE ?= docker compose

.DEFAULT_GOAL := help
.RECIPEPREFIX := >

.PHONY: help up down logs build dev-backend dev-frontend test test-backend test-frontend e2e

help: ## Show available targets
> @grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Build (if needed) and start the full stack in the background
> $(COMPOSE) up --build -d

down: ## Stop the stack (the statshed-data volume is preserved)
> $(COMPOSE) down

logs: ## Follow logs from both services
> $(COMPOSE) logs -f

build: ## Build both Docker images without starting them
> $(COMPOSE) build

dev-backend: ## Run the backend dev server locally (no Docker; needs uv)
> cd backend && uv sync && uv run python app.py

dev-frontend: ## Run the frontend dev server locally (no Docker; needs Node 20+)
> cd frontend && npm ci && npm run dev

test: test-backend test-frontend ## Run backend + frontend unit tests

test-backend: ## Run backend tests (pytest)
> cd backend && uv run pytest

test-frontend: ## Run frontend unit tests (vitest)
> cd frontend && npm run test:ci

e2e: ## Run frontend end-to-end tests (Playwright)
> cd frontend && npm run test:e2e

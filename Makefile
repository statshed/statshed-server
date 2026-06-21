# StatShed — common developer / operator tasks. Run `make help` for the list.
# Recipe lines use `>` (RECIPEPREFIX) instead of tabs.

COMPOSE ?= docker compose

.DEFAULT_GOAL := help
.RECIPEPREFIX := >

.PHONY: help up down logs build dev dev-frontend test test-frontend e2e contract-test prepare-static live-e2e lint

help: ## Show available targets
> @grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Build (if needed) and start the full stack in the background
> $(COMPOSE) up --build -d

down: ## Stop the stack (the statshed-data volume is preserved)
> $(COMPOSE) down

logs: ## Follow the statshed-server logs
> $(COMPOSE) logs -f

build: ## Build the statshed-server image without starting it
> $(COMPOSE) build

dev-frontend: ## Run the frontend dev server locally (no Docker; needs Node 20+)
> cd frontend && npm ci && npm run dev

dev: prepare-static ## Run the Go server locally with the freshly-built embedded SPA (Go + Node)
> go run ./cmd/statshed-server

test: ## Run the Go server unit tests with the race detector (C3)
> go test -race ./...

lint: ## Lint the Go code (golangci-lint)
> golangci-lint run ./...

test-frontend: ## Run frontend unit tests (vitest)
> cd frontend && npm run test:ci

e2e: ## Run frontend end-to-end tests (Playwright)
> cd frontend && npm run test:e2e

# AIDEV-NOTE: The shared HTTP contract suite (spec.md 8). runner.py boots a server
# (Python or Go) on a fresh DB under a config profile, runs the suite over HTTP, then
# tears it down. The same assertions run against both servers. The early gates
# (impl-guide Task 1.5 / Phase 3 / Task 6.1) invoke this; Task 7.2 adapts it for the Go
# Makefile. Everything after `--` is passed to pytest.
contract-test: ## Run the contract suite (TARGET=python|go [PROFILE=<name>] [K=<expr>])
> @test -n "$(TARGET)" || { echo "Usage: make contract-test TARGET=python|go [PROFILE=<name>] [K=<expr>]"; exit 2; }
> cd contract && uv run python runner.py --target $(TARGET) $(if $(PROFILE),--profile $(PROFILE)) -- $(if $(K),-k "$(K)")

# AIDEV-NOTE: Build the real React SPA into internal/staticfs/dist so the Go binary embeds
# it (I9). Overwrites the committed placeholder (a local-only change; not committed). The
# frontend build runs in a subshell so the rm/mkdir/cp stay rooted at the repo root.
prepare-static: ## Build the SPA into internal/staticfs/dist (embedded by the Go binary)
> (cd frontend && npm ci && npm run build)
> rm -rf internal/staticfs/dist
> mkdir -p internal/staticfs/dist
> cp -R frontend/dist/. internal/staticfs/dist/

# AIDEV-NOTE: Executable live SSE gate (Task 5.5). Builds the real Go server and runs the
# non-mocked e2e-live spec, which loads the app through the Vite dev proxy (:7827 -> the Go
# server at :7828) and verifies unbuffered live delivery + reconnect-driven refetch. The
# spec spawns/restarts Go itself (GO_BIN). PLAYWRIGHT_CHROMIUM_BIN points at a working
# Chrome on hosts where the bundled chromium can't link the system libs (e.g. NixOS); in CI
# the bundled chromium is used.
live-e2e: ## Run the live SSE proxy/reconnect-resync gate (Task 5.5) — needs Go + a browser
> CGO_ENABLED=0 go build -o statshed-server ./cmd/statshed-server
> cd frontend && GO_BIN=$(CURDIR)/statshed-server PLAYWRIGHT_CHROMIUM_BIN=$${PLAYWRIGHT_CHROMIUM_BIN:-$$(command -v google-chrome 2>/dev/null)} npx playwright test --config=playwright.live.config.ts

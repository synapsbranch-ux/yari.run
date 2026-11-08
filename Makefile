# yari.run developer entry points. Every target prints what it does.
SHELL := /bin/bash
.DEFAULT_GOAL := help

COMPOSE ?= docker compose
# Load .env when present (else .env.example, whose values are local-only placeholders) so local
# commands see the same configuration as the services.
ifneq (,$(wildcard .env))
include .env
export
else ifneq (,$(wildcard .env.example))
include .env.example
export
endif

.PHONY: catalog-table service-binaries help setup up down migrate gen lint test build api worker mcp agent-image client desktop mobile

help: ## List targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

setup: ## Create .env, download Go modules, install JS dependencies
	@echo ">> setup: creating .env if missing, downloading Go modules, installing pnpm workspace"
	@test -f .env || cp .env.example .env
	go mod download
	pnpm install

up: ## Start local infrastructure (Postgres) and wait until healthy
	@echo ">> up: starting Postgres (use 'docker compose --profile auth|agent up -d' for Zitadel or LiteLLM)"
	$(COMPOSE) up -d --wait postgres

down: ## Stop all local infrastructure, all profiles (keeps data volumes)
	@echo ">> down: stopping local infrastructure"
	$(COMPOSE) --profile auth --profile agent down

migrate: ## Apply goose migrations, then River migrations
	@echo ">> migrate: goose up, then rivermigrate (as the owner role)"
	go run ./cmd/migrate

gen: ## Generate code from proto (Go + TypeScript) and SQL (sqlc)
	@echo ">> gen: buf generate, sqlc generate (installs the pnpm workspace first if the TypeScript plugin is missing)"
	@test -x packages/api-client/node_modules/.bin/protoc-gen-es || pnpm install
	go tool buf generate
	go tool sqlc generate

lint: ## Lint proto, Go, and the TypeScript workspace
	@echo ">> lint: buf lint, go vet, TypeScript lint and type check"
	go tool buf lint
	go vet ./...
	pnpm -r --if-present lint
	pnpm -r --if-present check

test: ## Run Go tests (DB tests need YARI_TEST_DATABASE_URL)
	@echo ">> test: go test ./... (set YARI_TEST_DATABASE_URL, YARI_TEST_DOCKER=1 to enable more)"
	go test ./...

build: ## Build Go binaries into ./bin and the client SPA
	@echo ">> build: Go binaries into ./bin, client static build"
	mkdir -p bin
	go build -o bin/ ./cmd/...
	pnpm --filter client build

api: ## Run the Connect API server
	@echo ">> api: listening on $${YARI_API_ADDR:-:8080}"
	go run ./cmd/api

worker: ## Run the River worker (plan applies, agent sessions)
	@echo ">> worker: River worker with agent runtime '$${YARI_AGENT_RUNTIME:-fake}'"
	go run ./cmd/worker

mcp: ## Run the MCP server (talks only to the Connect API)
	@echo ">> mcp: MCP server on $${YARI_MCP_ADDR:-:8090}"
	go run ./cmd/mcp

catalog-table: ## Print the Provider Catalog as the Markdown table used in the README
	@go run ./internal/provider/catalog/catalogdoc

service-binaries: ## Cross-build the Go services for the compose images (bin/linux)
	@echo ">> service-binaries: linux binaries for the yari-mcp container"
	mkdir -p bin/linux
	GOOS=linux CGO_ENABLED=0 go build -trimpath -o bin/linux/mcp ./cmd/mcp

agent-image: service-binaries ## Build the hardened agent sandbox image (yari-agent:dev) and the MCP image
	@echo ">> agent-image: building yari-agent:dev from the pinned OpenHands agent server"
	docker build -t yari-agent:dev agent/image
	$(COMPOSE) --profile agent build yari-mcp

client: ## Run the Svelte client dev server on :5173
	@echo ">> client: Vite dev server on http://localhost:5173"
	pnpm --filter client dev

desktop: ## Run the Tauri desktop shell in dev mode
	@echo ">> desktop: Tauri dev (needs Rust and Tauri system dependencies)"
	pnpm --filter desktop tauri dev

mobile: ## Build the client, sync it into the Capacitor Android project
	@echo ">> mobile: client build, then cap sync android"
	pnpm --filter client build
	pnpm --filter mobile exec cap sync android

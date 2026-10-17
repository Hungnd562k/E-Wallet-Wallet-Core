# Common tasks. Run `make help` to list them.
# Variables from .env (copy .env.example) are exported to every command.
-include .env
export

GO      ?= go
PKG     ?= ./internal/ledger/...
RUN     ?= .

.DEFAULT_GOAL := help

.PHONY: help
help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## --- infrastructure -------------------------------------------------------
.PHONY: up down reset-db
up: ## Start Postgres (docker compose) and wait until healthy
	docker compose up -d --wait postgres

down: ## Stop containers (keeps data)
	docker compose down

reset-db: ## Drop all data and start a fresh Postgres
	docker compose down -v && $(MAKE) up migrate-up

## --- database -------------------------------------------------------------
.PHONY: migrate-up migrate-down migrate-version seed sqlc
migrate-up: ## Apply all migrations
	$(GO) run ./cmd/migrate up

migrate-down: ## Roll back the last migration
	$(GO) run ./cmd/migrate down 1

migrate-version: ## Print current migration version
	$(GO) run ./cmd/migrate version

seed: ## Create demo wallets with balance (idempotent)
	$(GO) run ./cmd/devseed

sqlc: ## Regenerate type-safe query code from */queries.sql
	$(GO) tool sqlc generate

## --- run ------------------------------------------------------------------
.PHONY: run-api run-worker
run-api: ## Run wallet-api on $(HTTP_ADDR)
	$(GO) run ./cmd/api

run-worker: ## Run wallet-worker
	$(GO) run ./cmd/worker

## --- quality --------------------------------------------------------------
.PHONY: build test test-integration test-one cover lint fmt learn
build: ## Compile everything
	$(GO) build ./...

test: ## Unit tests + architecture test + learn exercises (no Docker needed)
	$(GO) test -short ./...

test-integration: ## Integration tests against a throwaway Postgres (needs Docker), with -race
	$(GO) test -race -count=1 -run '$(RUN)' ./test/...

test-one: ## Run one test: make test-one PKG=./internal/ledger/... RUN='TestValidate/unbalanced'
	$(GO) test -count=1 -v -run '$(RUN)' $(PKG)

cover: ## Coverage report for unit tests (opens coverage.html)
	$(GO) test -short -coverprofile=coverage.out ./internal/... && $(GO) tool cover -html=coverage.out -o coverage.html

lint: ## gofmt + go vet (+ golangci-lint via Docker if available)
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "run: make fmt" && exit 1)
	$(GO) vet ./...
	@if command -v docker >/dev/null; then \
		docker run --rm -v $$(pwd):/app -v $$HOME/go/pkg/mod:/go/pkg/mod -w /app golangci/golangci-lint:v2.12.0 golangci-lint run ./...; \
	fi

fmt: ## Format all Go files
	gofmt -w .

learn: ## Run the Go learning exercises in learn/
	$(GO) test -v ./learn/...

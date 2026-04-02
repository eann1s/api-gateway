SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

APP ?= rate-limiter
GO ?= go
CMD_DIR ?= ./cmd/$(APP)
BIN_DIR ?= ./bin

.PHONY: help deps tidy fmt vet test build run clean lint

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?##"} /^[a-zA-Z_-]+:.*?##/ {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	$(GO) mod download

tidy: ## Tidy
	$(GO) mod tidy

fmt: ## Format
	$(GO) fmt ./...

vet: ## Run static checks
	$(GO) vet ./...

test: ## Run tests
	$(GO) test ./...

$(BIN_DIR): ## Build binary
	mkdir -p $(BIN_DIR)

build: | $(BIN_DIR) ## Build binary
	$(GO) build -o $(BIN_DIR)/$(APP) $(CMD_DIR)

run: build ## Run binary
	$(BIN_DIR)/$(APP)

lint: ## Lint
	golangci-lint run

clean: ## Remove binary
	rm -f $(BIN_DIR)

# watcht build and check commands. `make check` is the definition of done
# (see AGENTS.md) and is what CI runs.

SHELL := /bin/bash

# Use the toolchain pinned in go.mod even where the distro defaults to
# GOTOOLCHAIN=local (Fedora does), and build static binaries.
export GOTOOLCHAIN := auto
export CGO_ENABLED := 0

GOLANGCI_VERSION := $(shell cat .golangci-version)
GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

VERSION ?= $(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help build run test lint check clean

help: ## List the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ## Build bin/watcht
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/watcht ./cmd/watcht

run: build ## Build and run the server on :8080
	./bin/watcht serve

test: ## Run the Go tests
	go test ./...

lint: ## Run go vet and golangci-lint
	go vet ./...
	$(GOLANGCI) run ./...

check: lint test ## Everything CI runs

clean: ## Remove build output
	rm -rf bin

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

NODE_MAJOR := $(shell cat .node-version)
PNPM := cd web && pnpm
# The frontend build is copied here and embedded by internal/webui.
DIST := internal/webui/dist

.PHONY: help build build-go run dev test test-go test-web lint lint-go lint-web \
	check check-go check-web web-install web-build node-version clean

help: ## List the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'

# --- Build ---

build: web-build build-go ## Build the frontend, then bin/watcht with it embedded

build-go: ## Build bin/watcht with whatever is in internal/webui/dist
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/watcht ./cmd/watcht

web-build: web-install ## Build the frontend and copy it into internal/webui/dist
	$(PNPM) build
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete
	cp -R web/build/. $(DIST)/

web-install: node-version ## Install frontend dependencies from the lockfile
	$(PNPM) install --frozen-lockfile

# --- Run ---

run: build ## Build everything and run the server on :8080
	./bin/watcht serve

dev: web-install ## Go server on :8080 and Vite on :5173 (open :5173)
	@trap 'kill 0' EXIT; \
	go run ./cmd/watcht serve & \
	$(PNPM) dev

# --- Checks ---

check: check-go check-web ## Everything CI runs

check-go: lint-go test-go ## Go lint and tests

check-web: lint-web test-web ## Web lint, type check, and tests

lint: lint-go lint-web ## All linters

lint-go: ## go vet and golangci-lint
	go vet ./...
	$(GOLANGCI) run ./...

lint-web: web-install ## svelte-check, prettier, and eslint
	$(PNPM) check
	$(PNPM) lint

test: test-go test-web ## All tests

test-go: ## Go tests
	go test ./...

test-web: web-install ## Vitest
	$(PNPM) test

# Fail early with a clear message instead of odd build errors on the wrong
# Node. fnm, mise, and similar read .node-version automatically.
node-version:
	@actual=$$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null); \
	if [ "$$actual" != "$(NODE_MAJOR)" ]; then \
		echo "Node $(NODE_MAJOR) is required (from .node-version), found: $${actual:-none}." >&2; \
		echo "Use a version manager such as fnm that reads .node-version." >&2; \
		exit 1; \
	fi

clean: ## Remove build output
	rm -rf bin web/build web/.svelte-kit
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete

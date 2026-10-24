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

# Port for the Go server under `make dev`. Not 8080, so dev never talks to
# a real instance running on the same machine.
DEV_API_PORT ?= 8090

IMAGE ?= watcht:local

.PHONY: help build build-go run dev smoke docker docker-smoke test test-go test-web \
	lint lint-go lint-web check check-go check-web web-install web-build node-version clean

help: ## List the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'

# --- Build ---

build: web-build ## Build the frontend, then bin/watcht with it embedded
	@# Sequential on purpose: with make -j, prerequisites would race and
	@# embed a missing or stale frontend.
	$(MAKE) build-go

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

dev: web-install ## Go server on :$(DEV_API_PORT) and Vite on :5173 (open :5173)
	go build -o bin/watcht-dev ./cmd/watcht
	@export WATCHT_LISTEN=127.0.0.1:$(DEV_API_PORT); \
	./bin/watcht-dev serve & pid=$$!; \
	trap 'kill $$pid 2>/dev/null' EXIT; \
	for _ in $$(seq 1 20); do ./bin/watcht-dev healthcheck 2>/dev/null && break; sleep 0.25; done; \
	./bin/watcht-dev healthcheck && kill -0 $$pid 2>/dev/null || \
		{ echo "The Go server didn't start on :$(DEV_API_PORT) (port in use? set DEV_API_PORT)." >&2; exit 1; }; \
	cd web && WATCHT_DEV_API=http://127.0.0.1:$(DEV_API_PORT) pnpm dev

# --- Checks ---

check: check-go check-web smoke ## Everything CI runs

check-go: lint-go test-go ## Go lint and tests

check-web: lint-web test-web ## Web lint, type check, tests, and build
	$(PNPM) build

smoke: build ## Build, start the binary, and check health and the embedded SPA
	scripts/smoke.sh bin/watcht

# The image build needs Docker, so it isn't part of `make check`; CI runs it
# as its own job.
docker: ## Build the container image ($(IMAGE))
	docker build -t $(IMAGE) --build-arg VERSION=$(VERSION) .

docker-smoke: docker ## Build the image, run it, and check health, user, and the SPA
	scripts/docker-smoke.sh $(IMAGE)

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

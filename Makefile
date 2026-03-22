# =============================================================================
# ATO WFH Diary — Makefile
# =============================================================================

BINARY     := bin/server
GO_DIR     := backend

# Resolve the version from the nearest Git tag (e.g. v1.2.3).
# Falls back to "dev" when there are no tags or no Git repo.
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

BUILD_CMD  := go build -ldflags="-X main.version=$(VERSION)" -o ../$(BINARY) ./cmd/server

.DEFAULT_GOAL := help

# -----------------------------------------------------------------------------
# Help
# -----------------------------------------------------------------------------

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z/_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# -----------------------------------------------------------------------------
# Build
# -----------------------------------------------------------------------------

.PHONY: build
build: ## Build the server binary to bin/server
	mkdir -p bin
	cd $(GO_DIR) && $(BUILD_CMD)

# -----------------------------------------------------------------------------
# Run
# -----------------------------------------------------------------------------

.PHONY: run
run: build ## Build and run the server locally
	DB_PATH=./data/wfh.db FORWARD_AUTH_HEADER=X-Forwarded-User ./$(BINARY)

.PHONY: dev
dev: build ## Build and run in development mode (no auth proxy needed, user=alice)
	mkdir -p data
	DB_PATH=./data/wfh.db DEV_USER=alice ./$(BINARY)

# -----------------------------------------------------------------------------
# Test
# -----------------------------------------------------------------------------

.PHONY: test
test: ## Run all tests
	cd $(GO_DIR) && go test ./...

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	cd $(GO_DIR) && go test -v ./...

.PHONY: test-cover
test-cover: ## Run tests and show coverage summary
	cd $(GO_DIR) && go test -coverprofile=../bin/coverage.out ./... \
		&& go tool cover -func=../bin/coverage.out

.PHONY: test-e2e
test-e2e: ## Run end-to-end browser tests (requires Chrome/Chromium)
	cd $(GO_DIR) && go test -tags e2e -v ./e2e/...

# -----------------------------------------------------------------------------
# Code quality
# -----------------------------------------------------------------------------

.PHONY: fmt
fmt: ## Format all Go source files
	cd $(GO_DIR) && gofmt -w .

.PHONY: vet
vet: ## Run go vet
	cd $(GO_DIR) && go vet ./...

.PHONY: check
check: fmt vet test ## Format, vet, and test (run before committing)

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

.PHONY: docker-up
docker-up: ## Start the application via Docker Compose
	docker compose up --build -d

.PHONY: docker-down
docker-down: ## Stop and remove Docker Compose containers
	docker compose down

.PHONY: docker-logs
docker-logs: ## Tail logs from the running container
	docker compose logs -f

# -----------------------------------------------------------------------------
# Clean
# -----------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/

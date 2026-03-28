.PHONY: dev run build build-server build-cli build-linux build-server-linux build-cli-linux cli-release-local milestone-proof bootstrap-canonical-workspace test test-integration test-coverage test-all lint deadcode deadcode-production clean deploy setup-hooks validate-templates create-admin create-agent seed-defaults seed-security-test sentry-tests health docs-check sync-agent-cli-doc

ifneq (,$(wildcard ./.env))
include .env
export
endif

DATABASE_DSN ?= postgres://$(USER)@127.0.0.1:5432/postgres?sslmode=disable
TEST_DATABASE_ADMIN_DSN ?= $(DATABASE_DSN)

# Development with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	@echo "Starting with hot reload at http://localhost:8080"
	@DATABASE_DSN="$(DATABASE_DSN)" air

# Run without hot reload
run:
	@DATABASE_DSN="$(DATABASE_DSN)" go run ./cmd/api

# Build server and operator CLI
build: build-server build-cli

# Build server binary
build-server:
	@mkdir -p bin
	go build -o bin/mbr-server ./cmd/api

# Build operator CLI
build-cli:
	@mkdir -p bin
	go build -o bin/mbr ./cmd/mbr

cli-release-local:
	@bash scripts/build-cli-release.sh $(if $(VERSION),--version "$(VERSION)") --out "$(or $(OUT),dist/cli-release)"

milestone-proof:
	@bash scripts/run-milestone-1-proof.sh $(if $(VERSION),--version "$(VERSION)") --out "$(or $(OUT),dist/milestone-proof)"

bootstrap-canonical-workspace:
ifndef WORKSPACE_ROOT
	@echo "Usage: make bootstrap-canonical-workspace WORKSPACE_ROOT=/path/to/workspace"
	@exit 1
endif
	@bash scripts/bootstrap-canonical-workspace.sh --workspace-root "$(WORKSPACE_ROOT)"

# Build Linux artifacts
build-linux: build-server-linux build-cli-linux

# Build Linux server artifact
build-server-linux:
	@mkdir -p bin
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/mbr-server-linux ./cmd/api

# Build Linux CLI artifact
build-cli-linux:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/mbr-linux ./cmd/mbr

# Run tests against PostgreSQL
test:
	DATABASE_DSN="$(DATABASE_DSN)" TEST_DATABASE_ADMIN_DSN="$(TEST_DATABASE_ADMIN_DSN)" go test -v ./...

test-integration:
	DATABASE_DSN="$(DATABASE_DSN)" TEST_DATABASE_ADMIN_DSN="$(TEST_DATABASE_ADMIN_DSN)" go test -v -tags=integration ./...

test-all:
	DATABASE_DSN="$(DATABASE_DSN)" TEST_DATABASE_ADMIN_DSN="$(TEST_DATABASE_ADMIN_DSN)" go test -v -short ./... && \
		DATABASE_DSN="$(DATABASE_DSN)" TEST_DATABASE_ADMIN_DSN="$(TEST_DATABASE_ADMIN_DSN)" go test -v -tags=integration ./...

docs-check:
	@scripts/check-doc-links.sh
	@bash scripts/check-cli-contract-docs.sh

sync-agent-cli-doc:
	@GOCACHE="$${GOCACHE:-/tmp/mbr-go-cache}" go run ./cmd/tools/sync-agent-cli-doc

# Run tests with coverage
test-coverage:
	DATABASE_DSN="$(DATABASE_DSN)" TEST_DATABASE_ADMIN_DSN="$(TEST_DATABASE_ADMIN_DSN)" go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

sentry-tests:
	cd tests && npm install --no-audit --no-fund && npm test

# Run linter
lint:
	golangci-lint run

# Run dead-code analysis while excluding internal test utility packages
# (deadcode treats test-only usage as unreachable).
deadcode:
	@if ! command -v deadcode >/dev/null; then \
		go install golang.org/x/tools/cmd/deadcode@latest; \
	fi
	@repo_go=$$(go list -m -f '{{.GoVersion}}'); \
	tool_path=$$(command -v deadcode); \
	tool_go=$$(go version -m "$$tool_path" 2>/dev/null | sed -n '1s/.*: go//p'); \
	if [ -z "$$tool_go" ] || [ "$${tool_go%.*}" != "$${repo_go%.*}" ]; then \
		echo "deadcode toolchain mismatch: repo requires Go $$repo_go, but $$tool_path was built with $${tool_go:-unknown}." >&2; \
		echo "Reinstall with the current toolchain: go install golang.org/x/tools/cmd/deadcode@latest" >&2; \
		exit 1; \
	fi; \
	tmpcache=$$(mktemp -d); \
	tmpout=$$(mktemp); \
	go list -f '{{.ImportPath}}' ./... | grep -Ev '(^|/)internal/testutil($|/)' | xargs env GOCACHE=$$tmpcache "$$tool_path" > "$$tmpout" 2>&1; status=$$?; \
	filtered=$$(grep -Ev '^github.com/movebigrocks/platform/internal/testutil/|^internal/testutil/' "$$tmpout" || true); \
	rm -f "$$tmpout"; \
	rm -rf "$$tmpcache"; \
	if [ -n "$$filtered" ]; then echo "$$filtered"; fi; \
	if [ $$status -ne 0 ] && [ -n "$$filtered" ]; then exit $$status; fi

# Run dead-code analysis on production packages only (excluding internal test utilities).
deadcode-production:
	@if ! command -v deadcode >/dev/null; then \
		go install golang.org/x/tools/cmd/deadcode@latest; \
	fi
	@repo_go=$$(go list -m -f '{{.GoVersion}}'); \
	tool_path=$$(command -v deadcode); \
	tool_go=$$(go version -m "$$tool_path" 2>/dev/null | sed -n '1s/.*: go//p'); \
	if [ -z "$$tool_go" ] || [ "$${tool_go%.*}" != "$${repo_go%.*}" ]; then \
		echo "deadcode toolchain mismatch: repo requires Go $$repo_go, but $$tool_path was built with $${tool_go:-unknown}." >&2; \
		echo "Reinstall with the current toolchain: go install golang.org/x/tools/cmd/deadcode@latest" >&2; \
		exit 1; \
	fi; \
	tmpcache=$$(mktemp -d); \
	tmpout=$$(mktemp); \
	GOCACHE=$$tmpcache; \
	go list -f '{{.ImportPath}}' ./... | grep -Ev '(^|/)internal/testutil($|/)' | xargs env GOCACHE=$$tmpcache "$$tool_path" > "$$tmpout" 2>&1; status=$$?; \
	filtered=$$(grep -Ev '^github.com/movebigrocks/platform/internal/testutil/|^internal/testutil/' "$$tmpout" || true); \
	rm -rf "$$tmpcache"; \
	rm -f "$$tmpout"; \
	if [ -n "$$filtered" ]; then echo "$$filtered"; fi; \
	if [ $$status -ne 0 ] && [ -n "$$filtered" ]; then exit $$status; fi

# Deploy (CI handles this, but manual option available)
deploy:
	@echo "Deployment is handled by GitHub Actions"
	@echo "Push to main to trigger: gh workflow run production.yml"

# Clean build artifacts
clean:
	rm -rf bin coverage.out coverage.html
	go clean

# Install git hooks
setup-hooks:
	@echo "Installing git hooks..."
	@chmod +x .githooks/commit-msg .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "Git hooks configured via .githooks"

# Validate HTML templates
validate-templates:
	@go run cmd/tools/validate-templates/main.go

# Create admin user
create-admin:
ifndef EMAIL
	@echo "Usage: make create-admin EMAIL=user@example.com [NAME=name]"
	@exit 1
endif
	@go run cmd/create-admin/main.go -email "$(EMAIL)" $(if $(NAME),-name "$(NAME)")

# Create agent
create-agent:
ifndef WORKSPACE
	@echo "Usage: make create-agent WORKSPACE=slug NAME=name OWNER=email"
	@exit 1
endif
	@go run cmd/create-agent/main.go -workspace "$(WORKSPACE)" -name "$(NAME)" -owner "$(OWNER)"

# Seed default automation rules for all workspaces or one workspace
seed-defaults:
	@go run cmd/tools/seed-defaults/main.go $(if $(WORKSPACE),-workspace "$(WORKSPACE)")

# Seed local security-context demo data in filesystem store
seed-security-test:
	@go run cmd/seed-security-test/main.go

# Health check
health:
	@curl -s http://localhost:8080/health | jq .

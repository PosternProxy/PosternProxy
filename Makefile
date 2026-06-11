# PosternProxy Makefile
# Common developer and CI targets. Production builds should use
# scripts/build.sh (Linux/macOS) or scripts/build.ps1 (Windows).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
LDFLAGS  = -s -w -X main.Version=$(VERSION)

.PHONY: help dev build test lint frontend-lint clean version vet

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

dev: ## Run backend + frontend dev servers (requires two terminals or use with &)
	@echo "Starting backend (go run) and frontend (vite)..."
	@echo "  Terminal 1: go run ./cmd/posternproxy"
	@echo "  Terminal 2: cd frontend && npm run dev"
	@echo ""
	@echo "Or run them together:"
	@echo "  make dev-backend & make dev-frontend"

dev-backend: ## Run the Go backend in dev mode
	go run ./cmd/posternproxy

dev-frontend: ## Run the Vite frontend dev server
	cd frontend && npm run dev

build: ## Build controller and agent binaries for current platform
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="$(LDFLAGS)" -o posternproxy ./cmd/posternproxy
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="$(LDFLAGS)" -o posternproxy-agent ./cmd/posternproxy-agent

test: ## Run all Go tests
	go test ./...

test-race: ## Run all Go tests with race detector
	go test -race ./...

vet: ## Run go vet
	go vet ./...

lint: vet ## Run Go vet + frontend lint
	cd frontend && npm run lint:check

frontend-lint: ## Run frontend linting only
	cd frontend && npm run lint:check

frontend-build: ## Build the frontend for production
	cd frontend && npm ci && npm run build

clean: ## Remove build artifacts
	rm -f posternproxy posternproxy-agent
	rm -rf frontend/dist internal/ui/dist

version: ## Print the current version string
	@echo $(VERSION)

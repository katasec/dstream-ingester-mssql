.PHONY: help build push manifest clean rebuild test
.DEFAULT_GOAL := help

TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")
BUILD_DIR ?= .build

help: ## Show this help menu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build cross-platform Go binaries
	@bash scripts/build.sh $(BUILD_DIR)

manifest: build ## Create provider manifest
	@bash scripts/create-manifest.sh $(TAG) $(BUILD_DIR)

push: manifest ## Push provider to GHCR using ORAS
	@bash scripts/push.sh $(TAG) $(BUILD_DIR)

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts…"
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "✅ Clean complete"

rebuild: clean build ## Clean and rebuild cross-platform binaries
	@echo "✅ Rebuild complete"

test: ## Run provider with test configuration
	@echo "🧪 Testing provider with simple config…"
	@pwsh test.ps1

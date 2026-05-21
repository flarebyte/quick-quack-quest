.DEFAULT_GOAL := help

FLYB := flyb
GH := gh
GO := go
BUN := bun
DOC_CONFIG := doc/design-meta
GHF_CONFIG := .gh-flarebyte.cue
GO_CACHE_DIR := $(CURDIR)/.gocache
GO_MOD_CACHE_DIR := $(CURDIR)/.gomodcache
GO_PACKAGES := ./...
GO_ENV := GOTOOLCHAIN=local GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR)

.PHONY: help check-tools install-tools-help \
	format lint test e2e build release review complexity sec dup clean \
	doc-validate doc-generate doc-design \
	config-validate build-go test-go lint-go format-go test-unit test-race coverage

## Public developer targets
help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

check-tools: ## Report required tool availability.
	@printf "flyb=%s\n" "$$(command -v $(FLYB) >/dev/null 2>&1 && echo true || echo false)"
	@printf "gh=%s\n" "$$(command -v $(GH) >/dev/null 2>&1 && echo true || echo false)"
	@printf "go=%s\n" "$$(command -v go >/dev/null 2>&1 && echo true || echo false)"
	@printf "bun=%s\n" "$$(command -v bun >/dev/null 2>&1 && echo true || echo false)"

install-tools-help: ## Show how to install required tools.
	@echo "flyb: https://github.com/flarebyte/baldrick-flying-buttress"
	@echo "gh: https://cli.github.com/"
	@echo "go: https://go.dev/doc/install"
	@echo "bun: https://bun.sh/docs/installation"

format: format-go doc-design ## Format Go code and refresh generated design docs.

lint: config-validate doc-validate lint-go ## Run configured lint checks.

test: test-go ## Run default automated checks.

e2e: ## Run TypeScript end-to-end tests with Bun.
	$(BUN) run e2e

build: build-go ## Build distributable artifacts via gh flarebyte.

release: ## Build and publish a GitHub release via gh flarebyte.
	$(GH) flarebyte release

review: format test lint e2e ## Run standard review gate.

complexity: ## No complexity scan is defined yet.
	@echo "complexity=not_configured"

sec: ## No security scan is defined yet.
	@echo "sec=not_configured"

dup: ## No duplication scan is defined yet.
	@echo "dup=not_configured"

clean: ## Remove generated build artifacts.
	rm -rf ./build ./.gocache ./.gomodcache

## Documentation targets
doc-validate: ## Validate flyb design-meta config.
	$(FLYB) validate --config $(DOC_CONFIG)

doc-generate: ## Generate markdown docs from design-meta config.
	$(FLYB) generate markdown --config $(DOC_CONFIG)

doc-design: doc-validate doc-generate ## Validate and generate design docs.

## Tooling and build targets
config-validate: ## Validate gh flarebyte config.
	$(GH) flarebyte config validate --config $(GHF_CONFIG)

build-go: ## Build project artifacts from gh flarebyte config.
	$(GH) flarebyte build

test-go: test-unit ## Run Go test targets.

test-unit: ## Run Go tests.
	@if [ -f go.mod ]; then $(GO_ENV) $(GO) test $(GO_PACKAGES); else echo "go_tests=skipped (no go.mod)"; fi

test-race: ## Run Go tests with race detector.
	@if [ -f go.mod ]; then $(GO_ENV) $(GO) test -race $(GO_PACKAGES); else echo "go_tests_race=skipped (no go.mod)"; fi

coverage: ## Run Go tests with coverage summary.
	@if [ -f go.mod ]; then \
		mkdir -p $(CURDIR)/tmp; \
		$(GO_ENV) $(GO) test -coverprofile=$(CURDIR)/tmp/coverage.out -covermode=count $(GO_PACKAGES); \
		$(GO_ENV) $(GO) tool cover -func=$(CURDIR)/tmp/coverage.out; \
	else \
		echo "go_coverage=skipped (no go.mod)"; \
	fi

lint-go: ## Run Go vet.
	@if [ -f go.mod ]; then $(GO_ENV) $(GO) vet $(GO_PACKAGES); else echo "go_vet=skipped (no go.mod)"; fi

format-go: ## Run gofmt on all Go files.
	@if [ -f go.mod ]; then \
		files="$$(find . \
			-type d \( -name .git -o -name vendor -o -name .gocache -o -name .gomodcache -o -name build -o -name tmp \) -prune \
			-o -type f -name '*.go' -print)"; \
		if [ -n "$$files" ]; then \
			count="$$(printf "%s\n" "$$files" | wc -l | tr -d ' ')"; \
			before="$$(git status --porcelain -- $$files)"; \
			gofmt -w $$files; \
			after="$$(git status --porcelain -- $$files)"; \
			changed="$$(printf "%s\n" "$$after" | wc -l | tr -d ' ')"; \
			if [ -z "$$after" ]; then changed=0; fi; \
			echo "gofmt_scanned=$$count"; \
			echo "gofmt_changed=$$changed"; \
		else \
			echo "gofmt=skipped (no .go files)"; \
		fi; \
	else \
		echo "gofmt=skipped (no go.mod)"; \
	fi

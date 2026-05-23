.DEFAULT_GOAL := help

FLYB := flyb
GH := gh
GO := go
BUN := bun
THOTH := thoth
DOC_CONFIG := doc/design-meta
GHF_CONFIG := .gh-flarebyte.cue
GO_CACHE_DIR := $(CURDIR)/.gocache
GO_MOD_CACHE_DIR := $(CURDIR)/.gomodcache
GO_PACKAGES := ./...
GO_ENV := GOTOOLCHAIN=local GOCACHE=$(GO_CACHE_DIR) GOMODCACHE=$(GO_MOD_CACHE_DIR)

.PHONY: help check-tools install-tools-help \
	format lint test e2e build release review complexity sec dup clean \
	doc-validate doc-generate doc-design \
	config-validate build-go test-go lint-go format-go test-unit test-race coverage cov

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

format: format-go doc-design ## Format code via gh flarebyte and refresh generated design docs.

lint: config-validate doc-validate lint-go ## Run configured lint checks via gh flarebyte.

test: test-go ## Run default automated checks via gh flarebyte.

e2e: ## Run TypeScript end-to-end tests with Bun.
	$(BUN) run e2e

build: build-go ## Build distributable artifacts via gh flarebyte.

release: ## Build and publish a GitHub release via gh flarebyte.
	$(GH) flarebyte release

review: format test lint e2e ## Run standard review gate.

complexity:
	scc --sort complexity --by-file -i go . | head -n 15
	scc --sort complexity --by-file -i ts . | head -n 15

sec:
	semgrep scan --config auto

dup:
	npx jscpd --format go --min-lines 10 --ignore "**/.gomodcache/**,**/.gocache/**,**/.e2e-bin/**,**/node_modules/**,**/dist/**" --gitignore .
	npx jscpd --format typescript --min-lines 10 --gitignore .

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

test-go: ## Run project tests via gh flarebyte.
	$(GO_ENV) $(GH) flarebyte test

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

cov: ## Enforce minimum test coverage via gh flarebyte.
	$(GO_ENV) $(GH) flarebyte cov --min 90

lint-go: ## Run lint checks via gh flarebyte.
	$(GO_ENV) $(GH) flarebyte lint

format-go: ## Run formatting via gh flarebyte.
	$(GO_ENV) $(GH) flarebyte format

thoth-meta: thoth-meta-go thoth-meta-go-test thoth-meta-ts-e2e

thoth-meta-go:
	$(THOTH) run --config ./pipeline-go-maat.thoth.cue

thoth-meta-go-test:
	$(THOTH) run --config ./pipeline-go-test-maat.thoth.cue

thoth-meta-ts-e2e:
	$(THOTH) run --config ./pipeline-ts-e2e-maat.thoth.cue


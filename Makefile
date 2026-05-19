.DEFAULT_GOAL := help

FLYB := flyb
GH := gh
DOC_CONFIG := doc/design-meta
GHF_CONFIG := .gh-flarebyte.cue

.PHONY: help check-tools install-tools-help \
	format lint test e2e build release review complexity sec dup clean \
	doc-validate doc-generate doc-design \
	config-validate build-go test-go lint-go

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

format: doc-design ## Refresh generated design docs.

lint: config-validate doc-validate ## Run configured lint checks.

test: doc-validate ## Run default automated checks.

e2e: ## Run TypeScript end-to-end tests with Bun.
	bun test e2e

build: build-go ## Build distributable artifacts via gh flarebyte.

release: ## Build and publish a GitHub release via gh flarebyte.
	$(GH) flarebyte release

review: lint test ## Run standard review gate.

complexity: ## No complexity scan is defined yet.
	@echo "complexity=not_configured"

sec: ## No security scan is defined yet.
	@echo "sec=not_configured"

dup: ## No duplication scan is defined yet.
	@echo "dup=not_configured"

clean: ## Remove generated build artifacts.
	rm -rf build

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

test-go: ## Run Go tests when go.mod exists.
	@if [ -f go.mod ]; then go test ./...; else echo "go_tests=skipped (no go.mod)"; fi

lint-go: ## Run Go vet when go.mod exists.
	@if [ -f go.mod ]; then go vet ./...; else echo "go_vet=skipped (no go.mod)"; fi

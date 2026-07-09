# f5w-docgen build tooling.
#
# Formatting exception: Make requires real TAB characters to indent recipe
# lines, so this file uses TABs for recipes despite the project's 2-space rule
# (CLAUDE.md, Code formatting). Variable and comment lines stay space-aligned.
#
# The tool repo dogfoods its own generator: every docs target runs the CURRENT
# SOURCE via `go run ./cmd/f5w-docgen` - never a pinned release, which is what
# consuming repos use (`go run github.com/f5websites/f5w-docgen/cmd/f5w-docgen@<tag>`).

# ---- Directories and identity (defined once; No Magic Values) ----------------

KNOWLEDGE_DIR := knowledge
SITE_DIR      := _site
DOCGEN_RUN    := go run ./cmd/f5w-docgen
DOCS_DEPLOY   := scripts/docs-deploy.sh

.PHONY: help test lint docs docs-lint docs-guidance docs-deploy

help: ## List the available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-16s %s\n", $$1, $$2}'

# ---- Go gates -----------------------------------------------------------------

test: ## Run the full Go test suite
	go test ./...

lint: ## Vet and check formatting
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: files need formatting" && exit 1)

# ---- Docs site (dogfood) --------------------------------------------------------

docs: ## Build this repo's docs site from knowledge/ into _site/
	$(DOCGEN_RUN) build -root $(KNOWLEDGE_DIR) -out $(SITE_DIR)

docs-lint: ## Lint the knowledge tree against the authoring contract
	$(DOCGEN_RUN) lint -root $(KNOWLEDGE_DIR)

docs-guidance: ## Write/refresh the managed guidance docs from the current source
	$(DOCGEN_RUN) guidance -root $(KNOWLEDGE_DIR)

docs-deploy: docs ## Build the docs fresh, then sync _site/ to docs.f5w.nl/f5w-docgen via ansible (Mac-only)
	$(DOCS_DEPLOY)

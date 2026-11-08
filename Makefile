# The only interface to this repository: humans, agents and CI all call `make`.
# Targets are listed in AGENTS.md; `make -n <target>` shows what each runs.
SHELL := bash
.SHELLFLAGS := -euo pipefail -c

OPENAPI         ?= openapi.yaml
GENERATED_PATHS ?= api/internal/db/dbgen api/internal/api web/src/lib/api

# Go tools (goose, sqlc, oapi-codegen, govulncheck, tbls, gitleaks) are pinned in tools/go.mod.
GOTOOL := go tool -modfile=$(CURDIR)/tools/go.mod
GOOSE  := $(GOTOOL) goose

# golangci-lint is installed as a pinned binary into .bin/ (`make tools`).
GOLANGCI_LINT_VERSION ?= v2.15.0
BIN := $(CURDIR)/.bin
GOLANGCI_LINT := $(BIN)/golangci-lint

# Local development database (docker-compose.dev.yml). CI provides PostgreSQL as a service and sets CI=true.
COMPOSE := docker compose -f $(CURDIR)/docker-compose.dev.yml
ifdef CI
PSQL := psql -v ON_ERROR_STOP=1 -X -q
else
PSQL := $(COMPOSE) exec -T db psql -U postgres -v ON_ERROR_STOP=1 -X -q
endif
PG_SUPER_URL ?= postgres://postgres:postgres@localhost:5432
MIGRATE_URL  ?= postgres://migrator:migrator@localhost:5432/bappelitbangda_dev?sslmode=disable
export PSQL PG_SUPER_URL GOOSE

.PHONY: tools web-browsers gen gen-check lint test test-full test-api test-web vuln secret-scan build docker-build release smoke \
        verify verify-full docs-check skills-check contract-lint contract-diff \
        db-up db-down db-init db-migrate db-new db-generate db-test db-docs db-erd db-docs-check

tools: $(GOLANGCI_LINT) web-browsers

# Chromium for the web component tests (vitest browser mode). CI uses `--with-deps` to install system libraries too.
web-browsers:
	cd web && npx --no-install playwright install chromium

$(GOLANGCI_LINT):
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# ---- code generation ----
gen: db-generate
	cd api && $(GOTOOL) oapi-codegen -config oapi-codegen.yaml ../$(OPENAPI)
	cd web && npx --no-install openapi-typescript ../$(OPENAPI) -o src/lib/api/schema.d.ts

# Fails if regenerating changes anything under GENERATED_PATHS (stale or hand-edited generated code).
gen-check:
	@snap() { mkdir -p $(GENERATED_PATHS); find $(GENERATED_PATHS) -type f | sort | xargs sha256sum | sha256sum; }; \
	before="$$(snap)"; \
	$(MAKE) --no-print-directory gen; \
	after="$$(snap)"; \
	[ "$$before" = "$$after" ] || { echo "ERROR: generated code is stale or was edited by hand. Run 'make gen' and commit."; exit 1; }; \
	echo "gen-check: generated code is up to date"

# ---- quality gates ----
lint: $(GOLANGCI_LINT)
	@test -z "$$(gofmt -l api)" || { echo "gofmt needed:"; gofmt -l api; exit 1; }
	cd api && go vet ./... && $(GOLANGCI_LINT) run
	cd web && npm run lint && npm run check

test:
	cd api && go test -race -short ./...
	cd web && CI=true npm run test

test-full: db-init
	cd api && ../scripts/db-throwaway.sh go test -race ./...
	cd web && CI=true npm run test

# usage: make test-api RUN=TestPublications PKG=./internal/server/...   (focused run on a throwaway DB)
RUN ?= .
PKG ?= ./...
test-api: db-init
	cd api && ../scripts/db-throwaway.sh go test -race -count=1 -run '$(RUN)' $(PKG)

# usage: make test-web FILE=src/routes/publications
test-web:
	cd web && CI=true npx --no-install vitest run $(FILE)

# ---- API contract ----
contract-lint:
	cd api && $(GOTOOL) vacuum lint --ruleset ../.vacuum.yaml --details --no-banner --no-update-check ../$(OPENAPI)

# usage: make contract-diff BASE=origin/development   (report only; never fails on breaking changes)
BASE ?= origin/development
contract-diff:
	@tmp="$$(mktemp --suffix=.yaml)"; trap 'rm -f "$$tmp"' EXIT; \
	git show "$(BASE):$(OPENAPI)" > "$$tmp"; \
	cd api && $(GOTOOL) oasdiff breaking "$$tmp" ../$(OPENAPI) --format markdown

vuln:
	cd api && $(GOTOOL) govulncheck ./...
	cd web && npm audit --omit=dev --audit-level=high

secret-scan:
	cd api && $(GOTOOL) gitleaks git --no-banner --redact ..

build:
	@start=$$(date +%s); \
	mkdir -p dist; \
	(cd api && CGO_ENABLED=0 go build -trimpath -o ../dist/api ./cmd/api); \
	(cd web && npm run build); \
	echo "build time: $$(( $$(date +%s) - $$start ))s"

verify: gen-check skills-check contract-lint lint test vuln

verify-full: gen-check skills-check contract-lint lint test-full vuln db-test db-docs-check docs-check

docs-check:
	bash scripts/check-docs.sh

# Agent skills layout: .agents/skills/<name> canonical, .claude/skills/<name> symlink (see .agents/skills/README.md).
skills-check:
	bash scripts/check-skills.sh

# ---- database ----
db-up:
	$(COMPOSE) up -d --wait

db-down:
	$(COMPOSE) down

# Idempotent: roles and the dev database. Needs a superuser.
db-init:
	@for f in api/db/init/*.sql; do echo "init: $$f"; $(PSQL) -d postgres < "$$f"; done

db-migrate:
	cd api && $(GOOSE) -dir db/migrations postgres "$(MIGRATE_URL)" up

# usage: make db-new name=add_something
db-new:
	@test -n "$(name)" || { echo "usage: make db-new name=add_something"; exit 1; }
	cd api && $(GOOSE) -s -dir db/migrations create $(name) sql

db-generate:
	cd api && $(GOTOOL) sqlc generate

# Applies migrations to a throwaway database and runs api/db/tests/*.sql against it.
db-test: db-init
	cd api && ../scripts/db-throwaway.sh bash ../scripts/db-run-tests.sh

# Regenerates docs/schema from a freshly migrated throwaway database (see .tbls.yml).
db-docs: db-init
	cd api && ../scripts/db-throwaway.sh bash -c 'TBLS_DSN="$$TEST_DATABASE_URL" $(GOTOOL) tbls doc -c ../.tbls.yml --rm-dist'

db-erd: db-docs

# docs/schema is generated against PostgreSQL $(DOCS_PG_MAJOR): extension function lists differ between majors,
# so the check is skipped (and says so) on other versions.
DOCS_PG_MAJOR ?= 18
db-docs-check: db-init
	@major="$$($(PSQL) -d postgres -tAc 'SHOW server_version_num' | cut -c1-2)"; \
	if [ "$$major" != "$(DOCS_PG_MAJOR)" ]; then echo "db-docs-check: skipped (server is PostgreSQL $$major, docs are generated on $(DOCS_PG_MAJOR))"; exit 0; fi; \
	snap() { mkdir -p docs/schema; find docs/schema -type f | sort | xargs -r sha256sum | sha256sum; }; \
	before="$$(snap)"; \
	$(MAKE) --no-print-directory db-docs; \
	after="$$(snap)"; \
	[ "$$before" = "$$after" ] || { echo "ERROR: docs/schema is stale. Run 'make db-docs' and commit."; exit 1; }; \
	echo "db-docs-check: docs/schema is up to date"

# ---- container images, release bundle, smoke test (see docs/handover/runbook.md) ----
docker-build:
	docker build -f api/Dockerfile --target api -t bappelitbangda-api:dev .
	docker build -f api/Dockerfile --target migrate -t bappelitbangda-migrate:dev .
	docker build -f web/Dockerfile -t bappelitbangda-web:dev web

# usage: make release VERSION=v0.1.0   (builds into dist/; never deploys)
release:
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=vX.Y.Z"; exit 1; }
	scripts/release.sh $(VERSION)

# usage: make smoke URL=http://localhost:8081
smoke:
	@test -n "$(URL)" || { echo "usage: make smoke URL=http://host:port"; exit 1; }
	deploy/scripts/smoke.sh $(URL)

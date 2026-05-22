.PHONY: help check-env install-env setup setup-ui expand doc example-validate check-manifest
.PHONY: build build-service build-ui build-sdk-go dev quickstart dev-api dev-web deploy serve-ui status stop-all stop-dev stop-deploy test test-service test-ui test-ui-e2e test-capability test-quickstart-health test-ladybug verify verify-go verify-python verify-java guard ci clean

VENV_PYTHON := .venv/bin/python
CONDA_PYTHON := $(if $(CONDA_PREFIX),$(CONDA_PREFIX)/bin/python)
VIRTUAL_ENV_PYTHON := $(if $(VIRTUAL_ENV),$(VIRTUAL_ENV)/bin/python)
PYTHON ?= $(or $(wildcard $(VENV_PYTHON)),$(wildcard $(CONDA_PYTHON)),$(wildcard $(VIRTUAL_ENV_PYTHON)),python3)
GOCACHE ?= $(CURDIR)/.cache/go-build
PNPM ?= pnpm
API_ADDR ?= :8080
API_URL ?= http://localhost:8080
WEB_PORT ?= 5173
DATA_ROOT ?= data
GRAPHSTORE ?= file.memory
GO_TAGS ?= $(if $(filter local.ladybug,$(GRAPHSTORE)),ladybug,)
QUICKSTART ?= 0
QUICKSTART_WORKSPACE ?= demo
QUICKSTART_SAMPLE ?= multi-domain-quickstart
FORCE ?= 0
DRY_RUN ?= 0
PID_DIR ?= .run
LOG_DIR ?= $(PID_DIR)/logs
GO_RUN_TAGS = $(if $(strip $(GO_TAGS)),-tags "$(GO_TAGS)")
export GOCACHE

help:
	@echo "UModel open-source service"
	@echo ""
	@echo "Service:"
	@echo "  build-service          Build umodel-server, umctl, and umodel-mcp"
	@echo "  build-ui               Build the web UI under web/"
	@echo "  test-service           Run root Go tests"
	@echo "  test-ui                Type-check and build the web UI"
	@echo "  dev                    Start API and web dev server in the background"
	@echo "  quickstart             Start API and web dev server with bundled demo data preloaded"
	@echo "  deploy                 Build UI and start the production-style server in the background"
	@echo "  status                 Show local dev/deploy process, port, and health status"
	@echo "  stop-all               Stop local API, web dev, and deploy servers"
	@echo "  serve-ui               Build UI and serve it from umodel-server in the foreground"
	@echo "  test-ladybug           Run local.ladybug provider and E2E tests when UMODEL_TEST_LADYBUG=1"
	@echo "  guard                  Run architecture guard"
	@echo ""
	@echo "Schema and SDK assets:"
	@echo "  expand                 Expand schemas and regenerate SDK assets"
	@echo "  doc                    Generate schema HTML docs"
	@echo "  example-validate       Validate example UModel files"
	@echo "  verify                 Verify generated SDKs"
	@echo "  verify-go              Verify Go model SDK"
	@echo "  verify-python          Verify Python model SDK"
	@echo "  verify-java            Verify Java model SDK"
	@echo ""
	@echo "Workflows:"
	@echo "  check-env              Check local Go, Python, Node, Web, and optional Java tooling"
	@echo "  install-env            Install project dependencies for Python, Go, Web, and optional Java"
	@echo "  build                  Build service and Go SDK"
	@echo "  test                   Run guard, service tests, and SDK verification"
	@echo "  ci                     Run the local CI gate"
	@echo "  setup                  Alias for install-env"
	@echo "  clean                  Remove generated build outputs"
	@echo ""
	@echo "Node:"
	@echo "  Web UI requires Node.js 22+ and prefers pnpm 9+; corepack or npm exec fallback is supported"
	@echo ""
	@echo "Dev options:"
	@echo "  make dev defaults to GRAPHSTORE=file.memory DATA_ROOT=data with controlled Cypher enabled"
	@echo "  make quickstart loads QUICKSTART_WORKSPACE=demo and QUICKSTART_SAMPLE=multi-domain-quickstart into GRAPHSTORE=memory"
	@echo "  GRAPHSTORE=file.memory DATA_ROOT=data make dev"
	@echo "  GRAPHSTORE=memory GO_TAGS= DATA_ROOT=data make dev"
	@echo "  GO_TAGS=ladybug GRAPHSTORE=local.ladybug DATA_ROOT=data make dev"

build: build-service build-ui build-sdk-go

build-service:
	go build ./cmd/...

build-ui:
	@PNPM="$(PNPM)" bash ./scripts/env.sh web-build

build-sdk-go:
	cd ./sdk/go && go build ./...

dev:
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" DATA_ROOT="$(DATA_ROOT)" GRAPHSTORE="$(GRAPHSTORE)" GO_TAGS="$(GO_TAGS)" QUICKSTART="$(QUICKSTART)" QUICKSTART_WORKSPACE="$(QUICKSTART_WORKSPACE)" QUICKSTART_SAMPLE="$(QUICKSTART_SAMPLE)" PNPM="$(PNPM)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" bash ./scripts/dev.sh

quickstart: GRAPHSTORE = memory
quickstart: QUICKSTART = 1
quickstart: dev

deploy: GRAPHSTORE = file.memory
deploy: build-ui
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" DATA_ROOT="$(DATA_ROOT)" GRAPHSTORE="$(GRAPHSTORE)" GO_TAGS="$(GO_TAGS)" QUICKSTART="$(QUICKSTART)" QUICKSTART_WORKSPACE="$(QUICKSTART_WORKSPACE)" QUICKSTART_SAMPLE="$(QUICKSTART_SAMPLE)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" bash ./scripts/deploy.sh

dev-api:
	go run $(GO_RUN_TAGS) ./cmd/umodel-server --addr "$(API_ADDR)" --data "$(DATA_ROOT)" --graphstore "$(GRAPHSTORE)" $(if $(filter 1 true TRUE yes YES on ON,$(QUICKSTART)),--quickstart --quickstart-workspace "$(QUICKSTART_WORKSPACE)" --quickstart-sample "$(QUICKSTART_SAMPLE)")

dev-web:
	@API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" PNPM="$(PNPM)" bash ./scripts/env.sh web-dev

stop-all:
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" FORCE="$(FORCE)" DRY_RUN="$(DRY_RUN)" STOP_MODE="all" bash ./scripts/stop-dev.sh

stop-dev:
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" FORCE="$(FORCE)" DRY_RUN="$(DRY_RUN)" STOP_MODE="dev" bash ./scripts/stop-dev.sh

stop-deploy:
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" FORCE="$(FORCE)" DRY_RUN="$(DRY_RUN)" STOP_MODE="deploy" bash ./scripts/stop-dev.sh

status:
	@API_ADDR="$(API_ADDR)" API_URL="$(API_URL)" WEB_PORT="$(WEB_PORT)" DATA_ROOT="$(DATA_ROOT)" GRAPHSTORE="$(GRAPHSTORE)" PID_DIR="$(PID_DIR)" LOG_DIR="$(LOG_DIR)" bash ./scripts/status.sh

serve-ui: build-ui
	go run $(GO_RUN_TAGS) ./cmd/umodel-server --addr "$(API_ADDR)" --data "$(DATA_ROOT)" --graphstore "$(GRAPHSTORE)" --ui-dir web/dist $(if $(filter 1 true TRUE yes YES on ON,$(QUICKSTART)),--quickstart --quickstart-workspace "$(QUICKSTART_WORKSPACE)" --quickstart-sample "$(QUICKSTART_SAMPLE)")

test-service:
	go test ./...

test-ui:
	@PNPM="$(PNPM)" bash ./scripts/env.sh web-build

test-ui-e2e:
	@cd web && npx playwright test --reporter=list

test-capability:
	go test -v -run TestCapabilityGate ./tests/integration/

test-quickstart-health:
	go test -v -run TestQuickstartHealth ./tests/integration/

test-ladybug:
	@if [ "$$UMODEL_TEST_LADYBUG" != "1" ]; then \
		echo "Skipping local.ladybug provider and E2E tests; set UMODEL_TEST_LADYBUG=1 and provide liblbug to run them."; \
	else \
		go test -tags ladybug ./...; \
	fi

guard:
	@$(PYTHON) ./tools/guards/architecture_guard.py

expand:
	@echo "Expanding schemas..."
	@$(PYTHON) ./tools/generators/schema_expander.py
	@echo "Validating expanded schemas..."
	@$(PYTHON) ./tools/validators/schema_validator.py
	@echo "Generating Go model SDK..."
	@$(PYTHON) ./tools/generators/schema_go_generator_v2.py
	cd ./sdk/go && go fmt ./...
	@echo "Generating Python model SDK..."
	@$(PYTHON) ./tools/generators/schema_python_generator_v2.py
	@echo "Generating Java model SDK..."
	@$(PYTHON) ./tools/generators/schema_java_generator_v2.py
	@$(MAKE) schemas-embed

schemas-embed:
	@echo "Mirroring expanded schemas into internal/umodel/schemaspec/data/..."
	@mkdir -p internal/umodel/schemaspec/data
	@cp expanded_schemas/*.expanded.yaml internal/umodel/schemaspec/data/

schemas-embed-check:
	@if ! diff -r expanded_schemas/ internal/umodel/schemaspec/data/ --exclude='*.md' >/dev/null 2>&1; then \
		echo "Embedded schemas under internal/umodel/schemaspec/data/ differ from expanded_schemas/. Run 'make schemas-embed' and commit."; \
		diff -r expanded_schemas/ internal/umodel/schemaspec/data/ --exclude='*.md' | head -40; \
		exit 1; \
	fi

doc:
	@bash ./tools/converters/batch_convert_html.sh

example-validate:
	@$(PYTHON) ./tools/validators/umodel_validator.py --batch examples

verify:
	@bash ./tools/verify/verify_all.sh

verify-go:
	@bash ./tools/verify/verify_go.sh

verify-python:
	@bash ./tools/verify/verify_python.sh

verify-java:
	@bash ./tools/verify/verify_java.sh

test: guard test-service verify

check-manifest:
	@$(PYTHON) ./tools/verify/check_manifest.py

ci: guard schemas-embed-check build-service test-service test-capability test-quickstart-health verify check-manifest example-validate
	@echo "Local CI passed."

check-env:
	@PYTHON="$(PYTHON)" PNPM="$(PNPM)" bash ./scripts/env.sh check

install-env:
	@PYTHON="$(PYTHON)" PNPM="$(PNPM)" bash ./scripts/env.sh install

setup: install-env

setup-ui:
	@PNPM="$(PNPM)" bash ./scripts/env.sh web-install

clean:
	@rm -rf dist/
	@rm -rf .cache/
	@rm -rf web/dist/
	@rm -rf generated/java/target/
	@rm -rf .coverage/

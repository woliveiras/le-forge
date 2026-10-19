# LeForge developer tasks. Standard Go targets use Docker via scripts/go.sh.
# build-native and test-native-browser use the installed host Go toolchain.
# Python and Node/pnpm run locally for their corresponding suites.

GO        := ./scripts/go.sh
COMPOSE   := docker compose
PYTHON    ?= python3
VERSION   ?= dev
NAMESPACE ?= leforge
GRAPH     ?= examples/graphs/software-factory.yaml

.PHONY: egress-image test-egress test-environment-policy help build test test-compose test-go test-python test-web test-sandbox test-sandbox-capacity test-environment-context lint fmt up down logs images k8s-render clean

help: ## Show targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build forged and forge binaries into server/bin
	$(GO) build -o bin/forged ./cmd/forged
	$(GO) build -o bin/forge ./cmd/forge
	$(GO) build -o bin/forge-egress ./cmd/forge-egress
	GOOS=linux CGO_ENABLED=0 $(GO) build -o bin/forge-preview-linux ./cmd/forge-preview
	GOOS=linux CGO_ENABLED=0 $(GO) build -o bin/forge-workspace-journal-linux ./cmd/forge-workspace-journal

egress-image: ## Build the managed proxy image inside the dedicated sandbox daemon
	./scripts/build-egress-image.sh

test-egress: ## Real runtime/build/setup HTTP/HTTPS allowlist and isolation checks on the dedicated daemon
	./scripts/test-egress.sh

test-environment-policy: ## Installation policy network checks on the dedicated daemon (cached alpine:3.22 required)
	./scripts/test-environment-policy.sh

test: test-go test-python test-web ## Run all test suites

test-go: ## Go unit + integration tests (memory/fs/sqlite; live brokers via LEFORGE_TEST_* env)
	$(GO) test ./...

test-python: ## SDK and example agent tests (stdlib unittest)
	cd sdk/python && $(PYTHON) -m unittest discover -s tests -v
	PYTHONPATH=sdk/python:agents $(PYTHON) -m unittest discover -s agents/tests -v
	$(PYTHON) -m unittest discover -s scripts/tests -v

test-web: ## Type-check and run the web console tests (Vitest, jsdom)
	cd web && pnpm install --frozen-lockfile && pnpm exec tsc --noEmit && pnpm test

test-compose: ## Reviewed Compose services on the dedicated daemon (cached alpine:3.22 required)
	./scripts/test-compose-runtime.sh

test-sandbox: ## Sandbox runtime isolation checks against the Compose stack (needs Docker; see docs/sandbox-runtime.md)
	./scripts/test-sandbox-runtime.sh

test-sandbox-capacity: ## Real sandbox capacity/cgroup checks (running dedicated daemon and local Go required)
	./scripts/test-sandbox-capacity.sh

test-environment-context: ## Real COPY/ADD sandbox checks (running dedicated daemon and local Go required)
	./scripts/test-environment-context.sh

lint: ## gofmt check and go vet
	@out=$$(docker run --rm -v "$$(pwd)/server:/src" -w /src golang:1.26-alpine gofmt -l .); \
	  if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...

fmt: ## Format Go code
	docker run --rm -v "$$(pwd)/server:/src" -w /src golang:1.26-alpine gofmt -w .

up: ## Start the full stack (platform + observability)
	$(COMPOSE) up -d --build

down: ## Stop the stack (volumes are kept; use `docker compose down -v` to wipe)
	$(COMPOSE) down

logs: ## Follow control-plane logs
	$(COMPOSE) logs -f forged

images: ## Build all container images
	docker build --build-arg VERSION=$(VERSION) -t leforge/forged:$(VERSION) .
	docker build -f agents/Dockerfile -t leforge/agents:$(VERSION) .
	docker build -f Dockerfile.sandbox -t leforge/sandbox:$(VERSION) .   # outside Compose; Compose builds it inside sandbox-daemon

k8s-render: ## Render platform manifests (kustomize) and agent runtimes for GRAPH
	kubectl kustomize deploy/kubernetes
	@echo "---"
	$(GO) run ./cmd/forge k8s render -f ../$(GRAPH) --namespace $(NAMESPACE) || true

clean: ## Remove build outputs
	rm -rf server/bin web/dist

.PHONY: test-e2e
test-e2e: ## Chromium browser journey against a disposable backend (Go, Node, Git, Playwright Chromium required)
	cd server && go test -tags e2e ./cmd/forge-e2e
	cd web && pnpm run test:e2e

.PHONY: test-e2e-real
test-e2e-real: ## Browser journey with shell.exec on the running dedicated sandbox daemon
	cd server && go test -tags e2e ./cmd/forge-e2e
	cd web && pnpm run test:e2e:real

.PHONY: test-e2e-providers
test-e2e-providers: ## Real LangGraph + browser provider journey, no Docker (set LEFORGE_E2E_PYTHON)
	@test -n "$(LEFORGE_E2E_PYTHON)" || (echo "Set LEFORGE_E2E_PYTHON to Python with the langgraph extra installed"; exit 1)
	PYTHONPATH=sdk/python "$(LEFORGE_E2E_PYTHON)" -m unittest discover -s scripts/provider-e2e -v
	cd server && go test -tags e2e ./cmd/forge-provider-e2e
	cd web && pnpm run test:e2e:providers

.PHONY: test-preview
test-preview: ## Real sandbox preview gateway, HTTP client and lifecycle checks
	./scripts/test-preview.sh

.PHONY: test-workspace-journal
test-workspace-journal: ## Kernel process/transient/offline journal tests in the dedicated Linux daemon
	./scripts/test-workspace-journal.sh

.PHONY: build-native
build-native: ## Build the current host's native bundle without Docker; does not initialize or start it
	./scripts/build-native.sh

.PHONY: test-native-browser
test-native-browser: ## Chromium against a disposable production native profile; no Docker required
	./scripts/test-native-browser.sh

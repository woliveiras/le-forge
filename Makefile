# LeForge developer tasks. Only Docker is required: Go runs in a container
# via scripts/go.sh. Python (3.10+) and Node (20+) are used locally if present.

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

test-web: ## Type-check and run the web console tests (Vitest, jsdom)
	cd web && npm ci --no-audit --no-fund && npx tsc --noEmit && npm test

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
	cd web && npm run test:e2e

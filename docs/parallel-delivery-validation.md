# Parallel delivery integration — 2026-10-16

The editor, provider/LangGraph, workspace/Git and skills/steering branches were
reviewed together on `codex/integrate-parallel-deliveries`, based on `ee71557`.
This record covers those four deliveries, not completion of Release 1 or the
separate human-review/preview work.

## Integrated changes

| Area | Source commits | Evidence and boundaries |
| --- | --- | --- |
| Visual editor | `153fcbf`, `ed447ef` | Chromium pointer gestures, connection persistence, invalid graph rejection and Delete cancellation/confirmation; component tests retain discard/conflict coverage. |
| Providers/LangGraph | `b0e3b60`, `dd7a851` | Real SDK, LangGraph and gateway with a deterministic OpenAI-compatible endpoint and artificial credentials; selected model and explicit failures. No live vendor inference claim. |
| Workspace/Git | `1070e7b`, `7ada3c2` | Read-only API/UI, real repository/worktree tests, task result inspection and execution/operation context. Attribution remains inconclusive; arbitrary shell effects are not automatically attributed. |
| Skills/steering | `ab4e55c`, `ce8be07` | Versioned registry, admin associations, bounded authorized project reads, immutable manifests, runtime/SDK integration and Chromium administration/inspection. Guidance cannot grant capabilities. |

Review corrections include inert private copies of Git metadata (including
configuration replacement tests), preserving mandatory task text and legacy
prompt behavior, restoring operator requests on instruction failure, preserving
null-versus-empty skill selections across YAML, and preventing late UI responses
from changing the wrong selection. The shared browser harness retains one
validated loopback-port override, `LEFORGE_E2E_PORT`, for editor, Git, skills and scripted environment journeys.

The first standard container-backed test run exposed missing Git in the base
Go image. Commit `038cebc` adds a cached test-only toolchain image, wrapper unit
tests and documentation; real workspace/API integration tests then passed with
that wrapper. No tests were skipped to accommodate this dependency.

## Reproduction

Use the prerequisites in [CONTRIBUTING](../CONTRIBUTING.md), the
[provider guide](provider-langgraph-validation.md), and the
[real sandbox guide](sandbox-acceptance.md). A Python 3.10+ interpreter must be
selected explicitly when a worktree does not inherit local version-manager
configuration. This run used Python 3.15.0, LangGraph 1.2.15 and
langchain-core 1.6.11 in an isolated environment.

```sh
GOFLAGS=-p=1 make test PYTHON=/path/to/python3
make lint
make build
LEFORGE_E2E_PORT=18110 make test-e2e
LEFORGE_E2E_PYTHON=/path/to/provider-venv/bin/python \
  LEFORGE_PROVIDER_E2E_PORT=18111 make test-e2e-providers
make test-e2e-real
```

Go packages run serially to avoid the environment-suite timeouts observed when
several sessions were testing concurrently. Each suite retains its assertions,
timeouts and failure handling. The real-daemon browser suite runs separately
from other sandbox validation to avoid competing test workloads.

## Results

All commands above passed on the integrated code:

| Check | Result |
| --- | --- |
| `make test` with `GOFLAGS=-p=1` | Go unit/integration packages, 10 SDK + 14 agent + 3 wrapper Python tests, TypeScript check and 121 web tests passed. |
| `make lint` | Container-backed gofmt check and Go vet passed. |
| `make build` | `forged`, `forge` and `forge-egress` built successfully. |
| `make test-e2e` | Harness tests, E2E typecheck, production web build and 13 Chromium scenarios passed. |
| `make test-e2e-providers` | 2 real-adapter Python tests, Go HTTP/runtime integration and 3 Chromium scenarios passed after skills integration. |
| `make test-e2e-real` | 6 Chromium scenarios passed against the dedicated daemon: proposal/approval, all four discovered sources, real shell execution, image/cache reuse, private workspaces, reset retention and the mise toolchain/lockfile journey. |
| Documentation checks | OpenAPI YAML parsed successfully; `git diff --check` passed. |

The integration's standard Go suite runs in the Git-enabled test container;
real broker/PostgreSQL and dedicated-daemon tests remain opt-in, as described
below. Browser test results are from this combined branch, not merely the
individual sessions' earlier reports.

## Remaining scope

- Live broker/PostgreSQL conformance suites require their documented
  `LEFORGE_TEST_*` endpoints and were not enabled by this integration.
- The provider endpoint and sandbox model responses are deterministic; real
  LangGraph, API, storage, gateway and dedicated-daemon execution are exercised.
- Git inspection is bounded (64 MiB metadata, 4 MiB output per command,
  10-second deadline); unsupported executable configuration fails explicitly.
  It is a live observation, not a human-review candidate snapshot.
- Skills use immediate `.leforge/steering/*.md` files only, explicit opt-in,
  16 KiB bodies, a conservative 8192-byte composition budget and at most
  128 retained revisions. See [instruction contracts](instructions.md).
- MCPs, dynamic delegation, onboarding, full release acceptance, universal
  change provenance and the deferred rootless variant remain separate work.
- The primary checkout contains concurrent human-review/preview changes.
  This integration is committed in its own attached worktree; that checkout
  was not reset, overwritten or merged while it had uncommitted work. No push,
  release or deployment was performed.

# Parallel delivery integration — 2026-10-16

The editor, provider/LangGraph, workspace/Git and skills/steering branches were
reviewed together on `codex/integrate-parallel-deliveries`, based on `ee71557`.
The initial record covers those four deliveries. The later integration with
Docker human review/previews is recorded below; neither completes Release 1.

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

## Initial four-delivery results

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

## Preview integration and main validation

After the preview session completed, merge `1598749` combined its `ef12c93`
implementation and the preserved sandbox acceptance documentation with the four
parallel deliveries. Conflicts were resolved by retaining both API families,
client types, event/runtime contracts, test targets and checklist evidence.

Two additional cross-feature checks/fixes were committed:

- `d2bc221`: real engine/storage/review integration for a `once` task with
  skills. Approval resumes exactly once with the human decision and current
  skill revision; rejection does not resume. The earlier execution manifest
  remains immutable after the skill changes.
- `5f3c186`: Python `ToolResult.yield_turn` preserves the review protocol's
  `yield` field. Unit serialization and real HTTP callback tests cover legacy
  results, yielded results and denied callbacks. Custom framework loops still
  need to honor this signal; the engine enforces the review gate independently.

The combined result was fast-forwarded into `main`, then the general suites,
lint, build and provider journey were repeated directly in the primary checkout.
All checks passed:

| Check | Final result |
| --- | --- |
| `GOFLAGS=-p=1 make test PYTHON=/path/to/python3` | Go unit/integration packages, 13 SDK + 14 agent + 3 wrapper Python tests, TypeScript and 124 web tests passed on `main`. |
| `make lint` and `make build` | gofmt/vet and all four binaries, including the Linux preview relay, passed on `main`. |
| Engine review/skills integration | Approval and rejection passed locally; the full engine package then passed in the test container and again in the general `main` run. |
| `LEFORGE_E2E_PORT=18110 make test-e2e` | 13 combined Chromium scenarios passed. |
| `make test-e2e-providers` with the documented Python/port variables | Python adapter tests, Go callback integration and 3 Chromium scenarios passed; repeated on `main` after the SDK fix. |
| `make test-e2e-real` | 8 real-sandbox Chromium scenarios passed, including the original six environment journeys plus React/HTTP/WebSocket review approval and request-changes/new-review. |
| `make test-preview` | Real Docker HTTP/cURL, cache isolation, recovery, expiry/restart and shutdown test passed. |
| Contracts/history | Combined OpenAPI parsed with review, skill, instruction and Git routes; diff whitespace check passed; both source histories are ancestors of `main`. |

The browser/sandbox checks ran sequentially on the combined code in the
integration worktree before the fast-forward. General tests, build and provider
checks also ran in the primary checkout. Subsequent changes are documentation
only. Interactive Postman and a public DNS/TLS deployment were not tested; the
preview guide distinguishes these from the tested private-forwarder topology.

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
- Integration initially stayed in its attached worktree while preview work
  was active. After that session finished, preview commit `ef12c93` and the
  preexisting sandbox documentation (`5cb92c0`) were preserved and merged with
  the four deliveries in `1598749`. `main` then advanced by fast-forward. No
  source history was rewritten, and no push, release or deployment was performed.

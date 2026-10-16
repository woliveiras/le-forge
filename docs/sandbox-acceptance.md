# Sandbox release acceptance

Scope: repository discovery or an initial developer-approved proposal, real
isolated execution, compatible image/cache-seed reuse, bounded active resources,
idle resume and reference-safe cleanup. Replacement proposals, full format
compatibility, registry-wide network controls, exact queue ETAs and unregistered
legacy-resource deletion are not required for this release.

## Evidence map

| Contract | Existing evidence | Closure work |
|---|---|---|
| Priority, malformed input, containment | environment discovery/selection/context/stages unit tests; API discovery tests | Real browser matrix passed for all four sources; final suites pending |
| Initial proposal gate, stale/rejected/cancelled/failed work | proposal analysis/proposals/agent_tools tests; browser proposal/recovery journeys | Run final suites |
| Real shell, isolated workspaces and shared image | `make test-e2e-real`, reuse live tests | Real browser Reset preservation and lockfile rebuild passed |
| Cache seeds, lockfile invalidation, concurrent builders, failure cleanup | reuse unit/integration/live tests; cleanup unit/live tests | Run final suites |
| CPU/memory/count, build/setup admission, cancellation | capacity and progress integration/live tests; browser recovery tests | Browser preparation wait and subsequent setup passed |
| Idle resume | engine sandbox_idle tests; `make test-sandbox-capacity`; browser recovery | Run final suites |
| Network restrictions and diagnostics | policy/egress/history/retention tests; dedicated-daemon scripts | Run final network suites |
| Representative mise repository | `examples/projects/mise-node`; real browser approvals, tool/dependency install, offline build/tests, isolated task reuse | Passed on dedicated daemon |

Do not interpret unchecked legacy checklist rows as proof of missing code. This
map separates existing evidence from the outstanding closure checks. Final
commands and outcomes will be recorded here after execution.

## Closure validation so far

- `go test -tags e2e ./cmd/forge-e2e`: passed.
- `make test-e2e`: 10 browser tests passed.
- `make test-e2e-real`: 6 browser tests passed against the dedicated daemon.
  Inference is scripted; approval, builds, native `shell.exec`, workspace writes,
  invalidation and Reset use real components. No external model quality claim.

The mise journey uses a pinned public image and Node version, a committed npm
lockfile and `.dockerignore` exclusions for installed dependencies and outputs.
The first attempt without those exclusions correctly blocked continuation after
setup changed the approved context; the corrected example passed end to end.

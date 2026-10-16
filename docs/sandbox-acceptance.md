# Sandbox release acceptance

Scope: repository discovery or an initial developer-approved proposal, real
isolated execution, compatible image/cache-seed reuse, bounded active resources,
idle resume and reference-safe cleanup. Replacement proposals, full format
compatibility, registry-wide network controls, exact queue ETAs and unregistered
legacy-resource deletion are not required for this release.

## Evidence map

| Contract | Existing evidence | Closure work |
|---|---|---|
| Priority, malformed input, containment | environment discovery/selection/context/stages unit tests; API discovery tests | Run final suites; real browser source matrix |
| Initial proposal gate, stale/rejected/cancelled/failed work | proposal analysis/proposals/agent_tools tests; browser proposal/recovery journeys | Run final suites |
| Real shell, isolated workspaces and shared image | `make test-e2e-real`, reuse live tests | Extend real browser reset/rebuild coverage |
| Cache seeds, lockfile invalidation, concurrent builders, failure cleanup | reuse unit/integration/live tests; cleanup unit/live tests | Run final suites |
| CPU/memory/count, build/setup admission, cancellation | capacity and progress integration/live tests; browser recovery tests | Add browser preparation wait |
| Idle resume | engine sandbox_idle tests; `make test-sandbox-capacity`; browser recovery | Run final suites |
| Network restrictions and diagnostics | policy/egress/history/retention tests; dedicated-daemon scripts | Run final network suites |
| Representative mise repository | No completed example yet | Add runnable example and real validation |

Do not interpret unchecked legacy checklist rows as proof of missing code. This
map separates existing evidence from the outstanding closure checks. Final
commands and outcomes will be recorded here after execution.

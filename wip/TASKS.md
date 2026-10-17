# LeForge Implementation Tasks

This checklist breaks down the [product plan](./PLAN.md) into implementable work. Tasks are grouped by dependency and may be reordered when implementation reveals additional technical dependencies.

Checked boxes represent only work that has been implemented and verified. Release 1 is still in progress. Implemented features include browser interaction with the visual editor, managed providers through the real Python LangGraph adapter, read-only workspace/Git inspection, and opt-in skills/project steering with per-execution manifests. The supported sandbox lifecycle and durable Docker review/preview flows are implemented; see [sandbox acceptance](../docs/sandbox-acceptance.md) and section 4a. Rootless remains deferred. MCP live DCR/CIMD/vendor acceptance, dynamic delegation, onboarding, universal file-change provenance, native preview supervision, automatic PR/CI adapters and full Release 1 validation remain pending.

## Release 1 — Build and Operate Graphs Through the Interface

### 0. Prepare Product Contracts and Acceptance Criteria

- [x] Inventory the current API, interface, runtime, workspace, storage, and Docker deployment flows that will be retained or changed. (Recorded in [product-contracts.md](../docs/product-contracts.md#1-current-state-inventory).)
- [x] Define API contracts and persisted models for provider configurations, MCP connections, per-agent permissions, instructions, interface-initiated tasks, and child executions. (The current provider pattern and proposed MCP, skill, task, and delegation contracts are documented in [product-contracts.md](../docs/product-contracts.md#3-installation-managed-connections-and-skills) through [section 4](../docs/product-contracts.md#4-user-tasks-and-child-executions).)
- [x] Define authorization and isolation between installation, graph, agent, workspace, and execution; no credentials may be sent to the agent runtime. (The installation boundary, role matrix, scoped callback, workspace isolation, and grant inheritance are documented in [product-contracts.md](../docs/product-contracts.md#5-authorization-audit-and-error-states).)
- [x] Define auditable events and their relationship to graph, agent, replica, task, and execution. (Existing event envelope and proposed event families are documented in [product-contracts.md](../docs/product-contracts.md#5-authorization-audit-and-error-states).)
- [x] Define error contracts and interface-visible states for validation, unavailability, pending approval, timeout, and version conflict. (Common response mapping and corresponding UI states are documented in [product-contracts.md](../docs/product-contracts.md#5-authorization-audit-and-error-states).)
- [x] Update architecture diagrams and documentation to reflect the documented contracts. (Added the explicitly planned component-boundary view in [product-contracts.md](../docs/product-contracts.md#6-planned-component-boundaries) and linked it from [architecture.md](../docs/architecture.md).)

**Done when:** Contracts shared by the backend, interface, and runtime are documented and reviewed before implementations diverge. The inventory and proposed contracts are recorded in [product-contracts.md](../docs/product-contracts.md); planned behavior is clearly distinguished from shipped functionality.

### 1. Remove Out-of-Scope Product Functionality

- [x] Remove endpoints, interfaces, and import/conversion code that are out of scope.
- [x] Remove examples, fixtures, tests, and dependencies used exclusively by the removed flows.
- [x] Update affected documentation, navigation, copy, messages, and development instructions.
- [x] Search for residual references in code, documentation, configuration, and tests; confirm no out-of-scope flows or mentions remain.
- [x] Run tests and builds for affected areas and fix regressions directly related to the removal.

**Done when:** Removed flows are neither accessible nor referenced in the product, and relevant tests/builds pass.

### 2. Turn the Canvas into a Graph Editor

#### Model and API

- [x] Define create, read, update, and delete operations for agents and channels, keeping validation centralized in the backend. (Decision: the whole-document `PUT /graphs/{id}` with `If-Match` is the only write path; the editor applies agent/channel operations to a draft — `web/src/components/canvas/edit.ts` — and the engine validates on save. Documented in [api.md](../docs/api.md).)
- [x] Persist visual positions and required settings without discarding advanced fields or YAML extensions that the interface does not edit. (`x-` keys on graph, metadata, nodes and channels round-trip through JSON storage and YAML export; other unknown keys are rejected with their path instead of being dropped.)
- [x] Use version control/optimistic concurrency when saving graphs and return recoverable conflicts.
- [x] Ensure the backend rejects invalid graphs and returns errors associated with the relevant agent, channel, or field.

#### Interface

- [x] Add agent creation on the canvas and define valid initial values.
- [x] Add editing for agent position, properties, runtime, model, instructions, and permissions. (Name, description, runtime details, model, prompt, loop/schedule/stop, replicas, workspace, approval gates, tool grants and per-tool approval.)
- [x] Add channel creation by visually connecting agents.
- [x] Add editing for supported channel type, routing, filters, conditions, approvals, and other properties.
- [x] Add multi-selection or appropriate confirmation before removing connected elements. (Toolbar and Delete-key removals, including multi-selection, confirm the agents/channels affected.)
- [x] Implement editing states: pending changes, save, discard, save in progress, and error.
- [x] Show validation errors next to the relevant element and property; allow users to fix and revalidate.
- [x] Detect version conflicts, preserve local changes, and provide an explicit recovery path.
- [x] Preserve drag, zoom, pan, selection, and element positions during real-time state updates.
- [x] Keep the YAML editor as an advanced alternative, synchronized with the same model and version control.
- [x] Store graph documents as written: `spec.defaults` are applied when graphs are validated and run, not copied into nodes, so changing a default reaches nodes without their own value (found while running the bookaneer test: changing the default model had no effect). The editor shows effective values (e.g. loop mode) where nodes leave them unset.
- [x] Edit graph-wide settings in the visual editor (name, description, workspace repository, parameters, max hops) and per-turn agent limits (turn timeout, max tool calls, token budget). Repository URLs must be remote; local paths and non-network transports are rejected on save and before cloning.
- [x] Add interaction tests for creating, editing, connecting, validating, saving, discarding, and recovering from conflicts. — Covered by component tests against the real API client (`GraphWorkspace.test.tsx`: create, edit, validate/revalidate, save, server 422, discard, conflict overwrite/reload, removal confirmation, YAML sync) and unit tests for the edit operations (`edit.test.ts`, including connect and keyboard removal). Chromium E2E now exercises real drag-to-connect and Delete-key cancellation/confirmation, agent/channel persistence and validation against the disposable real controller (`web/e2e/graph-connections.spec.ts`).

**Done when:** A user can create, configure, validate, save, and start a graph through the visual interface without writing YAML.

### 3. Provide Containerized Project Environments

Decisions recorded for this section: a Docker daemon dedicated to LeForge (Docker-in-Docker, rootless where supported) replaces access to the host daemon; explicit interface configuration takes precedence, otherwise repository discovery follows `.leforge/environment.yaml` → `.devcontainer/devcontainer.json` → Dockerfiles → Docker Compose → agent analysis with developer approval. No LeForge-specific file is required. Compatible images and caches are reused across tasks for the same authorized repository, while workspaces and active containers remain isolated; execution runs without network by default, with an optional host allowlist through an egress proxy.

#### Isolated Container Runtime

- [x] Add a Docker-in-Docker service to the Compose stack (rootless variant where the host supports it, privileged fallback documented with its risk), with TLS between `forged` and the daemon and its own data volume. (`sandbox-daemon`, privileged `docker:28.5.2-dind` with mutual TLS and the `sandbox-daemon-data` volume; risk documented in [sandbox-runtime.md](../docs/sandbox-runtime.md) and [ADR 0009](../docs/adr/0009-dedicated-sandbox-daemon.md).)
- [ ] Offer the rootless Docker-in-Docker variant where the host supports it, and document how to choose it.
- [x] Point the docker sandbox and environment builds at that daemon (`DOCKER_HOST`); remove the need to mount the host Docker socket and document the change. (`isolation.docker.host` / `LEFORGE_SANDBOX_DOCKER_HOST`; the default image is built inside the daemon by the `sandbox-image` service. Environment builds will use the same daemon.)
- [x] Make workspaces visible to the dedicated daemon at the paths sandboxes mount (shared volume), replacing `hostRoot` translation for this setup.
- [x] Put sandboxes on a network that cannot reach platform services (database, broker, observability, control plane); verify with a test that a sandbox cannot connect to them. (Firewall in `deploy/sandbox-daemon/entrypoint.sh`, fail-closed and checked by the healthcheck; `make test-sandbox` probes the daemon, its gateway and `forged` from real sandboxes.)
- [x] Document reset: removing the daemon's volume deletes every image, container, and cache it created, and the next run rebuilds from definitions.
- [x] Report the sandbox daemon in `/readyz` (non-fatal) and in **Integrations → Sandbox runtime**; let operators choose docker isolation, network, image, and limits per agent in the visual editor.

#### Environment Model and Lifecycle

- [x] Define the environment contract: source (interface, repository, agent proposal), base image or Dockerfile, setup commands, working directory, resource limits, cache volumes, and network policy per phase (setup, execution); record it in [product-contracts.md](../docs/product-contracts.md). (`spec.environment` is the single effective definition; repository and agent sources produce a definition the operator accepts into it.)
- [x] Persist environments per graph with a content hash of the effective definition; rebuild only when the definition changes or on explicit request.
- [x] Implement the lifecycle `pending → building → ready | failed`, with build and setup logs stored and size-limited; record events for state changes with graph and environment identifiers. (States `absent/building/ready/failed`; 64 KiB log tail; `environment.*` events; interrupted builds detected through the store lease.)
- [x] Run image builds and setup steps in LeForge with resource limits, timeouts, and cancellation; never through agent-controlled Docker access.
- [x] Start agent sandboxes from the ready environment image, with dependency caches mounted per environment; refuse to start nodes that require an environment that is not ready, with an actionable error.
- [x] Implement Rebuild and Reset (remove containers, image, and caches of one environment), and clean up environments of deleted graphs. (Reset is refused while the graph runs.)
- [x] Add metrics and traces for build duration, outcome, and failures, following existing telemetry names.

#### Definition Sources

- [x] Read-only discovery API with unit and API integration tests: candidate inspection for the agreed priority, hashes, service names, explicit overrides and bounded checkout reads. Supported-format validation, build contexts and operator-triggered analysis are delivered below; broader format/policy support remains pending.

- [x] Allow operators to configure an environment in the interface: a prebuilt image, or a Dockerfile plus setup commands. (**Graph settings → Environment**.)
- [x] Persist discovery/selection provenance, accepted definitions, durable proposal approvals and reuse identity. Use bounded manifest/context fingerprints, not a full Git revision. Per-graph setup/images/caches remain private while prepared generations are shared; see environment/proposals and storage/API integration tests.
- [x] Discover definitions in order: `.leforge/environment.yaml`, `.devcontainer/devcontainer.json`, Dockerfiles, then Docker Compose. Preserve explicit interface overrides and show the effective source. A LeForge-specific file is never required.
- [x] Parse the LeForge definition and the supported image/post-create subset of devcontainer configuration. Report malformed and unsupported fields; an invalid higher-priority file must not silently trigger a fallback or agent proposal.
- [x] Support validated repository contexts for local COPY/ADD, Docker ignore rules, bounded tar snapshots and acceptance/cache invalidation by digest. Cover unit, API integration and real dedicated-sandbox builds. Additional Dockerfile features remain pending.
- [x] Discover Dockerfile candidates within bounded repository paths and present ambiguous candidates for operator selection; support validated build contexts confined to the authorized workspace, respecting ignore rules.
- [x] Add explicit build-target selection for multi-stage Dockerfiles in discovery review, LeForge YAML and Compose; validate local named/indexed COPY --from dependencies and preserve stage paths. Unit, API/UI integration and real sandbox tests cover target acceptance, cache identity, invalid references, cycles and build failures.
- [x] Support COPY --from external images (names, tags, registries and digests), preserving local-stage precedence and validating references before Docker. Unit, API integration and real sandbox tests cover review/persistence, reference changes, tag/digest copies and unavailable registries.
- [x] Discover standard Compose filenames and standalone variants (no implicit merging). Select a development service and explicitly review its transitive dependency closure. Support up to three prebuilt runtime helpers per replica on an internal network with atomic CPU/memory/count admission, suspension/resume and owned-resource teardown. Reject mounts, ports, environment injection, privileges, interpolation, network overrides and health conditions. Unit, API/storage/runtime/UI integration and cached-image real-daemon tests cover the supported subset.
- [x] Present supported normalized definitions and source for explicit acceptance into the graph draft, then admin Save graph before Build. Persist origin and referenced-file hashes; revalidate on save and before Docker operations, including cached builds. Unit, API integration and component tests cover review, stale files, unsupported settings, permissions and conflicts. Installation-wide policy configuration now validates preview/save/build/runtime and agent approval; see Network Policy.
- [x] When no definition file is found, let the operator invoke a bounded analysis agent with read-only repository access that works without the project environment. Analyze manifests, lockfiles and toolchain declarations; propose tools, image, setup, caches, network needs and rationale without installing dependencies or running repository commands.
- [x] Treat repository and agent definitions as untrusted: they cannot raise resource limits, mount host paths, request privileges, or widen network policy beyond installation settings.
- [x] Persist full environment proposals in developer Approvals; admin approval revalidates inputs, saves the definition and starts build. Block graph/node starts until ready, optionally start the graph after readiness, restore approved work after restart, and expose rejection/cancellation/expiration/errors and approved-build retry. Unit, API/UI integration and real sandbox tests cover the workflow. Repository fingerprinting covers analyzed inputs rather than Git commit identity.
- [x] Add the `environment.status` tool and the `environment.propose` tool (own graph, Docker node without image override; complete agent definition; durable approval/build and blocking turn continuation): the complete proposal opens a developer approval gate. Keep the task waiting until acceptance; rejection/cancellation must not build or run anything. Revalidate the approved revision, build after acceptance, and resume execution only when ready; expose build failures and retries.
- [x] Provide the [mise proposal example](../examples/projects/mise-node/README.md): pinned public image and Node, locked pnpm install, developer approval, offline build/tests and second-task reuse with private workspaces/caches. Verified by `make test-e2e-real`; inference is scripted.

#### Reuse Across Tasks and Resource Budgets

- [x] Define a repository/installation-scoped reuse key from the full image-definition digest, accepted context, bounded manifests/lockfiles, daemon platform/address and versioned policy. Setup output stays private; arbitrary setup reruns in each task. The fingerprint is bounded and is not a full Git revision.
- [x] Reuse compatible prepared images across repository graphs with renewable leases, force rebuilds and input invalidation. Setup runs per isolated workspace; shared images exclude setup container state. API/UI status and logs explain reuse. Unit, API integration and real sandbox tests cover the flow.
- [x] Reuse immutable dependency-cache seeds within the exact repository/installation scope. Copy seeds read-only into task-private writable volumes before setup; never publish runtime mutations. Cache declarations must exclude credentials and task outputs. Concurrent writers never share a volume across graphs.
- [x] Isolate graph/replica workspaces and active containers. Idle continuation reuses the same stopped container after capacity readmission; real daemon tests verify files/cache preservation and two-task isolation. Process-memory checkpointing and arbitrary controller-restart continuity are not promised.
- [x] Enforce finite memory/swap caps and a global count/reserved-memory budget for Docker execution sandboxes; persist reservations, inspect daemon inventory, wait/cancel admission and release only after confirmed removal. Preserve existing node CPU limits. Unit, configuration/app integration, concurrency and real cgroup tests verify the flow.
- [x] Extend global memory/concurrency admission to image builds, setup and cache-copy helpers. Use bounded temporary BuildKit workers, sequential setup/copies, cancellable waiting and durable reservations retained on cleanup failures.
- [x] Add global CPU accounting for execution/build/setup/cache-copy pipelines: finite total/default CPU, decimal parsing, atomic durable reservations, conservative legacy handling, kernel caps, cancellation and idle readmission. Unit, configuration/storage/component integration, concurrent-manager and real-daemon cgroup/CPU-only admission tests cover the flow.
- [x] Stop idle execution containers between turns with configurable `isolation.docker.idleTimeout` (5m default, 0s disables). Preserve container files, workspace and caches; reacquire capacity before new work, keep paused tasks paused, retry failures without losing inputs, and document restart versus explicit teardown behavior.
- [x] Preserve shared images/cache seeds on Reset and graph deletion; graph status retains resource references, and shared generations are conservatively retained.
- [x] Implement explicit admin preview/cleanup of registered shared generations and newly tracked crash leftovers. Track exact generations across rebuilds; revalidate status/definition/build/container references under reuse leases; preserve stopped containers, other tasks and private workspaces; invalidate reuse before partial deletion and support retry. Legacy unregistered resources remain untouched. Unit, storage/API integration and real daemon tests cover the flow.
- [x] Add Integrations UI for shared generation previews, retention reasons, selection (up to 100) and explicit confirmation; display partial/missing/retained outcomes and refresh after attempts. Cover policy units, component/API-client integration, and real sandbox cleanup validation.
- [x] Record reuse/build reasons in environment events and status, capacity transitions in events and bounded metrics, plus build/admission/lifecycle/cleanup outcome and duration metrics. Covered by progress/API, capacity metric and live tests; exact queue position/ETA is outside scope.

#### Network Policy

- [x] Enforce installation `environmentPolicy` on preparation/build RUN and runtime networks, manual/repository/agent definitions, approvals, cached images and node overrides. Add feature restrictions for proposals/Dockerfiles/setup/companions. Unit, storage/API integration and real sandbox command-egress tests cover the delivered subset; daemon registry egress remains external.
- [x] Add an opt-in runtime HTTP/HTTPS egress proxy with exact per-environment host lists bounded by the installation list, public-IP validation/pinned dials, isolated internal networks, bounded denial logs and aggregate capacity/lifecycle.
- [x] Manage installation network modes and host ceiling in Integrations with admin-only preview/confirmation, persisted revisions, usage/impact listing, dynamic enforcement and unit/integration/real-sandbox revocation/resume tests. Proxy diagnostics are delivered separately below.
- [x] Configure installation history retention in Integrations (admin review/confirmation, revision checks, bounded days/count, dynamic enforcement, unit/API/storage/UI integration and real sandbox tests).
- [x] Persist collected proxy denials per graph with seven-day/1,000-event retention, background and lifecycle collection, concurrent-safe deduplication, independent history UI/API, and unit/integration/real sandbox coverage. Guaranteed audit delivery remains outside this subset.
- [x] Show per-environment runtime/build proxy process state and bounded blocked/failed request samples, preserve the last preparation snapshot, handle unavailable diagnostics explicitly, and verify with unit, storage/API/UI integration and real sandbox tests.
- [x] Add preparation egress with separate setupAllowedHosts, installation validation, isolated BuildKit/setup networking, proxy capacity and cleanup, image reuse invalidation, editor fields and unit/integration/real sandbox tests. Daemon-wide registry restrictions remain pending.
- [x] Test that runtime egress cannot bypass the proxy, allowed HTTP/HTTPS hosts work, forbidden/private destinations fail, and suspension/resume/proxy failures remain isolated. Unit, API/storage/UI integration and real dedicated-daemon tests cover the subset.

#### Interface

- [x] Show the environment of a graph: source, effective definition, state, last build, logs, and cache usage. (Cache paths and volumes are listed; volume sizes are not shown yet.)
- [x] Add Rebuild and Reset actions with confirmation, restricted to operators/admins according to the role matrix.
- [x] Show discovery candidates, selected source and definition acceptance; show pending agent proposals in Approvals with rationale and full definition. Expose waiting, rejected, cancelled and build-failed states without implying execution succeeded. Proposals currently require no existing environment; diff/replacement workflows remain pending.
- [x] Show reuse decisions, build reason categories and structured build phases/capacity waits in settings and the agent panel; show recent execution admission history and shared-resource effects before Reset. Exact file diffs and durable queue positions/ETAs remain pending.
- [x] Show the graph environment name, image, readiness, failure and start blocker on the agent panel, respecting inherited isolation and explicit image overrides; report unknown readiness on fetch errors. Unit, component/client and storage/API integration tests plus real daemon progress/reuse validation cover this flow.

#### Tests and Documentation

- [x] Unit tests for definition parsing, precedence, hashing, policy enforcement on untrusted definitions, and lifecycle transitions, including build failures and cancellations. — Done for validation, hashing, the user layer, lifecycle, failures, cancellation, concurrency, interrupted builds and reset. Precedence and repository/agent-source validation are now covered; installation-wide policy, cached-image revocation and persisted approval revalidation are now covered.
- [x] `make test-e2e-real` discovers, accepts, builds and executes native `shell.exec` for all four supported repository sources plus approved proposals. It checks uid 10001, shared-image location in the dedicated daemon, workspace isolation, lockfile invalidation/rebuild and Reset preservation. `make test-sandbox` separately checks host sandbox inventory and network isolation.
- [x] Interface tests for configuration, state display, proposal approval/rejection, Rebuild and Reset: component/client integration, deterministic browser tests and real browser lifecycle checks.
- [x] Add unit tests for discovery priority, ambiguous candidates, malformed definitions, supported Compose/devcontainer fields, context containment and absent-definition fallback; include approval rejection, stale approval, timeout and analysis failure. See discovery, selection, context, proposal_analysis and proposals tests.
- [x] Add unit, storage/API integration and real sandbox tests for reuse keys, input invalidation, build deduplication across manager instances, lease-wait cancellation, private cache copies, failed setup/copy, workspace isolation and Reset retention.
- [x] Test execution-sandbox budgets, multi-controller admission, memory/count boundaries, queue cancellation, cleanup failures and restart accounting, including a real dedicated daemon.
- [x] Test build resource budgets with unit, storage/API integration and real dedicated-daemon checks, including selected contexts, failure, cleanup/recovery and competition with execution admission.
- [x] Test idle/resume lifecycle: unit eligibility/failure checks, engine/storage/queue integration for active turns, schedules, paused tasks and input retry, plus real daemon stop/start, preserved files/container identity and capacity release/readmission.
- [x] Add integration tests with repository fixtures and the real dedicated daemon for each source and the approved-proposal flow; prove no build/task starts before approval and failures remain visible. Covered by API/proposal integration, `make test-environment-context` and the real browser source matrix.
- [x] Add integration tests proving compatible tasks reuse an image/cache, changed lockfiles invalidate reuse, concurrent builds are deduplicated, task workspaces and unauthorized repositories remain isolated, and reset cannot remove referenced resources. Verify queue admission, resource release and idle-container resume. Covered by reuse/capacity/cleanup integration and dedicated-daemon suites.
- [x] Browser E2E covers discovery/review, initial proposal approval and rejection, pre-approval blocking, analysis/build failure, compatible reuse, CPU/memory execution waits, cancellation/restart, retry, build/setup waiting and idle resume with preserved files. `make test-e2e` uses real controllers with Docker/model fixtures; `make test-e2e-real` uses real containers, all discovery sources, rebuild/Reset and the mise project. Only model inference is scripted in the real suite.
- [x] Document discovery priority, supported file formats, ambiguity/error handling, approval, reuse/invalidation, resource budgets, shared-resource reset and troubleshooting in the environment, deployment and getting-started guides. See the updated guides and [acceptance record](../docs/sandbox-acceptance.md).

**Done when:** A repository needs no LeForge-specific file: discovery follows the agreed precedence, and absence of definitions leads to agent analysis with developer approval before building or executing. Compatible subsequent tasks reuse images/caches with isolated workspaces and bounded active resources. Rebuild, reset and cleanup respect shared references; all project commands stay inside LeForge containers, nothing is installed on the host, and agents never access Docker.

### 4. Create the File and Interaction Panel

#### Workspace API and Security

- [x] Define file listing, reading, diff, and history endpoints limited to the graph's authorized workspace.
- [x] Apply path containment, symlink validation, size limits, and traversal protection to every operation.
- [x] Expose read operations to users; do not implement user-invocable write, delete, or upload endpoints.
- [x] Enforce through backend authorization that only authorized agents can modify the workspace. — Tool grants remain default-deny; native `workspace.readOnly` overrides mutation/shell grants, Docker mounts read-only and rejects writable-container reuse. External runtimes cannot claim this policy; arbitrary process execution remains trusted code. Unit, integration and browser tests cover denied writes and unchanged files.
- [x] Provide Git, branch, worktree, agent/replica, and execution metadata associated with changes when available.
- [ ] Record file changes and operations in a queryable form associated with the execution that produced them. — Partial: durable `workspace.observed` events capture bounded before/after changes for filesystem, Git-write and shell calls, including failures, linked to execution/node/replica and shown in Files (latest 100 operation/observation events). Unit/integration tests cover changes, limits, read failures and persistence across restart; browser tests cover shell-created files. Continuous monitoring now also records net changes from concurrent/external/background writers without assigning an execution or author. The dedicated Linux daemon journal now adds kernel PID evidence, transient create/delete events and replay after controller downtime while the collector remains active. Exact agent/execution attribution for arbitrary processes, invisible PID namespaces, collector outages and lost kernel events remain outside the proven contract.

- [x] Monitor authorized shared workspaces and known replica worktrees independently of tool calls and task state; persist net changes from external/background and concurrent writers, show unknown authorship and coverage gaps in Files. — Unit/integration coverage includes baseline, discovery/read failures, recovery, cancellation, independent OS processes and restart persistence; browser coverage observes delayed concurrent writers after task completion.

- [x] Capture kernel process evidence, transient filesystem events and concurrent writers in the dedicated Linux daemon, persist a private journal and replay it idempotently after controller downtime. — Live fanotify tests cover PID/start identity, transient create/delete and concurrent processes; storage tests cover restart/replay/cursor failures; Chromium consumes the real pre-controller journal. Other runtimes and collector-offline history remain unsupported.

#### Operational Interface

- [x] Add a file browser with a tree, loading and error states, and safe navigation.
- [x] Add a read-only file viewer with a server-enforced content limit.
- [x] Add read-only Git status and diff views.
- [x] Show the directory, branch, replica, and execution associated with the workspace/change.
- [x] Allow users to start a task for a graph or agent from the interface. — Existing run controls exercised by provider, environment and Git browser journeys.
- [x] Allow users to send instructions/commands to an agent using existing operator mechanisms, without enabling direct file editing. — Existing operator-message controls and message-driven environment resumption have browser coverage.
- [x] Present task state, progress, events, messages, approvals, and results in an accessible flow. — Graph Activity combines current replica state/wait reasons, recent executions and summaries/errors, messages, approvals, graph-wide reviews and events. Explicit execution/message identities filter related records; file inspection and agent controls are linked. Independent failures, refresh/recovery, viewer-safe controls and bounded recent-history limits are visible. Unit and real-client component integration tests cover attribution, failures/retry, decisions and graph switching; real-controller Chromium covers approval, rejection, message inspection, persisted results and navigation to files. No first-class task resource or percentage-complete estimate is implied.
- [x] Link changed files, diffs, messages, and executions; clearly indicate when attribution is inconclusive.
- [x] Add authorization tests ensuring the interface cannot modify, delete, or upload files to the workspace.
- [x] Add end-to-end tests for starting a task on the web, monitoring execution, and inspecting resulting files/diffs.

**Done when:** Users can operate tasks in the browser and inspect results, but no user action in the API or interface directly changes workspace files.

**Verified scope:** `/git/status`, `/git/diff` (tracked changes against HEAD) and `/git/history` are read-only. Git runs against a private bounded metadata copy; executable configuration, external metadata redirection, path escapes and concurrent configuration replacement are covered by real repository tests. The UI shows recent execution/operation context rather than claiming diff authorship. See [API contracts](../docs/api.md).

### 4a. Durable Human Review and Application Previews

Delivered for Docker sandboxes. See `PLAN.md` section 5a and the operational
contract in `docs/sandbox-runtime.md#human-review-and-application-previews`.
The explicitly selected native `process` mode remains outside managed preview
supervision.

#### Review Contract and Continuation

- [x] Persist candidate SHA-256, frozen content including dirty changes, generation, reviewer/method policy, decision actor/time and a durable continuation outbox. Preserve existing approval clients.
- [x] Support managed HTTP previews and manually supplied external URLs, local validation, and acceptance without execution; label reported checks/external links as unverified.
- [x] Yield the runtime turn and release its input lease; block further model/tool callbacks and output. Recheck candidate identity at decision, continuation delivery and completion.
- [x] Implement approve, request changes, reject, lifecycle cancellation and review expiry. Keep preview expiry separate; decisions do not merge or deploy.
- [x] Insert continuation messages once across controller recovery without resetting consumed messages. External effects retain normal idempotency requirements.

#### Service Supervision and Protected Access

- [x] Supervise an explicitly authorized argv/cwd/HTTP port/readiness path in Docker with bounded lifetime, process group termination and bounded logs.
- [x] Reuse shared CPU/memory/slot admission and approved helper configuration; suspend the agent sandbox during human waiting, keep preview accounting separate, and copy caches into private runtime storage.
- [x] Reconnect after controller restart; stop on readiness loss, expiry or decision, expose cleanup failures, retry cleanup and retain accounting until confirmed release. Preserve persistent workspaces/snapshots/containers.
- [x] Bundle a local-by-default gateway with loopback host publication and per-review origins, without external services or public DNS. Supply a scoped stdio relay without published application ports.
- [x] Authenticate browser exchange/cookies and dedicated client headers per session, preserve application Authorization, enforce origins and body/concurrency/stream bounds, and revoke active streams.
- [x] Support HTTP, React routes/assets, application cookies and WebSockets. Document remote TLS/DNS/private-tunnel configuration and local resolver troubleshooting.

#### Interface, Contracts and Validation

- [x] Show candidate evidence, instructions, readiness/expiry/errors, preview URL, browser opening, private credential issuance, cURL template, logs, decisions, revoke/restart/renew actions.
- [x] Document API, tool yield/continuation, configuration, lifecycle events and bounded metrics/traces without credential or HTTP payload logging.
- [x] Add unit/integration coverage for invalid inputs, permissions, stale candidates, concurrent decisions, cleanup failure, expiry, generation changes, cross-session credentials, storage recovery and engine gating.
- [x] Exercise real Docker HTTP requests and cURL, network-none/no-published-port isolation, private cache mutation, controller reconstruction, expiry/restart and final shutdown (`make test-preview`).
- [x] Exercise real Chromium React navigation, mutable HTTP with application auth, cookies, WebSockets, approve/revoke and request-changes/new-review (`make test-e2e-real`), through a private gateway forwarder while sandbox IPs remain unreachable.
- [x] Provide a manual Postman recipe and remote TLS configuration. Automated tests use HTTP/cURL and a private remote topology; an interactive Postman session and a public DNS/TLS deployment are not claimed as tested.
- [ ] Later: native process supervision under explicit host-execution policy, with its weaker isolation clearly reported.
- [ ] Later: provider-specific PR/CI adapters with authenticated event validation and exact revision matching. Manual external/local review already works.

**Done when:** The remote Docker web and HTTP review journeys in `PLAN.md` 5a
pass, including protected access, durable human waiting, exact candidate binding,
recovery and resource release. A stored link or a blocking `human.ask` call alone
does not satisfy this milestone.

### 5. Configure Model Providers Through the Interface

#### Backend and Secrets

- [x] Define secure provider credential storage with an installation-configurable encryption key. (`internal/secrets`: record-bound AES-256-GCM, `LEFORGE_SECRET_KEY`/0600-generated file, previous-key rotation; revisioned `llm/providers` setting in all three storage adapters.)
- [x] Persist configurations without returning or logging plaintext secrets; allow credentials to be updated, replaced, and removed.
- [x] Protect read, write, and connection-test endpoints with authentication and authorization. (Viewers can read, operators can test, admins can modify.)
- [x] Implement connection/available-model tests without exposing credentials in errors, logs, or events. (The test performs real inference through the gateway; errors that echo the key are redacted.)
- [x] Allow selecting the default provider/model and per-agent overrides.
- [x] Ensure runtimes, prompts, and agents receive only mediated model access, never the provider key. (Tested for native and external runtimes through the `llm/chat` callback.)

#### Interface and Runtime

- [x] Add a screen for creating and editing providers and their required parameters. (Extension of the existing **Integrations** screen.)
- [x] Indicate when a credential is configured without revealing its value.
- [x] Add a connection-test action and show actionable success/error states.
- [x] Offer default provider/model selection and per-agent configuration in the visual editor.
- [x] Validate configuration before starting an execution and explain missing or invalid settings.
- [x] Check tool calling in the provider connection test, and send/configure the context window (Ollama `num_ctx`, default 32768 instead of Ollama's silent 4096 truncation); prompts that do not fit fail with a clear error. (Found running the bookaneer test with local models.)
- [x] Make native agent turns fail with the cause instead of succeeding without work: tool calls written as text (recovered when they name an offered tool), empty or cut-off answers, and running out of model rounds (rounds now follow `limits.maxToolCalls`). (Warning on save; `422` with the field path when starting a graph/node.)
- [x] Report a once-mode agent whose only turn failed as `failed` (not `completed`), explain model timeouts (not retried; `OLLAMA_TIMEOUT_SEC`), run environment setup with `CI=true`, and offer "Build and start when ready" when a start is refused because the environment is not ready.
- [x] Verify and document the full path from the LangGraph runtime to the selected provider. — Real Python SDK/LangGraph → callback gateway → managed provider, including node override, failure propagation and credential isolation. The local model endpoint is deterministic; this is not live vendor/model-quality validation. See [provider-langgraph-validation.md](../docs/provider-langgraph-validation.md).
- [x] Add tests for encrypted persistence, secret update/removal, connection testing, and per-agent selection.
- [x] Add an end-to-end test from interface configuration to an execution that uses the selected provider. — `make test-e2e-providers` configures/tests a managed provider in Chromium, selects the model, executes the real LangGraph adapter and checks explicit errors and artificial-credential redaction. Requires Python with the SDK LangGraph extra.

**Done when:** An operator creates and tests a provider on the web, selects it for an agent, and runs the agent without exposing the secret to the runtime.

### 6. Integrate User-Configurable MCPs

**Quota/history validation:** `make test` (179 frontend tests plus Go/Python),
`make lint`, `make build`, and `make test-e2e` (31 standard Chromium cases plus
1 hosted CIMD case) passed. Race-enabled Go tests passed for MCP, gateway, API
and storage. Coverage includes concurrent shared admission, restart persistence,
minute/clock boundaries, approval refusal accounting, storage failures, SQL/TLS
API integration, sparse cursor pages, safe projection and browser quota/history
filters with failure/retry. No deployment or live-connection quota changes were
performed for this increment.

**Lifecycle/recovery validation:** `make test` (176 frontend tests plus Go/Python),
`make lint`, `make build` and `make test-e2e` (30 standard Chromium cases plus
1 hosted CIMD case) passed. Targeted Go race tests passed. Deletion/recovery
were exercised against disposable API/storage/TLS/browser fixtures, not deployed
to the personal installation or tested by deleting a live vendor connection.

**Live evidence (2026-10-17):** GitHub browser OAuth consent/callback and discovery
succeeded on the local installation. Explicit `get_me`/`get_file_contents` agent
grants were exercised through the gateway with a deterministic model and real
Ollama `llama3.2:latest` (4,096-token context). The latter read the profile and
`octocat/Hello-World` README and completed successfully. No writes, private
repository access, live renewal/revocation or organization-policy tests were
performed. Atlassian DCR remains uncertain; do not register again without external
evidence. These checks do not complete the vendor acceptance items below.

#### Connectivity and Integrations

- [x] Document the official endpoint, transport, authentication and scope contracts for GitHub, Bitbucket Cloud, Jira Cloud and Confluence Cloud. — [Source survey, 2026-10-17](../docs/mcp-integrations.md); documentary verification only. Runtime OAuth qualifications, per-tool permission mappings and live acceptance remain below.
- [x] Choose an initial transport compatible with Docker and implement connections without running arbitrary commands in the main process. — Remote HTTPS Streamable HTTP discovery, protocol 2025-06-18, JSON/SSE, public-IP-only dialing, no redirects or subprocesses; see [MCP scope](../docs/mcp.md).
- [x] Design a generic MCP connection registry, with no artificial connection limit and support for multiple connections per service. — Revisioned per-connection settings with stable IDs and safe read models.
- [x] Implement the connection lifecycle: create, edit, test, disable, re-enable, revoke, and delete. — Deletion requires admin, current revision, disable and prior OAuth revocation, with typed UI confirmation. Local credentials/catalogue are removed; a minimal tombstone permanently reserves the ID. Historical grants and stale callbacks/writes cannot revive it. Remote applications/registrations are not deleted.
- [x] Implement discovery and refresh of tools offered by each connection. — Initialization, paginated tools/list, persisted catalogue revisions and visible failure state; fresh sessions per operation.
- [x] Implement timeouts, cancellation, payload limits, and explicit handling for disconnection, errors, and invalid responses. — Discovery and calls: 15s deadline, 2 MiB response bound, 64 KiB call arguments, 20 discovery pages/500 tools, JSON/SSE validation and redacted errors.
- [x] Implement bounded session recovery for the supported request-scoped transport. — A session-bearing HTTP 404 reinitializes once within the original deadline; discovery restarts and pre-dispatch initialization can recover. Dispatched tools/call requests are never replayed. Network/other HTTP failures remain explicit; persistent SSE cursor resumption is not supported.
- [x] Add contract tests for connection, discovery, invocation, failures, and timeouts. — Unit and real TLS/API tests cover calls, JSON/SSE, cancellation, failures, session cleanup, payload limits and stale revisions.

#### Credentials, Authorization, and Auditing

- [x] Encrypt tokens/secrets at rest and ensure they never appear in read responses, logs, prompts, or events. — Bearer credentials use the installation secret box; safe views and lifecycle events omit values/ciphertext; no runtime exposure.
- [x] Implement assignment of connections and allowed tools to specific agents. — Exact discovered grants saved in node tools; validation rejects disabled/unknown/wildcard MCP names. The shared gateway enforces these grants on each call.
- [x] Enforce authorization at the gateway for every invocation, regardless of what the agent/model requests. — Exact MCP grants only; wildcard/cross-connection requests denied.
- [x] Add per-connection/agent rate limits, timeouts, and response-size limits. — Persistent fixed-minute connection quotas aggregate agents/replicas using shared storage; admin configuration, atomic admission, restart persistence, fail-closed storage failures and rate-limit audit outcomes are covered. Per-agent grants, deadlines and payload bounds remain independent. In-flight concurrency and provider billing quotas are outside this increment.
- [x] Allow human approval to be configured for selected tools. — Existing requiresApproval grants use the shared approval flow, with revision revalidation before dispatch.
- [x] Audit invocations with agent, connection, tool, result, duration, and error, removing or masking sensitive data. — Qualified tool name encodes connection; events contain outcome, not raw arguments/results; metrics aggregate MCP names.
- [x] Ensure an agent cannot select another connection, escape its grants, or read credentials. — Trusted execution principal and registered endpoint/credential only; exact names, no wildcard authority.
- [x] Test effective revocation: after access is revoked, the next invocation must be denied. — Connection disable denies subsequent calls and calls waiting for approval; already dispatched operations are not rolled back. Graph grants follow the execution snapshot contract.

#### Interface

- [x] Add a screen for registering multiple Bitbucket, GitHub, Confluence, and Jira connections. — Generic remote MCP form with service labels; vendor-specific authentication/scopes and live-service acceptance remain pending.
- [ ] Complete vendor-specific authentication/scope guidance. — Generic pre-registered OAuth configuration, requested/granted scope display and explicit authorization are delivered; provider qualification remains pending.
- [x] Show connection status, last validation, discovered tools, and errors without revealing secrets.
- [x] Allow connections to be enabled/disabled and discovered tools to be refreshed.
- [x] Allow available connections/tools to be selected for each agent. — Dedicated selector, persisted graph edits and removal of unavailable grants.
- [x] Show a filterable history of calls, approvals, results, and errors. — Viewer-readable persisted history filters connection/graph/agent/tool/event/outcome/time, with newest-first exclusive cursors and bounded sparse scans. Results are safe outcome summaries; raw arguments, results, approval comments and upstream errors are omitted. Deleted connection history remains queryable.
- [x] Add interface tests for connection setup, validation, agent authorization, and revocation. — Components and Chromium cover setup, saved grants, successful agent execution, wildcard denial and connection disable against a real TLS fixture.
- [ ] Add end-to-end tests for each priority integration using appropriate test credentials.

#### Vendor Delivery Checklist

All behavior changes below require unit and real-boundary integration tests;
complete each product with browser E2E and an opt-in live test using authorized
credentials/resources. Fixture coverage alone does not complete live acceptance.

- [x] Implement the initial versioned template catalogue with public endpoint/service defaults, authentication guidance, source/review date and validation status; persist and validate template references without granting tools. — Five v1 presets, unit tests, API/persistence integration and Chromium save/edit coverage.
- [ ] Extend templates with parameterized endpoints and credential modes, and preview version upgrades without silently expanding access. — Initial presets are fixed endpoints; manual customization detaches the reference.
- [ ] GitHub: add the github.com PAT template, read-only/toolset profiles and a tested per-tool fine-grained permission matrix; distinguish repository defaults from enforced credential restrictions.
- [x] GitHub.com OAuth wire compatibility: handle omitted authentication methods with the pinned confidential-client profile, POST client secret, comma-separated scopes and HTTP 200 OAuth errors; unit, TLS API integration and Chromium fixture coverage. See `docs/mcp.md` for manual registration. Live account consent, discovery and explicit profile/public-file reads through the gateway are verified (2026-10-17); broader vendor acceptance remains pending.
- [ ] GitHub: implement host-owned OAuth app registration/configuration and guided consent; test expiry, organization/SSO restrictions and insufficient permissions.
- [ ] GitHub: validate repository/issue/PR reads, an approved test mutation, inaccessible repository denial and revocation in browser and live-service acceptance.
- [ ] Atlassian foundation: qualify v2 protocol, tool schemas and catalogue bounds; support the exact reviewed `tools=all` query option while rejecting unknown/secret query parameters; test pagination and catalogue drift.
- [ ] Atlassian foundation: implement OAuth and encrypted Basic email/token credentials, retain service-account Bearer support, and explain admin enablement, site consent and IP/domain restrictions.
- [ ] Atlassian foundation: use flat per-operation discovery and product-specific selection; prevent `executeRead`/`executeWrite`/`executeDestructive` dispatcher grants from bypassing exact authorization; resolve site context and test cross-product denials.
- [ ] Bitbucket Cloud: guide organization-linked workspace/repository selection and read/write MCP scopes; test unlinked workspaces, cross-workspace denial, PR reads and an approved test mutation; pipelines/merge are separate grants.
- [ ] Jira Cloud: guide site/project selection and read/search/write MCP scopes; test JQL/issue reads, missing scope/project access, field/transition failures and approved test writes; exclude delete/manage from initial defaults.
- [ ] Confluence Cloud: guide site/space selection and read/search/write MCP scopes; test restricted content, page/search reads, approved updates, version conflicts and permission failures; do not fetch returned attachment links automatically.
- [x] Offer GitHub, Atlassian, GitLab instance and custom OAuth setup defaults and exact callback/registration guidance; preserve legacy labels and explicitly report pending external qualification.
- [x] Exercise independent installation-owned connections, including repeated services, overlapping OAuth transactions, renewal/revocation isolation and per-agent exact grants with unit, TLS integration and browser fixtures.
- [x] Support the documented GitLab public pre-registration contract with PKCE, canonical instance endpoints and `mcp` scope; reject REST scope substitution and unrelated metadata.
- [ ] Complete live Atlassian MCP client provisioning/consent and corporate domain/site/IP approval with the administrator; do not substitute an unqualified REST 3LO app.
- [ ] Qualify the chosen GitLab instance/version using a user-approved registration, real tool read, renewal, revocation and scope/policy denials.

#### Automatic OAuth Client Setup — DCR and CIMD

- [x] Verify advertised provider capabilities using official documentation and public metadata (2026-10-17). — GitLab.com advertises DCR, not CIMD; instance administrators may disable DCR. Atlassian MCP v2 binds to an issuer advertising both DCR and CIMD. GitHub documents no DCR. See [evidence and exact metadata URLs](PLAN.md#6-add-mcp-configuration) in the MCP plan; this is capability verification only, without registration or consent.
- [x] Add capability-driven selection: reuse configured pre-registration first, otherwise prefer usable advertised CIMD, then advertised DCR, then manual setup. Show the mechanism/reason and retain an explicit manual option; never register on link opening or metadata inspection, silently replace an existing client, or switch mechanisms after authorization failure.
- [x] Implement backend DCR with validated resource/issuer/registration endpoint, reviewed callback/scopes and supported client authentication; validate responses, bound requests and handle denied registration, malformed responses, rate limits and ambiguous POST outcomes without blind retries.
- [x] Persist registration identity and encrypted credentials per connection/issuer; reuse after restart, serialize concurrent registration, and invalidate incompatible pending flows on reviewed issuer/callback/mode changes. Keep local revoke, upstream grant revoke and remote registration deletion distinct; do not implicitly delete registrations used elsewhere.
- [x] Implement CIMD with a stable, provider-reachable public HTTPS metadata URL, matching URL-form client ID and exact approved callbacks derived from publicUrl; publish no secrets or connection-private data. Explain localhost/unreachable-document limitations and offer DCR/manual setup without automatically exposing the installation.
- [ ] Qualify live GitLab DCR for the selected instance/version and `mcp` scope, including administrator disablement and manual fallback; enable CIMD only if that instance explicitly advertises it. Do not extend the existing public-client compatibility exception to unqualified registration responses.
- [x] Persist safe DCR failure categories and provider HTTP status, refresh failure state in the UI and clear stale confirmation; cover 403/503, restart/concurrency, secret redaction and browser reload without a duplicate POST. Historical uncertain attempts remain blocked pending external evidence; diagnostics cannot reconstruct a past response.
- [ ] Qualify live Atlassian v2 CIMD and DCR against the advertised issuer, callback and corporate policies; keep compatible pre-registration available and explain provisioning constraints. Neither metadata advertisement nor REST 3LO support is proof of live MCP acceptance.
- [x] Add unit tests for selection, callback/document validation, registration responses, credential redaction, expiry and failures; preserve PKCE/state protections and public-only endpoint validation.
- [x] Add real TLS/API/encrypted-storage/gateway integration tests for DCR and CIMD through consent, discovery, renewal and revoke; include concurrent registration, restart reuse, dependency failures, issuer mismatch, connection isolation and denial without explicit agent grants.
- [x] Add browser E2E for DCR, hosted CIMD and manual alternatives, unavailable CIMD on localhost, consent refusal, registration failure and reauthorization; show capability availability separately from successful authorization/discovery. — 28 standard cases plus 1 hosted CIMD case passed via `make test-e2e`; provider-specific administrator disablement remains in live qualification.
- [ ] Record separately authorized live GitLab and Atlassian registration, consent, discovery/tool read, renewal/revocation and corporate-policy denials; update affected guides and bounded audit outcomes. Keep external acceptance pending until proven; no credentials, registrations or installation authentication changes are authorized by this checklist.

**Validation:** `make test` (171 frontend tests plus Go/Python suites), `make lint`, `make build` and `make test-e2e` passed. Targeted Go race tests cover registration and the API. Public-client CIMD and DCR none/basic/post are supported; initial registration tokens, expiring client secrets, private-key JWT and remote registration deletion are outside this increment. No live provider registration or installation deployment was performed.

#### Browser-Link Connection and OAuth Lifecycle

- [x] Add shareable links to a template/version on the local or hosted LeForge installation; validate public parameters, reject extra parameters/unknown templates and preserve selection through login. — Hash route, review screen, explicit admin save, unit and Chromium tests; opening a link never creates a connection or grants tools.
- [x] Add endpoint/scope review and an explicit authorize action for administrators; retain protected API-key/PAT entry for providers without OAuth. Never include secrets in connection links.
- [x] Implement resource/issuer metadata validation and a configured backend callback independent of the frontend hash route for pre-registered public or client_secret_basic clients; preserve the public-only HTTPS policy. — Generic TLS fixture coverage; see [runtime limits](../docs/mcp.md#browser-oauth-pre-registered-clients).
- [ ] Qualify each vendor's metadata/resource URI, client registration, scopes and refresh/revocation behavior with live credentials; deliver DCR/CIMD through the dedicated checklist above and qualify provider-specific compatibility where required.
- [ ] Add distributed pending OAuth transaction storage for multi-instance routing. — Current pending flows are bounded, expire in ten minutes, require sticky routing and restart after backend restart; completed credentials persist.
- [x] Implement authorization-code flow with PKCE and expiring, single-use state bound to the initiating user/session, installation and connection; reject forged, mismatched, replayed or expired callbacks and untrusted return destinations.
- [x] Return to the originating LeForge connection screen after consent; exchange the code on the backend, encrypt tokens and run test/discovery before showing success. Redact callback codes and clean the visible URL; keep tokens out of frontend storage, logs and model context. Handle denial, cancellation, lost login session, provider failure and discovery failure explicitly.
- [x] Require explicit per-agent tool assignment after connection; OAuth consent or a template link never grants agent authority or silently expands existing grants on re-consent.
- [x] Implement server-side renewal where supported, with serialized concurrent refresh, atomic rotated-token persistence and bounded failure handling; show reauthorization required when refresh is unavailable, expired or revoked. Never retry ambiguous remote writes automatically.
- [x] Implement revocation that immediately blocks subsequent local calls, invalidates pending callbacks/refreshes and removes stored tokens; request upstream revocation where supported and report remote failure/unsupported revocation separately with provider-side instructions. Prevent stale callbacks/refreshes from restoring a revoked connection; distinguish revocation from temporary disable and from connection deletion.
- [x] Add unit tests for link validation, callback/state/PKCE handling, credential redaction, expiry, renewal and revocation races; retain the documented boundary that already dispatched calls cannot be rolled back.
- [x] Add integration tests across API, encrypted persistence, gateway and a real TLS OAuth/MCP fixture: code exchange, discovery, concurrent refresh, rotation, revocation, dependency failures and denial after revocation.
- [x] Add browser E2E for template link/review → provider consent → callback → discovery → explicit agent grants → execution → revocation; cover denied consent, invalid return, renewal failure and failed upstream revocation. — Existing template E2E covers login preservation; replay/session mismatch and races have API/unit coverage.
- [ ] Validate each supported OAuth provider with opt-in live credentials and authorized test resources before marking its vendor journey complete.


#### Additional Template Backlog

- [x] Survey additional official MCP servers for software, data, ML and AI engineering. — [13 candidates and delivery order](../docs/mcp-templates.md), reviewed 2026-10-17; this does not certify compatibility or add all candidates to Release 1.
- [ ] Qualify first-wave templates with protocol/live smoke tests: DeepWiki, Context7, Hugging Face and W&B. — Presets and template/link unit/integration/browser coverage are delivered; current live vendor qualification remains pending, with prior installed-flow evidence only for DeepWiki.
- [ ] Add typed, encrypted custom-header support and bounded non-secret metadata headers for dbt/LangSmith; qualify per-tool auth requirements; add Sentry OAuth and Databricks workspace/service authentication profiles.
- [ ] Qualify BigQuery stateless protocol and Snowflake lifecycle differences before implementing compatibility profiles, IAM/role guidance and data-query cost controls.
- [ ] Design isolated managed MCP hosting for Playwright, MLflow and Qdrant, including inbound TLS/auth, upstream credentials, process lifecycle, resource limits and explicit private-network policy; never run arbitrary server commands in the main process.

**Done when:** Connections for each priority integration can be opened from a browser template link, reviewed, created, tested, assigned to agents and audited; supported OAuth flows authorize and return to LeForge, renew or request reauthorization, and revoke access with clear local/upstream outcomes. Unauthorized calls are denied, and browser/live-provider acceptance is recorded separately from fixture coverage.

### 7. Add Skills and Steering

#### Content Model and Loading

- [x] Define formats and storage for global instructions and application-managed skills.
- [x] Define recognized formats and paths for project-specific instructions in the repository.
- [x] Implement explicit association of a workspace/repository with authorized graphs.
- [x] Implement deterministic precedence among global, graph, agent, project, and task instructions.
- [x] Define explicit behavior for conflicts, invalid/duplicate files, size limits, and missing instructions.
- [x] Load project instructions only from the authorized workspace and record their source, version/hash, and scope.
- [x] Treat instructions and skills as untrusted content; they cannot change grants, credentials, policies, or limits.
- [x] Enforce size/token limits and inform operators when content is omitted or truncated.

#### Interface, Auditing, and Tests

- [x] Add an interface for creating, editing, versioning, enabling/disabling, and deleting global instructions/skills.
- [x] Add an interface for associating instruction sets with graphs and agents.
- [x] Show recognized repository instructions, their source, and whether they were accepted for use.
- [x] Record and show the effective set of instructions loaded for each execution.
- [x] Add tests for precedence, workspace isolation, malformed instructions, size limits, and privilege non-escalation.
- [x] Add interface tests for administration and inspection of loaded content.

**Verified scope:** Administration and graph associations require admin; guidance is opt-in and cannot alter grants. Project loading only considers immediate `.leforge/steering/*.md` files in the authorized workspace. Composition reserves agent/task text before optional content; required-budget/storage failures stop before runtime invocation and preserve operator requests. Immutable manifests record loaded content, versions/hashes and omissions. Unit, storage/API/engine/SDK integration, component and Chromium tests cover this scope. See [instructions.md](../docs/instructions.md) for precedence, 16 KiB content/128-entry bounds, 8192-byte conservative budget and retained-history limits.

**Done when:** Global instructions are managed in the application, project instructions travel with the repository, and each execution is traceable without allowing instructions to grant privileges.

### 8. Allow Dynamic Delegation to Subagents

#### Model, Policies, and Execution

- [ ] Define entities and states for delegation, child execution, approval, cancellation, result, and consumption.
- [ ] Add a runtime capability for requesting a delegated subtask and waiting for/returning its result to the parent agent.
- [ ] Implement temporary child execution instantiation without editing the permanent graph.
- [ ] Define how child configurations are selected from installation-approved templates; do not accept arbitrary configurations that could elevate privileges.
- [ ] Implement grant inheritance and guarantee that a child receives no more permissions than its parent.
- [ ] Implement a configurable global policy: approval required for every delegation or bounded automatic execution.
- [ ] Implement pending-approval state and block the child execution until a human decision when required.
- [ ] Enforce global server-side limits for depth, children per parent, concurrency, duration, and cost.
- [ ] Account for consumption across the entire tree and stop/reject new delegations when limits are reached.
- [ ] Implement child execution cancellation and defined propagation of failure/cancellation to the parent agent.
- [ ] Define behavior for timeout, failure, partial result, and delegation with no result.
- [ ] Add auditing with parent/child, objective, model/configuration, effective grants, approval decision, and consumption.

#### Interface and Tests

- [ ] Add configuration for global approval policy and limits.
- [ ] Add a delegation-approval queue with enough context for an informed decision.
- [ ] Show the execution tree, state, agent, duration, consumption, and parent/child relationship.
- [ ] Allow an operator to stop a child execution or tree according to their permissions.
- [ ] Add tests for approval/automatic policies, limits, grant inheritance, recursion, concurrency, and cancellation.
- [ ] Add end-to-end tests for delegation, returning results to the parent, and viewing the tree in the interface.

**Done when:** Agents decide when to delegate; operators choose individual approval or bounded automation; the server enforces limits and permissions; the tree is observable and controllable through the web.

### 9. Make the Docker Installation Operable Through the Interface

- [ ] Review `docker-compose` for data persistence, secrets, internal networks, and restart behavior of required services.
- [ ] Ensure MCP services, providers, storage, and runtimes are reachable as configured without exposing unnecessary ports.
- [ ] Define secure initial configuration for credentials, authentication, and first-operator creation.
- [ ] Add health checks and show dependency status in the console.
- [ ] Document installation, upgrades, backup, restore, key rotation, and failure recovery.
- [ ] Ensure users can perform initial setup, connect tools, configure models, build a graph, and operate tasks without shell access, manual server-file editing, or CLI usage.
- [ ] Run a clean-install test following only the documentation and interacting through the browser after starting the containers.

**Done when:** A new installation can be configured and operated in the browser; infrastructure commands are limited to initial deployment and documented administrative maintenance.

### 10. Guide First Use with Onboarding

#### Journey and Content

- [ ] Define the initial journey from arriving at the application through the first completed task, identifying prerequisites and alternatives when providers/MCPs are not yet configured.
- [ ] Explain graph, agent, channel, tool, execution, and approval concepts in plain language.
- [ ] Guide installation and model-provider health checks without presenting configuration/listing as proof that inference succeeded.
- [ ] Guide MCP connection and permission assignment when available; adapt the steps and honestly identify unavailable features.
- [ ] Guide creation of agents and channels in the visual editor, validation, and saving of the first graph.
- [ ] Guide task submission, monitoring of events/messages/delegations/approvals, and inspection of files/diffs when available.
- [ ] Create or review a safe, functional example graph, documenting what users must configure or adapt before running it.

#### Interface, Progress, and Validation

- [ ] Add progressive, dismissible, resumable onboarding that does not block experienced users or erase current work when the guide starts or resumes.
- [ ] Allow users to return to onboarding and contextual help after completion without inadvertently resetting the completion state.
- [ ] Show the real state of prerequisites and dependencies; provide navigation to configuration and never mark steps complete through fallback or mere configuration presence.
- [ ] Define and document how progress is persisted and how to deliberately restart the guide, respecting the installation's authentication and privacy model.
- [ ] Add unit and integration tests for progress, resumption, dismissal, intentional restart, missing prerequisites, and errors during guided steps.
- [ ] Add end-to-end tests for the first-use journey, including canvas graph creation and execution/result inspection when corresponding steps are available.
- [ ] Update quickstart documentation and contextual help to remain consistent with onboarding.

**Done when:** A new operator can follow onboarding in the browser, understand essential concepts, configure available dependencies, create or adapt an example graph, and run a first task; they can skip steps, resume, and revisit help without server or CLI access.

### 11. Integrated Validation of Release 1

- [ ] Create an end-to-end test flow: installation → login → provider → MCP connection → permissions → visual editor → environment build → task → execution → files/diff → approval, when enabled.
- [ ] Repeat the flow with both approval-required and automatic delegation policies.
- [ ] Validate onboarding on a clean installation, including steps that depend on real test MCPs and providers.
- [ ] Verify that no interface path allows users to directly edit workspace files.
- [ ] Verify that no secret reaches the browser after saving, the runtime, a prompt, or logs.
- [ ] Verify that no project toolchain, image, or container is installed or created on the host; everything runs on the LeForge Docker daemon and is removed by its reset.
- [ ] Verify that data and instructions from one graph/workspace do not appear in another scope.
- [ ] Run relevant backend, frontend, SDK/runtime, and Docker image tests; record known limitations.
- [ ] Update user, operations, and troubleshooting guides to reflect delivered behavior.

**Done when:** Release 1 acceptance criteria in `PLAN.md` are covered by automated tests and a documented manual test.

## Release 2 — Reusable Components

Start only after Release 1 is complete and stable.

- [ ] Define a versioned format for reusable agent and subgraph components.
- [ ] Define component parameters, defaults, validation, and version compatibility.
- [ ] Implement permission-aware component storage, listing, reading, updating, and deletion.
- [ ] Implement saving an existing agent/subgraph as a component.
- [ ] Implement inserting a component into a graph through the visual editor.
- [ ] Implement parameter resolution on insertion without inadvertently overwriting local settings.
- [ ] Show the version in use, available updates, and diffs before updating an instance.
- [ ] Implement component sharing according to installation permissions.
- [ ] Add round-trip, version, parameter, permission, and update-without-configuration-loss tests.
- [ ] Add documentation and end-to-end tests for the reuse workflow.

**Done when:** Components can be created, versioned, shared, and reused through the interface without manual copying or loss of local configuration.

## UX Reorganization — Approved Direction

- [x] Deliver separate Connections, Models and Administration destinations with stable navigation, scoped loading, compatible legacy MCP/template/OAuth links and unchanged role gates. — Route unit tests and real-client component integration cover destination isolation, viewer controls and health failure/recovery; disposable-backend Chromium covers navigation, reload/back, legacy links and narrow viewer access. The complete standard and hosted-CIMD browser suites pass. This first slice retains existing forms; no installation activation is implied.
- [x] Add a scalable graph selector and agent list alongside the canvas, preserving links and draft state. — Search graph/agent names and IDs, restore visited destinations, mark unsaved graph drafts and keep inactive editor sessions in memory with effects paused. Preserve original draft revisions across remote refreshes and retain local drafts when a graph becomes unavailable. Canvas links and explicit agent permissions remain intact; narrow layouts move the list above the canvas. Unit/component integration covers search, errors/reload, selection, cached editor selection and revision conflicts. Real-controller Chromium verifies independent drafts, conflicts, persisted grants and viewer navigation. All 226 frontend tests and 44 standard/hosted-CIMD browser scenarios pass; six navigation/editor scenarios were repeated after the final selection fix. make test, make lint and make build pass. Drafts do not survive page reload; installation activation and developer usability validation remain separate.
- [x] Deliver MCP connection list/detail screens (Overview, Tools, Usage & history, Settings), explicit creation, authorization/revocation/error recovery and grant visibility. — Search/filter and route unit tests; real-client component integration for failures, revision conflicts, credential preservation, scoped history and inherited/explicit grants; disposable-backend browser journeys for creation, discovery, edit/disable, reload, history scoping, viewer restrictions and missing records. Existing OAuth/DCR/CIMD suites remain passing. Legacy setup links retain their compatibility flow. Installation activation and developer usability validation remain separate.
- [x] Deliver dedicated agent configuration sections with effective permission/isolation summaries and a whole-graph change review before save. — General, Instructions, Tools & permissions and Advanced share the graph draft; the summary explicitly describes configured boundaries rather than claiming runtime authorization. Review lists changed fields across the entire graph without copying values. Unit and real-client component tests cover inherited/explicit grants, isolation, routes, multiple-agent/graph changes, validation failures, dependency errors, revision conflicts and viewers. Disposable-backend Chromium verifies reviewed persistence, constrained and MCP grants, reload/back navigation, invalid-input recovery and narrow read-only access. All 207 frontend tests and 39 standard/hosted-CIMD browser scenarios pass; make test, make lint and make build pass. Existing canvas/Definition editing remains compatible; activation and developer usability validation remain separate.
- [x] Deliver Runs list/detail screens over existing execution records, highlighting blockers and distinguishing execution outcome from individual tool outcomes. — Search/filter the latest 100 executions; Result, Timeline, Tool calls, Messages and Diagnostics use existing authorized APIs. Details verify graph identity, correlate records by persisted IDs, label bounded history and current-workspace links, and reuse approval role gates. Unit/real-client integration tests cover filters, scope, independent tool outcomes, polling, missing records, failures and recovery. Chromium covers approval-to-completion, deep links/reload/back, viewer restrictions, graph mismatch and failed-history retry. All 219 frontend tests and 42 standard/hosted-CIMD E2E scenarios pass; the six Runs/Activity scenarios were repeated after final changes. make test, make lint and make build pass. Installation activation and developer usability validation remain separate.
- [x] Consolidate typography, spacing, button hierarchy, contextual help and accessible responsive components across the remaining screens. — Shared page/card/form styles cover Models, Administration, Approvals and Skills; canvas guidance is a native disclosure. Skills adopts shared action buttons with unchanged handlers and disabled fieldsets. Security notices and errors remain visible; focus and reduced-motion styles are explicit. Unit tests cover disclosure and disabled actions; real-client integration proves help does not invoke providers or expose viewer actions. Chromium covers keyboard disclosure, narrow layouts, viewer permissions and unchanged canvas editing. make test (229 frontend tests), make lint, make build and make test-e2e (45 standard plus one hosted-CIMD scenario) pass. A canvas-edge rendering failure after reload occurred in an earlier suite attempt; its isolated repeat and the final full suite passed without changing assertions. Preview was inspected at narrow and desktop widths. This slice was subsequently activated as commit 7ef1178 on 2026-10-17; see the activation evidence below. Developer usability acceptance remains pending.
- [ ] Validate navigation and the three redesigned journeys with developers; record evidence separately from proposed usability targets.
- [ ] Complete resumable onboarding after the underlying journeys stabilize; keep first-class tasks and delegation in their existing workstreams.


**Navigation baseline activation (2026-10-17):** Commit b0f7670 and preceding UX
slices were activated and checked in the local installation. forged was rebuilt
and recreated using the existing volumes; health was ok, all three graphs and
nine saved versions remained present, and the browser checks covered connections,
agent configuration, draft switching/review/discard, saved Runs, Models and system
health. Earlier slice notes describe evidence before that activation. Developer
usability acceptance remains pending; later visual changes are not yet activated.


**Visual consolidation activation (2026-10-17):** Commit 7ef1178 was built with
docker compose build forged and activated with docker compose up -d --no-deps
--force-recreate --wait forged. Health returned ok and the existing four forged
volume mounts were retained. The three graphs and nine saved versions remained
unchanged. Authenticated browser checks covered Models and its four providers,
keyboard-opened help, Skills guidance, Approvals, Administration health and canvas
guidance. Software Factory rendered its ten edges before and after reload;
Save graph remained disabled. No configuration, grant or credential edits were
submitted. Developer usability acceptance and resumable onboarding remain pending.


**Developer usability round (2026-10-17, in progress):** The first task asks the
developer to locate reader in MCP installation validation and identify its model
and grants without saving changes. Feedback is pending; MCP connection and
execution-tracking tasks have not started. Keep the usability checkbox open.
Record confirmed adjustments after developer observations; do not treat the
previous assistant-operated installation checks as developer acceptance.

**First-round feedback:** The developer described the Graphs screen as outdated and supplied a screenshot with the new navigation and graph selector. The served HTML references the activated visual build assets, matching the running container. Whether the concern is visual fidelity, graph data, or task instructions remains unconfirmed; clarification is pending. Do not classify this as a cache defect or accept the agent journey yet.


- [x] UX-01: Make graph cards clickable and agent discovery explicit. — Card
  surfaces open the graph through the accessible title button; action controls
  stay independent. Named Configure links open agent settings. Unit and browser
  tests cover navigation, keyboard activation, independent Start and retained
  drafts. Developer revalidation in the installation remains pending.


**Agent journey progress:** Developer screenshots confirm General and Instructions
were reached with assistance; ollama-local / llama3.2:latest is visible. Exact tool
grants are not yet demonstrated. An execution start is visible in Events, but its
outcome is not established. Continue with the existing run rather than starting
another. MCP connection journey and overall usability acceptance remain pending.

- [x] UX-02: Prevent long event types from overlapping actors. — Grid tracks
  and children allow wrapping; narrow layouts stack event details. Component and
  real-browser tests cover long names, separate actor bounds and expandable data
  at 1280 and 580 pixels. Developer revalidation remains pending.


**Developer round evidence update (19:28 local time):** The developer explicitly
confirmed that GitHub get_me and get_file_contents are selected in Tools &
permissions. Agent model and grant identification is complete with navigation
assistance; independent discoverability is not yet accepted. Subsequent screenshots
show the existing reader execution (graph version 7, started 19:23:57) as Completed,
with completion at 19:25:15 in Diagnostics. Tool calls shows one invoked/completed
pair each for get_me (2010 ms) and get_file_contents (1712 ms). Result, Timeline,
Tool calls, Messages and Diagnostics were reached. Messages reports no linked
messages in the recent window; this alone is not a failure. Screenshots establish
the displayed outcomes, not an inspection of raw MCP responses. The developer's
assessment of clarity and the MCP connection journey remain pending.

**UX-03 — tool-call count clarity:** Result says “4 tool calls recorded”, whereas
Tool calls displays four lifecycle events for two calls. Investigate the counting
source before implementation; distinguish actual calls from lifecycle events and
make summary/detail counts consistent. Preserve separate execution and tool
outcomes, including partial histories and failures. This is a screenshot-confirmed
presentation discrepancy; its implementation cause has not been established.

- [x] UX-03: Reconcile recorded tool attempts and lifecycle records. — Native
  runtimes no longer report gateway-counted calls a second time. Unit and engine
  integration tests cover counting, including failed attempts; browser coverage
  checks a real native/gateway execution and its summary. Runs distinguishes
  observed invocation events from lifecycle events and explains partial history
  and preserved historical totals.


**Confirmed finding UX-04 — connection validation versus agent execution:** The
developer explicitly reported losing context between Connections / Personal GitHub
and Graph, and had not understood that reader belongs to the dedicated MCP
installation validation graph. The guided round mixed connection setup validation
with an agent execution check without explaining their different purposes. The
existing connection Test and Refresh tools actions do not require creating an
agent. The dedicated validation agent was configured during the earlier assisted
gateway validation; it is not an automatic consequence of connecting an MCP.

Proposed adjustment: keep connection health/discovery checks and their results in
connection detail; explain separately that using tools requires explicit grants to
an agent. Make optional end-to-end agent checks clearly labeled, with the target
graph/agent and a return path to the connection. Do not create agents or grant tools
automatically. Retest whether the developer can distinguish connection readiness
from successful agent tool execution without facilitator explanations. This finding
also concerns the validation instructions, not only the product navigation.

- [x] UX-04: Clarify connection checks versus optional agent execution. —
  Connection detail explains discovery without agents and separate explicit
  grants. Agent-access links name the graph and open Tools & permissions, with
  a return banner retained across tabs and reloads. Unit/API-client integration
  and browser tests verify context, unchanged grants and no automatic execution.
  Developer usability acceptance remains pending.


- [x] UX-05: Automatically discover tools after saving in the console. — Enabled
  token-based and unauthenticated connections attempt discovery; pending OAuth
  waits for consent and authorized OAuth can refresh. Disabled connections stay
  inactive. Save and discovery states are distinct; saved connections survive
  discovery failures, with retry and explicit agent-grant guidance. Unit and
  API-client integration cover eligibility, empty catalogues, failures and no
  implicit grants; real-backend browser journeys cover automatic discovery,
  authentication failure, recovery, OAuth and disabled connections. API clients
  retain separate save and refresh endpoints.

**UX-01–UX-05 verification (2026-10-17):** make test passed (Go unit/integration,
Python and 241 frontend tests), make lint passed, make build passed, and
make test-e2e passed (48 standard Chromium scenarios plus one hosted CIMD case).
Desktop graph cards and narrow Events screenshots were inspected. Earlier targeted
runs exposed outdated test expectations for automatic discovery and missing test
setup; these were corrected with assertions retained. The existing sandbox journal
supervision test failed in the first full run and one isolated repeat, then passed
in a diagnostic run and the final full run without source or assertion changes.
Treat that intermittent test behavior as unresolved; it is not an established UX
regression or a claimed fix. No installation rebuild/restart, deployment, secret
change, data migration or commit was performed. Activation and another developer
usability round remain pending; the overall usability checkbox stays open.

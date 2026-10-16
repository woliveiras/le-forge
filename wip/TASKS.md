# LeForge Implementation Tasks

This checklist breaks down the [product plan](./PLAN.md) into implementable work. Tasks are grouped by dependency and may be reordered when implementation reveals additional technical dependencies.

Checked boxes represent only work that has been implemented and verified. Release 1 is still in progress; web-based provider configuration (section 5) and the visual graph editor (section 2) have been implemented and validated, except for browser-driven end-to-end tests (the project has no browser E2E tooling yet) and verification with the real LangGraph adapter. Containerized project environments are partially implemented: interface configuration, builds, logs, caches, and reset exist; repository discovery, approved proposals, prepared-image and immutable cache-seed reuse are implemented; shared execution/build/setup memory and concurrency budgets are implemented; idle suspension/resume and explicit cleanup of unused registered shared generations are implemented; historical unregistered resources are retained. MCPs, skills, steering, dynamic delegation, Git diffs, onboarding, and end-to-end validation remain pending.

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
- [ ] Add interaction tests for creating, editing, connecting, validating, saving, discarding, and recovering from conflicts. — Covered by component tests against the real API client (`GraphWorkspace.test.tsx`: create, edit, validate/revalidate, save, server 422, discard, conflict overwrite/reload, removal confirmation, YAML sync) and unit tests for the edit operations (`edit.test.ts`, including connect and keyboard removal). Keyboard removal was also checked manually in the browser (mock mode). Pending: browser E2E for drag-to-connect and Delete-key gestures.

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
- [ ] Extend the environment API/data contracts to record discovery candidates, selected source, repository revision, accepted definition, approval state and reuse identity; reconcile the existing per-graph lifecycle with shared images/caches before implementation.
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
- [ ] Add the `environment.status` tool and the `environment.propose` tool: the complete proposal opens a developer approval gate. Keep the task waiting until acceptance; rejection/cancellation must not build or run anything. Revalidate the approved revision, build after acceptance, and resume execution only when ready; expose build failures and retries.
- [ ] Provide an example proposal flow for a repository using `mise.toml` (base image with mise, `mise install`, dependency download).

#### Reuse Across Tasks and Resource Budgets

- [x] Define a repository/installation-scoped reuse key from the full image-definition digest, accepted context, bounded manifests/lockfiles, daemon platform/address and versioned policy. Setup output stays private; arbitrary setup reruns in each task. The fingerprint is bounded and is not a full Git revision.
- [x] Reuse compatible prepared images across repository graphs with renewable leases, force rebuilds and input invalidation. Setup runs per isolated workspace; shared images exclude setup container state. API/UI status and logs explain reuse. Unit, API integration and real sandbox tests cover the flow.
- [x] Reuse immutable dependency-cache seeds within the exact repository/installation scope. Copy seeds read-only into task-private writable volumes before setup; never publish runtime mutations. Cache declarations must exclude credentials and task outputs. Concurrent writers never share a volume across graphs.
- [ ] Create isolated task workspaces/worktrees and active containers; allow continuation of the same task to reuse its container without sharing mutable task state with other tasks.
- [x] Enforce finite memory/swap caps and a global count/reserved-memory budget for Docker execution sandboxes; persist reservations, inspect daemon inventory, wait/cancel admission and release only after confirmed removal. Preserve existing node CPU limits. Unit, configuration/app integration, concurrency and real cgroup tests verify the flow.
- [x] Extend global memory/concurrency admission to image builds, setup and cache-copy helpers. Use bounded temporary BuildKit workers, sequential setup/copies, cancellable waiting and durable reservations retained on cleanup failures.
- [x] Add global CPU accounting for execution/build/setup/cache-copy pipelines: finite total/default CPU, decimal parsing, atomic durable reservations, conservative legacy handling, kernel caps, cancellation and idle readmission. Unit, configuration/storage/component integration, concurrent-manager and real-daemon cgroup/CPU-only admission tests cover the flow.
- [x] Stop idle execution containers between turns with configurable `isolation.docker.idleTimeout` (5m default, 0s disables). Preserve container files, workspace and caches; reacquire capacity before new work, keep paused tasks paused, retry failures without losing inputs, and document restart versus explicit teardown behavior.
- [x] Preserve shared images/cache seeds on Reset and graph deletion; graph status retains resource references, and shared generations are conservatively retained.
- [x] Implement explicit admin preview/cleanup of registered shared generations and newly tracked crash leftovers. Track exact generations across rebuilds; revalidate status/definition/build/container references under reuse leases; preserve stopped containers, other tasks and private workspaces; invalidate reuse before partial deletion and support retry. Legacy unregistered resources remain untouched. Unit, storage/API integration and real daemon tests cover the flow.
- [x] Add Integrations UI for shared generation previews, retention reasons, selection (up to 100) and explicit confirmation; display partial/missing/retained outcomes and refresh after attempts. Cover policy units, component/API-client integration, and real sandbox cleanup validation.
- [ ] Record reuse, rebuild reasons, queue waits and resource-limit failures through existing events/metrics, using bounded labels and no sensitive payloads.

#### Network Policy

- [x] Enforce installation `environmentPolicy` on preparation/build RUN and runtime networks, manual/repository/agent definitions, approvals, cached images and node overrides. Add feature restrictions for proposals/Dockerfiles/setup/companions. Unit, storage/API integration and real sandbox command-egress tests cover the delivered subset; daemon registry egress remains external.
- [x] Add an opt-in runtime HTTP/HTTPS egress proxy with exact per-environment host lists bounded by the installation list, public-IP validation/pinned dials, isolated internal networks, bounded denial logs and aggregate capacity/lifecycle. Setup/build proxying remains pending.
- [x] Test that runtime egress cannot bypass the proxy, allowed HTTP/HTTPS hosts work, forbidden/private destinations fail, and suspension/resume/proxy failures remain isolated. Unit, API/storage/UI integration and real dedicated-daemon tests cover the subset.

#### Interface

- [x] Show the environment of a graph: source, effective definition, state, last build, logs, and cache usage. (Cache paths and volumes are listed; volume sizes are not shown yet.)
- [x] Add Rebuild and Reset actions with confirmation, restricted to operators/admins according to the role matrix.
- [x] Show discovery candidates, selected source and definition acceptance; show pending agent proposals in Approvals with rationale and full definition. Expose waiting, rejected, cancelled and build-failed states without implying execution succeeded. Proposals currently require no existing environment; diff/replacement workflows remain pending.
- [x] Show reuse decisions, build reason categories and structured build phases/capacity waits in settings and the agent panel; show recent execution admission history and shared-resource effects before Reset. Exact file diffs and durable queue positions/ETAs remain pending.
- [x] Show the graph environment name, image, readiness, failure and start blocker on the agent panel, respecting inherited isolation and explicit image overrides; report unknown readiness on fetch errors. Unit, component/client and storage/API integration tests plus real daemon progress/reuse validation cover this flow.

#### Tests and Documentation

- [x] Unit tests for definition parsing, precedence, hashing, policy enforcement on untrusted definitions, and lifecycle transitions, including build failures and cancellations. — Done for validation, hashing, the user layer, lifecycle, failures, cancellation, concurrency, interrupted builds and reset. Precedence and repository/agent-source validation are now covered; installation-wide policy, cached-image revocation and persisted approval revalidation are now covered.
- [ ] Integration tests against a real dedicated daemon: build from each source, run `shell.exec` in the environment, reset, and rebuild; verify no container or image is created on the host daemon. — API integration tests run with a scripted Docker CLI; a manual run against the real `sandbox-daemon` verified Dockerfile build, setup, commit, cache population, uid 10001 workspace writes, `network none`, reset (refused while running) and cleanup on graph deletion. Pending: automating that run and the repository/agent sources.
- [ ] Interface tests for configuration, state display, proposal approval, Rebuild, and Reset. — Done except proposal approval (not implemented yet).
- [ ] Add unit tests for discovery priority, ambiguous candidates, malformed definitions, supported Compose/devcontainer fields, context containment and absent-definition fallback; include approval rejection, stale approval, timeout and analysis failure.
- [x] Add unit, storage/API integration and real sandbox tests for reuse keys, input invalidation, build deduplication across manager instances, lease-wait cancellation, private cache copies, failed setup/copy, workspace isolation and Reset retention.
- [x] Test execution-sandbox budgets, multi-controller admission, memory/count boundaries, queue cancellation, cleanup failures and restart accounting, including a real dedicated daemon.
- [x] Test build resource budgets with unit, storage/API integration and real dedicated-daemon checks, including selected contexts, failure, cleanup/recovery and competition with execution admission.
- [x] Test idle/resume lifecycle: unit eligibility/failure checks, engine/storage/queue integration for active turns, schedules, paused tasks and input retry, plus real daemon stop/start, preserved files/container identity and capacity release/readmission.
- [ ] Add integration tests with repository fixtures and the real dedicated daemon for each source and the approved-proposal flow; prove no build/task starts before approval and failures remain visible.
- [ ] Add integration tests proving compatible tasks reuse an image/cache, changed lockfiles invalidate reuse, concurrent builds are deduplicated, task workspaces and unauthorized repositories remain isolated, and reset cannot remove referenced resources. Verify queue admission, resource release and idle-container resume.
- [ ] Add interface tests for candidate selection, developer approval/rejection, reuse/invalidation explanations and capacity waiting; add browser E2E for a repository without LeForge files through discovery or proposal, approval, build, execution and a subsequent reused environment.
- [ ] Document discovery priority, supported file formats, ambiguity/error handling, approval, reuse/invalidation, resource budgets, shared-resource reset and troubleshooting in the environment, deployment and getting-started guides.

**Done when:** A repository needs no LeForge-specific file: discovery follows the agreed precedence, and absence of definitions leads to agent analysis with developer approval before building or executing. Compatible subsequent tasks reuse images/caches with isolated workspaces and bounded active resources. Rebuild, reset and cleanup respect shared references; all project commands stay inside LeForge containers, nothing is installed on the host, and agents never access Docker.

### 4. Create the File and Interaction Panel

#### Workspace API and Security

- [ ] Define file listing, reading, diff, and history endpoints limited to the graph's authorized workspace.
- [ ] Apply path containment, symlink validation, size limits, and traversal protection to every operation.
- [x] Expose read operations to users; do not implement user-invocable write, delete, or upload endpoints.
- [ ] Enforce through backend authorization that only authorized agents can modify the workspace.
- [ ] Provide Git, branch, worktree, agent/replica, and execution metadata associated with changes when available.
- [ ] Record file changes and operations in a queryable form associated with the execution that produced them.

#### Operational Interface

- [x] Add a file browser with a tree, loading and error states, and safe navigation.
- [x] Add a read-only file viewer with a server-enforced content limit.
- [ ] Add read-only Git status and diff views.
- [ ] Show the directory, branch, replica, and execution associated with the workspace/change.
- [ ] Allow users to start a task for a graph or agent from the interface.
- [ ] Allow users to send instructions/commands to an agent using existing operator mechanisms, without enabling direct file editing.
- [ ] Present task state, progress, events, messages, approvals, and results in an accessible flow.
- [ ] Link changed files, diffs, messages, and executions; clearly indicate when attribution is inconclusive.
- [ ] Add authorization tests ensuring the interface cannot modify, delete, or upload files to the workspace.
- [ ] Add end-to-end tests for starting a task on the web, monitoring execution, and inspecting resulting files/diffs.

**Done when:** Users can operate tasks in the browser and inspect results, but no user action in the API or interface directly changes workspace files.

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
- [ ] Verify and document the full path from the LangGraph runtime to the selected provider. — Verified at the protocol level (fake external runtime → `llm/chat` callback → managed provider, with no key in `TurnInput`). Pending: execution with the Python SDK's real LangGraph adapter and documentation of that path.
- [x] Add tests for encrypted persistence, secret update/removal, connection testing, and per-agent selection.
- [ ] Add an end-to-end test from interface configuration to an execution that uses the selected provider. — Covered by an API integration test (configure → test → select on node → execute), component interaction tests, and a clean-install HTTP smoke test. Pending: browser-driven test (the project has no browser E2E tooling).

**Done when:** An operator creates and tests a provider on the web, selects it for an agent, and runs the agent without exposing the secret to the runtime.

### 6. Integrate User-Configurable MCPs

#### Connectivity and Integrations

- [ ] Confirm contracts, authentication methods, scopes, and supported transports for Bitbucket, GitHub, Confluence, and Jira.
- [ ] Choose an initial transport compatible with Docker and implement connections without running arbitrary commands in the main process.
- [ ] Design a generic MCP connection registry, with no artificial connection limit and support for multiple connections per service.
- [ ] Implement the connection lifecycle: create, edit, test, disable, re-enable, revoke, and delete.
- [ ] Implement discovery and refresh of tools offered by each connection.
- [ ] Implement timeouts, cancellation, payload limits, and explicit handling for disconnection, errors, and invalid responses.
- [ ] Implement reconnection/session renewal as required by the selected transport.
- [ ] Add contract tests for connection, discovery, invocation, failures, and timeouts.

#### Credentials, Authorization, and Auditing

- [ ] Encrypt tokens/secrets at rest and ensure they never appear in read responses, logs, prompts, or events.
- [ ] Implement assignment of connections and allowed tools to specific agents.
- [ ] Enforce authorization at the gateway for every invocation, regardless of what the agent/model requests.
- [ ] Add per-connection/agent rate limits, timeouts, and response-size limits.
- [ ] Allow human approval to be configured for selected tools.
- [ ] Audit invocations with agent, connection, tool, result, duration, and error, removing or masking sensitive data.
- [ ] Ensure an agent cannot select another connection, escape its grants, or read credentials.
- [ ] Test effective revocation: after access is revoked, the next invocation must be denied.

#### Interface

- [ ] Add a screen for registering multiple Bitbucket, GitHub, Confluence, and Jira connections.
- [ ] Guide operators through authentication and explain required scopes and granted access.
- [ ] Show connection status, last validation, discovered tools, and errors without revealing secrets.
- [ ] Allow connections to be enabled/disabled and discovered tools to be refreshed.
- [ ] Allow available connections/tools to be selected for each agent.
- [ ] Show a filterable history of calls, approvals, results, and errors.
- [ ] Add interface tests for connection setup, validation, agent authorization, and revocation.
- [ ] Add end-to-end tests for each priority integration using appropriate test credentials.

**Done when:** Connections for each priority integration can be created through the interface, tested, assigned to agents, and audited; unauthorized calls are denied.

### 7. Add Skills and Steering

#### Content Model and Loading

- [ ] Define formats and storage for global instructions and application-managed skills.
- [ ] Define recognized formats and paths for project-specific instructions in the repository.
- [ ] Implement explicit association of a workspace/repository with authorized graphs.
- [ ] Implement deterministic precedence among global, graph, agent, project, and task instructions.
- [ ] Define explicit behavior for conflicts, invalid/duplicate files, size limits, and missing instructions.
- [ ] Load project instructions only from the authorized workspace and record their source, version/hash, and scope.
- [ ] Treat instructions and skills as untrusted content; they cannot change grants, credentials, policies, or limits.
- [ ] Enforce size/token limits and inform operators when content is omitted or truncated.

#### Interface, Auditing, and Tests

- [ ] Add an interface for creating, editing, versioning, enabling/disabling, and deleting global instructions/skills.
- [ ] Add an interface for associating instruction sets with graphs and agents.
- [ ] Show recognized repository instructions, their source, and whether they were accepted for use.
- [ ] Record and show the effective set of instructions loaded for each execution.
- [ ] Add tests for precedence, workspace isolation, malformed instructions, size limits, and privilege non-escalation.
- [ ] Add interface tests for administration and inspection of loaded content.

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

# LeForge Implementation Tasks

This checklist breaks down the [product plan](./PLAN.md) into implementable work. Tasks are grouped by dependency and may be reordered when implementation reveals additional technical dependencies.

Checked boxes represent only work that has been implemented and verified. Release 1 is still in progress; web-based provider configuration (section 4) and the visual graph editor (section 2) have been implemented and validated, except for browser-driven end-to-end tests (the project has no browser E2E tooling yet) and verification with the real LangGraph adapter. MCPs, skills, steering, dynamic delegation, Git diffs, onboarding, and end-to-end validation remain pending.

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
- [ ] Add interaction tests for creating, editing, connecting, validating, saving, discarding, and recovering from conflicts. — Covered by component tests against the real API client (`GraphWorkspace.test.tsx`: create, edit, validate/revalidate, save, server 422, discard, conflict overwrite/reload, removal confirmation, YAML sync) and unit tests for the edit operations (`edit.test.ts`, including connect and keyboard removal). Keyboard removal was also checked manually in the browser (mock mode). Pending: browser E2E for drag-to-connect and Delete-key gestures.

**Done when:** A user can create, configure, validate, save, and start a graph through the visual interface without writing YAML.

### 3. Create the File and Interaction Panel

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

### 4. Configure Model Providers Through the Interface

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
- [x] Validate configuration before starting an execution and explain missing or invalid settings. (Warning on save; `422` with the field path when starting a graph/node.)
- [ ] Verify and document the full path from the LangGraph runtime to the selected provider. — Verified at the protocol level (fake external runtime → `llm/chat` callback → managed provider, with no key in `TurnInput`). Pending: execution with the Python SDK's real LangGraph adapter and documentation of that path.
- [x] Add tests for encrypted persistence, secret update/removal, connection testing, and per-agent selection.
- [ ] Add an end-to-end test from interface configuration to an execution that uses the selected provider. — Covered by an API integration test (configure → test → select on node → execute), component interaction tests, and a clean-install HTTP smoke test. Pending: browser-driven test (the project has no browser E2E tooling).

**Done when:** An operator creates and tests a provider on the web, selects it for an agent, and runs the agent without exposing the secret to the runtime.

### 5. Integrate User-Configurable MCPs

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

### 6. Add Skills and Steering

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

### 7. Allow Dynamic Delegation to Subagents

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

### 8. Make the Docker Installation Operable Through the Interface

- [ ] Review `docker-compose` for data persistence, secrets, internal networks, and restart behavior of required services.
- [ ] Ensure MCP services, providers, storage, and runtimes are reachable as configured without exposing unnecessary ports.
- [ ] Define secure initial configuration for credentials, authentication, and first-operator creation.
- [ ] Add health checks and show dependency status in the console.
- [ ] Document installation, upgrades, backup, restore, key rotation, and failure recovery.
- [ ] Ensure users can perform initial setup, connect tools, configure models, build a graph, and operate tasks without shell access, manual server-file editing, or CLI usage.
- [ ] Run a clean-install test following only the documentation and interacting through the browser after starting the containers.

**Done when:** A new installation can be configured and operated in the browser; infrastructure commands are limited to initial deployment and documented administrative maintenance.

### 9. Guide First Use with Onboarding

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

### 10. Integrated Validation of Release 1

- [ ] Create an end-to-end test flow: installation → login → provider → MCP connection → permissions → visual editor → task → execution → files/diff → approval, when enabled.
- [ ] Repeat the flow with both approval-required and automatic delegation policies.
- [ ] Validate onboarding on a clean installation, including steps that depend on real test MCPs and providers.
- [ ] Verify that no interface path allows users to directly edit workspace files.
- [ ] Verify that no secret reaches the browser after saving, the runtime, a prompt, or logs.
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

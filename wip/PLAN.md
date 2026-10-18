# LeForge Evolution Plan

**Goal:** Make the web interface the place where users configure models and integrations, build graphs, and monitor agents at work. LangGraph remains supported; reusable components are deferred to the second release.

## Delivery Order and Final Acceptance (2026-10-18)

Developer usability acceptance is deferred by user decision until the remaining
Release 1 implementation is ready. It is not a prerequisite for the next delivery
and must remain unchecked until performed. The UX reorganization and UX-01–UX-05 are implemented and activated. The global
onboarding panel was removed from the application on 2026-10-18 at the user's
request after reported layout breakage. The removal was activated and browser-validated on 2026-10-18;
earlier onboarding evidence below is historical, not current acceptance.

Continue unit, integration and relevant browser regression tests with each change.
Defer the consolidated developer walkthrough and clean-install acceptance exercise
to the final Release 1 validation stage. Live vendor qualification remains separately
pending where credentials, corporate approval or authorized test resources are
required; postponement does not qualify a provider or authorize external changes.

The native analysis delegation foundation and installation-managed profiles,
approval policy, finite limits and execution-tree controls are implemented (section 8).
Next: broader runtime/profile support and enforceable monetary ceilings. Cumulative token admission is implemented below. Preserve explicit delegation grants
and the empty child permission set. Final developer acceptance remains deferred;
installation activation of the execution-tree controls was validated on 2026-10-18 (section 8).

Remaining Release 1 implementation areas also include vendor-specific MCP
capabilities and multi-instance OAuth transaction persistence, installation/setup
and operations gaps, and the unresolved limits of workspace change attribution.
Reconcile partially delivered checklist entries against code before implementation;
an unchecked umbrella item does not mean all of its subfeatures are absent.
Rootless sandbox support stays deferred by the earlier user decision. Items marked
Later and Release 2 reusable components do not block Release 1 UX completion.
Do not add a new first-class task resource merely to close this UI checklist:
current task operation uses existing graph/agent execution and message controls.

## Product Principles

- **The web interface is the primary path:** Normal workflows must not require server access or CLI usage.
- **Configurable models:** Use the provider and credentials configured by the user, without coupling the platform to a vendor.
- **Bounded autonomy:** Agents may delegate work, but they cannot expand their own permissions or consume unlimited resources.
- **Security independent of instructions:** Skills and steering guide agents but do not replace platform-enforced permissions.
- **Genuine visual construction:** The canvas must allow users to edit graphs, not just view those already defined.
- **Containers are the primary managed workflow:** Project environments run on LeForge's dedicated daemon. Native `process` execution remains supported as an explicit trust choice with the platform user's OS rights; it does not gain container isolation. Deployment location and execution isolation are separate choices (see the README). Resource cleanup must preserve persistent data and respect ownership and approval policy.
- **Users operate; they do not edit files:** Users send commands/instructions through the interface; only agents may modify workspace files. Users may inspect files and diffs.

## Release 1 — Build and Operate Graphs Through the Interface

### 1. Remove Out-of-Scope Product Functionality

Remove compatibility features and references that are not part of the planned product:

- converter and import endpoint;
- conversion button, examples, and related tests;
- remaining documentation, references, and dependencies for that compatibility.

**Acceptance criterion:** The repository and application contain no compatibility flows, references, or documentation for out-of-scope external products.

### 2. Complete Model Configuration

Preserve support for LangGraph and the existing model provider system. Make provider configuration available in the interface so users can:

- add and edit providers;
- store credentials securely without displaying them again after saving;
- choose the provider and model for each agent;
- validate the connection and show actionable errors.

**Acceptance criterion:** Users configure credentials through the interface and run an agent using the selected provider, without configuring secrets directly on the server during normal use.

**Verified implementation:** Browser configuration and the Python SDK’s real LangGraph adapter are connected through the managed gateway. Tests cover model override, failures and credential isolation using an artificial-key deterministic model endpoint; live vendor inference quality remains outside this evidence. See [validation guide](../docs/provider-langgraph-validation.md).

### 3. Turn the Canvas into a Graph Editor

Evolve the existing `GraphCanvas.tsx` view to allow users to:

- create, move, configure, and remove agents;
- create, edit, and remove channels connecting agents;
- configure agent and channel properties in panels;
- validate the graph before saving;
- identify unsaved changes, errors, and version conflicts;
- keep the YAML editor as an advanced option.

**Acceptance criterion:** Users can create, configure, save, and start a complete graph in the browser without editing YAML or accessing the machine running the service.

**Verified implementation:** Component/API tests cover editing, validation, discard and version conflicts; Chromium now covers real connection dragging, Delete cancellation/confirmation and persisted agent/channel edits.

### 4. Provide Containerized Project Environments

Agents need the project's real toolchain (for example Go, Node, and pnpm) to install dependencies, build, and run tests. The supported sandbox scope is implemented: dedicated daemon, repository discovery, approved initial proposals, compatible reuse, capacity, idle resume and reference-safe cleanup. No LeForge-specific file is required. See [acceptance evidence and explicit limits](../docs/sandbox-acceptance.md); this does not complete the other Release 1 workstreams.

Give each graph a managed **environment**:

- run every environment build and sandbox container on a Docker daemon dedicated to LeForge (a Docker-in-Docker service in the Compose stack, rootless where the host supports it) instead of the host daemon; images, containers, and caches live in that service's volumes, so removing them resets everything and nothing is installed on the host;
- keep sandboxes on a network separate from platform services (database, broker, control plane); only the control plane talks to both;
- preserve an explicit environment configured by the operator in the interface; otherwise discover repository definitions in this order:
  1. `.leforge/environment.yaml` (highest repository priority, optional);
  2. `.devcontainer/devcontainer.json`;
  3. Dockerfiles;
  4. Docker Compose files (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`, and explicitly selected variants);
- show the discovered source and effective definition for operator acceptance. When multiple Dockerfiles, build targets, or Compose services are candidates, ask the operator to select the development environment; do not start every service automatically. Report invalid or unsupported definitions rather than silently falling through to a lower-priority source;
- when no supported definition file is found, use an agent with bounded, read-only repository access to analyze manifests, lockfiles and toolchain declarations such as `mise.toml`, `go.mod`, and `package.json`. This analysis must work before a project environment exists. Propose the image, tools, setup commands, caches, network needs and rationale; a developer approval gate blocks building and task execution until the proposal is accepted. Rejection keeps the task blocked; approval permits building, and execution continues only once the environment is ready;
- build images and run setup steps in LeForge, never through agent access to Docker; agents get `environment.propose` and `environment.status`, and keep running commands only through `shell.exec` inside the ready environment;
- apply a network policy per phase: setup may reach the internet to download dependencies; execution defaults to no network, with an optional host allowlist enforced by an egress proxy;
- reuse prepared images and compatible dependency caches across tasks for the same authorized repository and effective environment. The delivered implementation shares prepared base/user layers and immutable cache seeds, copies seeds into private writable caches, and reruns setup in each isolated graph workspace; Identify compatibility from the definition, relevant setup inputs (including lockfiles), platform and policy; reuse unchanged environments and rebuild when those inputs change or the operator requests it;
- keep task workspaces/worktrees and active execution containers isolated. Share only caches that support concurrent use, otherwise serialize access or isolate them. Never share task outputs, credentials or private repository data across unauthorized scopes;
- bound memory and CPU per sandbox and enforce a global concurrency/resource budget, queueing tasks when capacity is exhausted. The delivered budget reserves memory and slots across execution sandboxes and environment pipelines, with persistent accounting and cancellation. BuildKit workers, setup and cache copies run sequentially under one reservation; global CPU accounting now shares those atomic durable reservations and applies kernel caps to execution, BuildKit, setup and cache copies. Delivered idle suspension stops execution containers between turns after a configurable timeout (5m by default, 0s disables), preserves their files/workspaces/caches, and readmits the same container before new work. Active turns are excluded; process memory is not checkpointed. Image/cache reuse saves build time and disk, while active-container limits control memory consumption;
- expose environment state (`absent`, `building`, `ready`, `failed`), structured build phases and reason categories (delivered in settings and the agent panel), approval and capacity waiting states, reuse decisions, build logs, **Rebuild**, and **Reset** in the console. Reset and graph deletion must respect shared-resource ownership and active references: never remove images or caches another task still needs; delivered Integrations UI previews exact resources, requires selection/confirmation, and reports partial outcomes; admin preview/cleanup reclaims unused registered generations after rechecking environment/build/container references under reuse leases. Keep historical resources without ownership records; never force removal or delete workspaces/private caches.

Environment definitions coming from repositories or agents are untrusted content: they cannot change grants, credentials, limits, or network policy beyond what the installation allows.

**Acceptance criterion:** An operator connects a repository without requiring a LeForge-specific file, accepts a discovered definition or approves an agent proposal, and runs tasks only after the environment is ready. Later tasks reuse compatible images and caches while keeping workspaces isolated, with bounded active resource consumption. Rebuild and reset preserve resources referenced by other tasks. Nothing is installed on the host and agents never receive Docker access.

### 5. Create the File and Interaction Panel

Add a per-graph workspace to the console, including:

- a file tree and content viewer;
- Git diff, branch, directory, and responsible replica views;
- links between files, messages, and executions;
- an interface for sending tasks/commands to agents and tracking progress;
- a history of agent-made changes, with review and approval according to the configured policy.

Users may read and inspect files, but cannot edit, delete, or upload them through the console. Only authorized agents may modify the workspace; every operation must respect workspace boundaries and permissions without exposing the server's filesystem.

**Acceptance criterion:** Users can start and direct tasks, monitor agents, and inspect changes entirely through the browser, without server access or CLI usage.

**Delivered inspection:** Files now exposes read-only Git status, tracked diff against HEAD and bounded history, plus branch/directory/replica metadata and recent execution/tool-operation context. Real repository/API tests cover confinement and unsafe Git configuration; browser coverage starts a task and inspects its result. Native agents can explicitly opt into `workspace.readOnly`, enforced before tool execution regardless of grants; Docker also mounts their workspace read-only. The event log and Files panel include bounded before/after workspace observations for filesystem, Git-write and shell calls, associated with agent and execution, including failed calls. A controller-lifetime monitor also scans authorized shared workspaces and known replica worktrees after tasks stop, recording net changes from external/background processes and concurrent writers without inventing an execution or author. Baseline/restart, discovery failures and incomplete scans expose coverage gaps. The dedicated Linux daemon additionally supports a fanotify collector with a private durable journal: kernel PID evidence, create/delete events for transient files, concurrent writers, and idempotent replay after controller downtime. PID namespace and boot identity are recorded; process start time is included only when verified through a live pidfd. No agent/execution identity is inferred from a PID. Collector outages, kernel overflow, unresolved paths and unsupported runtimes remain explicit gaps; the collector must remain active during controller downtime. See the kernel-journal contract in [tool permissions](../docs/tool-permissions.md#kernel-journal-in-the-dedicated-linux-daemon). See [tool permissions](../docs/tool-permissions.md#workspace-write-access-and-observations) for the authorization boundary and capture limits. Durable candidate review is delivered independently in section 5a.

**Delivered task activity:** The graph's **Activity** tab combines current graph/replica state and wait reasons with recent executions, summaries/errors, messages, approvals, human-review state and events. Execution selection uses persisted execution/message IDs, never agent-name guesses; reviews and replica progress remain explicitly graph-wide. Existing approval endpoints enforce permissions; the view offers decisions to operators/admins and environment decisions only to admins. File/diff inspection and agent controls remain linked from the flow. Independent API failures are visible and retryable; a five-second poll and event-triggered refresh recover missed updates. Windows are bounded to 100 executions, 100 messages and 300 events; this is not a new task entity, a full historical query, or a percentage-complete estimate. Unit/component integration tests and real-controller Chromium journeys cover correlation, errors, permissions, approval/rejection, results, messages and file inspection.

### 5a. Human Review with Managed or External Previews

**Status: implemented for Docker sandboxes.** LeForge ships its own local-first
preview gateway, supervised HTTP processes, durable review gates and console
review actions. Browser and cURL acceptance use real containers on the dedicated
daemon. Setup and limits are in `docs/sandbox-runtime.md`; API/tool/runtime
contracts are documented alongside the existing contracts. Native process
supervision and automatic provider-specific PR/CI adapters remain later work.

The implementation uses a scoped Docker exec stdio relay to a registered
container/port, without opening sandbox ports or changing egress policy. Review
records hold a SHA-256 workspace snapshot, generation, reviewer/method policy,
decision timestamp and durable continuation outbox. Runtime cache copies are
private but are not included in the source digest. Snapshots and stopped
containers are retained; offline-controller cleanup waits for reconciliation.
Private tunnels are administrator-configured, never an external-service
requirement for local operation.

Make human review a durable workflow step tied to the exact proposed change.
The agent prepares the change, runs required checks, supplies review instructions,
and requests review. The platform persists the request and yields execution;
no model call, tool invocation or input-message lease stays open for the human's
entire review. Approval queues an idempotent continuation through insert-once delivery.
Retried business operations still require their own idempotency controls.
Reuse approval storage/decision conventions, but add a durable continuation:
`human.ask` alone is not a lifecycle manager for this workflow.

#### Review choices and evidence

- Managed preview: LeForge runs the candidate and supplies a protected URL for
  browser testing or an HTTP base URL for cURL/Postman.
- External preview: attach a CI-provided environment URL and its build/revision
  evidence. LeForge does not imply that it hosts or controls that environment.
- Local branch review: show repository, branch and commit so the human can
  fetch and test it independently, then record the decision in LeForge.
- Review without manual execution: explicitly record that the human accepted
  based on the diff, checks or other evidence. Do not label this as a successful
  manual test. Operator policy can require a specific review method; the agent
  cannot waive it. The default product flow permits explicit acceptance without
  a preview when policy allows it.

A PR on GitHub, GitLab, Bitbucket or another provider is an optional handoff and
evidence source, not a prerequisite for review. PR creation may trigger an
external CI preview. Creating a PR, passing CI, approving a review, merging and
deploying are distinct actions with separate authorization. A provider event
may satisfy a gate only under an explicitly configured policy and after actor,
repository and exact revision verification. Start with manually attached
external links; provider-specific automation is a later increment.

Each review records graph, task/execution, requester, authorized reviewers,
candidate identity (commit plus any uncommitted-content snapshot), checks,
instructions, preview generation or external evidence, expiry, decision,
review method, actor and timestamp. Freeze the candidate used by a preview;
editing the working branch must not silently change what the reviewer sees.
New candidate content supersedes the prior review and invalidates its approval
for the new candidate. The continuation verifies the approved identity again.

#### Managed application sessions

Introduce a platform-managed service lifecycle separate from `shell.exec`.
An authorized agent requests a command/argv, workspace-relative directory,
HTTP port, readiness path and bounded lifetime under operator policy. These
are proposed API/tool inputs, not existing supported configuration. The platform
validates them and starts a supervised process using the project's prepared
environment and frozen candidate, with required approved helper services.

The first increment targets Docker environments on local or remote installations.
Native execution can use the same review contract, but a native supervisor and
local binding policy are a later backend; it must clearly report the weaker
isolation. Neither implementation may turn arbitrary user-supplied URLs, host
paths or network addresses into forwarding targets.

Track session states independently from review decisions: queued, starting,
ready, failed, expired and stopped. Expose readiness failures and bounded logs;
a process starting successfully is not proof that HTTP is ready. Unexpected
exit removes routing immediately and leaves the review pending or unavailable,
never approved. A restart creates a new session generation for the same frozen
candidate and requires readiness again.

Account for preview CPU, memory, slots and helper services in the installation's
shared capacity budget. While a preview lease is active, ordinary sandbox idle
suspension must not stop it. Leases have a maximum lifetime and bounded renewal;
expiry releases compute and revokes access while preserving the candidate and
review record. Users can restart an expired preview after capacity admission.
Cancellation, supersession and review completion revoke routing and stop owned
processes. Retain workspace and persistent test data unless separately authorized
for deletion. Cleanup failures remain visible and retain resource accounting.
Controller restarts reconcile leases, sessions and pending continuations.

#### Access from browsers and HTTP clients

The preview gateway is a component shipped and managed as part of LeForge,
not an external service. It works locally by default: browser, gateway and
sandbox can all run on the user's machine, without a cloud account, third-party
tunnel, public domain or external preview service. Bind its host-facing listener
to loopback only by default, including Compose port publication. Provide local
preview addressing without requiring public DNS; test supported browsers and
HTTP clients. This does not remove network requirements of the application,
dependency downloads or the selected model provider.

Use authentication and a separate preview origin from the console. Loopback-only
HTTP is supported for the default local workflow; require TLS for remote access
unless HTTP is carried entirely through an explicitly configured secure private
tunnel. Keep local authentication enabled and use appropriate cookie settings
for each transport. Prefer a dedicated host per session so applications using
absolute paths, redirects, cookies and WebSockets do not need a URL prefix.
The URL routes only to a registered session's approved HTTP port. A scoped
Docker exec stdio relay connects the registered container loopback port to the
gateway without publishing application ports or opening general network access.
Inbound review access and outbound application egress are separate policies;
the existing egress proxy does not provide this ingress feature.

Browser access uses an authenticated exchange into a session-scoped cookie on
the preview origin. Never forward the console cookie or platform authorization
headers to the application. cURL/Postman use a short-lived, revocable credential
scoped to one preview, issued explicitly to the human through a secure UI flow;
the gateway consumes it in a dedicated header so application `Authorization`
headers remain usable. Do not put credentials in URLs, prompts, exported
collections, logs or events. Application authentication remains independent.

Enforce review/session authorization on every HTTP request and WebSocket
handshake; bound and terminate existing connections on revocation or expiry.
Define CSRF protection for browser cookie access, exact CORS origins, request
size/time limits and WebSocket limits. HTTP methods needed for API testing may
mutate isolated test data; they must not expose production credentials or data.
Provide copyable base URLs and credential-free cURL/Postman templates, plus
test-account instructions without embedding secret values.

For remote hosting, the same LeForge gateway must be reachable through an administrator-
configured domain/TLS route or private tunnel. The agent cannot create public
exposure itself. A local browser must receive a reachable gateway URL, never
the sandbox's internal address or the remote server's `localhost`.

#### Gate outcomes and delivery

The console presents the candidate, check results, instructions, review options,
preview state and URL. Actions are approve, request changes, reject/cancel, and
restart/extend preview within policy. Request changes returns feedback to the
agent and requires a new review for the next candidate. Decision and continuation
creation must be atomic or use a transactional outbox; duplicate decisions and
replayed deliveries must not duplicate continuation messages; downstream effects
retain their own idempotency requirements. Review expiration
never auto-approves this gate; preview expiry alone is not review rejection.

Record session lifecycle, readiness, access denials, review decisions and
continuation outcomes with correlation IDs, bounded metrics and traces. Exclude
credentials and request/response bodies from access logs. Distinguish technical
readiness, automated test success and a human's explicit approval in the UI.

Deliver in order: durable review/candidate contract with manual external/local
evidence; Docker service supervisor and resource leases; scoped HTTP gateway
and browser/client authentication; integrated console flow and recovery tests;
then native supervision and automated SCM/CI adapters. End-to-end acceptance
requires the gateway and UI increments, not only a stored URL.

**Acceptance criterion:** On a local installation, the bundled gateway exposes
browser and cURL/Postman previews through loopback without public DNS, cloud
services or third-party tunnels; other machines cannot reach that listener.
On a remotely hosted LeForge installation, a React
candidate in an isolated Docker environment is accessible in the reviewer's
browser, and an HTTP backend is testable using cURL/Postman. The graph stays
pending until an authorized decision, survives controller restart, rejects
stale approval, and releases access/compute at the end of the session. Tests
demonstrate isolation from other sessions and platform services. External CI,
local branch review and explicit acceptance without manual execution remain
available according to policy, without requiring a managed preview or a PR.

### 6. Add MCP Configuration

**Connection quotas and history:** Configurable `callsPerMinute` (0/unset is
unlimited, maximum 60,000) uses persisted CAS admission in fixed UTC-minute
windows, shared across agents and controllers using the same storage. It is
charged after approval and validation, before dispatch; admitted failures count,
refusals do not. Restart and limit edits do not reset stored usage. This is a
rate quota, not an in-flight concurrency or billing limit. Existing timeouts and
payload limits remain. The console offers a viewer-readable, newest-first MCP
history filtered by connection, graph, agent, tool, type, outcome and time, with
exclusive sequence cursors and bounded scans. Tool/approval events are projected
to safe summaries without raw arguments/results/comments. Deleted connection
history remains available. Automated coverage uses disposable stores and TLS
fixtures. `make test` (179 frontend tests), lint/build, 32 Chromium E2E cases
and targeted Go race checks passed. This increment does not deploy or modify
live connections.

**Installed GitHub acceptance (2026-10-17):** A dedicated manually registered
GitHub OAuth App completed browser consent, backend callback and encrypted
connection persistence. Discovery succeeded. An agent with only `get_me` and
`get_file_contents` grants executed both through the gateway, first with a
controlled model and then with local Ollama `llama3.2:latest` (4,096-token
context). The real-model run read the authenticated profile and public
`octocat/Hello-World` README, completed successfully and was stopped afterward.
No repository writes were performed. Private repository/issue/PR permissions,
expiry/renewal, revocation and organization/SSO restrictions remain unqualified
against the live GitHub service. Atlassian's historical uncertain DCR outcome
still requires provider evidence before another registration; GitLab and live
CIMD acceptance remain pending.

**Connection removal and session recovery:** Admin deletion requires a disabled
connection, completed local OAuth revocation when applicable, current revision
and explicit UI confirmation. It removes local configuration/catalogue/secrets,
retains a minimal reserved-ID tombstone and audit history, and leaves graph
references visible but unusable. Reusing a deleted ID is prohibited. Remote
applications/registrations are never deleted implicitly. Stale writers and OAuth
callbacks cannot restore the connection. A session-bearing HTTP 404 triggers
one reinitialization within the same operation deadline. Discovery restarts from
its first page; pre-dispatch initialization can recover. A dispatched tool call
is never replayed, even after reinitialization. Other HTTP/network failures and
broken SSE streams remain explicit failures; persistent SSE cursor resumption
is outside this increment.

**Delivered first increment:** The console registers revisioned remote HTTPS MCP
connections, protects bearer credentials, discovers bounded JSON/SSE tool catalogues,
and saves exact per-agent grants in the graph editor. Discovery uses the
2025-06-18 Streamable HTTP protocol and public network destinations only. Unit,
SQLite/real TLS integration and Chromium tests cover the supported flow.
**Gateway increment:** Remote tools are offered only to explicitly granted agents
and invoked through the shared gateway. Each call checks agent grants and current
connection state, including revalidation after approval waits. Existing per-agent
grant rate limits and approvals apply; bounded JSON/SSE calls use fresh sessions,
no replay of dispatched tool calls, redacted audit metadata and capped runtime results. Unit,
real TLS/API integration and Chromium cover success, denial and connection disable.
Private-network policy and full live
vendor-specific acceptance remain pending; service names are metadata.
See [MCP scope and limits](../docs/mcp.md).

Implement MCP connection management in the interface, starting with the integrations used by the team:

- Bitbucket;
- GitHub;
- Confluence;
- Jira.

Users must be able to add multiple connections for each service, edit, test, disable, and remove connections; inspect available tools; and grant access to specific agents by connection/tool.

Use connections and transports that can be reached securely from the Docker environment. Store tokens and credentials encrypted, hide them after saving, and allow revocation/rotation. Calls go through the LeForge gateway, with per-agent authorization, call/error auditing, limits, and approvals when configured. Do not execute arbitrary user commands in the main process to start local MCP servers.

**Contract survey (2026-10-17):** [Priority vendor contracts](../docs/mcp-integrations.md)
record official endpoints, authentication, scopes and remaining compatibility
qualifications. This is documentary evidence, not live vendor acceptance.
[Additional template candidates](../docs/mcp-templates.md) cover software, data,
ML and AI engineering; they do not expand Release 1 acceptance automatically.

Deliver the next increments in this order:

1. **Versioned templates and GitHub:** reviewed endpoint/credential/profile fields,
   exact grants, github.com PAT setup and read-only validation; map selected tools
   to minimum token permissions. Add GitHub OAuth with host-owned app registration.
2. **Shared Atlassian foundation:** qualify v2 Streamable HTTP and flat paginated
   discovery with a narrowly validated `tools=all` setting. Add OAuth and encrypted
   Basic credentials alongside service-account Bearer support. Prevent dispatcher
   tools from bypassing per-operation grants; validate site context and catalogue
   bounds without silently changing network or authorization policy.
3. **Bitbucket Cloud:** organization-linked workspace and repository selection,
   read/write scope guidance, repository/PR tests and cross-workspace denials.
4. **Jira Cloud:** site/project selection, read/search/write profiles, issue/JQL and
   approved mutation tests; keep delete/manage out of initial defaults.
5. **Confluence Cloud:** site/space selection, read/search/write profiles, content
   access/restriction tests and approved updates including version conflicts.
6. **Operational completion:** token renewal/reauthorization, deletion, aggregate
   connection quotas and filterable call/approval/outcome history. For each vendor,
   require unit, real-boundary integration, browser E2E and authorized live tests.

**Delivered template/link increment:** The authenticated backend catalogue now
publishes five immutable v1 presets (DeepWiki, GitHub read-only, Context7, Hugging
Face and W&B). Browser links pin ID/version, survive login and show a review before
an administrator fills and saves the connection. The persisted reference is
validated against endpoint/service; custom endpoints require detachment. Invalid
links/versions fail explicitly. Opening a link does not create, discover or grant.
Unit/API integration and Chromium cover this increment; live vendor acceptance,
template upgrade previews and provider-specific OAuth qualification remain pending.

**Delivered generic OAuth increment:** Pre-registered public or confidential clients
can authorize through the browser, return to a configured backend callback,
complete with their original LeForge admin session, discover tools and renew/revoke
credentials. PKCE, cookie/session binding, encrypted token persistence, serialized
refresh and revocation races are covered by unit/TLS integration tests and the race
detector. Chromium covers consent, renewal, explicit agent execution and revocation,
including provider failures. Distributed pending-flow
storage and real-vendor qualification remain pending. See [OAuth scope](../docs/mcp.md#browser-oauth-pre-registered-clients).

**GitHub.com OAuth compatibility:** A pinned issuer/endpoint profile supports the documented confidential POST exchange and renewal despite omitted authentication-method metadata, normalizes comma-separated scopes, and rejects OAuth errors without leaking upstream payloads. Unit, TLS API integration and browser fixtures cover this contract; live consent and public profile/file reads are now verified as recorded above; broader repository permissions and organization/SSO acceptance remain pending. Manual registration is documented in `docs/mcp.md`.

**Service setup and multiple connections:** GitHub, Atlassian Cloud MCP v2,
GitLab instance and custom profiles prepare reviewed defaults and manual client
registration guidance. Existing connection IDs remain the installation-owned
credential/authorization boundary; repeated services are supported without
per-user credentials or social login. GitLab's documented public pre-registration
uses a narrowly scoped PKCE compatibility rule. API/TLS/gateway and browser
fixtures cover concurrent pending authorizations, independent revocation and
explicit agent grants. Live Atlassian registration/corporate approval and GitLab
instance qualification remain pending; see `docs/mcp.md`. This increment does not
deploy itself or change external OAuth applications.

**Delivered automatic OAuth client setup (DCR/CIMD; fixture-qualified):** Keep
pre-registered clients as an explicit, always available alternative. Reuse an
existing registration first; otherwise prefer CIMD when advertised and usable,
then DCR when advertised, then guided manual setup. Show the selected mechanism
and reason before the administrator starts it. Opening a template or inspecting
metadata must not register an application. Registration does not replace provider
consent or explicit per-agent tool grants.

The UI now offers manual, automatic, DCR and CIMD modes and a read-only preview
before explicit confirmation. Persistent CAS claims prevent duplicate concurrent
registration and automatic replay of uncertain POST outcomes. Completed clients
survive restart; manual clients remain the default for existing connections.
CIMD publishes a minimal public client document only for prepared enabled
connections. Public CIMD (`none`) and DCR `none`/basic/post clients are supported;
expiring registration secrets, initial registration access tokens, private-key JWT
and remote client deletion remain unsupported. See [operational limits](../docs/mcp.md#automatic-client-setup-dcr-and-cimd).

Verified: unit and real TLS/filesystem tests for selection, response validation,
secret encryption, concurrency and restart; SQLite/API tests for both mechanisms
through consent, discovery, renewal and revoke; multiple-service DCR gateway tests
for exact grants and independent revocation. Chromium covers DCR review, reuse,
denial, uncertain registration/manual recovery and hosted CIMD through a real TLS
fixture proxy. `make test`, `make lint`, `make build` and `make test-e2e` passed
(171 frontend tests; 28 standard browser cases plus 1 hosted CIMD case). Live
GitLab/Atlassian registration, consent and corporate acceptance remain pending.
No production installation or external application was changed by this increment.

**Capability verification, 2026-10-17:** Public documentation and read-only metadata
GETs establish the following advertised capabilities, not end-to-end acceptance:

| Provider | Evidence | Planned behavior |
| --- | --- | --- |
| GitLab.com / chosen GitLab instance | [Official MCP guide](https://docs.gitlab.com/user/model_context_protocol/mcp_server/#reuse-a-single-oauth-application) documents DCR and administrator disablement. [GitLab.com metadata](https://gitlab.com/.well-known/oauth-authorization-server) advertises `https://gitlab.com/oauth/register`; it does not advertise CIMD. | Discover each instance independently; support DCR with `mcp` scope when enabled. Keep pre-registration when disabled or unavailable. Do not infer CIMD support from DCR or general MCP support. |
| Atlassian Cloud MCP v2 | [Resource metadata](https://mcp.atlassian.com/.well-known/oauth-protected-resource/v2/mcp) identifies the issuer `https://auth.atlassian.com/VCeDsk8ZHncYF1g234fKtc4lNipbBhu3`. Its [authorization metadata](https://auth.atlassian.com/.well-known/oauth-authorization-server/VCeDsk8ZHncYF1g234fKtc4lNipbBhu3) advertises `client_id_metadata_document_supported: true` and `https://auth.atlassian.com/VCeDsk8ZHncYF1g234fKtc4lNipbBhu3/dcr/register`. | Prefer usable CIMD; offer DCR and compatible pre-registration. Qualify callback/domain/site/IP policies and real consent separately; do not substitute REST 3LO registration. |
| GitHub remote MCP | [Official host guide](https://github.com/github/github-mcp-server/blob/main/docs/host-integration.md) states that DCR is unsupported. | Preserve the existing dedicated, pre-registered application flow; do not promise automatic registration. |

The backend implements capability-driven selection following the
[MCP registration approaches](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#client-registration-approaches),
with the supported specification revision recorded during implementation. DCR
runs on the backend against a validated advertised registration endpoint, sends
reviewed client metadata and the exact configured callback, and validates the
returned registration/authentication method before use. Persist registration
identity and encrypted credentials per installation-owned connection and issuer;
reuse after restart, serialize concurrent registration, and avoid blindly retrying
an ambiguous registration POST. Explain denied registration, invalid metadata,
rate limits and unsupported client methods without leaking provider payloads.

DCR failure diagnostics retain only locally authored categories and the provider
HTTP status, survive reloads/restarts, and do not permit automatic retries.
Historical uncertain attempts without diagnostics require external reconciliation;
HTTP localhost is supported by Atlassian's documented domain policy, subject to
organization restrictions. Neither a browser 403 nor an uncertain marker proves
whether a remote client was created. See the recovery guidance in `docs/mcp.md`.

CIMD identifies the client through a stable public HTTPS JSON document; it does
not create an application by POST. Publish only non-secret client metadata,
including an exact matching URL-form client ID and approved callback URIs derived
from `server.publicUrl`/`LEFORGE_PUBLIC_URL`. The provider must be able to retrieve
that document. The current `http://localhost:8080` installation cannot itself host
a usable public HTTPS CIMD document: offer DCR/manual setup and explain the
prerequisite, without opening a tunnel or changing installation configuration.
Do not silently change mechanism after a failed authorization or overwrite an
existing manual registration. Changing issuer, callback or registration mode
requires explicit review and must invalidate incompatible pending flows.

Preserve public-only transport, resource/issuer binding, PKCE S256, state/cookie
protection, encrypted token storage and independent renewal/revocation. Separate
local connection revocation from provider grant revocation and remote client
registration deletion; never delete a shared remote registration implicitly.
Report bounded registration outcomes through existing audit mechanisms without
secrets or unbounded labels. Add unit, real TLS/API/storage integration and browser
E2E coverage for both automatic paths, manual alternatives, failures, concurrency,
restart and isolation between connections. Require separately authorized live
GitLab and Atlassian registration/consent checks before claiming vendor acceptance;
no external registration was performed in this survey.

**Browser-link and OAuth journey:** A link to the user's LeForge
installation opens a reviewed template in the connection screen, preserving the
pending selection through LeForge login. Links contain only public template
identifiers/version and validated configuration; opening one never creates a
connection, starts a server or grants agent permissions. Support configured local
and hosted installation URLs, with a registered backend OAuth callback separate
from the frontend hash route.

After reviewing the endpoint and requested scopes, the authorized operator starts
OAuth, consents at the provider and returns to the same LeForge connection flow.
The backend validates a short-lived, single-use state bound to the initiating
user/session, installation and connection, verifies PKCE/issuer/callback, exchanges
the code and stores tokens encrypted. Access/refresh tokens never enter shared
links, frontend storage, model context or logs; callback codes are redacted and
removed from the visible URL after handling. Test/discovery follows successful
authorization, then the operator explicitly assigns tools to agents. Show denied,
cancelled, expired and failed authorization without claiming a working connection.

Renew tokens server-side when the provider supports refresh, serialize concurrent
refresh and persist rotated credentials atomically. If renewal is unavailable or
fails permanently, require reauthorization without expanding grants or replaying
an ambiguous remote mutation. Revocation blocks subsequent calls locally,
invalidates pending authorization/refresh work and removes locally stored tokens;
request provider-side revocation when supported and show its result separately.
If remote revocation fails or is unavailable, explain the remaining provider-side
action. Already dispatched calls cannot be rolled back. Unit, integration and
browser E2E must cover the complete journey, callbacks/replay, renewal races and
revocation, plus opt-in live-provider validation.

The additional template backlog can reuse the registry: DeepWiki, Context7, Hugging
Face and W&B first; Sentry, dbt, LangSmith and Databricks after their authentication
and header prerequisites; BigQuery/Snowflake after protocol qualification;
Playwright/MLflow/Qdrant after isolated MCP hosting design. Discover current tools
and require explicit grants; installing a template must never authorize an agent.

**Acceptance criterion:** Users can open a template by browser link, review and connect the four priority integrations through the interface, complete OAuth authorization and return where supported, renew or reauthorize credentials, revoke access, test connections, grant tools to specific agents, and inspect calls and their results. Credential modes remain provider-specific; the complete browser journey requires automated and live-provider acceptance.

### 7. Add Skills and Steering

Use hybrid storage with separate responsibilities:

- **Application:** Installation-wide instructions, user-managed skills, and precedence/permission configuration.
- **User repository:** Project-specific steering and instructions, versioned with the code and loaded only for graphs authorized to use that workspace.

Define clear precedence: global, graph, agent, and task instructions; repository instructions belong to the project scope and cannot override security policies. Show the source and effective version/content of instructions loaded in each execution. Treat repository content as untrusted input: it may guide the agent, but must never grant tools, credentials, or permissions.

**Acceptance criterion:** Operators can manage global instructions through the application; project-specific instructions travel with the repository; every execution records the loaded instruction set; no instruction can elevate privileges.

**Delivered scope:** Admin-managed revisioned Markdown skills, explicit graph/default/project opt-ins and narrower agent selections are available in **Skills & steering**. Project sources are immediate `.leforge/steering/*.md` files in the authorized workspace. Immutable execution manifests show included content, revision/hash and omission reasons. Bounded composition preserves mandatory agent/task text; graphs without opt-in retain their previous prompt semantics. This does not execute skill scripts, discover arbitrary instruction formats, grant permissions or implement delegation. Limits and verification are recorded in [instructions.md](../docs/instructions.md).

### 8. Allow Dynamic Delegation to Subagents

**Tree-budget recovery and retention (2026-10-18):** Startup and periodic
reconciliation now close budgets whose root execution is durably terminal.
The engine's existing restart/replica-takeover recovery supplies interruption
state; age alone never marks a worker dead. CAS closure refunds undispatched
holds, charges unknown dispatched usage conservatively and preserves known
settlements. Active foreign workers, missing execution evidence, corrupt state
and failed writes keep capacity unavailable. Errors are surfaced in engine logs;
healthy trees continue reconciling, with failed work retried on a later sweep.

After 30 days from terminal completion, per-call details are compacted into an
indefinitely retained closed summary. Historical totals remain readable and late
admission/dispatch cannot reopen credit; late callbacks cannot alter compacted
usage. Undated and active roots are not compacted. Execution, approval and
cancellation history remain intact; only old per-call budget detail is compacted,
retaining its totals and closed-tree marker. This bounds per-tree reservation
details, not the total number of retained summaries. No new API,
permission, migration or admin policy is introduced.

- [x] Recover terminal tree reservations idempotently after restart or lease takeover, preserving active workers and conservative unknown consumption.
- [x] Compact old terminal reservation details while retaining historical totals and closed-tree fencing.

**Recovery verification:** `make test` passed (Go unit/integration, Python and
274 frontend tests), as did `make lint`, `make build` and `make test-e2e`
(58 main Chromium scenarios plus one hosted-CIMD scenario). Focused race-enabled
recovery tests passed. Unit tests cover idempotent closure and unchanged known
usage. SQLite integration covers concurrent reconcilers/dispatch, active and
missing roots, corrupt state, storage failures/retries, the 30-day boundary,
undated roots, compaction, late calls and preserved totals. Engine integration
reopens storage containing crash leftovers and runs actual startup recovery,
while preserving another worker's active reservations. Browser coverage seeds
crash/active/historical records in the disposable SQLite installation before
startup, then verifies recovered/compacted totals, reload stability and viewer
restrictions through the real API. It does not kill an installed process.

The initial full test run hit an existing environment start/stop timeout; five
focused repetitions and the full rerun passed without changing that test. The
initial new E2E restored admin credentials on reload in its test setup; correcting
that setup made the unchanged viewer-restriction assertion pass. No security
checks were removed or relaxed.

Local installation activation is recorded below. Real multi-process crash
qualification remains pending; the earlier installed token-budget validation
remains valid for its original scope. Cancellation-marker retention, broader
runtimes and monetary ceilings remain separate work.

**Recovery installation activation (2026-10-18):** Activated commit `acafb69`
with `docker compose build forged` and
`docker compose up -d --no-deps --no-build --force-recreate forged`. Both commands
completed successfully. Only forged was recreated; its four named volume mounts,
five graph versions and PostgreSQL migration versions 1–3 remained unchanged.
Execution/settings counts increased with normal activity (23,346/22,081 before,
23,378/22,113 after); no history or volumes were removed. The application is
healthy, and the previous image is retained as `leforge/forged:before-budget-recovery`.

Startup reconciliation closed both existing terminal ledgers. The reported-usage
tree `exe_mvdd8sd60e4739a0459e18136e6c` retained 904 charged tokens and zero held
reservations; its revision advanced once from 5 to 6. The uncertain-consumption
tree `exe_mvddb2to3970ef76e1626003a006` retained 4,966 charged/estimated tokens and
zero reservations; its revision advanced once from 3 to 4. Authenticated Chrome
inspection and reload confirmed both totals. More than one minute after startup,
both revisions and update timestamps remained unchanged, confirming repeated
reconciliation did not rewrite or double-charge them. Validation graphs remained
stopped. No credentials, connections, grants or global policies were changed.

These existing ledgers were already settled before restart: this installed check
qualifies closure, historical totals and idempotency, not recovery from an induced
crash with outstanding holds. The unit/integration and disposable-browser tests
above supply that recovery evidence. No installed record was old enough for
30-day compaction; retention-boundary coverage remains automated. Real
multi-process crash qualification remains pending.

**Cumulative tree tokens (2026-10-18):** Native parents with an exact
`delegation.request` grant share a persistent CAS ledger with their children.
Admin policy adds `maxTreeTokens` (default 65,536), intersected with parent token
limits. Estimated input plus bounded output is reserved per call, including
children awaiting approval. Each call settles once; inclusive parent/child usage
is never summed. Pre-dispatch failure/rejection/cancellation refunds the reservation;
unknown post-dispatch usage converts it to an estimated charge. Reported overages
block subsequent admission. Settlement failures fail closed without freeing token capacity.
Existing policies receive the new default without rewriting stored settings.
Runs exposes the limit, charges, held reservations and estimated portion. Fixed
cardinality metrics and spans cover CAS operations. No grants, approval policies,
recursive capability or external runtimes are added.

This is conservative token admission, not a provider tokenizer or monetary cap.
Terminal-root reconciliation and safe detail retention are implemented above;
reservations without terminal execution evidence remain held. Installation validation with local Ollama is recorded below;
broader provider qualification remains pending. See [delegation](../docs/delegation.md#cumulative-tree-token-budget).

**Tree-token installation activation (2026-10-18):** Activated code commit
`b516e34` with `docker compose build forged` and
`docker compose up -d --no-deps --no-build --force-recreate forged`. Only the
application was recreated; its four existing named volume mounts were preserved,
PostgreSQL migrations remained 1–3, and all four existing graph versions stayed
unchanged. The application is healthy. A rollback image was retained locally.
No credentials, MCP connections, global approval policy or existing grants changed.

Installed Chrome validation used local Ollama `llama3.2:latest` and PostgreSQL:
- Root `exe_mvdd8sd60e4739a0459e18136e6c` reserved 4,966 tokens, then settled
  904 reported tokens. Its child reserved 2,020 while awaiting approval. Stopping
  that child released the entire child reservation without charging it; the root
  succeeded, with 904 charged and zero reserved. The saved ledger agrees with UI.
- A separate `tree-token-budget-limit` graph with parent `maxTokens: 1` rejected
  execution `exe_mvddasa69b022c641bea74834408` with tree-budget exhaustion before
  model dispatch: zero usage, no child, no approval and no persisted reservation.
- Stopping the entire tree during root inference
  (`exe_mvddb2to3970ef76e1626003a006`) produced cancelled status and zero reserved.
  Unknown post-dispatch consumption correctly retained 4,966 estimated charges,
  rather than refunding possibly consumed tokens. The first tree stayed unchanged.

Both validation graphs were left stopped with their history preserved; delegation
capacity leases returned to zero. These checks qualify reservation, admission and
cancellation on the installed local provider, not provider billing accuracy or
crash reconciliation. The existing unit/integration, race and 58 browser scenarios
below passed before activation; the deployment additionally rebuilt the web and
server images successfully. No behavior changes were needed during activation.

**Tree-token verification:** `make test` passed (Go/Python suites and 274
frontend tests), as did `make lint`, `make build` and `make test-e2e` (57 main
Chromium scenarios plus one hosted-CIMD scenario). Race-enabled delegation/engine
budget tests passed. Tests cover atomic concurrent admission, idempotent settlement,
provider-reported overages, independent trees, retained reservations after storage
reopen, corrupt/unavailable storage, settlement write failures, legacy policy
compatibility, pre-inference exhaustion, rejection and child/tree cancellation,
unknown usage, parent/child accounting, admin review and existing viewer restrictions.
Browser tests use a real disposable backend and deterministic model fixture.
Initial new browser assertions incorrectly assumed the fixture reported parent
usage; they now verify conservative estimated charges. An unrelated graph-drag
visibility timeout passed on the full rerun without changing that test.

**Execution tree and interruption (2026-10-18):** Runs → Execution tree now
shows persisted parent/child links, agent/profile, state, duration and inclusive
reported usage, independently of the recent execution/event windows. Viewer
reads remain separate from operator/admin controls. Reviewed child interruption
returns an error to the parent, which may continue; tree interruption resolves
the root and cancels its current descendants. Durable idempotent requests retain
the first actor/time and are observed across shared storage. The console reports
requested versus terminal states separately. Pending approvals expire and child
capacity is released; graph definitions and grants remain unchanged. Cancellation
is cooperative, does not undo effects, and does not stop future independent runs.
Native depth remains one; broader runtime/profile support and cumulative billing
ceilings remain pending. See [delegation](../docs/delegation.md).

**Execution-control verification:** `make test` passed (Go/Python and 272 frontend
tests), together with `make lint`, `make build`, and `make test-e2e` (56 main
browser scenarios plus one hosted-CIMD scenario). Focused race tests passed for
execution controls, approval cleanup and engine cancellation. One existing cache
selection test exceeded its 5-second timeout during concurrent suite execution;
the complete `make test` rerun without browser-suite contention passed without
changing or skipping tests. Coverage includes
child/tree cancellation during approval and active inference, parent continuation,
late callback rejection, a late successful runtime result, unchanged grants,
viewer denial, explicit confirmation, storage failures/corruption, invalid tree
relationships/bounds, historical lookup outside the recent list, idempotency (including terminal retries),
capacity cleanup, and SQLite migration/backfill preserving existing records.
Browser journeys use the real disposable backend with a deterministic model.
Broader multi-process/provider qualification and final developer usability
acceptance remain pending. Local installation activation is recorded below. Cancellation-marker retention/cleanup is not yet implemented.

**Execution-tree installation activation (2026-10-18):** Activated commit
`27e13bf` using `docker compose build forged` and
`docker compose up -d --no-deps --no-build --force-recreate forged`. Only the
application service was recreated. Existing volume mounts were verified unchanged,
`/healthz` returned OK, and additive PostgreSQL migration 3 completed with no
parent-index/document mismatches. The three pre-existing graphs retained versions
1, 7 and 1; existing execution history was preserved.

Authenticated Chrome validation used a separate `execution-tree-validation`
graph with the existing local Ollama `llama3.2:latest` provider and only the
`delegation.request` grant. With a child awaiting approval, Stop child plus
confirmation produced a cancelled child and a successful parent. A second run
used Stop entire tree plus confirmation and both parent and child became
cancelled. Both pending approvals expired, no delegation capacity leases remained,
and the graph stayed at version 1 with its original grant. The validation graph
was stopped afterward; its four execution records were retained for inspection.
No repositories, MCP tools, secrets or authentication settings were changed.
The previous application image is retained as
`leforge/forged:before-execution-tree`. This validates a local PostgreSQL-backed
worker and pending-approval interruption; live interruption during child inference
and distributed-worker qualification remain outside this installation check.

**Configurable profiles and policies (2026-10-18):** Administration → Delegation
now supports reviewed admin-only edits to revisioned native analysis profiles and
required/automatic approval policies. Viewer/operator reads remain available;
mutations require admin and compare-and-set revisions. Profiles customize guidance
and output allowances while retaining the parent's model and no tools/workspace.
Depth stays one. Child count, shared concurrency, duration, output tokens and
objective/result sizes have finite configurable bounds. Snapshots retain policy,
profile and objective; requests revalidate before inference. Lowered capacity
counts existing high-numbered slots. Automatic policy does not grant tools or
bypass a tool grant's own approval requirement. Disable profiles rather than
removing historical configuration. See [delegation](../docs/delegation.md).


**Configuration verification:** `make test` passed (Go/Python suites and 263
frontend tests); `make lint`, `make build`, and `make test-e2e` passed (54 main
browser scenarios plus one hosted-CIMD scenario). Race-enabled delegation/settings
integration tests passed. The first full run encountered an existing workspace
review test failure; five diagnostic repetitions and full reruns passed without
changing or disabling that test. Configuration tests cover persistence/CAS, invalid
inputs, storage failures, required/automatic policy, generic tool approval,
disabled profiles, immutable execution snapshots, revision changes before start,
finite limits and capacity reduction under concurrency. Browser tests cover
reviewed admin saves, automatic execution and parent result, disabled profiles,
explicit grant denial, viewer restrictions and stale revisions. Tests use SQLite
and a deterministic model fixture; live-provider, multi-process PostgreSQL and
installation validation are not claimed. Developer acceptance remains deferred.

**Earlier foundation (2026-10-18; configuration extended above):** Explicit native `delegation.request` grant,
immutable analysis template, same parent model, no child tools/workspace/history,
mandatory operator approval, persisted parent/child/approval linkage, structured
result/error return and fixed depth/count/shared-capacity/deadline/output limits.
Runs distinguishes children and links to the parent. Reported child consumption
is included in the parent without double-counting native gateway inference.
Pending approval cleanup and restart interruption are covered. See
[delegation](../docs/delegation.md) for boundaries, including the absence of a
hard monetary ceiling and external runtime delegation. Execution-tree controls
are extended below.

**Verification:** `make test` (Go suites and 257 frontend tests), `make lint`,
`make build`, `make test-e2e` (52 standard browser tests plus one hosted-CIMD
scenario) and `go test -race ./internal/engine -run Delegation -count=1` passed.
Browser scenarios use a disposable real backend and a deterministic model fixture;
live-provider qualification, installation activation and developer acceptance
are not claimed.

Give orchestrator agents the ability to delegate subtasks when they determine it is necessary, without requiring users to pre-draw every subagent in the graph.

**Execution audit (2026-10-18):** Native delegated executions retain their enforced
access/output bounds and approval status, actor and timestamp independently of
mutable settings. Automatic admission is identified explicitly. Runs exposes the
snapshot with objective, resolved model and parent link; historical missing data
is labelled unknown. Approval free text is not duplicated into the snapshot.
Unit, real-engine integration and approve/reject browser tests cover this slice.
Installation activation and final acceptance remain pending.

Start with **temporary child executions**, not unrestricted edits to the permanent graph. Each delegation records its parent, objective, agent/configuration used, result, state, and resource consumption.

Allow administrators to configure the delegation policy; operators decide pending approvals:

- **Approval required:** Each new delegation waits for human approval before starting.
- **Automatic:** Delegations start without individual approval, subject to configured global limits.

Set global limits for depth, number of children, concurrency, duration, and cost. Child permissions must be the same as or more restrictive than the delegating agent's permissions. Configuration and approval policy are controlled by the operator, never by the agent.

**Acceptance criterion:** An agent can delegate, receive the result, and continue; the interface displays the execution tree and can stop executions; approval and limits are enforced by the server.

### 9. Prepare Local Docker Operation

Review deployment and documentation so the primary workflow is:

1. start the application on local infrastructure;
2. open the console in a browser;
3. configure providers, integrations, and permissions;
4. create or open a graph;
5. send tasks/commands to agents and monitor executions, approvals, and changes through the interface.

**Acceptance criterion:** Setting up and operating a new installation does not depend on editing files on the server or submitting tasks through the CLI.

### 10. First-use guidance — global onboarding withdrawn

**Decision (2026-10-18):** Remove the global resumable onboarding panel following
user-reported layout breakage. It is no longer a Release 1 implementation or
acceptance requirement. Keep direct navigation and contextual help. Historical
implementation/activation evidence below describes the previous version.
Browser-local progress is ignored, not deleted; graphs, connections, grants,
drafts and execution history are preserved. Installation activation was validated on 2026-10-18.

Removal regression coverage checks absent onboarding for fresh and existing
browser state, direct navigation, draft preservation, explicit echo execution,
viewer restrictions, dependency failures, and narrow-screen layout.

**Removal verification:** `make test` passed (Go/Python and 272 frontend tests),
`make lint` and `make build` passed, and seven targeted Playwright journeys passed
(onboarding removal, navigation and UX feedback). The browser suite builds the
web application and uses the real disposable backend. Existing isolated guide
unit tests are retained, but the guide has no application mount or production
import.

**Onboarding removal installation activation (2026-10-18):** Commit `a2b33b3`
was activated as a frontend-only update over the installed `9b8d1e8` baseline.
The web assets were rebuilt and only `forged` was recreated with existing
volumes. The backend binary hash and volume mounts were verified unchanged;
the service is healthy, `/healthz` succeeds, and schema versions remain 1 and 2.
Three graphs and existing providers and MCP connections remain present, while
background execution history continues to grow. Authenticated browser checks
covered Models, Graphs, the reader configuration and Connections without the
global onboarding panel. GitHub remains authorized and the reader retains its
two tool grants. No configuration edits or live tool calls were needed. The
previous image is retained as `leforge/forged:before-onboarding-removal` for
rollback. This activation does not include the separate delegation changes.

## Release 2 — Reusable Components

After graph creation and operation have stabilized:

- allow users to save agents and subgraphs as components;
- insert components into other graphs through the visual editor;
- define editable parameters when inserting a component;
- version components and indicate available updates;
- allow sharing according to installation permissions.

**Acceptance criterion:** Users can reuse a component across graphs without manually copying definitions or breaking existing configurations.

## Suggested Implementation Sequence

1. **Scope cleanup** and confirmation of the flows that remain.
2. **API contracts and data model** for the visual editor, workspace, providers, MCP integrations, instructions, and child executions.
3. **Visual editor**, as the central experience and a dependency for the graph model.
4. **Containerized project environments**, on a Docker daemon dedicated to LeForge, so agents run the project's toolchain without touching the host.
5. **File and interaction panel**, read-only for users, with tasks and changes tracked by execution.
   Follow with **durable human review and preview sessions** (section 5a):
   candidate identity, supervised services, protected HTTP access and resumable gates.
6. **Web configuration for models and MCPs**, with protected credentials and Bitbucket, GitHub, Confluence, and Jira integrations.
7. **Skills and steering**, with hybrid storage, precedence, traceability, and trust boundaries.
8. **Dynamic delegation**, with a configurable choice between individual approval and bounded automatic execution.
9. **Contextual guidance and direct navigation**; the global onboarding panel has been withdrawn.
10. **End-to-end tests** for the full interface workflow, including direct navigation, authorization, and isolation.
11. **Component library** as the second release.

Compose delivery note: development-service selection and reviewed list-form
`depends_on` groups are implemented for up to three image-based helpers per
replica. Variants are standalone (no override merging). Runtime-only helpers use
private internal networks and atomic CPU/memory/count admission; idle suspension
preserves their data. Advanced Compose fields remain explicitly unsupported; see
`docs/environments.md` for the accepted subset and lifecycle.

Installation policy delivery note: `isolation.docker.environmentPolicy` now
validates allowed preparation/execution networks and feature restrictions across
manual, repository and agent definitions, including cached builds and persisted
approvals. Dockerfile RUN obeys the preparation network. Node overrides cannot
widen permitted execution networks. Configuration takes effect after restart for
new admissions; no automatic revocation/deletion of running resources. Registry
egress restrictions remain planned; runtime HTTP/HTTPS host allowlists via a
managed per-replica proxy are implemented. Setup/build proxying with a separate approved host list, isolated BuildKit worker, aggregate capacity and real sandbox tests is implemented.
Installation network administration is delivered in Integrations: revisioned shared settings, admin preview/confirmation, graph host usage and current-policy checks at new admission and idle resume. Existing running work requires explicit stop for revocation. Per-environment proxy process state and sampled denial diagnostics are delivered in environment/agent panels, with bounded reads and a retained preparation snapshot. Collected denials now persist for seven days/up to 1,000 per graph by default, with admin-configurable installation bounds in Integrations (1–90 days, 1–10,000 blocks, confirmation and revision checks), through background and lifecycle collection, with an independent history UI/API and explicit sampling limitations. Daemon provisioning and client-side registry authentication remain outside it. See `docs/environments.md`.

Agent environment tools delivery note: explicit grants expose own-graph
`environment.status` and `environment.propose`. The latter submits a complete
validated definition from a Docker node using the graph environment, waits for
administrator approval/build, and prepares the approved sandbox before commands
continue with the same workspace. Existing repository definitions take priority.
Timeout/restart semantics follow the turn and persisted proposal lifecycle; no
in-memory call restoration or arbitrary graph targeting is implied.

## UX Reorganization — Incremental Delivery

The reviewed three-screen prototype establishes the direction for agent configuration,
MCP connection detail and execution inspection. It is a design reference with fictional
data, not an implementation or vendor-validation claim.

Preserve graph ownership, whole-document draft/version semantics, explicit per-agent
permissions, OAuth gates, redacted history, read-only workspace inspection and API role
checks. Keep Graphs as the product entity; first-class projects/tasks and dynamic delegation
remain separate planned work. All interface copy remains in English.

1. Separate installation destinations: Connections, Models and Administration, with
   stable global navigation. Administration separates health, network policies, shared
   resources, retention and the built-in tool catalogue. Existing integration/template/
   OAuth URLs remain compatible. Mount only the selected area; navigation must not test
   providers, grant tools, or mutate settings. Delivered as the first implementation slice, with route, real-client component and disposable-backend browser coverage. Existing MCP/provider forms remain unchanged within their new destinations; installation activation is not part of this evidence.
2. Replace graph tabs in global navigation with a scalable graph selector; preserve
   deep links, history and unsaved drafts. Add a list alternative to the canvas.
   Implemented with searchable graph/agent navigation and per-graph editor sessions
   retained in memory while inactive effects pause. Drafts retain their base revision
   across server refreshes; missing graphs retain their local editor with an error.
   Unit/component integration and disposable-backend Chromium verify search, failures,
   cached selection, independent drafts, revision conflicts and viewer restrictions.
   No browser-storage draft persistence or installation activation is implied.
3. Build connection list/detail routes with Overview, Tools, Usage & history and Settings;
   show connection creation only on request, preserve manual/DCR/CIMD and uncertain states.
   Delivered with searchable/filterable connections, searchable tools, scoped history and
   saved graph grant visibility. Existing setup handlers and legacy OAuth links remain
   compatible. Unit, real-client component integration and disposable-backend Chromium
   coverage verify creation, discovery, editing, missing records, errors and role gates.
   No installation activation or additional external provider validation is implied.
4. Give agents a dedicated configuration page: General, Instructions, Tools & permissions
   and Advanced. Keep effective isolation/grants visible; review the complete graph draft
   before saving. Keep operational controls separate from editing. Delivered through the
   dedicated agent route with shared graph drafts, section/agent deep links, configured
   permission/isolation summaries and a complete changed-field review (values stay in
   their existing editors). Existing canvas and Definition editors remain compatible.
   Server validation, revision conflicts and role authorization are preserved; no
   installation activation or runtime permission changes are implied.
5. Introduce Runs list/detail navigation over existing execution records. Show results and
   actionable blockers first, with timeline, calls, messages and diagnostics on demand.
   Do not infer task completion, percentage progress or record relationships. Delivered
   as Runs list/detail routes over existing APIs with explicit recent-window limits,
   persisted execution/message links, independent error recovery and existing approval
   role gates. Results distinguish execution status from blocked tool calls; workspace
   links explicitly refer to current graph files. Full-history pagination and replay
   are outside this interface slice.
6. Consolidate typography, spacing, contextual help, focus and responsive behavior.
   The global onboarding panel has been withdrawn. Prototype/user research does not establish
   usability success: validate with developers before claiming the proposed task-time goals.

Each slice requires unit tests, real-client/component integration tests and disposable-
backend browser coverage for success, error recovery, permissions, deep links and relevant
narrow-window interactions. Installation activation is a separate approved operation.


### Installation validation of the navigation baseline

On 2026-10-17, commit b0f7670 and the preceding navigation, connection, agent and
Runs slices were activated on the local Compose installation. Only forged was
rebuilt and recreated with --no-deps and --wait; the existing data, workspace,
journal and certificate volumes were reused. Health returned ok. The three graphs
and nine saved versions remained present; the execution history continued growing.
Authenticated browser checks covered connection/tool search, agent sections,
independent in-memory drafts, review/discard without saving, saved run results,
Models and Administration health. This updates the earlier slice-local activation
disclaimers; it does not establish usability acceptance or activate later changes.

### Visual consolidation

The remaining pages now share heading, spacing, form, card, button and focus
styles, including Skills, Approvals, Models, Administration and canvas controls.
Optional guidance uses native disclosures; security/error context stays visible.
Existing handlers, role checks and draft semantics are retained. Developer journey
validation remains pending; global onboarding has since been withdrawn. This visual slice
was subsequently activated with approval; see the evidence below.


Validation: make test, make lint, make build and make test-e2e pass, including
229 frontend tests and 46 browser scenarios. The new keyboard/disclosure and
viewer journeys use the real disposable backend. An earlier canvas reload scenario
failed to render edges once; the isolated repeat and final full suite passed with
the original assertions. No claim of resolving an intermittent canvas issue is made.


### Visual consolidation installation validation

Commit 7ef1178 was activated on the local Compose installation on 2026-10-17.
Only forged was rebuilt and recreated; existing data/workspace/journal/certificate
volumes were reused. The service became healthy, with three graphs and nine saved
versions retained. Authenticated browser validation covered Models, contextual
keyboard help, Skills, Approvals, Administration and canvas navigation. The saved
Software Factory canvas displayed ten edges both before and after reload without
dirtying the graph. No configuration, credential or grant changes were submitted.
This is installation smoke-test evidence, not developer usability acceptance.


### Developer usability validation — historical round (2026-10-17; continuation deferred to final acceptance on 2026-10-18)

The developer-led round uses the current local installation with visual baseline
7ef1178. Automated checks and assistant-operated smoke tests remain separate
evidence. Present one task at a time before offering navigation hints; record the
developer's route, outcome, confusion and assistance required. Do not infer
completion or timing from silence or from assistant navigation.

1. Agent configuration: locate reader in MCP installation validation and identify
   its model and explicit tool grants without saving changes. First task issued;
   developer feedback pending.
2. MCP connection: assess finding the connection flow and understanding setup,
   authorization and the separate agent-grant step. Actual connection creation or
   external consent must use a reviewed target and the applicable approval gates.
   Not started.
3. Execution tracking: locate an existing execution, explain its outcome and
   distinguish it from individual tool outcomes. Assess a new execution only if
   its target and effects have been agreed. Not started.

Log confirmed friction as concrete follow-up work with affected screen, expected
behavior and validation criterion. No usability adjustments have been confirmed
yet; no journey is accepted based on this initial task alone.

**First-round feedback:** The developer described the Graphs screen as outdated and supplied a screenshot with the new navigation and graph selector. The served HTML references the activated visual build assets, matching the running container. Whether the concern is visual fidelity, graph data, or task instructions remains unconfirmed; clarification is pending. Do not classify this as a cache defect or accept the agent journey yet.


**Confirmed finding UX-01 — locating an agent from Graphs:** The developer clarified
that “outdated” meant they could not find reader. Their screenshot shows the graph
selector open on Graphs; it lists graphs, while the graph card only reports the
agent count. The first journey required assistance explaining the graph/agent
hierarchy and opening MCP installation validation. Model/grant identification is
still pending, so the journey is not accepted.

Proposed adjustment: make the route from a graph to its agents explicit in Graphs,
for example a labeled “View agents” action or an agent preview that opens the
existing workspace. Keep graph selection distinct from agent selection and avoid
implying that the graph selector searches agents. Validate the design with the
developer: they should locate a named agent and its configuration from Graphs
without a navigation hint, with unchanged permissions and draft behavior.


**Developer round follow-up:** Screenshots confirm the developer reached reader's
General and Instructions sections after navigation assistance. General shows
provider ollama-local and model llama3.2:latest. The summary reports two agent-level
grants, but the screenshots do not show the selected tools; instructions naming
tools are not evidence of authorization. Grant identification is still pending.
The Events screenshot shows graph.started and execution.started at 19:23:57 local
time, not completion or successful tool calls. The developer has reached execution
tracking; no additional execution needs to be started for this round.

**Confirmed visual finding UX-02 — Events column overlap:** In the supplied Events
screenshot, execution.instructions.loaded overlaps the adjacent reader label.
Adjust event-type column sizing or wrapping so long event types and actor links
remain legible without overlap. Validate long event names at desktop and narrow
widths while preserving access to event details. Whether Events versus Runs is
confusing remains a question for the developer, not an established finding.


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


**Planned adjustment UX-05 — automatic tool discovery after saving:** Requested
by the developer during the usability round. After successfully saving an enabled
MCP connection, automatically discover its tools when authentication is available,
including token-based and unauthenticated connections. For OAuth connections that
still require consent, show authorization as the next step and perform discovery
after authorization succeeds. Disabled connections remain inactive. Keep this flow
inside connection detail; no validation agent is required or created.

Present saving, discovery in progress, tools discovered (including an empty
catalogue), authorization required and discovery failure as distinct states. A
failed discovery must not discard the saved connection or suggest that saving
failed; show a useful error and retain Test / Refresh tools for retry and later
catalogue updates. Preserve existing endpoint validation, credential protection,
role restrictions and revision/concurrency checks.

Explicitly state after discovery: “Tools discovered. Grant access to agents before
they can use them.” Discovery must never create or broaden agent grants, including
when a refreshed catalogue contains new tools. Validate this behavior with unit,
integration and browser tests for each authentication mode, failures, disabled
connections and unchanged permissions. This is planned work, not implemented or
validated behavior.


### UX-01–UX-05 implementation evidence (2026-10-17)

The preceding developer-round notes describe observations and plans at their time;
this section records the resulting implementation, without claiming installation
activation or developer acceptance.

- UX-01: Graph card surfaces open their graph, with native button keyboard/focus
  behavior, independent action controls, and explicit per-agent Configure links.
  Existing in-memory graph drafts and permission gates are preserved.
- UX-02: Events wraps long types and actors in bounded grid tracks, stacking details
  at narrow widths. Browser geometry checks and expandable data checks pass.
- UX-03: Investigation found native runtime and TurnEnv both counted gateway calls;
  the engine then added them. Native TurnResult no longer repeats those counts.
  Runtime-local counts remain additive under the documented runtime contract.
  Runs separates recorded attempts, observed invocations and lifecycle events.
  Historical records are unchanged and their possible duplicate totals and bounded
  event windows are explained in contextual help; no migration was performed.
- UX-04: Connection checks and optional agent execution are explained separately.
  Connection agent links open Tools & permissions in the named graph, with a
  return-to-connection banner preserved through graph navigation and reload.
  Navigating does not run agents or grant tools.
- UX-05: Console save orchestrates existing authorized save and discovery APIs.
  Enabled connections discover automatically when authentication is available;
  OAuth consent still gates discovery and disabled connections are not contacted.
  Progress distinguishes save from discovery, errors preserve saved connections,
  and discovery results explicitly require agent grants. Direct API clients retain
  the separate save/refresh contract. No automatic agent or grant creation occurs.

Validation: make test passed (including Go integration and 241 frontend tests),
make lint and make build passed, and make test-e2e passed with 48 standard Chromium
scenarios plus one hosted CIMD scenario. Browser coverage exercises actual fixture
MCP protocol exchanges and gateway execution, role gates, recovery, graph draft
retention, card hit targets, keyboard activation, connection return paths and event
layout at desktop/narrow widths. Screenshots were visually inspected.

The existing sandbox journal supervision test was intermittent: the first full
run and an isolated repeat failed on its daemon-stopped marker; a diagnostic run
and final make test passed without modifying that test or daemon source. This
remains an observation to investigate, not a resolved defect. Installation
activation and developer usability revalidation remain pending. No persistent data,
installation secrets or OAuth applications were changed.


**UX-01–UX-05 installation activation (2026-10-17):** Implementation commit
322f3bd was activated using docker compose build forged and docker compose up -d
--no-deps --force-recreate --wait forged. Only forged was rebuilt/recreated. Health
returned ok; the existing workspace, data, journal and client-certificate volumes
were reused. PostgreSQL retained three graphs and nine graph versions. Other
services, including Ollama and the sandbox daemon, remained running.

Authenticated Chrome validation on localhost:8080 confirmed graph opening by
clicking the card description outside its title, explicit Configure reader links,
unchanged saved graph version/grants, connection-to-agent context and return links
through Runs and reload, and separate invocation/lifecycle counts. Saving the
existing public DeepWiki validation connection without changing its fields showed
“Connection saved. Discovering tools…” and automatically discovered three tools.
It remained credential-free and had no saved agent grants. Its connection revision,
catalogue revision and discovery timestamp advanced as expected.

A single new read-only reader execution (exe_mvcoynbnbb657e8bc131ce1afd81, graph v7)
completed using ollama-local / llama3.2:latest. Runs recorded two tool attempts and
two observed invocations with four lifecycle events; get_me and get_file_contents
both completed through the GitHub gateway. No graph or tool grants were edited and
no repository-writing tool was used. The fresh execution.instructions.loaded event
wrapped without overlapping reader n0 in Events. The earlier event was outside the
general recent-event window, so validation used this new execution.

Screenshots of installed graph cards, automatic discovery, Events and tool outcomes
were inspected. This is installation activation evidence, not developer usability
acceptance. The broader developer usability round and the previously recorded
intermittent sandbox supervision test remain open. No secret values, authentication
configuration, external OAuth applications or persistent volumes were changed.


**Historical resumable onboarding evidence (withdrawn 2026-10-18):** Implemented an optional, initially
collapsed Getting started panel without route redirects or background mutations.
Six topics cover concepts, health, models, MCP discovery/grants, canvas editing
and execution inspection. Live prerequisite reads report errors and retry explicitly;
topic acknowledgements are never inference, tool-access or installation-health
certification. Progress is versioned, local to the browser origin and sign-in
identity name/role, with no credentials or configuration payloads. Skipping does
not complete a topic; revisiting completion preserves progress; confirmed restart
only resets the guide. Storage failure/corruption is visible. The explicit example
uses the existing validation/create APIs, a finite echo runtime, no tools or model,
and no automatic start; viewers cannot create it. Echo forwards messages and does
not validate model inference or MCP access. The native-only read-only workspace
flag is intentionally absent from echo, following existing graph validation.

Validation: make test passed (Go unit/integration, Python suites and 256 frontend
tests across 43 files), make lint passed, make build passed, make test-e2e passed
(50 standard Chromium scenarios plus one hosted-CIMD scenario), and git diff --check
passed. New unit/real-client component tests cover progress parsing, invalid data,
skip/review/resume/restart, storage failures, prerequisite errors/retry, rejected
validation/create attempts, stable retry IDs and viewer restrictions. New real
disposable-backend browser tests create the example, preserve an unsaved agent
draft across guide/navigation changes, save it, execute echo and inspect its real
result, reload completed progress, confirm restart without deleting the graph,
and retry failed prerequisites as a viewer without mutations. Existing canvas
agent/channel creation, MCP, grant, OAuth and execution journeys also pass.
Desktop and narrow browser captures were inspected.

Earlier focused E2E failures exposed the native-only read-only workspace rule
and an incorrect expectation that echo returns its prompt; the example and test
were corrected to the actual runtime contract before the full passing suite.
README, getting-started and interface-navigation documentation describe the
browser journey and persistence limitations. No installation activation, external
OAuth changes, real model inference, fresh Docker installation, or developer
usability acceptance is claimed. First-class tasks and delegation remain in their
existing workstreams. Activation and the final developer usability round are pending.


**Historical onboarding installation activation (before removal):** Activated commit 9b8d1e8
with docker compose build forged and docker compose up -d --no-deps
--force-recreate --wait forged; both completed successfully. Only forged was
recreated, without removing volumes, running migrations, or changing authentication.
The health endpoint returns ok; forged, agents, NATS, PostgreSQL and the sandbox
daemon are healthy. Graph/version counts remained 3/9 before and after activation.

Browser validation on localhost:8080 confirmed the initially collapsed guide,
reviewing one topic, skipping health without marking it reviewed, pausing on
Choose a model, reloading, and resuming the same topic with 1/6 reviewed.
Navigation to Models and back to Graphs also preserved the topic/progress.
The health observation showed ok and the model observation showed four configured
providers without claiming successful inference. No graph was created, no agent
was started, and no connection, grant, credential or model configuration was
modified during this browser check. Only browser-local guide progress changed.
The implementation's passing unit/integration and 51 E2E scenarios remain the
automated evidence; this activation adds installed-browser pause/resume evidence.
Developer usability acceptance and a fresh-install exercise remain separate.

### Release 1 operations runbook (2026-10-18)

The [operations runbook](../docs/operations.md) now covers the full persistence and
key inventory, quiescence, isolated restore, version-fenced upgrades, rollback and
failure diagnosis. Model-provider re-sealing must not be mistaken for a complete
MCP credential migration; previous keys remain necessary. This is documentation
verified against the implementation, not a completed restore drill or deployment.

### Compose installation contract (2026-10-18)

Reviewed persistence, required credentials, restart behavior and network exposure.
Published services default to loopback; `LEFORGE_BIND_ADDRESS` is the explicit
remote-host override, while previews stay local. Real Compose-render tests use
synthetic credentials and no `.env`, covering network/volume/restart contracts and
missing-secret rejection. No existing installation was recreated. Live reachability
and clean-install acceptance remain separate from configuration validation.

### Managed MCP hosting design (2026-10-18)

[Managed MCP hosting](../docs/mcp-hosting.md) defines the isolation/lifecycle,
transport authentication, upstream credential, resource admission, private-network
and qualification contracts for candidate Playwright, MLflow and Qdrant profiles.
Only the design task is complete; implementation and vendor qualification are not
implied, and no service has been installed or launched.

### Native child grant inheritance (2026-10-18)

Installation profiles can select exact capabilities already explicitly granted to
the parent. The server copies every constraint and reuses the gateway for child
calls, including workspace restrictions, approvals, shared rate/connection limits
and attribution. The child has no independent workspace, memory, emit, recursive
or environment/review continuation capability. A bounded tool/model loop charges
parent tool limits and root-tree token reservations. Profile review and execution
audit expose the effective access. Unit/integration/browser coverage includes
out-of-path denial, ungranted writes, bounded repeat calls and per-tool approval.
No installation activation or live-provider qualification is claimed.

### Dependency health contract (2026-10-18)

Administration already exposes liveness and store/queue/sandbox readiness. The
public readiness endpoint now returns bounded states instead of raw dependency
errors that could contain credentials or internal endpoints. Unit and real HTTP
integration cover failure, redaction and recovery; sandbox failure still does not
make non-sandbox work globally unready. Provider/MCP qualification remains separate.

### Delegation entity/state contract (2026-10-18)

The [state-machine reference](../docs/state-machines.md#dynamic-delegation-entities-and-states)
now defines the existing native parent/child, approval, cancellation, result and
consumption entities without inventing new graph nodes or task records. It explains
persisted approval waits, terminal outcomes, per-tool approvals, cancellation races,
inclusive reported usage and ledger recovery. The implementation suites validate
these boundaries; external runtimes and recursion are still pending.

### Initial installation identity (2026-10-18)

The bootstrap contract now explicitly distinguishes the initial admin token,
named operator provisioning through installation tokenFile/OIDC configuration,
and provider OAuth used only for MCP access. New-install instructions refuse to
overwrite existing `.env` credentials. No browser account-management feature or
live authentication/secret change is claimed. Routine model/MCP/graph operation
continues through the console after infrastructure bootstrap.

### Bounded native recursive delegation (2026-10-18)

The default depth remains one. Administrators can review a depth of up to four;
profiles must explicitly inherit delegation.request at every generation. Native
children share the root token ledger, preserve grant constraints and consume
ancestor tool/output allowances. Tree admission remains bounded to 256 nodes and
shared capacity; exhausted capacity rejects rather than waiting on an ancestor.
Child cancellation propagates to active descendants and is shown throughout that
subtree. Unit/integration/race tests cover depth denial, exact inclusive usage,
shared-ledger settlement and cancellation during grandchild approval; browser
coverage reviews the depth and displays a completed three-generation tree.
External runtimes and a guaranteed monetary cap remain separate pending work.

### Pause admission race correction (2026-10-18)

A successful pause is consulted from durable graph state before claiming operator
input and again after waiting for queue deliveries. A wake-up no longer relies on
the asynchronous reconciler's cached state. Storage failures defer execution;
paused deliveries wait for resume or are returned on cancellation. This preserves
sandbox suspension and queued input when a message arrives immediately after pause.

### Shared pending MCP OAuth transactions (2026-10-18)

Pending browser authorizations now use encrypted shared storage with compare-and-set
updates, preserving PKCE and the original session/browser binding across controller
restart and routing. Capture and consumption are single-winner operations, with a
shared 128-entry limit and ten-minute expiry. Storage/key failures refuse progress;
expired entries are reclaimed on successful updates. A claimed exchange is never
retried automatically after interruption. Live vendor consent remains separately
pending; this delivery proves the protocol with TLS fixtures and durable storage.

### Parameterized MCP templates and reviewed upgrades (2026-10-18)

Immutable legacy templates remain available. GitHub v2 adds a bounded read-only
toolset parameter and PAT/OAuth modes; GitLab v1 adds an HTTPS instance origin and
OAuth configuration. Parameters are non-secret and validated against the pinned
version. Settings show an upgrade preview, support cancellation and apply changes
only to the draft. Explicit saving retains optimistic concurrency, endpoint-change
credential handling and per-agent grants. Unit/API integration/browser tests cover
resolution, invalid parameters, stale writes, preview cancellation and persisted
updates. Live vendor/tool permission qualification remains pending.

### Atlassian v2 flat catalogue foundation (2026-10-18)

Only the exact reviewed tools=all query is accepted on the canonical Atlassian v2
endpoint. The advertised issuer and exact or query-free resource remain validated;
OAuth requests retain the accepted resource identifier. Existing transport limits,
revision checks and explicit grants apply to the paginated flat catalogue. TLS
fixtures and browser coverage prove routing, schema discovery, catalogue drift,
stale/new-tool denial and failed discovery. Live corporate qualification remains
pending; dispatcher protection and product/site policy are separate work below.

### Atlassian operation authorization (2026-10-18)

The generic executeRead, executeWrite and executeDestructive dispatchers are
excluded from discovery results and previously saved catalogues. Exact grants
cannot resolve them, and invocation rejects retained tool instances before
credential access or transport dispatch. Identification uses either the service
family or official hostname, preventing custom labels from bypassing the guard.
Individual operations remain available with explicit grants. Unit, persistence/
gateway integration and paginated TLS/browser fixtures cover these boundaries.
Product-specific selection and site context remain separate pending work.

Full-suite validation also reproduced an environment start/stop race: a stale
reconciler graph-record write can replace the operator desired state. The MCP
unit/integration and browser checks pass; the lifecycle race is a separate
follow-up before final full-suite acceptance.

### Graph state reconciliation preserves operator commands (2026-10-18)

A full-suite run reproduced stale observed-state writes overwriting a newer stop
command. Observed-state persistence now compares the complete graph-record
snapshot atomically in filesystem and SQL storage before updating state. It never
replaces desired state, graph version or run identity. A changed snapshot is
retried by the next reconciliation; storage failures are reported without
publishing a successful state event. Old runs do not publish running state after
a stop, new version or new run. Storage conformance, focused engine tests, the
repeated environment API lifecycle and browser start/stop journeys cover this fix.

### Workspace inspection response isolation (2026-10-18)

File inspection resets local state when switching graphs and invalidates pending
content reads when changing directory/workspace or selecting another file. Late
responses, failures and loading completions cannot replace the latest selection.
Tests exercise the real API client with controlled responses; browser integration
delays an actual backend response and verifies the newer file remains visible.
The file/Git inspection routes are read-only for every role: POST, PUT, PATCH and
DELETE are rejected, and the file remains unchanged. No editor or mutation action
is offered in the inspection components. Broader graph/instruction isolation
acceptance remains tracked separately.

### Atlassian Basic credentials (2026-10-18)

Personal email/token pairs now have an explicit Basic mode, separate from Bearer
service-account credentials and OAuth. Both values are sealed with the existing
connection-bound encryption; views expose only mode/configured status. The manager
selects the transport scheme, including initialization, pagination and cleanup;
agent arguments and remote metadata cannot select it. Result/catalogue echo guards
check the encoded pair, email and token. Replacement/clear, endpoint/mode changes,
failed discovery and exact per-agent grants retain their existing boundaries.
Unit, TLS manager/storage integration and browser fixtures cover the contract.
Real corporate access and scope/site qualification remain pending.

### Typed MCP headers for dbt and standalone LangSmith (2026-10-18)

Custom connections support an encrypted standalone LangSmith API-key mode and
bounded, named metadata settings. dbt accepts numeric production/development/user
IDs with Bearer/OAuth; standalone LangSmith accepts a workspace UUID and reviewed
US/EU API endpoints. Arbitrary names, Host/protocol headers, malformed values and
unreviewed upstream URLs are rejected. Metadata changes invalidate discovery and
require token replacement/clear or renewed OAuth authorization. The UI uses typed
fields without exposing stored keys. Unit, real TLS transport/storage integration
and browser fixtures cover persistence, calls, discovery and invalid settings.
This profile does not qualify hosted LangSmith OAuth, self-hosted private URLs,
provider tool permissions, Sentry or Databricks; those remain separate work.

### Sentry MCP configuration profile (2026-10-18)

The immutable Sentry v1 template and service selector provide the public endpoint,
issuer, minimal advertised org:read scope and provider setup guidance. Saving a
connection neither registers a client nor authorizes/grants tools. Existing PKCE,
resource/issuer discovery, registration review, token sealing and revocation
flows remain unchanged; there is no Sentry compatibility bypass. Unit, API/storage
integration and browser configuration tests cover pending authorization and invalid
Bearer template use. Live consent, dispatcher/catalogue qualification and actual
per-tool scope requirements remain pending despite advertised DCR/CIMD support.

### Databricks workspace/service configuration (2026-10-18)

A versioned profile constructs the reviewed Unity Gateway MCP service path from
an HTTPS workspace origin and bounded three-part name. Typed template parameters
render URL and text fields appropriately. Invalid paths, query strings and service
identifiers fail before persistence. Bearer testing and pre-registered user OAuth
remain separate; OAuth defaults request ai-gateway/offline_access but deliberately
leave the issuer empty until supplied from workspace MCP metadata. Workspace
changes reset draft client/issuer identity. Existing encrypted credentials,
resource binding and per-agent grants are preserved. Unit, API/storage and browser
configuration tests cover valid/invalid parameters, safe persistence and pending
consent. Live workspace acceptance, service-principal renewal, downstream provider
consent and service-specific data/cost controls remain unproven or unimplemented.

### External parents requesting native delegation (2026-10-18)

Runtime Protocol callbacks now accept delegation.request from active external
parents with the exact grant, a managed model and the existing reviewed policy.
Children remain native server-managed executions; no external child launch or
runtime/model override is accepted. Parent callback inference and descendants share
the root token ledger. Approval, cancellation, grants and expiry use the same
server paths. The Python helper blocks for the result without automatic replay;
gateway usage/tool attempts are no longer duplicated in runtime-local totals.
Unit and real HTTP runtime → callback → engine → provider/storage tests cover
approval/automatic success, rejection, invalid inputs, missing grants, provider
failure, budget denial and expired tokens. Browser tests cover approval, rejection,
tree cancellation and unchanged graph/grants. External inference outside the
gateway, hard monetary guarantees and idempotent request keys remain outside scope.

### Canvas connection measurement recovery (2026-10-18)

Full browser acceptance exposed missing rendered edges after reload/reopening a
cached graph, despite preserved nodes/channels. An isolated repeated scenario
reproduced two failures in five runs. Structural node rebuilds discard React Flow
handle bounds without necessarily changing DOM dimensions. The canvas now requests
handle measurement after the structure render and when Activity restarts effects
on reveal; cleanup cancels pending frames. Graph data, selection, permissions and
saved node positions are unchanged. Component integration exercises actual rendered
paths through rebuild, hide/reveal, removal and restoration. The original browser
journey retains all assertions and uses independent IDs for repeat runs; all five
repetitions pass after the change.

### Public MCP live qualification (2026-10-18)

Opt-in live tests exercise production DNS/TLS/protocol discovery against anonymous
DeepWiki (three tools) and Hugging Face (four tools). A DeepWiki public documentation
read uses the real manager/store/gateway, proves denial without an exact grant and
success after explicit authorization. Tests never access installation credentials,
create OAuth clients, mutate repositories or execute Hugging Face Space compute.
The normal offline suite skips network checks; explicit local execution passed.
Authenticated vendor journeys and the rest of the first-wave catalogue remain
pending rather than inferred from public discovery.

### Warehouse MCP qualification boundary (2026-10-18)

Reviewed official BigQuery/Snowflake contracts against the existing stateful
transport. A public BigQuery stateless discovery probe returned nine tools with
no authentication or query invocation. Its read-query schema lacks a maximum
billed-bytes input; neither dry-run estimates nor client timeouts provide a hard
cost ceiling. Snowflake documents lifecycle/notification/version exclusions and
account-specific issuer/role/warehouse requirements; no live account was tested.
The template guide now separates documented contracts, observed public evidence
and pending transport/data/cost controls. No compatibility fallback or template
is enabled from documentation alone. Documentation-only change; no behavior or
installation changes and no new unit/integration tests required.

### Concurrent graph scope acceptance (2026-10-18)

Added a real engine/SQLite/instruction manager/filesystem gateway acceptance test
for two graphs sharing the same worker ID. Each receives its selected skill,
workspace steering, prompt, operator task and file content; runtime input and
persisted snapshot are checked for the other graph's marker. Relative traversal,
absolute sibling paths and symlink escapes are denied. Existing instruction and
filesystem unit tests cover lower-level validation/error behavior. This adds
integration evidence without changing behavior or making claims about arbitrary
host processes, Docker/external-runtime acceptance or the deferred UX walkthrough.

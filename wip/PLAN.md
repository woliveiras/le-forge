# LeForge Evolution Plan

**Goal:** Make the web interface the place where users configure models and integrations, build graphs, and monitor agents at work. LangGraph remains supported; reusable components are deferred to the second release.

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

**Delivered first increment:** The console registers revisioned remote HTTPS MCP
connections, protects bearer credentials, discovers bounded JSON/SSE tool catalogues,
and saves exact per-agent grants in the graph editor. Discovery uses the
2025-06-18 Streamable HTTP protocol and public network destinations only. Unit,
SQLite/real TLS integration and Chromium tests cover the supported flow.
**Gateway increment:** Remote tools are offered only to explicitly granted agents
and invoked through the shared gateway. Each call checks agent grants and current
connection state, including revalidation after approval waits. Existing per-agent
grant rate limits and approvals apply; bounded JSON/SSE calls use fresh sessions,
no transport retry, redacted audit metadata and capped runtime results. Unit,
real TLS/API integration and Chromium cover success, denial and connection disable.
Aggregate per-connection quotas, private-network policy, deletion and live
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

**GitHub.com OAuth compatibility:** A pinned issuer/endpoint profile supports the documented confidential POST exchange and renewal despite omitted authentication-method metadata, normalizes comma-separated scopes, and rejects OAuth errors without leaking upstream payloads. Unit, TLS API integration and browser fixtures cover this contract; live GitHub consent, repository permissions and organization/SSO acceptance remain pending. Manual registration is documented in `docs/mcp.md`.

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

Give orchestrator agents the ability to delegate subtasks when they determine it is necessary, without requiring users to pre-draw every subagent in the graph.

Start with **temporary child executions**, not unrestricted edits to the permanent graph. Each delegation records its parent, objective, agent/configuration used, result, state, and resource consumption.

Allow operators to configure the delegation policy:

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

### 10. Guide First Use with Onboarding

Add an onboarding experience in the interface to introduce LeForge and guide operators from initial configuration through their first completed task. The journey must be progressive, resumable, and dismissible without blocking experienced users; guidance must remain available after completion.

The onboarding should guide users to:

- understand the basics of graphs, agents, channels, and tools;
- check installation services and configure/test a model provider;
- discover and configure MCP connections and permissions when available;
- create a first graph on the canvas, configure agents, and connect them with channels;
- start a task and monitor agents, messages, executions, and approvals;
- inspect resulting files and diffs while respecting the user's read-only access.

Include a safe, working example graph to explore or duplicate, clearly indicating which settings must be adjusted before running it. Show unconfigured dependencies and provide paths to the relevant screens; do not claim success until real tests pass. Let users return to tutorials and contextual help without restarting or losing existing work.

**Acceptance criterion:** A new operator can follow onboarding in the browser, understand the essential concepts, configure available dependencies, create or adapt an example graph, and run their first task. They can skip steps, resume the journey, and revisit help without server or CLI access.

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
9. **Guided onboarding**, integrated into delivered screens and flows, with an example graph and resumable help.
10. **End-to-end tests** for the full interface workflow, including onboarding, authorization, and isolation.
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

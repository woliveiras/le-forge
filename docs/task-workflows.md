# Task-oriented Workflow Contracts

Status: FLOW-01 design reviewed against code on 2026-10-19. FLOW-03 draft persistence and requirement review are implemented as recorded below; initial execution-copy admission is delivered; project behavior remains planned. OpenAPI documents only delivered endpoints.
The current wire contract remains [OpenAPI](../api/openapi.yaml).

## Vocabulary and existing foundations

| Term | Meaning | Current implementation |
|---|---|---|
| Project | Registered repository and execution preferences | Workspace configuration belongs to graphs; a reusable project record is planned. |
| Workflow | Saved graph defining agents, channels and policies | `domain.Graph`, with versioned definitions. |
| Task | One user request with input, chosen project/workflow and review outcome | Revisioned request and execution-copy binding in installation settings. |
| Graph run | Graph lifecycle from start through stop | `GraphRecord.RunID`; starting an already running graph is not a new run. |
| Execution | One turn of one agent replica | `domain.Execution`, including graph version, run ID, input/output IDs and parent execution ID. |
| Workspace | Files where work executes | Shared graph root or per-replica worktree; neither implies a per-task directory. |

Evidence reviewed: `server/internal/domain/{graph,state,message}.go`,
`server/internal/engine/{control,publish,replica}.go`,
`server/internal/workspace/workspace.go`, `web/src/App.tsx` and
`web/src/components/RunsView.tsx`. Message injection currently reads the latest
graph definition, stamps graph/run metadata and publishes through the engine.
Message history recording is best-effort. Runs currently lists agent turns and
links events from a bounded recent window; it cannot be the authoritative task
ledger. Worktrees are keyed by graph/node/replica and reused across turns.

## Navigation and scope

Work contains Tasks, Projects and Approvals. Configuration contains Workflows,
Connections, Models, Skills & instructions and Administration. These are groups
for the same user; existing viewer/operator/admin checks remain unchanged.
Workflow configuration retains graph editing and direct graph/agent URLs.
Existing Runs, Activity, event, message and diagnostic routes remain accessible.
Configuration drafts must survive navigation as they do today; durable task drafts
must additionally survive reload. No global onboarding overlay is reintroduced.

Outcome presets (understand, implement, investigate, review) prepare editable
instructions and expected outputs. They grant no tools, create no hidden agents
and do not change a shared graph. A synthetic demonstration is clearly labeled
and cannot masquerade as real model/MCP evidence.

## New task and workflow compatibility

1. Describe the request or choose a preset; optionally supply an issue URL/ID.
2. Select a registered project when repository access is needed and a saved
   workflow, such as Software Factory.
3. Review the resolved input target, workflow version, models, effective grants,
   required connections, execution location and environment. Start explicitly.

A workflow needs a reviewed launch binding: either a specific node receiving an
operator instruction, or a channel receiving a typed payload. For a channel,
validate the type, payload and target against the saved topology. Multiple possible
entry points require selection; never choose the first node or inject the request
into every agent. Graphs with background, startup or cyclic behavior need a
supported launch/completion binding before being advertised as task-ready.

The launch binding also declares the terminal result source (a terminal output
channel or supported target execution outcome). Intermediate messages and model
claims of completion cannot close a multi-agent task. Unsupported completion
semantics produce an actionable incompatibility before launch.

Persist the reviewed graph ID/version and resolved launch parameters. Revalidate
at admission: a changed graph or permissions requires renewed review, not silent
use of a newer definition. Starting a stopped workflow can trigger other agents;
show that effect explicitly. Do not promise historical-version execution unless
the engine can bind that version for the whole task. Initially reject incompatible
active graph versions rather than updating a running graph behind the user.

An issue source uses an explicitly chosen MCP connection and granted read tools.
The server validates source/tool availability and preserves the request draft
through authorization repair. Retrieved issue text is untrusted task data. Local
MCP stdio processes remain a Phase B capability, not an assumed Docker feature.

## Durable metadata and admission

Add revisioned project/task metadata through the existing persistence boundary;
no second scheduler, broker or execution engine. Schema, migrations and public
endpoint shapes must be implemented with store conformance and API tests before
being added to OpenAPI. The global Tasks page requires bounded installation-wide
listing; a graph-only endpoint proposal is insufficient for this navigation.

A project record needs an opaque ID, display name, repository reference/location,
execution-location capability, revision and preferred environment. Registering a
path grants no new filesystem access. Native paths on the user's computer are
unavailable to a remote server unless explicitly supported; secrets do not belong
in repository URLs or project metadata.

A task record needs ID, revision, actor, title/request, optional project reference,
workflow ID/version, launch binding, reviewed environment, timestamps, admission
key, status/blocker, authoritative message/execution links and result references.
Store only credential references. Apply existing content limits and redaction;
request text and tool payloads must not become metric labels or audit log dumps.

Creation saves a draft. Start atomically claims its revision and persists an
admission intent before dispatch. Retries with the same key and content return
the original task; conflicting content is rejected. The admission intent and
engine dispatch need a durable deduplication/reconciliation handshake. A broker
publish timeout is not proof of failure: mark it unresolved and reconcile rather
than blindly publish again. Do not infer exactly-once remote tool side effects.

Server-owned task correlation propagates through operator inputs, channel
messages, turns and delegated children; client/runtime supplied IDs cannot attach
work to an arbitrary task. A turn must not batch unrelated tasks. Persist explicit
membership rather than infer it from time, actor, node, run ID or recent events.
Historical executions lacking task identity remain historical runs, not invented
tasks. Restart recovery loads durable bindings and reports uncertain/gapped state
without presenting success or automatically repeating work.

## Concurrent reuse and workspace boundaries

Selecting a workflow reuses its saved definition. Admission creates an explicitly
reviewed execution copy (`task-run-<task suffix>`), preserving normalized runtime,
model, grants, policies and channel definitions. The review explains that starting
runs the entire workflow and the request targets only the selected entry agent.
This replaces the earlier proposed shared-graph serialization approach: a copy
reuses existing graph-scoped queues, memory, approvals, children and managed
workspaces without changing the original definition or adding a second engine.
Copies are visible in workflow listings and linked from task details; they are
not silently created or usable as templates. Legacy graph mutation routes reject
the reserved namespace. Task control routes verify the persisted ownership binding.

Two tasks therefore have distinct managed roots and engine state. This does not
isolate OS processes or external resources referenced by tools. Existing absolute
workspace paths and environment definitions are currently blocked at admission;
registered local project worktrees and task-owned environment preparation remain
pending. No automatic commit, push or cleanup occurs.

## Lifecycle, results and interruption

Task states are draft, queued, running, succeeded, failed, cancel_requested,
cancelled and interrupted. A separate blocker describes approval, capacity,
connection repair or uncertain dispatch; approvals are not successful completion.
Terminal status comes from the declared completion binding and settled relevant
work. A successful intermediate agent turn cannot imply whole-task success.

The task view presents request, progress, required user action, result, changes
and test evidence. Tool outcomes remain distinct from agent summaries. Partial
telemetry is explicitly identified. Details link to existing executions, events,
approvals, traces, token budgets and file inspection without broadening access.

Cancel targets task-owned queued input and running roots/descendants through the
existing control mechanisms. It must not stop unrelated graph activity. Keep
cancel_requested until workers/leases settle; an unavailable runtime does not
justify claiming cancellation completed. If task-scoped cancellation is not yet
supported, show that limitation instead of substituting Stop all. Retain bindings
and budget/accounting evidence needed for reconciliation under retention policy.

Review readiness is separate from execution success. A result may be ready for
human review without a commit. Open in VS Code is available only for an accessible
local workspace; remote paths are not presented as local paths. Commit, push,
merging and cleanup are separate explicit decisions. Browser file inspection
remains read-only; manual editing/review in the user's editor is supported.

## Capability gates and delivery dependencies

FLOW-02 may implement navigation while preserving existing execution views. It
must not relabel all agent turns as complete user tasks. FLOW-03 owns durable
metadata/admission and launch compatibility before enabling New task. FLOW-04 adds
issue input; FLOW-05 adds capability-aware environment selection; FLOW-06 adds
aggregated tracking; FLOW-07 adds review handoff. Native unavailable capabilities
remain visibly unavailable until LOCAL tasks deliver them.

Local worktree executes with host OS authority and is not a sandbox. Show brief
English help linking to [Why Git Worktrees Matter](https://woliveiras.com/posts/why-git-worktrees-matter-coding-agents/).
Sandbox selection retains existing environment approval and runtime policies.
No automatic fallback from sandbox to native, secret import or expanded grants.

## Required implementation acceptance matrix

| Boundary | Required evidence |
|---|---|
| Navigation | Old deep links, OAuth return, keyboard/narrow layouts, preserved drafts and role-denied controls. |
| Admission | Missing/ambiguous targets, invalid payloads, stale graph/revision, denied grants, unavailable model/connection and idempotent duplicate start. |
| Persistence | SQLite/filesystem/PostgreSQL adapter coverage as applicable; reload, publish timeout, crash between intent/dispatch and restart reconciliation. |
| Scope | Two requests sharing a workflow cannot mix messages, memory, files, approvals or results; legacy paths respect admission constraints. |
| Completion | Multi-stage/cyclic workflow boundaries, tool failure with agent success, missing events and declared result evidence. |
| Cancellation | Pending inputs, running parents/children, repeated cancel, unavailable runtime and no impact on unrelated tasks or token totals. |
| Review | Dirty checkout preservation, unsupported remote editor handoff, no automatic commit/push and protected cleanup. |

Each behavior delivery requires unit and real component-boundary integration tests,
relevant browser E2E, make test, make lint and make build. This design-only delivery
requires documentation/link checks, not new runtime tests. Human walkthrough,
external provider consent and clean native installation remain final acceptance;
no implementation, deployment or external validation is claimed here.

## FLOW-03 foundation — durable drafts (2026-10-19)

Implemented: operator/admin creation and revision-protected editing of drafts,
viewer reads, bounded cursor listing, explicit saved workflow version and enabled
entry-agent selection, five editable outcome choices, reload/resume and a read-only
requirements preview. Drafts use existing installation settings CAS storage under
`tasks/drafts/`; no new database service or execution engine is introduced. The
UI saves on explicit request, not automatically; save before leaving an unsaved
form. A stale graph version or revision does not overwrite the draft.

The review endpoint returns current graph/provider/environment validation plus
blockers. Draft review blocks start until an enabled result agent is selected and current requirements pass.
Saving/reviewing never injects a message, starts an agent, modifies a graph or grants
a tool. The task form currently targets an agent; channel bindings, registered
projects, current effective grant presentation, project-specific admission, channel-target input and broader workflow completion semantics remain pending.

`task.draft_save_requested` records metadata-only mutation intent before storage
CAS. Audit failure blocks the save. This event does not claim a save succeeded;
the revisioned draft is the authoritative outcome. History contains no request
body in this event. SQLite and filesystem reopen tests prove draft persistence;
API tests prove revision/version errors and viewer denial. This foundation is not
full FLOW-03 completion or standalone installation acceptance.

## FLOW-03 execution copies (2026-10-19)

Admin-only start preserves the existing graph-creation permission. Operators can
prepare drafts, and viewers can inspect evidence. Start claims the draft revision
before preparing resources, records metadata-only transition intents, inserts a
stable operator-message ID, then starts the copy. Repeated start requests return
the existing state without another dispatch. Submitted requests are immutable.
Uncertain partial preparation remains visible and is never automatically retried
by the task API. Normal engine recovery/redelivery rules still apply to work
already admitted; this is not an exactly-once guarantee for external tool effects.

Progress derives from persisted copy executions and current replica, queue and
approval state. The explicitly selected result agent supplies the result; active
work and pending approvals prevent success. Failed/interrupted turns prevent a
success projection. At the 1,000-execution inspection cap, completion is unknown.
The stored admission status and observed progress are separate; the list displays
the observed state when available. Missing telemetry is not evidence of success. Once/event
workflows are supported initially; unsupported modes are blocked before start.

Stop affects only the owned copy, waits for stopped workers in the progress view,
and retains queued messages, workspace files and history. It does not restart
retained messages. Generated summaries remain distinct from tool outcomes, with
links to execution details and workspace inspection. This increment does not
complete FLOW-03, FLOW-06 or the standalone installation.

## Registered project worktrees (2026-10-19)

Projects registers an absolute repository root on the Forge server, with admin
write/viewer read checks. Registration resolves symlinks, verifies Git and records
an immutable initial HEAD; it does not start agents or copy uncommitted files.
The task explicitly selects a project and full commit. Review and admission
recheck the registered path and commit. A workflow with its own repository/path
override cannot silently replace this choice.

Admission creates `leforge/task-run-<id>` in the managed task workspace from the
reviewed commit. Existing directories or branches cause an error, never force
replacement. Failure preserves partial resources for inspection. Source dirty
files and HEAD remain unchanged; different tasks have different worktrees and
branches. Git hooks and fsmonitor are disabled for these management commands;
repository-configured checkout filters can still run, so task start authorizes
local repository processing under the server OS account. Worktrees do not isolate
OS permissions. No commit, push, merge, source reset or cleanup is automatic.

The UI explains server versus container paths and links worktree background
information. A container-mounted repository still runs commands in the container;
this delivery does not pretend it is the user's host. Native packaging, automatic
editor handoff, preferred environment configuration and explicit project updates
remain pending. Existing file/diff inspection is linked from task progress.

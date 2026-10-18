# Native Delegation Profiles and Policy

Native agents can request a temporary analysis execution through the tool gateway.
Grant the exact `delegation.request` tool explicitly; wildcard grants do not enable
it. Existing gateway constraints and approval rules still apply. External runtimes
and standalone gateway calls cannot delegate.

```json
{"template":"analysis","objective":"Review the supplied design and return concerns"}
```

Only these two fields are accepted. The selected installation profile uses the
parent's resolved provider/model, one inference request and no tools, workspace,
memory, conversation history or delegation callback. It never edits the graph.
This empty permission set is a subset of the parent's permissions. Profiles cannot
inherit nonempty tool grants in this delivery.

## Installation profiles and policy

Administrators manage **Administration → Delegation**. Viewers and operators can
read settings but cannot modify them. Saves require review in the console and
compare-and-set revisions in storage; conflicts never retry silently.

Profiles contain an ID, name, guidance, enabled flag and output allowance. The
built-in `analysis` profile is available at revision zero until customized.
Custom profiles are selected through the tool's `template` field. Mention the
profile ID in the parent's instructions when assigning a specialization. Guidance
cannot add tools, model overrides, credentials, runtimes or workspace access.
The effective output allowance is the minimum of policy, profile and parent limits.

Policy exposes enabled, approvalMode (`required` or `automatic`) and the limits
below. Defaults preserve the previous mandatory-approval behavior. Disable
profiles rather than deleting them; historical execution snapshots remain intact.

| API | Minimum role | Contract |
|---|---|---|
| GET /api/v1/delegation-policy | viewer | Policy plus revision and updating actor. |
| PUT /api/v1/delegation-policy | admin | Full policy; If-Match required, including 0 for defaults. |
| GET /api/v1/delegation-profiles | viewer | Built-in and custom profiles, including disabled profiles. |
| POST /api/v1/delegation-profiles | admin | Create a custom profile; duplicate IDs conflict. |
| GET /api/v1/delegation-profiles/{id} | viewer | Profile and revision. |
| PUT /api/v1/delegation-profiles/{id} | admin | Full profile, matching ID, required If-Match. |

Updates do not mutate running children. Each child stores the policy and profile
snapshot with their revisions and objective in its protected execution record.
A waiting request checks revisions and enabled flags again before inference;
changed configuration produces an error and requires a new request/decision.
Already-started work retains its snapshot; disabling is not a kill operation.
Capacity admission counts existing children, even after the limit is reduced.
This works through shared storage leases across service instances; filesystem
storage retains its existing single-instance restriction.

## Approval and result

In required mode, each child waits for an operator/admin decision in Approvals.
Automatic mode skips this delegation decision only; explicit grants and any
configured generic tool approval remain mandatory. The request shows
its objective, parent/child IDs, model, empty effective tool set and deadline.
OAuth consent and existing tool grants cannot approve delegation automatically.
An additional generic tool approval applies if configured on the tool grant.

After approval, the server checks the deadline and capacity ownership again.
The parent waits for a structured tool result containing childExecutionId,
parentExecutionId, status, result and usage, then continues its native turn.
Rejection, expiration, provider failure, missing/truncated/oversized results and
attempted child tool calls return errors to the parent. No partial result is
reported as success. Provider diagnostic payloads are not returned.

Children are persisted as executions with parentExecutionId, delegationTemplate,
delegationModel and approvalId. Runs labels them and provides an Execution tree tab. Objectives are in protected
approval/execution records and results in execution
records, not general delegation telemetry.
`delegation.requested`, `delegation.started` and `delegation.completed` events
carry parent ID, template and status with normal execution/trace correlation.

## Bounded limits and recovery

| Setting | Default | Allowed range |
|---|---:|---:|
| maxDepth | 1 | 1 only (no recursion) |
| maxChildrenPerExecution | 3 | 1–32 |
| maxConcurrentChildren | 4 | 1–32 |
| maxDurationSeconds | 120 | 1–600 |
| maxOutputTokens | 1024 | 1–4096 |
| maxObjectiveBytes | 4096 | 1–16384 |
| maxResultBytes | 16384 | 1–65536 |

The profile output allowance is also 1–4096 tokens. Durations include approval
and respect the shorter parent deadline. Failed/rejected admitted requests count
against the child count. Capacity includes approval waits and rejects exhausted
requests without inference. Children cannot override installation limits.
No automatic retry, recursion or durable continuation is provided.

Parent cancellation propagates through the request context. Pending approvals
are expired on cleanup; restart recovery interrupts unfinished children and
expires their pending approvals. Leases expire after the bounded deadline plus
10 seconds if cleanup cannot run. Providers must honor request cancellation;
this is not process isolation for a non-cooperating provider.

Reported child usage is recorded separately and included in parent usage. Do not
sum parent and child totals. Native gateway inference is counted once, not again
as runtime-local usage. Existing historical usage rows are unchanged. Admission
uses a conservative byte-based input estimate and remaining parent token allowance;
reported output over the limit is rejected. These are not a guaranteed monetary
or provider-billing ceiling. Configurable cumulative budgets require stronger
provider-specific enforcement and remain planned.

Arbitrary tool-bearing children, other runtimes/models, external callbacks,
idempotent delegation requests and enforceable cumulative budgets
remain planned. Full configurable token/cost budgets are not implemented.
No installation activation or
live-provider acceptance is claimed by the automated fixture tests.

## Configuration audit

`delegation.policy.updated` records revision, enabled and approvalMode;
`delegation.profile.updated` records profileId, revision and enabled. Both carry
the administrator actor. Instruction text and objectives are omitted from these
events. Stored settings include updating actor and timestamp; execution snapshots
retain the applied profile content. As elsewhere in the platform, event-bus
publication is best effort; settings/execution persistence errors fail the call.


## Execution tree and interruption

Open **Runs → an execution → Execution tree** from either a parent or child.
The tree shows persisted relationships, status, agent/profile, start/finish time,
duration and inclusive reported token usage. It is independent of the recent
100-execution list and event-history window. Queries reject inconsistent families,
cycles or more than 256 nodes / 16 levels rather than presenting a truncated tree.
These read bounds do not enable recursive delegation: native children still have
no tools and maximum delegation depth remains one.

Viewers can read; operators and administrators can request interruption after
reviewing the target and scope. Approval or OAuth consent does not grant this
operator capability to agents.

| API | Minimum role | Contract |
|---|---|---|
| GET /api/v1/executions/{id}/tree | viewer | Root/selected IDs and bounded nodes containing execution and optional cancellation metadata. |
| POST /api/v1/executions/{id}/cancel | operator | Body `{"scope":"child","confirm":true}` or `{"scope":"tree","confirm":true}`; `child` requires a delegated execution. |

A child stop affects that child only; its parent receives the tool error and may
continue. A tree stop resolves the root even when requested from a child and
cancels the root plus active descendants. Requests use immutable execution IDs;
other trees, saved graph configuration, grants and future runs are unchanged.
Inputs of an explicitly cancelled root are terminated rather than automatically
requeued, and consumed operator messages are not automatically resubmitted.
Already-published effects cannot be rolled back.

HTTP 202 acknowledges a persisted request, not completed cancellation. Repeated
requests preserve the first actor/time. HTTP 200 with `alreadyTerminal:true`
means no new request was necessary, including a retry after the worker has
finished an earlier interruption request. The original actor/time remain intact.
The worker owns the final execution status;
the console polls and shows requested versus terminal outcomes separately.
Pending synchronous approvals close as expired on cancellation, even when their
timeout policy would approve. Child cleanup releases its capacity slot.

Workers poll shared storage every 250 ms and check cancellation at gateway
boundaries; callback operations inherit the execution context. Storage read
failures fail closed. This is cooperative interruption: providers/runtimes must
honor context cancellation; this is not a hard process kill or an undo operation.
A late successful runtime result is not reported as success after interruption.
Persisted markers survive service restart; existing interrupted-execution recovery
still applies. Markers are retained with installation settings; automatic marker
retention/garbage collection is not implemented. Filesystem storage remains
single-instance; live multi-process PostgreSQL validation is pending.

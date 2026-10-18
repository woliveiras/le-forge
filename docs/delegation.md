# Dynamic Delegation Foundation

Native agents can request a temporary analysis execution through the tool gateway.
Grant the exact `delegation.request` tool explicitly; wildcard grants do not enable
it. Existing gateway constraints and approval rules still apply. External runtimes
and standalone gateway calls cannot delegate.

```json
{"template":"analysis","objective":"Review the supplied design and return concerns"}
```

Only these two fields are accepted. The immutable built-in template uses the
parent's resolved provider/model, one inference request and no tools, workspace,
memory, conversation history or delegation callback. It never edits the graph.
This empty permission set is a subset of the parent's permissions. Configurable
profiles and nonempty grant inheritance are not implemented.

## Approval and result

Every child waits for an operator/admin decision in Approvals. The request shows
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
delegationModel and approvalId. Runs labels them and links to the parent; this
is not yet an interactive execution tree. Objectives are in protected approval
records and results in execution records, not general delegation telemetry.
`delegation.requested`, `delegation.started` and `delegation.completed` events
carry parent ID, template and status with normal execution/trace correlation.

## Fixed limits and recovery

- Depth one; at most three admitted requests per parent turn, including failures.
- Four installation-wide slots, including approval waits, coordinated by shared
  storage leases. Capacity exhaustion returns an error without starting a child.
- Two minutes including approval, or the shorter parent deadline.
- Objective at most 4096 bytes, requested output at most 1024 tokens (or the
  smaller parent allowance), result at most 16 KiB.
- No automatic retry, recursive execution or durable continuation of a child.

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

Policy/profile administration, automatic approval mode, arbitrary tool-bearing
children, external callbacks, idempotent requests, independent child/tree stop
controls and full tree navigation remain planned. No installation activation or
live-provider acceptance is claimed by the automated fixture tests.

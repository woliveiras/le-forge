# Console navigation

The console separates installation-wide configuration from graph operations.

| Destination | Scope |
| --- | --- |
| Graphs | Existing graph editor, activity, files, messages and advanced YAML |
| Connections (`#/connections`) | Installation MCP connections, OAuth setup, discovered tools and call history |
| Models (`#/models`) | Model providers, credentials and explicit connection/inference tests |
| Administration (`#/administration/health`) | Installation health and policy sections |
| Skills & steering | Existing installation instruction management |
| Approvals | Existing approval and application-review flows |

Administration has independent hash routes for `health`, `network`, `resources`,
`retention` and `tools`. Opening a section loads only that section's resources.
Health checks do not test model inference; use Models for explicit provider tests.

Existing `#/integrations`, `#/integrations/connect?...` and
`#/integrations/oauth?...` links still open the MCP destination. Existing OAuth
callback URLs and query validation are unchanged. New navigation uses Connections.
Provider management previously located under Integrations is now under Models.

Roles and API authorization remain unchanged. Viewers inspect; operators can perform
existing permitted tests; administrative edits remain admin-only. An OAuth grant
never assigns tools to agents. Credentials remain managed by the existing server APIs.

## Connection pages

`#/connections` lists installation connections with name/service/ID search and a
status filter. Administrators start creation with **Add connection** at
`#/connections/new`; template links use `#/connections/connect?...`.
Saving opens `#/connections/c/<id>/overview`. The detail sections are:

- **Overview**: discovery state, OAuth authorization/revocation and saved agent grants.
- **Tools**: explicit discovery/refresh, searchable tool names, descriptions and schemas.
- **Usage & history**: existing redacted audit events and filters, always scoped to this
  connection, including after clearing filters or moving to older pages.
- **Settings**: configuration/quota editing and the existing safe deletion workflow.

The list also links to `#/connections/history` for cross-connection history, including
removed connections. History loads on explicit request. Quotas shown are configured
limits, not inferred consumption counters. Graph grants shown on Overview come from
saved graph definitions (including defaults when a node has no override); running
versions can differ and each call still undergoes gateway authorization.

Legacy `#/integrations` setup and OAuth return pages retain the existing combined
setup flow for compatibility. New global navigation uses the list/detail pages.
Editing preserves revision checks, blank-credential semantics, role restrictions,
DCR uncertainty handling and explicit OAuth consent. Missing connections and API
failures are shown explicitly; navigation never registers or authorizes a client.

A scalable graph selector is still planned in `wip/PLAN.md` and `wip/TASKS.md`. Developer usability validation and
installation activation are separate from automated test coverage.

## Agent configuration

Open **Agent configuration** in a graph, or **Configure agent** in its canvas
inspector. Select an agent and use General, Instructions, Tools & permissions or
Advanced. Deep links have the form
`#/g/<graph>/agent?agent=<node>&section=<general|instructions|tools|advanced>`.
Reload and browser history preserve the selected agent/section; unsaved drafts
remain in memory and are not persisted across a page reload.

General contains identity, runtime and model fields. Instructions edits the prompt
directly in the graph draft. Tools & permissions reuses the existing grant pickers,
approval gates and isolation fields. Advanced contains loop, replica/workspace and
turn-limit settings. The boundary summary distinguishes default grants from agent
overrides and shows read-only workspace access and configured isolation. It is a
draft summary, not proof of currently effective runtime access: installation policy,
constraints and gateway checks still apply.

Section and agent changes share the existing whole-graph draft. **Review & save**
lists all changed document paths, including other agents, channels, graph settings
and extension fields, before **Save graph draft** runs server validation and the
existing version check. Values are not copied into the review; inspect their editor
sections or Definition. Validation/dependency failures and revision conflicts keep
the draft available for correction or the existing explicit conflict workflow.

Viewers inspect a read-only page; operators and administrators can edit under the
existing backend authorization. Opening a section never grants tools or starts an
agent. Execution controls remain on the graph's operational views. The existing
canvas inspector and Definition editor remain available for compatibility, including
fields not represented by dedicated form controls. No schema or runtime permission
contract changes are introduced by this organization.

## Runs

Open **Runs** in a graph to search/filter its latest 100 execution records.
`#/g/<graph>/runs?execution=<id>&section=<result|timeline|tools|messages|diagnostics>`
opens a dedicated detail, even when that execution is outside the list window.
The detail verifies the returned execution belongs to the selected graph before
showing its contents or loading associated records. A missing execution and a
failed request are explicit error states with retry controls.

- **Result** shows the persisted summary/error, execution outcome, recent blocked
  tool calls, linked approvals and a link to the current graph workspace.
- **Timeline** and **Tool calls** show exact execution-ID matches within the latest
  300 events for that agent, ordered by sequence. Tool denial does not overwrite
  the persisted execution status. This window is not a complete execution trace.
- **Messages** shows explicit execution/input/output ID matches within the latest
  100 graph messages; a shared agent or timestamp never establishes attribution.
- **Diagnostics** shows recorded identifiers, version, replica, iteration and trace
  context. Its agent link opens current configuration, which can differ from the
  execution's graph version.

Loaded sections refresh every five seconds and offer manual retry. Failed requests
are not presented as fresh data or successful empty results. Approval controls reuse
the existing role gates (environment decisions remain admin-only); opening a run
never starts an agent or grants permissions. Graph execution controls remain in the
operational views. Workspace links inspect current files, not historical snapshots
attributed to a run. Activity remains available and links to the new detail.

These pages use existing authorized APIs. Full historical pagination, inferred task
progress, replay/retry controls and execution-specific workspace snapshots are not
introduced by this interface change.


## Graph selector and agent list

The header's **Select graph** control searches graph names and IDs, shows saved
runtime state and marks unsaved graph drafts. Switching to a visited graph restores
its last destination. The canvas has a searchable agent list: select an agent to
inspect its panel or use **Configure** to open its dedicated configuration page.
Names reflect the local draft; runtime state belongs to the saved graph and is
explicitly unavailable when status loading fails. On narrow screens the list moves
above the canvas.

Visited graph editors remain in memory for the current page session. Switching
graphs or installation destinations preserves each graph's draft, selection and
editor state; inactive editors pause their effects. Nothing is automatically saved
or granted. Draft saves retain their original base revision, so an external update
still requires conflict resolution. Missing graphs and failed reloads retain the
local editor and display an error instead of silently discarding edits.

Drafts are not persisted in browser storage and do not survive closing or reloading
the page. A browser leave warning is registered while graph drafts are dirty.
Existing role gates, server validation and optimistic concurrency remain in force.

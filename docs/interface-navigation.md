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

Dedicated agent configuration pages, a scalable graph selector and Runs pages are
still planned in `wip/PLAN.md` and `wip/TASKS.md`. Developer usability validation and
installation activation are separate from automated test coverage.

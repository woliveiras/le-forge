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

This is the first slice of the UX reorganization. Dedicated connection details,
agent configuration pages, a scalable graph selector and Runs pages are still planned
in `wip/PLAN.md` and `wip/TASKS.md`; existing forms and graph workflows remain available.

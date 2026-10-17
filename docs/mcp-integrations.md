# Priority MCP integration contracts

Research date: 2026-10-17. These are source-confirmed vendor contracts and planned
LeForge deliveries. Implemented compatibility is identified below; this is not
live-service certification.
The current [generic client](mcp.md) supports public HTTPS Streamable HTTP,
protocol 2025-06-18 and optional Bearer credentials. It rejects URL queries,
redirects and private destinations. Generic pre-registered OAuth now supports the
browser lifecycle; see [implementation and limits](mcp.md#browser-oauth-pre-registered-clients).
Vendor qualification and other credential modes remain pending.
See also the [additional template survey](mcp-templates.md).

## GitHub

The official remote endpoint is `https://api.githubcopilot.com/mcp/`, using HTTP
MCP with Bearer PAT or OAuth. A host implementing OAuth must configure its own
GitHub App or OAuth App. Enterprise Cloud with data residency has a different
host; GitHub Enterprise Server requires separately hosted server infrastructure
and is outside the initial github.com acceptance. [Official server](https://github.com/github/github-mcp-server)

Remote toolset paths and `/readonly` allow narrower catalogues. Prefer a reviewed
read-only profile before enabling write tools; toolsets do not replace token
permissions or gateway grants. [Remote configuration](https://github.com/github/github-mcp-server/blob/main/docs/remote-server.md)

There is no single minimum scope for every tool. For selected repositories, map
read operations to fine-grained Contents, Issues, Pull requests and Actions read
permissions as applicable; enable corresponding write permissions only for the
chosen mutations. Review each underlying API endpoint, including any additional
permission requirements. [GitHub permission reference](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)
Classic PAT scopes such as `repo` and `read:org` are broader alternatives, not the
default minimum. Discovery cannot prove authority: fine-grained tokens do not
support the server's classic-scope filtering. [Scope filtering](https://github.com/github/github-mcp-server/blob/main/docs/scope-filtering.md)

Implemented OAuth compatibility: the GitHub.com profile handles omitted client
authentication metadata, confidential POST exchange/renewal, comma-separated scopes
and OAuth errors returned with HTTP 200. Unit, TLS API integration and browser
fixtures cover the contract. Follow the [manual app registration guide](mcp.md#registering-githubcom-oauth-for-a-local-leforge); live consent remains pending.

Remaining delivery: guided OAuth app setup and PAT guidance; reviewed
per-tool permission matrix; repository selection with an honest distinction
between UI defaults and enforced token restrictions; discovery and read-only
smoke test; separately approved issue/PR write tests. Diagnose token expiry,
organization approval/SSO and policy denials without exposing credential values.

## Shared Atlassian Cloud contract

Use `https://mcp.atlassian.com/v2/mcp` with remote HTTP MCP. Bitbucket Cloud is
supported when its workspace is linked to an organization. For a gateway, the
vendor offers `https://mcp.atlassian.com/v2/mcp?tools=all` for flat paginated
discovery. The legacy `/v1/sse` endpoint is no longer the onboarding target.
[Official server](https://atlassian.github.io/atlassian-mcp-server/)

Authentication options are OAuth 2.1, personal scoped API tokens via
`Authorization: Basic <base64(email:api_token)>`, or service-account API keys via
`Authorization: Bearer <api_key>`. Token authentication requires organization
admin enablement. API tokens are not bound to one cloud ID; IP restrictions still
apply. They are not interchangeable with an OAuth access token or an arbitrary
Bitbucket REST credential. [API-token contract](https://support.atlassian.com/atlassian-ai-gateway/docs/configure-authentication-via-api-token/)
OAuth grants product/site access through consent and sends a Bearer access token;
redirect/domain policy and renewed consent must be handled. Runtime authorization
metadata, registration support and refresh behavior must be validated during
implementation rather than assuming the generic Atlassian REST OAuth flow.
[OAuth contract](https://support.atlassian.com/atlassian-ai-gateway/docs/configure-oauth-2-1/)

The following are literal MCP permission scopes, not ordinary REST API scope
names. Request only those needed for the chosen profile:

| Integration | Read/search scopes | Optional mutation scopes |
| --- | --- | --- |
| Jira | `read:jira:agent-interface`, `search:jira:agent-interface` | `write:jira:agent-interface`; exclude `delete:jira:agent-interface` and `manage:jira:agent-interface` from initial defaults |
| Confluence | `read:confluence:agent-interface`, `search:confluence:agent-interface` | `write:confluence:agent-interface` |
| Bitbucket Cloud | `read:bitbucket:agent-interface` | `write:bitbucket:agent-interface` |

The default catalogue also exposes `discover` and dispatcher tools
`executeRead`, `executeWrite`, `executeDestructive`. The flat catalogue is the
proposed LeForge route to exact per-operation grants; broad dispatcher grants
must not provide an escape from those grants. Discover accessible resources and
resolve `cloudId` as required. [Supported tools and scopes](https://support.atlassian.com/atlassian-ai-gateway/docs/supported-tools/)

Planned common work: a typed, narrowly validated `tools=all` option with bounded
pagination, retaining rejection of unknown/secret query parameters; Basic
credential support; OAuth lifecycle; product-specific tool filters and explicit
site context. Qualify actual protocol version, schema/name compatibility and
catalogue size against current bounds. Do not relax limits silently. Show site,
project, workspace and space selections as defaults unless a verified policy
actually restricts arguments. User permissions and LeForge grants both apply.
Jira/Confluence/Bitbucket Data Center are separate future hosting contracts.

## Per-product acceptance and deliveries

| Integration | Planned guided flow | Required product-specific evidence |
| --- | --- | --- |
| GitHub | Select github.com, credential mode, repository/tool profile and exact agent grants | Read selected repository/issue/PR; deny inaccessible repository; approved write in test repository; expiry, scope denial and disable during approval wait |
| Bitbucket Cloud | Verify organization-linked workspace, select workspace/repository and read or reviewed write profile | Repository/file/PR read; unlinked workspace failure; cross-workspace denial; approved test PR/comment; no pipeline execution or merge granted implicitly |
| Jira Cloud | Select accessible site/project; explain read/search versus write scopes | Issue/JQL read; project permission denial and missing scope; approved test issue/comment; field/transition validation and no delete/manage by default |
| Confluence Cloud | Select accessible site/space; explain read/search versus write scopes | Page/search read; restricted-page denial; approved test page update, version conflict and permission failure; returned attachment links remain unfetched |

Every row requires unit tests, real-boundary integration tests and browser E2E.
Fixtures cover authentication failures, protocol errors, pagination, cancellation,
redaction, stale revisions and approval rejection. Opt-in live tests require
operator-provided credentials and authorized test resources; capture sanitized
protocol/catalogue evidence and distinguish fixture coverage from live acceptance.
Never replay a possibly completed write automatically after a lost response.

## Shared completion criteria

Version templates and record the reviewed source/date; guide credential setup,
show granted access, discover tools, authorize each agent call and support
revocation. Add deletion, aggregate quotas and a filterable call/approval/outcome
history without exposing secrets or sensitive payloads. Implement OAuth state,
PKCE, callback/issuer validation, encrypted tokens, refresh concurrency and
reauthorization errors. Do not forward credentials across hosts or use a template
to bypass the existing network policy. Additional-header support belongs to the
broader template backlog; it is not a prerequisite for the initial PAT GitHub path.

The survey is complete at documentation level. OAuth registration/refresh details,
exact tool-to-API permission mappings and live acceptance are explicit remaining
implementation qualifications. The additional candidates do not expand Release 1
acceptance automatically.

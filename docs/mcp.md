# MCP connections and agent permissions

MCP supports registration, discovery and remote tool invocation through the gateway.
Native agents and external runtime callbacks use the same authorization boundary.
Models receive only the currently available tools explicitly granted to their agent.

In **Integrations → MCP connections**, an administrator can create or edit multiple
connections, set a bearer credential, and disable/re-enable a connection. IDs are
stable; edits require the current revision. A blank credential preserves the
stored value; explicit clear removes it. Changing the endpoint requires replacing
or clearing an existing credential so it cannot be forwarded accidentally.
Credentials use the installation's existing AES-GCM secret box and are bound to
the connection ID. Read responses expose only `credentialConfigured`.

Operators and administrators can test or refresh a connection. In this increment,
both actions initialize a new MCP session and refresh the tool catalogue. The
server negotiates protocol `2025-06-18`, sends `notifications/initialized`, and
reads every supported `tools/list` page. JSON and SSE POST responses are supported.
Session identifiers stay in memory and are terminated after discovery where the
remote server supports DELETE. Server-requested client operations are not executed.

The supported transport is remote **Streamable HTTP over HTTPS**, following the
[MCP transport specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
and [tool discovery contract](https://modelcontextprotocol.io/specification/2025-06-18/server/tools).
TLS verification is enabled. Redirects and environment HTTP proxies are disabled;
DNS answers are validated at connection time and the validated IP is dialled
directly. Private, loopback, link-local and reserved addresses are denied. No
local server processes or arbitrary commands are launched. Endpoint URLs cannot
contain user information, query parameters or fragments.

Discovery has a 15-second deadline, a 2 MiB response/aggregate catalogue limit,
at most 20 pages and 500 tools. Tool names currently accept only ASCII letters,
digits, underscores and hyphens, up to 128 characters. Schemas must be JSON objects
with `type: object`; descriptions are bounded to 8 KiB. Duplicate names, repeated
cursors, unsupported protocol versions and malformed responses fail discovery.
Errors exclude upstream payloads; catalogues containing the supplied bearer value
are rejected. A failed refresh clears the selectable catalogue and reports
`unavailable`; existing grants remain visible in the graph and can be removed.
Changing endpoint, credential or enabled state also invalidates discovery.

In an agent's **MCP permissions** section, select exact
`mcp.<connectionId>.<toolName>` grants and save the graph. Validation rejects
unknown, wildcard and disabled MCP grants. Existing non-MCP grants and their
restrictions are preserved. The selector reads `/api/v1/mcp/tools`; this catalogue
is separate from `/api/v1/tools`, which describes the static built-in registry.
The runtime combines both catalogues when preparing model schemas.

Registry changes and discovery produce `mcp.connection.changed`,
`mcp.connection.tested` and `mcp.tools.refreshed` events with actor, connection ID,
revision/status and discovery latency, excluding credentials, endpoint payloads
and tool arguments. Concurrent edits and refreshes use storage compare-and-swap;
a delayed refresh cannot restore a disabled connection.

Every call requires an exact grant; `*`, capability grants and `mcp.*` do not grant
MCP access. Authorization uses the execution's agent principal, not model-supplied
identity. Graph edits apply to subsequent graph runs under the existing execution
snapshot contract. Disabling a connection, changing its endpoint/credential or
refreshing its catalogue is observed on subsequent calls, even in existing runs.
The connection revision is checked again after any approval or preparation wait.
Already dispatched remote operations are not rolled back by disabling a connection.

The existing `requiresApproval` and `rateLimit` grant fields apply to MCP and can
be set in the graph definition. The MCP checkbox selector preserves these fields. Approval
requests show arguments for review; ordinary tool audit events omit MCP arguments
and results, retaining the qualified tool name, agent/execution, outcome and latency.
Metric tool labels aggregate MCP calls under `mcp`. Host constraints restrict the
registered endpoint; filesystem path/command constraints are rejected for MCP.

Each call initializes a fresh session and sends one `tools/call` with the unqualified
remote name and a JSON object (maximum 64 KiB). Calls share the discovery network
policy, a 15-second operation timeout and 2 MiB response bound; session cleanup has
an independent 3-second bound. No transport retry is performed: after a timeout or
lost response, a remote mutation may already have succeeded. Sessions are closed
where supported. JSON/SSE, protocol errors and `isError` tool results are handled.
Tool content blocks and structured content reach the runtime as JSON text, capped
by the gateway (32 KiB by default, with a truncation marker); resource links are not
fetched, and media is not rendered as native model attachments. The remote server
validates tool-specific input schemas; LeForge validates the object envelope and
size. Credentials and upstream error payloads are not exposed as transport errors.

Pending increments include aggregate per-connection quotas, dynamic OAuth client registration,
private-network deployment policy, automatic
session renewal, connection deletion, vendor-specific setup/scopes, call-history
UI, and live GitHub/Bitbucket/Jira/Confluence acceptance. The service selector is
metadata, not a vendor adapter or proof that a vendor supports this transport.

Validation: unit tests cover input/network rules, credential handling, catalogue
limits and concurrent updates; API integration tests use SQLite and a real TLS
MCP fixture; Chromium covers registration, discovery, graph grant persistence,
disable/removal, authentication failure, successful agent invocation and denied calls. Fixture credentials are synthetic;
no live vendor credentials are used by these tests.

Vendor setup is being specified in [priority integration contracts](mcp-integrations.md).
The [template candidate catalogue](mcp-templates.md) distinguishes proposed
configurations from verified compatibility. Both are planning documents; the
current runtime limits described above remain unchanged.

## Versioned templates and browser links

The authenticated `GET /api/v1/mcp/templates` catalogue provides version 1 of
DeepWiki, GitHub read-only, Context7, Hugging Face and W&B. Each template includes
public defaults, authentication guidance, source/review date and validation status.
These presets do not certify vendor compatibility; test each connection before use.

Open `#/integrations/connect?template=deepwiki&version=1` on your installation
(for example `http://localhost:8080/#/integrations/connect?template=deepwiki&version=1`).
The same hash route works on a hosted installation and survives the login reload.
Catalogue links can be copied from the browser. Links accept only one template ID
and one positive version; unknown versions and extra parameters are rejected.
Never include credentials in a link.

The review screen displays the endpoint and authentication guidance. An administrator
must choose **Use template defaults**, review/edit the connection ID and name, enter
any credential in the protected field, and explicitly save. Opening a link performs
no discovery, writes or grants. The saved connection records `template: {id, version}`
and starts untested with no tools. Existing discovery and per-agent authorization
remain separate actions. Duplicate connection IDs fail rather than overwrite.

Template versions are immutable source-controlled defaults. The API rejects an
unknown reference or a service/endpoint that differs from the pinned version.
**Customize without template** detaches that reference before manual configuration;
existing endpoint-change credential protections still apply. Editing a pinned
connection in the console preserves its version. There is no automatic upgrade or
upgrade-preview workflow yet. OAuth is available for pre-registered clients as described below; live vendor
qualification remains planned.

## Browser OAuth (pre-registered clients)

Administrators can select **Use browser OAuth** when saving a connection and
configure its issuer, client ID, requested scopes and optional client secret.
Public clients use token endpoint authentication `none`; confidential clients use
`client_secret_basic` or advertised `client_secret_post`. Missing authentication
metadata defaults to `client_secret_basic` (RFC 8414); an explicitly empty list
is rejected. GitHub.com has the narrowly scoped compatibility profile below.
Automatic client registration and live-provider qualification remain pending.
Historical v1 template guidance describes their original Bearer/no-auth presets;
OAuth settings are configured separately and do not come from a shared link.

Set `server.publicUrl` or `LEFORGE_PUBLIC_URL` to the browser-facing installation
origin. The default is `http://localhost:8080`. Hosted installations require HTTPS;
HTTP is allowed only for localhost/loopback. Paths, credentials, query strings and
fragments are rejected. The OAuth form displays the configured callback via authenticated
`GET /api/v1/mcp/oauth/config`. Register that exact backend callback with the provider:

```text
https://your-forge.example/api/v1/mcp/oauth/callback
```

The callback is derived from this configuration, never the request Host or forwarded
headers. The browser must use that same origin so its binding cookie returns.
This delivery does not change any running installation's configuration.

After saving, review the issuer/scopes and choose **Authorize**. LeForge probes the
MCP resource without credentials, reads `resource_metadata` from its authentication
challenge (or the path-specific well-known location), and verifies that the
configured issuer is advertised for the exact MCP endpoint. Issuer metadata must
match exactly and advertise PKCE S256. Client authentication follows the
advertised methods, the RFC default, or the explicit GitHub.com profile below. Metadata/token/revocation requests use the existing public-only HTTPS
transport, no redirects, a 15-second bound and 64 KiB response limits. Other resource
URI canonicalization/metadata fallback variants need separate compatibility work.
These mechanisms follow the [MCP authorization contract](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization).

The browser goes to provider consent and returns to the backend callback. A
10-minute, single-use transaction binds the connection revision, issuer, callback,
initiating LeForge identity/session and an HttpOnly SameSite=Lax cookie (Secure
on HTTPS). The callback holds the code in bounded server memory and redirects to
a frontend route containing only an opaque transaction handle. **Complete OAuth
connection** requires the original authenticated admin session plus the binding
cookie before exchanging the code with PKCE and a resource indicator. Denial,
expiry, replay, identity mismatch and stale connection revisions fail explicitly.
No access/refresh token or client secret is sent to the frontend. Discovery follows
the exchange; failure remains visible and does not mark the connection ready.
OAuth consent never grants an agent tools.

Transactions are limited to 128 pending entries per manager and expire after ten
minutes; a backend restart loses unfinished authorizations, which must be restarted.
Multi-instance deployments need sticky routing for this in-memory authorization
transaction; distributed pending-transaction storage remains future work. Completed
credentials and issuer endpoints are encrypted/persisted as appropriate and survive
restart: access tokens, refresh tokens and client secrets are encrypted separately
and bound to the connection; issuer endpoint metadata is public configuration.

Before discovery or invocation, a token within five seconds of expiry is renewed
when a refresh token is available. A persisted compare-and-swap claim serializes
renewal; rotated credentials are saved atomically. Waiters have a 15-second bound;
a refresh left interrupted for 45 seconds requires reauthorization. Missing expiry
means the provider did not supply a schedule; an HTTP 401 requires authorization
again. Renewal failure also requires reauthorization, with no replay of a possibly
completed mutation. Requested scopes cannot expand through a token response, and
renewal cannot expand the previously granted scopes. Read the new connection
revision after a renewal; requests already waiting on older revisions can be denied.

**Revoke OAuth** requires confirmation and clears local access/refresh tokens and
the tool catalogue before contacting the provider. A late callback/refresh cannot
restore revoked credentials. The client registration/secret remains for deliberate
reauthorization; changing issuer/client clears the previous client secret. Temporary
disable retains credentials and is distinct from revocation or deletion. Provider
revocation is reported separately as pending, requested, unsupported or failed;
requested means the token revocation endpoint accepted the request, not proof that
an entire provider consent grant was removed. Use the provider's account settings
when necessary, including an ambiguous network failure. In-flight remote actions
cannot be undone. Concurrent discarded token responses trigger bounded upstream
cleanup and an audit outcome; they never restore local access.

OAuth lifecycle events (`mcp.oauth.exchanging`, `authorized`, `refreshing`, `renewed`,
`reauthorization_required`, `revoked`, `revocation_result`, and `cleanup`, all under
`mcp.oauth.`) contain state/outcome metadata, never codes or tokens. API latency and
outcomes use existing route metrics. Any external reverse proxy must also redact
callback query strings; LeForge logs/traces record route paths, not callback queries.

Validation includes unit tests for metadata/configuration and callback boundaries,
TLS/API/persistence integration, concurrent refresh/revocation under Go's race
detector, and Chromium consent/return/renewal/revocation and failure journeys.
Live-provider credentials, provider-specific scopes and production activation are
not covered by these fixture tests.


## Registering GitHub.com OAuth for a local LeForge

The GitHub.com profile requires the exact issuer `https://github.com/login/oauth`
and its exact `/authorize` and `/access_token` endpoints. GitHub currently omits
`token_endpoint_auth_methods_supported`; LeForge uses GitHub's documented
`client_secret_post` with an encrypted registered client secret. Authorization
and renewal use the same method. Comma-separated response scopes are normalized
before scope-expansion checks. HTTP 200 responses carrying OAuth errors are
rejected without displaying upstream descriptions or credentials. PKCE S256,
resource/issuer/session checks, TLS and public-network restrictions still apply.
This profile does not automatically qualify GitHub Enterprise hosts.

For an initial OAuth App connection (no private repository access):

1. In GitHub, open **Settings → Developer settings → OAuth Apps → New OAuth App**
   ([registration](https://github.com/settings/applications/new)). Use a dedicated
   app, for example `LeForge local`; homepage `http://localhost:8080`; authorization
   callback `http://localhost:8080/api/v1/mcp/oauth/callback`. Use the callback shown
   by your LeForge form if the installation origin differs. Device flow is not needed.
2. Register the app yourself, copy its **Client ID**, and generate a client secret.
   Enter the secret directly into LeForge's protected field, never into chat,
   connection URLs, source files or screenshots. The browser's existing GitHub
   session is used for login; it does not replace this application registration.
3. In **Integrations → MCP connections**, create a connection with service `github`,
   endpoint `https://api.githubcopilot.com/mcp/`, **Use browser OAuth** checked,
   issuer `https://github.com/login/oauth`, and that client ID/secret. For this
   initial check, request `read:user offline_access`. The latter requests expiring
   tokens and renewal; it is not repository access. If starting from the v1 PAT
   template, use **Customize without template** to configure this OAuth connection.
4. Save, select **Authorize**, review GitHub's consent and authorize the dedicated
   app. On return select **Complete OAuth connection** in the same LeForge session.
   Check the authorized status and discovered tools, then explicitly grant only
   the desired read tools to an agent. OAuth consent alone grants no agent tools.
5. Use a read operation against your account or a public test repository for initial
   acceptance. Private repository access is a separate decision: OAuth App `repo`
   scope is broad and includes writes. For repository-specific permissions, prefer
   a GitHub App installed only on selected repositories; its live acceptance and
   permission matrix remain separate work. Organizations may require administrator
   approval/SSO authorization.
6. **Revoke OAuth** clears local access. GitHub's discovery metadata currently has
   no standard revocation endpoint, so LeForge reports `unsupported` for provider
   revocation. Remove the grant in GitHub **Settings → Applications → Authorized
   OAuth Apps** as well when revoking upstream access. A refresh failure requires
   reauthorization; do not assume every token has a refresh token.

Evidence: unit contract/error tests, API integration with encrypted storage and
real TLS OAuth/MCP fixtures, and a Chromium consent/renewal/revocation journey
against the GitHub wire-contract fixture. This is not a successful live GitHub
account authorization; that remains pending a user-registered app and consent.

Sources: [GitHub MCP host integration](https://github.com/github/github-mcp-server/blob/main/docs/host-integration.md),
[OAuth authorization and refresh](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps),
[RFC 8414 metadata defaults](https://www.rfc-editor.org/rfc/rfc8414.html#section-2).

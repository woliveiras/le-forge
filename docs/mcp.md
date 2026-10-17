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

Pending increments include aggregate per-connection quotas, OAuth,
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
upgrade-preview workflow yet. OAuth consent/callback/renewal/revocation and live
vendor qualification remain planned.

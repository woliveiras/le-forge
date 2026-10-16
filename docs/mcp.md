# MCP connections and agent permissions

The first MCP increment provides registration, discovery and permission preparation.
**Remote tool invocation is not implemented yet.** MCP tools are not registered in
the runtime gateway or offered to models. Saving a grant does not enable a call.

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
is deliberately separate from `/api/v1/tools`, which describes runnable tools.

Registry changes and discovery produce `mcp.connection.changed`,
`mcp.connection.tested` and `mcp.tools.refreshed` events with actor, connection ID,
revision/status and discovery latency, excluding credentials, endpoint payloads
and tool arguments. Concurrent edits and refreshes use storage compare-and-swap;
a delayed refresh cannot restore a disabled connection.

Pending increments include gateway invocation and runtime revocation, per-agent
call limits and approvals, OAuth, private-network deployment policy, automatic
session renewal, connection deletion, vendor-specific setup/scopes, call-history
UI, and live GitHub/Bitbucket/Jira/Confluence acceptance. The service selector is
metadata, not a vendor adapter or proof that a vendor supports this transport.

Validation: unit tests cover input/network rules, credential handling, catalogue
limits and concurrent updates; API integration tests use SQLite and a real TLS
MCP fixture; Chromium covers registration, discovery, graph grant persistence,
disable/removal and authentication failure. Fixture credentials are synthetic;
no live vendor credentials are used by these tests.

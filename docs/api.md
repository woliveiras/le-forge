# LeForge HTTP API

The control plane (`forged`) exposes one HTTP listener (default `:8080`) with four surfaces:

| Surface | Prefix | Auth | Purpose |
|---|---|---|---|
| Public API | `/api/v1` | bearer (token or OIDC JWT) + role | Graphs, lifecycle, messages, approvals, history |
| Event stream | `/api/v1/events/stream` | bearer or `?access_token=` | Server-Sent Events tail of the event log |
| Runtime callbacks | `/internal/v1/executions/{id}` | per-execution bearer token | Runtime Protocol v1 (see [runtime-protocol.md](runtime-protocol.md)) |
| Operations | `/healthz`, `/readyz`, `/metrics` | none | Probes and Prometheus scrape |
| Console | `/` | (the SPA authenticates against the API) | Static web UI when `server.webDir` is set |

The machine-readable contract is [`api/openapi.yaml`](../api/openapi.yaml) (OpenAPI 3.1). JSON Schemas for
the shared documents live in [`schemas/`](../schemas). This page explains the conventions.

## Authentication and roles

```
Authorization: Bearer <token>
```

| Credential | Configuration | Notes |
|---|---|---|
| Static token | `auth.tokens[]` with `role` and one of `tokenEnv`, `tokenFile`, `token` (inline, dev only); `LEFORGE_ADMIN_TOKEN` adds an admin token | Minimum 12 characters. Compared by SHA-256 digest in constant time. |
| OIDC JWT | `auth.oidc: {issuer, audience, roleClaim}` | RS256 and ES256 (P-256). Keys from the issuer's discovery document / JWKS, cached 1 h. Checks `iss`, `exp`, `aud`. Role from `roleClaim` (default `leforge_role`; string or array, highest wins; default `viewer`). Name from `preferred_username` or `sub`. |
| Disabled | `auth.disabled: true` / `LEFORGE_AUTH_DISABLED=true` | Every caller is `admin`. Local development only; a warning is logged. |

Roles are hierarchical: a higher role can do everything a lower one can.

| Role | Can |
|---|---|
| `viewer` | All `GET`s, `POST /graphs/validate`, event stream |
| `operator` | + start/stop/pause/resume/kill graphs and nodes, scale, inject messages, operator chat, decide approvals, test model providers, build project environments |
| `admin` | + create, update and delete graphs; manage model providers and their credentials; reset project environments |

`?access_token=<token>` is accepted **only on `GET`** requests, because `EventSource` cannot send headers.
Prefer the header everywhere else: query strings end up in proxy logs.

Startup fails if no credential is configured and auth is not explicitly disabled.

## Conventions

* **Content types.** JSON everywhere. Graph documents may also be sent as YAML
  (`Content-Type: application/yaml`; a body that does not start with `{` is parsed as YAML).
* **Field names.** camelCase (Go JSON tags). Message *metadata* keys are snake_case (`correlation_id`).
* **Errors.** Non-2xx responses carry:
  ```json
  {"error": "human readable", "code": "invalid|unauthorized|forbidden|not_found|conflict|rate_limited|internal", "details": {}}
  ```
  | HTTP | `code` | When |
  |---|---|---|
  | 400 | `invalid` | Malformed body/query, bad `If-Match`, invalid action argument |
  | 401 | `unauthorized` | Missing/invalid credential |
  | 403 | `forbidden` | Role too low; on callbacks: tool denied or rejected by a human |
  | 404 | `not_found` | Unknown graph/node/channel/approval/execution, unknown action segment |
  | 409 | `conflict` | `If-Match` mismatch, graph exists, delete while running, approval already decided, pause when not running, provider in use or default |
  | 409 | `secret_key_missing` | A credential was sent but the server has no installation key to encrypt it |
  | 422 | `invalid` | Graph validation failed, or a graph/node start whose agents cannot reach their model provider — `details` is the full `ValidationReport`; provider input invalid — `details.errors[]` with `path` and `message` |
  | 429 | `rate_limited` | Tool grant rate limit (callbacks) |
  | 500 | `internal` | Unexpected; details are logged server-side, never returned |
* **Optimistic concurrency.** `GET /graphs/{id}` returns `graph.metadata.version`. Send it back as
  `If-Match: 3` on `PUT`; a concurrent edit makes the save fail with 409. Without `If-Match` the save
  is unconditional (last writer wins). The whole-document `PUT` is the only graph write path: the web
  editor (canvas and YAML view) edits a local draft and saves it this way, so validation and version
  checks happen once, on the server.
* **Documents as written.** `GET /graphs/{id}`, versions and export return the document as it was
  saved: `spec.defaults` are applied when the graph is validated and run, not copied into each
  node, so changing a default reaches every node that does not set its own value. Only identity
  derived from names (graph, node and channel ids) is filled in on save. Graphs saved by earlier
  versions keep the per-node values they were stored with.
* **Strict graph documents.** Graph `POST`/`PUT` reject keys the schema does not define with `422`
  and one `details.errors[]` entry per key (`path` such as `spec.nodes[0].promt`, message
  `unknown field "promt"`); `POST /graphs/validate` reports them as errors too. Keys starting with
  `x-` on the graph, `metadata`, a node or a channel are extensions: they are stored and returned
  verbatim (JSON, YAML export) and never interpreted. `x-` keys anywhere else are rejected. Free-form
  maps (`node.config`, `env`, `labels`) are not inspected.
* **Pagination.** List endpoints take `limit` and return newest first (messages, executions,
  approvals). The event log pages by sequence number (`after=<seq>`) and returns ascending order.
  Defaults: messages 100, executions 50, events 200, approvals 200.
* **Asynchronous lifecycle.** Lifecycle actions record the *desired* state and return the current
  `GraphStatus`; replicas converge within one reconcile interval (default 2 s). Poll `/status` or follow
  the event stream.
* **Security headers.** Every response carries `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`. CORS is closed unless `server.corsOrigins`
  lists the origin (or `*`).
* **Body size.** Bounded by `server.maxBodyBytes` (default 8 MiB).

## Model providers

Providers come from the server configuration (`source: config`, read-only)
or are managed through the API (`source: managed`). Responses never contain
a credential or its ciphertext; `credential` only says whether one is
configured and where it comes from (`stored`, `env` with the variable name,
`file`, `none`).

| Method and path | Role | Purpose |
|---|---|---|
| `GET /api/v1/providers` | viewer | List providers (superset of the earlier `ProviderInfo`: adds `source`, `editable`, `credential`, `status`, `error`, `lastTest`). `ETag` = settings revision. |
| `GET /api/v1/providers/{name}` | viewer | One provider. |
| `POST /api/v1/providers` | admin | Create a managed provider. |
| `PUT /api/v1/providers/{name}` | admin | Update a managed provider (the name cannot change). |
| `DELETE /api/v1/providers/{name}` | admin | Remove a managed provider. `409` while a graph selects it explicitly or it is the default. |
| `POST /api/v1/providers/{name}/test` | operator | Connection test of a saved provider. |
| `POST /api/v1/providers/test` | admin | Connection test of unsaved settings (same body as create; a saved provider's stored credential is reused when the body has none). Saves nothing. |
| `GET /api/v1/providers/{name}/models` | viewer | Models reported by the provider. |
| `GET /api/v1/provider-settings` | viewer | `{revision, default, configDefault, secrets: {enabled, keySource, keyId}, types[]}`. |
| `PUT /api/v1/provider-settings` | admin | `{"default": "<name>"}` selects the default provider; `""` returns to the configuration default. |

Create/update body:

```json
{"name": "team-openai", "type": "openai", "baseUrl": "https://api.openai.com/v1",
 "defaultModel": "gpt-4o-mini", "timeoutSec": 120, "maxRetries": 2, "concurrency": 8, "contextWindow": 65536,
 "apiKey": {"value": "sk-..."}}
```

`apiKey` is optional: omit it to keep the stored credential, send
`{"value": "..."}` to replace it, `{"clear": true}` to remove it. Unknown
fields are rejected (server-side `apiKeyEnv`/`apiKeyFile` cannot be set
through the API). Writes accept `If-Match: <revision>`; a stale revision
returns `409`. Without it, each write applies to the latest settings.

A connection test lists models (informational), sends one short chat
request through the same gateway agent turns use (`ok` reflects that
inference), then offers the model one tool and reports in `tools` whether it
called it in the structured form agents need (`{checked, ok, reply?, error?}`;
advisory — a model that writes the call as text fails it). Errors are returned with any echoed credential replaced by
`[redacted]`. Tests are audited as `provider.tested` events; changes as
`provider.changed`.

## Installation network policy

| Method + path | Role | Behavior |
|---|---|---|
| `GET /api/v1/network-policy` | viewer | Effective `network`, `revision`, `source`, update metadata and graph `usage` (requested hosts and admission status). |
| `POST /api/v1/network-policy/preview` | admin | `{revision, network: {setupNetworks, runNetworks, egressHosts}}`; validates the draft and returns affected graphs without persisting it. |
| `PUT /api/v1/network-policy` | admin | Saves the reviewed network settings with strict revision matching. Revision `0` requires no existing saved policy. Returns saved settings and new revision. |

Network arrays must be nonempty and contain unique `none`, `bridge` or `egress`
values. Hosts are exact lowercase DNS names, with the same 64-host ceiling as
other environment allowlists. Unknown fields, malformed input or invalid hosts
return 400; revision conflicts return 409. Bodies are limited to 32 KiB.
The settings override only network fields, preserve YAML feature restrictions,
and apply on new admission/resume without automatically stopping active work.
See [network policy management](environments.md#manage-installation-network-policy-in-the-console).

## Project environments

The environment definition is part of the graph (`spec.environment`) and is
saved with the graph. These endpoints manage its build; see
[environments.md](environments.md).

| Method and path | Role | Purpose |
|---|---|---|
| `GET /api/v1/graphs/{id}/environment/proxies` | viewer | Read-only `{proxies, lastPreparation?, truncated}`: managed proxy state and sanitized recent request samples. Maximum 32 containers, 100 log lines per proxy within 24h; unavailable items include a warning. Last preparation is a historical snapshot. |
| `GET /api/v1/graphs/{id}/environment` | viewer | Build state compared with the saved definition: `state` (`absent`, `building`, `ready`, `failed`), `defined`, `currentHash`, `upToDate`, `image`, `caches`, `error`, `log` (last 64 KiB). |
| `POST /api/v1/graphs/{id}/environment/build` | operator | Body `{"force": bool}` (optional). Starts a background build of the saved definition (`202`); an up-to-date environment without `force` is left alone (`200`). `400` without `spec.environment`; `409` while a build runs. |
| `POST /api/v1/graphs/{id}/environment/reset` | admin | Deletes the environment's images and cache volumes; `409` while the graph is running. |

Starting a graph or node whose docker-isolated agents (without an image of
their own) need an environment that is not ready returns `422` with an issue
at `spec.environment`.

## Event stream (SSE)

```
GET /api/v1/events/stream?graph=software-factory&types=execution.completed,approval.requested
Accept: text/event-stream
```

```
retry: 3000
: connected

id: 4211
event: execution.completed
data: {"seq":4211,"id":"evt_…","type":"execution.completed","graphId":"software-factory",…}

: ping
```

| Behaviour | Detail |
|---|---|
| Start position | `?after=<seq>` or `Last-Event-ID: <seq>` resumes after that event; with neither the stream starts at the newest event ("now"). |
| Ordering | Strictly by `seq`; no gaps, no duplicates within one connection. |
| Delivery | The handler tails the durable event log (wake-up on local publish, 1 s poll otherwise), so events written by *any* control-plane instance arrive on every connection. |
| Heartbeat | A `: ping` comment after 15 s without events keeps proxies from closing the stream. |
| Filters | `graph`, `node`, `types` (comma separated). |
| Reconnect | Browsers reconnect after `retry` ms and send `Last-Event-ID` automatically. |

The event catalogue is in [events.md](events.md).

## Typical flows

```bash
export LEFORGE=http://localhost:8080
: "${LEFORGE_ADMIN_TOKEN:?Set LEFORGE_ADMIN_TOKEN from your local .env file}"
export TOKEN="$LEFORGE_ADMIN_TOKEN"
H="Authorization: Bearer $TOKEN"

# Create a graph from YAML (admin)
curl -sf -X POST -H "$H" -H 'Content-Type: application/yaml' \
  --data-binary @examples/graphs/hello-echo.yaml $LEFORGE/api/v1/graphs

# Validate a change before saving it
curl -s -X POST -H "$H" -H 'Content-Type: application/yaml' \
  --data-binary @my-graph.yaml $LEFORGE/api/v1/graphs/validate | jq

# Update with optimistic concurrency
curl -s -X PUT -H "$H" -H 'Content-Type: application/yaml' -H 'If-Match: 3' \
  --data-binary @my-graph.yaml $LEFORGE/api/v1/graphs/my-graph

# Start, inspect, stop
curl -s -X POST -H "$H" $LEFORGE/api/v1/graphs/software-factory/start | jq .state
curl -s -H "$H" $LEFORGE/api/v1/graphs/software-factory/status | jq '.nodes[] | {nodeId, state, pending}'
curl -s -X POST -H "$H" $LEFORGE/api/v1/graphs/software-factory/stop

# Put work into the factory (operator)
curl -s -X POST -H "$H" -H 'Content-Type: application/json' \
  $LEFORGE/api/v1/graphs/software-factory/channels/requests/messages \
  -d '{"type":"feature.request","payload":{"title":"slugify","description":"Add slugify(text)."}}' | jq .id

# Follow one workflow end to end by correlation id
curl -s -H "$H" "$LEFORGE/api/v1/graphs/software-factory/messages?correlationId=msg_…" | jq '.[] | {type: .message.type, channel, status}'

# Human in the loop
curl -s -H "$H" "$LEFORGE/api/v1/approvals?status=pending" | jq '.[] | {id, kind, title}'
curl -s -X POST -H "$H" -H 'Content-Type: application/json' \
  $LEFORGE/api/v1/approvals/apr_…/approve -d '{"comment":"ship it"}'

# Talk to a running agent, scale a cluster, kill a stuck node
curl -s -X POST -H "$H" -H 'Content-Type: application/json' \
  $LEFORGE/api/v1/graphs/software-factory/nodes/developer/chat -d '{"text":"prefer stdlib only"}'
curl -s -X PUT  -H "$H" -H 'Content-Type: application/json' \
  $LEFORGE/api/v1/graphs/hello-echo/nodes/workers/scale -d '{"replicas":5}'
curl -s -X POST -H "$H" $LEFORGE/api/v1/graphs/software-factory/nodes/developer/kill

# Live events
curl -N -H "$H" "$LEFORGE/api/v1/events/stream?graph=software-factory"
```

The `forge` CLI wraps these calls (`forge help`).

## Differences from the early working reference (`docs/dev/contracts.md`)

| Topic | contracts.md | Implemented |
|---|---|---|
| `GET /graphs/{id}/events` without `after` | not specified | Returns the latest page, ascending |
| SSE without resume point | not specified | Starts at "now" |
| `GET /graphs/{id}/nodes/{node}/chat` | all messages | Latest 200, oldest first |
| `/info.identity` | not specified | `{"name","role"}` of the caller |
| Event `replica` | number | Always present; `-1` means the node as a whole |
| Error codes | listed | Also `rate_limited`, `llm_error`, `no_answer` (callbacks) |

`GET /api/v1/graphs/{id}/environment/blocked-history` (viewer+) returns
`requests` (newest first; safe proxy request fields plus `proxy` and `phase`),
`retentionDays` and `limit` (effective installation limits, default 7/1000), and optional `collectedAt` (last successful
collection). It reads shared storage independently of Docker and applies retention.
Unknown graphs return 404; storage/corruption failures return an error, not an
empty successful history. This sampled diagnostic history is not a complete audit.

`GET /api/v1/blocked-history-retention` (viewer+) returns `{policy: {days,
maxEvents}, revision, updatedAt?, updatedBy?}`. Revision 0 means defaults (7 days,
1,000 blocks). `PUT /api/v1/blocked-history-retention` (admin only) accepts exactly
`{policy: {days, maxEvents}, revision, confirm: true}`. Days must be an integer
1–90 and maxEvents an integer 1–10,000. Missing confirmation/revision, invalid
values, unknown fields or trailing JSON return 400; stale revisions return 409.
A successful save returns the acknowledged revision and policy. New limits apply
to subsequent reads/collection/retention sweeps, including histories of deleted
graphs; saving is not a synchronous purge of every history. Storage failures return
an error. Lower limits can permanently discard history; higher limits do not
restore discarded records. Publication: `installation.history_retention.updated`.

# LeForge web console

The operator console for LeForge: a visual graph editor, read-only workspace
browser, node and channel inspectors, human-in-the-loop approvals, the event
timeline and a message explorer that walks a workflow's causal chain.

React 19 + TypeScript (strict) + Vite, with `@xyflow/react` for the canvas,
`@dagrejs/dagre` for auto-layout and `yaml` for the definition editor. There
is no UI kit and no state library: hand-written CSS (`src/styles.css`), React
context and hooks.

## Develop

```bash
npm ci
npm run dev          # http://localhost:5173, proxies /api, /healthz, /metrics to :8080
LEFORGE_API=http://host:8080 npm run dev   # proxy somewhere else
npm run dev:mock     # no server needed: fixture-backed fake API + simulated events
```

In mock mode (`VITE_MOCK=1`) `src/mock/server.ts` answers every API call and
pushes fake engine activity through `src/mock/events.ts`, which stands in
for `EventSource`. The mock is loaded lazily, so it adds nothing to the
production bundle's main chunk.

## Build

```bash
npm run build        # tsc --noEmit && vite build  ->  dist/
npm run typecheck
```

`dist/` is served by the Go server at `/`. Every API path is relative
(`/api/v1/...`), so the console works behind any prefix-free reverse proxy.

## Auth

The bearer token lives in `localStorage.leforgeToken`. Any 401 opens a
sign-in dialog; enter the `LEFORGE_ADMIN_TOKEN` configured for the server.
`EventSource` cannot send headers, so the SSE stream passes the token as
`?access_token=`.

## Layout of the code

```
src/
  api/types.ts        TS mirrors of the Go domain types (JSON tags verbatim)
  api/client.ts       typed client for every endpoint in docs/dev/contracts.md
  hooks/useEventStream.tsx  ONE shared SSE connection for the app (provider + hook)
  hooks/useData.ts    useAsync, useThrottled, useGraphStatus (2s poll + SSE nudges)
  components/
    canvas/           graph editor and model (graph -> nodes/edges, consumer
                      resolution, dagre layout), AgentNode, ChannelEdge
    FilesView         read-only workspace browser and file content viewer
    GraphWorkspace    toolbar (graph editing and Run/Stop/Pause/Resume/Kill) + tabs
    NodePanel         replicas bar, start/stop/scale, live output, operator
                      chat, prompt editor, grants/settings, recent turns
    ChannelPanel      semantics, consumer groups, inject form, history
    ApprovalsView     pending/decided approvals; answers for human.ask
    EventsView        live timeline with filters and pause
    MessagesExplorer  correlation-id causal tree (causation_id) + trace link
    YamlEditor        validate, save (If-Match), versions, export
    GraphList         create from YAML/JSON, lifecycle
  mock/               VITE_MOCK fixtures (dev only)
```

### Live updates

`EventStreamProvider` opens one `EventSource` to `/api/v1/events/stream`.
The server names SSE events by type, so the provider registers a listener
per known event type. It handles reconnecting itself, with exponential
backoff, and resumes with `?after=<last seq>`, dropping any replayed
sequence numbers. Components subscribe with `useEventStream(filter, fn)`.
Graph status is polled every 2 s and refetched (throttled) whenever a
`replica.*`, `execution.*`, `message.*`, `graph.*` or `approval.*` event
arrives, so changes show up without waiting for the next poll.

### Visual language of the floor

| Element | Meaning |
|---|---|
| solid amber edge | `queue`: one shared backlog, consumers compete; the count sits on the first branch only |
| blue double rail | `topic`: copy per subscribing node (including `subscribe:` pattern matches) |
| purple dashed | `fanout`: static copy per listed target |
| teal dotted | `broadcast`: copy per replica |
| lock icon | approval-gated channel |
| italic `if` | conditional route (`when`); hover the label for the expression |
| moving dashes | the channel has pending or in-flight messages |
| ghost cards behind a node | cluster; the chip reads `live of size` |
| purple `ring` badge | ring comms (replicas read neighbours) |
| dashed, faded card | disabled node |
| yellow `done` | every replica completed by its stop policy |
| pulsing `approval` | a replica is awaiting a human |

Back edges, where cycles occur, curve underneath the graph, and parallel
edges between the same two nodes fan out. Channels without a producer get
an "ingress" pill, and topics with no subscribers get a "no subscribers" pill.

Deep links: `#/g/<graph>/canvas?sel=node:<id>` or `?sel=channel:<id>`, and
`#/g/<graph>/messages?c=<correlation id>`.

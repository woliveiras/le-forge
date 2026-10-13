# Tool permission model

Agents act on the world only through **tools**, and every tool call — from native agents and from
external runtimes alike — passes through one **tool gateway** (`server/internal/tools/gateway.go`)
that checks the calling replica's **grants** before anything runs. The model is **default deny**:
a node with no `tools:` has no tools (native agents keep the four platform tools below).

Grants are enforced, not requested: the model only ever *sees* schemas of granted tools
(`Registry.Schemas`), and a call to anything else is refused by the gateway regardless of what the
model or agent code asks for.

## Capabilities and tools

A **tool** is what an agent invokes; a **capability** is the permission it requires. Several tools
share a capability, so granting `filesystem.read` grants all four read tools.

| Capability | Tools | Constraint axis | Notes |
|---|---|---|---|
| `filesystem.read` | `filesystem.read` `{path, offset?, limit?}`, `filesystem.list` `{path?, recursive?, glob?, maxEntries?}`, `filesystem.search` `{pattern (regexp), path?, glob?, maxResults?}`, `filesystem.stat` `{path}` | `paths` | Listing/search skip `.git`, `node_modules`, `.venv`, `__pycache__` |
| `filesystem.write` | `filesystem.write` `{path, content, append?}` (atomic), `filesystem.edit` `{path, old, new, replaceAll?}`, `filesystem.move` `{from, to}` (both checked), `filesystem.mkdir` `{path}` | `paths` | Creates parent directories |
| `filesystem.delete` | `filesystem.delete` `{path, recursive?}` | `paths` | Separate from write on purpose |
| `shell.exec` | `shell.exec` `{command? \| argv?, cwd?, timeoutSec? (≤1800), env?}` | `commands` | Runs in the replica's sandbox; default timeout 2 m, output 64 KiB |
| `git.read` | `git.status`, `git.diff` `{staged?, ref?, path?}`, `git.log` `{n?, ref?}`, `git.show` `{ref}`, `git.branches` | `paths` (pathspecs) | Executed through the sandbox (works in docker isolation); refs starting with `-` rejected, paths after `--` |
| `git.write` | `git.add` `{paths}`, `git.commit` `{message, all?, author?}`, `git.branch` `{name, from?, checkout?}`, `git.checkout` `{ref}`, `git.merge` `{branch, noFF?, message?}` | `paths` | Fallback identity `LeForge Agent <agent@leforge.local>` |
| `http.fetch` | `http.fetch` `{url, method?, headers?, body?, maxBytes? (≤5 MiB)}` | `hosts` | http/https only, ≤ 5 redirects (each re-checked), HTML → text, SSRF guard |
| `human.ask` | `human.ask` `{question}` | — | Opens a `question` approval and blocks until a human answers |
| `mcp.<server>.*` | *(extension point)* | — | Requires registering an MCP tool provider |

`GET /api/v1/tools` lists the registry with JSON Schemas.

### Platform tools (no grant needed)

Native agents always have four tools that only touch their own turn; external runtimes reach the
same functions through the callback API.

| Tool | Callback equivalent | Effect |
|---|---|---|
| `emit {type, payload, target?, channel?}` | `POST /emit` | Queue an output; published only if the turn succeeds |
| `state_get {key}` / `state_set {key, value}` | `GET/PUT /state/{key}` | Node memory (shared by replicas), ≤ 1 MiB per value |
| `finish {summary?, signal?, broadcast?}` | `TurnResult` | End the turn; stop handshake; ring broadcast |

## Grant syntax

A grant is a bare string or an object:

```yaml
tools:
  - filesystem.read                          # capability
  - git.status                               # single tool
  - "git.*"                                  # wildcard (quote in YAML)
  - {name: filesystem.write, paths: ["src/**", "tests/**"]}
  - {name: shell.exec, commands: ["python3", "go test *"], requiresApproval: false, rateLimit: 30}
  - {name: http.fetch, hosts: ["*.wikipedia.org", "docs.python.org"]}
  - {name: git.write, requiresApproval: true}
```

| Field | Applies to | Semantics |
|---|---|---|
| `name` | all | Matched with `*`/`?` globs (where `*` crosses dots) against the tool name **or** its capability. `*` alone = every tool (validation warns) |
| `paths` | filesystem.*, git (pathspecs) | Workspace-relative globs: `**` = any number of segments (incl. none), `*`/`?`/`[…]` within one segment. Empty = whole workspace |
| `commands` | shell.exec | Pattern **without** a space → matched against `basename(argv[0])` (`go`, `pytest*`). Pattern **with** a space → matched against the full command line with `argv[0]` reduced to its basename (`npm *`, `git status`). Empty = any command |
| `hosts` | http.fetch | Case-insensitive host globs; also applied to every redirect. Empty = any **public** host |
| `requiresApproval` | all | Each call waits for a human (`tool` approval) |
| `rateLimit` | all | Calls per minute per (graph, node, tool): token bucket, burst = limit |

### Matching and merge semantics (as implemented in `policy.go`)

1. **Default deny.** Collect every grant whose `name` matches the tool name or capability. None → `tool.denied` (HTTP 403 on callbacks).
2. **Union of permissions.** If several grants match, the call is allowed if *any* of them would allow it:

| Axis | Merged value |
|---|---|
| `paths` / `commands` / `hosts` | If **any** matching grant leaves the axis empty → unrestricted; otherwise the union of patterns |
| `requiresApproval` | `true` only if **every** matching grant requires it |
| `rateLimit` | `0` (unlimited) if any matching grant is unlimited; otherwise the largest limit |

   Consequence: `[filesystem.read, {name: filesystem.read, paths: [docs/**]}]` is **unrestricted** —
   narrowing only works when no broader grant also matches.
3. **Constraints are checked by the tool** after authorisation; a violation is a denial (`tool.denied`,
   outcome `denied`), not a tool error.
4. **Path jail (always on, independent of grants).** Every path is cleaned, joined to the replica's
   workdir and resolved through symlinks of its deepest existing ancestor; anything that leaves the
   workdir — `../`, absolute paths outside, planted symlinks — is refused. Absolute paths inside the
   workdir are accepted.
5. **Restricted shells.** When `commands` is set, a `command` string must be a single simple command:
   any of `` ; | & ` $ < > ( ) { } `` or a newline is refused, and the command runs as argv **without a
   shell**, so what was checked is exactly what runs. Without `commands`, `command` runs via `/bin/sh -c`
   (`cmd /C` on Windows).
6. **SSRF guard (http.fetch).** Resolved addresses are checked at dial time (defeats DNS rebinding):
   loopback, RFC 1918/ULA private, link-local (incl. `169.254.169.254` metadata), multicast, CGNAT,
   unspecified and reserved ranges are refused unless `tools.allowPrivateNetworks: true`.

### Gateway pipeline

```mermaid
flowchart LR
  A[call name+args] --> B{registered?}
  B -- no --> D1[deny: unknown]
  B -- yes --> C{grant matches?}
  C -- no --> D2[deny]
  C -- yes --> E{args JSON object?}
  E -- no --> X[error]
  E -- yes --> F{rate limit}
  F -- exceeded --> D3[deny: rate_limited]
  F -- ok --> G{requiresApproval?}
  G -- yes --> H[approval: human decides] -->|rejected/expired| D4[deny: rejected]
  G -- no --> I[execute in sandbox]
  H -->|approved| I
  I --> J[cap output 32 KiB, audit, metrics, span]
```

Tool *failures* (non-zero exit, missing file, HTTP error) are returned to the agent as
`{isError: true, content}` so the model can recover; only refusals are errors.

## Audit and observability

| Signal | Content |
|---|---|
| Events | `tool.invoked`, `tool.completed`, `tool.denied` with `{tool, args (≤ 2 KiB), outcome, durationMs, error}`, actor `agent:<node>#<n>`, execution and trace ids |
| Metrics | `leforge_tool_invocations_total{tool,outcome}`, `leforge_tool_duration_seconds{tool}` |
| Traces | Span `tool.<name>` with `leforge.graph/node/replica/execution`, `leforge.tool.outcome` |
| Approvals | `approval.requested/decided` with detail `{tool, args}` |

## Recipes

| Role | Grants | Rationale |
|---|---|---|
| Read-only reviewer | `[filesystem.read, git.read]` | Can inspect code and diffs, change nothing |
| Developer | `[filesystem.read, {name: filesystem.write, paths: ["src/**","tests/**"]}, git.read, git.write]` | Writes only source and tests; no shell, no deletes |
| QA (test runner only) | `[filesystem.read, {name: shell.exec, commands: ["python3", "pytest"]}]` | Can run the suite, nothing else; no pipes or redirects |
| Researcher | `[{name: filesystem.write, paths: ["research/**"]}, {name: http.fetch, hosts: ["*.wikipedia.org"]}]` | Writes notes, browses an allowlist of public hosts |
| Release / integrator | `[git.read, {name: git.write, requiresApproval: true}]` | Every write needs a human |
| Fully trusted builder | `[filesystem.*, shell.exec, git.*, http.fetch]` | Only in docker/kubernetes isolation |

**`.git` is protected.** Mutating filesystem tools refuse any path inside a `.git` directory, and
git runs with hooks and fsmonitor disabled, so `filesystem.write` plus `git.*` does not amount to
code execution. Repository internals change only through the git tools.

## Adding a tool

1. Implement `tools.Tool` (`Name`, `Capability`, `Description`, `Schema`, `Invoke`). Use
   `ResolvePath` + `CheckPath` for paths, `CheckCommand` for commands, `CheckHost` for hosts, and run
   processes through `inv.Sandbox` (never `os/exec` directly) so isolation modes apply. Return
   `ErrDenied`-wrapped errors for constraint violations, plain errors or `IsError` results for failures.
2. Register it: `registry.Register(t)` in the composition root (`internal/app`), or add it to
   `tools.Builtin`.
3. Pick a capability name `<domain>.<verb>`; reuse an existing one only if the risk is identical.
4. Document the arguments in the JSON Schema — it is what the model reads.

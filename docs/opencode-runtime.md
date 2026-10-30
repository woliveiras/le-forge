# OpenCode as a gateway-bound runtime

Status: **proposed design, not implemented.** This page describes how LeForge
could run [OpenCode](https://opencode.ai) as an agent runtime while every model
call and every tool call still passes through LeForge's gateways. Statements
about OpenCode's configuration must be verified against the pinned OpenCode
version before implementation (see [open questions](#open-questions)).

## Why

LeForge's strength is orchestration and enforcement: default-deny grants,
task consent, approvals, sandboxing and audit. Writing and maintaining a
competitive coding-agent loop is a separate, large effort. OpenCode already
provides a mature loop (planning, editing, iterating on test output). Running it
**inside** LeForge's boundaries combines both without weakening either.

The rule this design must keep: **OpenCode decides what to do; LeForge decides
what is allowed and does it.**

## Boundaries

| Concern | Who owns it | How |
|---|---|---|
| Model credentials | LeForge | OpenCode talks only to a LeForge model endpoint scoped to the execution. Provider keys never enter the runtime. |
| Model choice, limits and cost | LeForge | The endpoint applies the node's model, `limits`, delegation ledger and [cost control](cost-control.md). |
| File, Git, shell, HTTP, MCP tools | LeForge | OpenCode's built-in tools are disabled. LeForge tools are exposed to OpenCode as an MCP server scoped to the execution and evaluated by the tool gateway. |
| Approvals and questions | LeForge | Gateway approvals, task authorization modes and `human.ask` behave as for any other runtime. |
| Isolation | LeForge | OpenCode runs in the node's Docker sandbox with no network except the LeForge endpoints. |
| Agent loop, prompts, context management | OpenCode | Unchanged. |

## Components to build

1. **Execution-scoped model endpoint.** An OpenAI-compatible (or
   Anthropic-compatible) chat endpoint under the existing callback base URL,
   authenticated with the callback token. It translates requests into
   `llm.ChatRequest`, so every LeForge provider works behind it, and it applies the
   same accounting as `/llm/chat`. Streaming is needed for an interactive
   experience; LeForge's providers do not stream today.
2. **Execution-scoped MCP server.** An MCP endpoint under the callback base URL
   that lists only the tools the node is granted and forwards each call to the
   tool gateway. Denials, approvals and constraint errors are returned as MCP tool
   errors. Today LeForge is only an MCP *client*; this adds the server side.
3. **OpenCode runtime adapter.** A `runtime.Runtime` (or an external runtime
   implementing [Runtime Protocol v1](runtime-protocol.md)) that, per turn:
   - writes a **read-only, generated** OpenCode configuration: the LeForge model
     endpoint as the only provider, the LeForge MCP server as the only tool source,
     built-in tools disabled and no other MCP servers;
   - ignores any OpenCode configuration committed to the repository, and does not
     let OpenCode load `AGENTS.md` or other repository guidance on its own: project
     guidance stays opt-in through [skills and project steering](instructions.md),
     which never searches for `AGENTS.md` automatically;
   - starts OpenCode in the sandbox, sends the turn input as the prompt, and maps
     the result to `TurnResult` (output text, emits, reported usage).
4. **Sandbox network profile.** A mode that allows only the LeForge callback
   host and port from the sandbox, so OpenCode cannot reach model vendors,
   package registries or the internet directly. Dependency installs go through
   LeForge tools and the environment's existing egress policy.
5. **Workflow template.** A Plan → Build → Verify → hand-off template built on the
   graph, where Plan uses read-only grants, Build uses workspace grants and Verify
   is a read-only checker.

## Security properties to prove

- With built-in tools disabled, every file change, command and network request
  in the transcript corresponds to a gateway call with a recorded decision.
- OpenCode cannot read provider keys, MCP connection tokens or Git credentials:
  none are in its environment, configuration or filesystem.
- A repository that ships an OpenCode configuration, plugins or custom tools
  cannot enable them.
- Removing a grant or reducing the task authorization mode takes effect on the
  next tool call of a running turn.
- With the sandbox network profile, a direct HTTP request from the OpenCode
  process to any host other than LeForge fails.

Each property needs an integration test that runs a real OpenCode binary against
a scripted model.

## Open questions

These must be answered against the OpenCode version LeForge pins:

- Can **all** built-in tools (shell, edit, write, read, fetch, task/subagents) be
  disabled by configuration, and is that setting honoured when the model asks for
  them anyway?
- Can repository-level configuration, plugins, custom tools and automatic
  `AGENTS.md` loading be fully disabled (for example through an explicit
  configuration path and environment)?
- Which provider protocol is most robust for a custom base URL: OpenAI-compatible
  or Anthropic-compatible?
- How does OpenCode report usage per turn, and can it be attributed to LeForge's
  calls rather than double-counted?
- Is one OpenCode process per turn acceptable, or is a long-lived session per
  execution needed (for example `opencode serve`) to keep conversation state?
- What is the licence and update policy for shipping or downloading the OpenCode
  binary into sandbox images?

If built-in tools cannot be reliably disabled, the design falls back to running
OpenCode only in the Docker sandbox with no network and treating its built-in
tools as `shell.exec`-equivalent: weaker, and it must be reported as such in the
console.

## Relationship to other runtimes

The same pattern (execution-scoped model endpoint + execution-scoped MCP server +
generated read-only configuration) applies to other coding agents that accept a
custom model endpoint and MCP tools. OpenCode is the first target because it is
open source and provider-neutral.

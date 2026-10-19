# Standalone Forge

Status: dependency audit completed on 2026-10-19. Native packaging and its
installation commands are not delivered yet. The existing Docker installation
remains independent; no migration or activation is implied by this document.

## Dependency and capability contract

| Component | Native installation | Docker/server installation |
|---|---|---|
| Control plane and UI | `forged` plus bundled web assets; browser UI | Existing image and mounted/configured assets |
| Persistent state | Existing SQLite adapter (pure-Go modernc driver) | Existing SQLite/filesystem/PostgreSQL choices |
| Queue | Existing durable filesystem broker | Existing filesystem/NATS/Redis/RabbitMQ/Kafka choices |
| Git/workspaces | Installed Git, explicitly registered repository roots and task worktrees | Paths visible to the server/container; a mount is not host execution |
| Native agents | Existing in-process runtime, configured model provider | Same gateway and permissions |
| Models | Explicit cloud provider or separately available local inference service | Existing provider configuration |
| External runtimes | Optional separately configured runtime endpoint; Python/agent frameworks are not silently installed | Existing optional agent services |
| MCP | Existing explicitly registered HTTP connections; local stdio is pending LOCAL-05 | Existing remote HTTP connections |
| Sandboxes | Optional compatible Docker runtime; never required for process execution | Existing dedicated sandbox daemon and policy controls |
| Observability | Structured logs, API history and protected metrics; optional exporters | Existing optional collectors/dashboards |

A basic installation must not install PostgreSQL, NATS, Docker or a companion
executor. It runs as the invoking user. Process execution has that OS account's
authority; worktrees are file organization, not security isolation. Remote MCP
credentials and local OS credentials must never be imported implicitly.

## Initial platform targets

The initial native qualification target is macOS arm64 (the current development
host). macOS amd64 and Linux amd64/arm64 are intended build targets, each requiring
its own smoke evidence before being advertised as validated. Windows packaging is
outside this initial matrix: current process/workspace code uses Unix facilities,
including nonblocking file opens and signal handling. Cross-compilation alone is
not runtime validation.

Source builds require the Go version declared by `server/go.mod`, Node/pnpm for
the web assets, and Git for repository operations/tests. End users of a packaged
bundle should need neither the Go toolchain nor Node merely to launch Forge.
Git and a usable model provider are necessary for real repository work. An echo
or mock run proves plumbing only and must be labeled synthetic.

## Delivery requirements still pending

- A native bundle and explicit private data-directory initialization, loopback
  access, protected first login and simple foreground start/stop.
- Single-instance locking, occupied-port detection before executing agents,
  graceful failure/shutdown and restart evidence.
- Explicit operational backup/restore and upgrade procedures covering the database,
  queue, encryption key, configuration and workspaces. Worktree metadata also
  depends on the registered source repository; copying only the checkout is not
  a complete backup.
- Native capability information for environment/editor choices; no silent sandbox
  fallback, secret import, existing Docker data migration or service activation.
- Local MCP subprocess lifecycle and exact per-agent gateway grants.
- A clean disposable native browser journey and platform qualification.

The existing `make build` uses the Docker toolchain wrapper and is not evidence
of a native distribution. See [task-workflows.md](task-workflows.md) for the UI and
worktree behavior already delivered, and [deployment.md](deployment.md) for
existing server operations. Phase A task-flow work remains in progress; this audit
prepares Phase B without activating another installation.

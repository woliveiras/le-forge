# Standalone Forge

Status: initial native bundle/profile implemented on 2026-10-19; qualification
evidence and remaining scope are recorded below. The existing Docker installation
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

## Initial native bundle

Build on the target machine with `make build-native`. The command prints a new
bundle directory under `dist/native/`; it never starts an installation. The bundle
contains `forged`, the existing `forge` CLI and `web/` assets. Keep them together.
From that bundle, initialize a **new** profile and run it:

```sh
./forged --init-local
./forged --local
```

The default private data directory is the OS user configuration directory plus
`LeForge` (on macOS, `~/Library/Application Support/LeForge`). Override it with
`--data-dir /absolute/new/path` on both commands for a separate test profile.
Initialization refuses an existing directory, including a partial initialization;
it never overwrites credentials or data. A missing bundled UI is an error.

Open `http://localhost:8787`. Read `admin.token` in your own profile directory and
enter it in the login screen yourself. Initialization never prints its value.
The encryption key is generated inside this profile on first start. Do not share
these files. The native profile ignores `LEFORGE_*` configuration overrides so a
Docker installation environment is not silently imported. Existing HTTP MCP and
model credentials are entered explicitly through the UI.

`--port 8789` is an initialization option for a different loopback port. Start
uses the saved profile. All listeners are reserved before the engine starts;
occupied ports fail without starting agents. A file lock prevents two native
processes from opening the same data directory. Stop the foreground process with
Ctrl-C and start it again to retain state. SQLite and filesystem queues need no
separate services. Preview serving is initially disabled; optional preview/sandbox
qualification remains pending. No Docker services or existing volumes are changed.

For a first synthetic test, open Tasks → Preview example, confirm preparation,
then Review requirements → Start task. This saves an echo-only workflow and draft;
it uses no model, repository, tools or connected account and does not perform AI
analysis. Nothing starts during preparation. If preparation fails, inspect Tasks
and the linked example workflow before retrying; partial records are retained.

For a first repository test:

1. In Configuration → Models, configure a real available provider. The initial
   mock provider and echo runtime are synthetic plumbing tests, not real AI work.
2. Create a workflow with a native agent, select that provider/model and review
   its explicit tool grants. For manual code review, prefer scoped filesystem
   tools; do not grant Git write or unrestricted shell actions unintentionally.
3. Register your repository in Projects. New task lets you select that project,
   workflow, entry/result agents and a reviewed commit.
4. Review and start. Task progress offers Open in VS Code after you confirm that
   Forge and the browser are on the same computer and VS Code is installed.
   Alternatively, open the displayed worktree path manually.
   Dirty files in the original checkout are not copied or overwritten.
5. Use Inspect changes in task progress for Git status/diff/history, and Tool calls
   for recorded command outcomes. Inspect results and tests before your own commit/push. Task completion
   itself performs neither action; agent permissions remain those you reviewed.

## Offline backup and restore

Stop Forge with Ctrl-C and stop other processes writing its workspaces before
backup. Do not open the same profile through the standard server entry point.
The native lock rejects an active native instance; it cannot stop external writers.
Create a new archive outside the profile:

```sh
./forged --backup-local /absolute/backups/forge-backup.tar.gz
```

Use the same `--data-dir` as initialization for a nondefault profile. The archive
includes SQLite, filesystem queues, pending approvals, configuration, credentials,
the encryption key and managed workspaces. It is private (0600), **not encrypted**;
store it securely. Registered source repositories and their Git metadata need
separate backups. Workspace symlinks are stored without following their targets.
Special files are rejected. An unsuccessful backup remains `.partial` and is never
published as a complete archive; inspect it before deciding on cleanup.

Restore only a trusted archive, with Forge stopped, to its **original absolute
profile path**, which must not exist:

```sh
./forged --restore-local /absolute/backups/forge-backup.tar.gz
./forged --local
```

The command never overwrites a profile. Retain existing data separately before a
recovery attempt; restore also refuses relocation because worktree metadata and
configuration reference absolute paths. Keep the matching bundle/UI and external
source repositories available at their original locations. Invalid archives leave
a partial restore for inspection rather than deleting data automatically. Restore
checks path boundaries, duplicate entries, gzip integrity and the profile format;
it limits archives to 100 GiB and one million entries.

The profile format is versioned and unknown formats are refused. Before replacing
a bundle, preserve the stopped profile and source repositories and keep the old
bundle. There is no automated cross-version migration or downgrade guarantee.
Restart retains pending messages and approvals. Existing engine recovery may
redeliver interrupted work; this does not guarantee exactly-once external effects.

## Delivery requirements still pending

- Full platform qualification, real-model work and remaining task UX features.
- Cross-version upgrade qualification and native token-reservation reconciliation.
  Offline backup/restore is implemented; external repository backups remain the
  user’s responsibility.
- Native capability information for environment/editor choices; no silent sandbox
  fallback, secret import, existing Docker data migration or service activation.
- Local MCP subprocess lifecycle and exact per-agent gateway grants.
- Broader real-provider and cross-platform native qualification beyond the
  disposable synthetic browser journey already recorded below.

The existing `make build` uses the Docker toolchain wrapper and is not evidence
of a native distribution. See [task-workflows.md](task-workflows.md) for the UI and
worktree behavior already delivered, and [deployment.md](deployment.md) for
existing server operations. Phase A advanced task-flow work remains in progress. The native baseline enables
a first independent local test without claiming the entire roadmap complete.

## Qualification evidence (2026-10-19)

The darwin/arm64 bundle built successfully and its executable was inspected as
Mach-O arm64. Native Go integration tests exercised authentication, durable draft
storage across shutdown/restart, occupied-port failure and single-instance locks.
The Chromium native journey used the production `forged --local` entry point,
registered a disposable Git repository, reviewed/started an echo task in its own
worktree, reloaded its result and opened file inspection while preserving dirty
source files. This is explicitly synthetic execution, not real model/MCP evidence.

`CI=true make test` passed (303 frontend tests plus Go/Python suites), as did
`make lint`, `make build`, `make build-native`, `make test-e2e` (85 main cases,
1 hosted CIMD and 1 isolated provider case) and `make test-native-browser` (1 case).
The existing installation was not rebuilt, restarted, migrated or configured.
The remaining task-flow, stdio MCP and operational items in PLAN/TASKS are still
pending; this bundle permits an initial native test rather than closing them.

Offline recovery evidence: native unit tests cover active-instance refusal,
non-overwrite, cancellation, path traversal, destination symlink aliases, corrupt
archive checksums and synthetic credential preservation. The production native
HTTP integration restores a stopped profile and verifies its draft, pending queue
message and approval, then processes the retained messages after restart. These
checks use disposable fixtures only, not the existing installation.

Latest native task evidence: offline backup/recovery, explicit VS Code handoff,
synthetic first-use preparation and task-local Git inspection are implemented.
The final changed-code suite passed 312 frontend tests plus Go/Python, lint/build,
nine relevant browser cases and two native production-binary journeys. Earlier
full browser regression in this delivery passed 86 main cases plus hosted CIMD
and isolated provider cases. Actual VS Code launch, a real model, local stdio MCP,
optional native sandbox and other OS/architecture smoke checks remain unverified.

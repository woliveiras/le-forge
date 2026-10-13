# 0008 — Deterministic mock provider as the default model

**Status:** Accepted

## Context
`docker compose up` must produce a fully functional platform with no
Internet access, no GPU and no API keys, and CI must exercise agent graphs
deterministically.

## Decision
Ship a `mock` provider (default `llm.default`) that answers deterministically:
verbatim scripted responses via request metadata, an `emit` tool call for
native agents, a small JSON object in JSON mode, otherwise an echo. Example
Python agents fall back to deterministic heuristics when model output lacks
the fields they need, so the software factory runs end to end offline
(including real files, commits and test runs). Real providers are one
environment variable away (`LEFORGE_DEFAULT_PROVIDER`).

## Consequences
- The whole machinery — queues, clusters, approvals, recovery, observability —
  is demonstrable and testable for free; the `echo` runtime complements it.
- Output quality in the default setup is intentionally trivial; documentation
  makes switching providers prominent.

## Alternatives
- **Bundle a small local model by default:** large download, slow on CPU,
  hardware-dependent.
- **Require a provider at install:** breaks the offline, one-command goal.

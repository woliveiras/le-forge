# Getting started

This guide takes you from a clone to a running agent factory, then to real
models, your own graph, and running without Docker.

## 1. Start the platform

Requirements: Docker Engine (or Docker Desktop / Colima / Rancher Desktop /
Podman with the compose plugin) and about 3 GB of free memory for the full
stack.

```bash
for name in LEFORGE_ADMIN_TOKEN LEFORGE_RUNTIME_SECRET POSTGRES_PASSWORD GRAFANA_ADMIN_PASSWORD; do
  printf '%s=%s\n' "$name" "$(openssl rand -hex 32)"
done > .env
chmod 600 .env
docker compose up -d --build
docker compose ps              # forged, agents, nats, postgres, otel-collector, prometheus, tempo, loki, grafana
```

| URL | What |
|---|---|
| <http://localhost:8080> | Web console and API (use `LEFORGE_ADMIN_TOKEN` from `.env`) |
| <http://localhost:3000> | Grafana (anonymous viewer; admin password is in `.env`) |
| <http://localhost:9090> | Prometheus |

The compose stack runs **PostgreSQL** for storage and **NATS JetStream** for
queues, loads every graph under `examples/graphs/` and starts `hello-echo` and
`software-factory` (override with `LEFORGE_BOOTSTRAP_START`). The four generated
credentials are required. Keep `.env` private and never commit it. Optional
settings can be added to the same file:

```dotenv
LEFORGE_DEFAULT_PROVIDER=mock
LEFORGE_BOOTSTRAP_START=hello-echo
```

To run the lighter stack without observability: `docker compose up -d forged`
(it pulls in `nats`, `postgres` and `agents`).

## 2. Watch work move

```bash
set -a; . ./.env; set +a
alias forge='docker compose exec -e LEFORGE_TOKEN="$LEFORGE_ADMIN_TOKEN" forged forge'

forge graphs
forge status hello-echo
forge inject software-factory requests feature.request \
  '{"title":"slugify","description":"Add a slugify(text) helper. It lowercases text and joins words with dashes."}'
forge events software-factory --follow        # Ctrl-C to stop
forge approvals
forge approve <approval-id> --comment "ship it"
forge messages software-factory --channel release
```

In the console, the canvas shows replica states, queue depth on each edge,
and approval badges; click a node for its prompt, executions, live output and
operator chat; click an edge for its message history and the inject form.

The software factory's agents write real files into the graph workspace and
commit them:

```bash
docker compose exec forged sh -c 'cd /var/lib/leforge/workspaces/software-factory && git log --oneline && ls src tests plans'
```

## 3. Use a real model

The default provider is `mock`: deterministic, offline, free. Every agent
handles it (the Python agents fall back to heuristics), which is what makes
`docker compose up` fully functional without a model. Switch providers with
`LEFORGE_DEFAULT_PROVIDER`; individual nodes can also pin one with
`model: {provider: <name>, name: <model>}`.

### From the browser (recommended)

Open **Integrations → Model providers** and choose **Add provider** (admin
token required). Pick the type, enter the base URL if it is not the
default, the default model and the API key, then **Test settings**: the test
lists models and sends one short prompt through LeForge, so a green result
means agents can actually use it. Save, optionally **Make default**, and pick
the provider per agent in the graph editor (agent panel → *Provider* /
*Model*). The key is encrypted on the server and never shown again; leave
the field empty when editing to keep it. See
[installation key](configuration.md#installation-key) for backups and
rotation.

The URL must be reachable from the `forged` container: a model server on
your machine is `http://host.docker.internal:<port>` on Docker Desktop, and
the bundled Ollama service is `http://ollama:11434`.

The sections below configure providers through environment variables
instead; those appear in the console as read-only *server configuration*.

### Ollama (local, offline after the first pull)

```bash
docker compose --profile ollama up -d ollama
docker compose exec ollama ollama pull llama3.2
LEFORGE_DEFAULT_PROVIDER=ollama docker compose --profile ollama up -d
```

`OLLAMA_BASE_URL` (default `http://ollama:11434`) and `OLLAMA_MODEL` (default
`llama3.2`) control the provider. Pick a model with tool-calling support for
native agents (e.g. `llama3.1`, `qwen2.5`).

### HuggingFace models (Text Generation Inference)

```bash
HF_MODEL_ID=Qwen/Qwen2.5-0.5B-Instruct docker compose --profile hf up -d tgi
LEFORGE_DEFAULT_PROVIDER=huggingface docker compose --profile hf up -d
```

Weights are cached in the `hf-models` volume; set `HF_HUB_OFFLINE=1` once they
are present. Any OpenAI-compatible server that serves HF weights works the same
way via `HF_BASE_URL`.

### vLLM, OpenAI, Anthropic, OpenRouter

Each provider is enabled by its environment variable (see
[configuration](configuration.md#providers-from-environment-variables)):

```dotenv
# vLLM or any OpenAI-compatible server (LM Studio, llama.cpp server, LocalAI)
VLLM_BASE_URL=http://vllm:8000/v1
VLLM_MODEL=Qwen/Qwen2.5-7B-Instruct
# OpenAI-compatible hosted APIs
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=https://api.openai.com/v1
# Anthropic-compatible APIs
ANTHROPIC_API_KEY=...
# OpenRouter
OPENROUTER_API_KEY=...

LEFORGE_DEFAULT_PROVIDER=vllm
```

Note: the compose file passes only the variables listed in its `x-forged-env`
block; add others (e.g. `VLLM_MODEL`, `OPENAI_MODEL`, `ANTHROPIC_MODEL`,
`OPENROUTER_MODEL`, `HF_MODEL`) there or mount a config file with an explicit
`llm.providers` list. Keys are read by the platform only; agents never see them.

## 4. Write your first graph

```yaml
# my-first-graph.yaml
apiVersion: leforge.io/v1
kind: Graph
metadata: {name: Summariser}
spec:
  parameters:
    - {name: style, value: "three bullet points"}
  nodes:
    - id: summariser
      prompt: Summarise each document from @input as @style and hand the summary to @output.
      tools: [filesystem.read]
    - id: archivist
      prompt: Append every summary from @input to summaries.md in @workspace.
      tools: [{name: filesystem.write, paths: ["summaries.md"]}, filesystem.read]
  channels:
    - {id: docs, to: [summariser]}              # ingress: inject work here
    - {id: summaries, from: summariser, to: [archivist]}
```

Defaults fill in the rest: nodes with inputs wait for work (`loop.mode: event`),
nodes without inputs run once, runtime is `native`, one replica, process
isolation, shared workspace. Then:

```bash
forge validate -f my-first-graph.yaml     # errors and warnings, locally and on the server
forge apply -f my-first-graph.yaml
forge start summariser
forge inject summariser docs document '{"text":"..."}'
```

(With the compose alias, copy the file into the container first or use
`curl -X POST -H 'Content-Type: application/yaml' --data-binary @my-first-graph.yaml`.)

### Dry-run a topology for free

Set `runtime: {kind: echo}` (per node or in `spec.defaults`) to run the whole
machinery — queues, routing, clusters, approvals, persistence, tracing —
without calling a model. Echo agents forward each input to their outputs, and
source nodes emit a `tick` per turn. `config: {fail: true}` makes a node fail
(to test retries and dead-lettering); `config: {stopAfter: N}` exercises the
auto-stop handshake. See `examples/graphs/hello-echo.yaml`.

### Prompt tokens

| Token | Meaning |
|---|---|
| `@input` | the messages claimed this turn (listed in the system preamble) |
| `@output` | the node's outgoing channels; hand work on with the `emit` tool |
| `@workspace` / `@project` | the graph workspace (a git repository when `spec.workspace.git`) |
| `@loop` | the replica's private scratch folder (`.leforge/<node>/r<n>`) |
| `@<parameter>` | substituted with the parameter's value each turn |

## 5. Run without Docker

You need Go 1.26+, Python 3.10+, git and (for the console) Node 20+.

```bash
# console (optional)
(cd web && npm ci && npm run build)

# control plane: SQLite + filesystem queues + mock provider, no services needed
cd server
go build -o bin/forged ./cmd/forged && go build -o bin/forge ./cmd/forge
LEFORGE_ADMIN_TOKEN="$(openssl rand -hex 32)" \
LEFORGE_WEB_DIR=../web/dist \
LEFORGE_BOOTSTRAP_GRAPHS=../examples/graphs \
LEFORGE_BOOTSTRAP_START=hello-echo \
LEFORGE_LOG_FORMAT=text \
./bin/forged
```

Data goes to `server/data/` (`leforge.db`, `queues/`, `workspaces/`).
For local-only experiments `LEFORGE_AUTH_DISABLED=true` turns authentication off
(never on a shared host).

Python agents run as a separate process:

```bash
PYTHONPATH=sdk/python:agents PORT=8000 python3 -m leforge_agents
```

The example graph addresses them as `http://agents:8000`; when running locally,
change `spec.defaults.runtime.endpoint` to `http://127.0.0.1:8000` (or add
`127.0.0.1 agents` to `/etc/hosts`).

`scripts/go.sh` runs the Go toolchain in a container if you prefer not to
install Go (it produces Linux binaries, suitable for containers and CI).

## Next steps

- [Architecture](architecture.md) to understand the engine, queues and runtimes.
- [Extensibility](extensibility.md) to write an agent in any language or framework.
- [Deployment](deployment.md) and [production hardening](production-hardening.md) before going live.

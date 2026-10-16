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
docker compose ps              # forged, agents, nats, postgres, sandbox-daemon, otel-collector, prometheus, tempo, loki, grafana
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
(it pulls in `nats`, `postgres` and `agents`). Add `sandbox-daemon sandbox-image`
to that command when agents should run commands in containers.

`sandbox-daemon` is a Docker daemon dedicated to LeForge: agents with docker
isolation run their commands in containers there, never on your machine's
Docker. `sandbox-image` builds the default sandbox image inside it on every
`up` and then exits (status `Exited (0)` is expected). The console shows its
state under **Integrations → Sandbox runtime**. See
[sandbox runtime](sandbox-runtime.md).

### Prepare a project environment

In **Graph settings → Environment**, choose **Discover environment**. LeForge
checks its optional YAML definition, devcontainer, Dockerfiles and Compose in
that order. Select and review the source (and Compose development service),
accept it into the draft, save and build. Invalid higher-priority definitions
must be corrected or explicitly replaced; they do not silently fall through.
When nothing is found, **Analyze repository** creates an initial proposal for
administrator review in **Approvals**. Approval permits build; commands only run
when ready. No `.leforge/environment.yaml` is required.

Compatible tasks reuse prepared images and cache seeds while retaining separate
workspaces and writable caches. Setup runs per task. The agent panel explains
readiness, reuse, build failures and capacity waiting. Use **Integrations** for
network policy, resource cleanup preview and confirmation. Reset affects a stopped
graph's environment and preserves its workspace and shared resources needed by
other tasks. See [environments and troubleshooting](environments.md), the
[mise example](../examples/projects/mise-node/README.md), and the
[validated scope](sandbox-acceptance.md).

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
your machine is `http://host.docker.internal:<port>` (see
[Ollama installed on the host](#ollama-installed-on-the-host) for Linux and
listen-address notes), and the bundled Ollama service is `http://ollama:11434`.

The sections below configure providers through environment variables
instead; those appear in the console as read-only *server configuration*.

### What a model needs to run agents

Agents with tools (shell, git, files) need two things from a model. **Test
connection** in **Integrations** checks both:

1. **Tool calling.** The model must return tool calls in the provider's
   structured format. Some models only write the call as JSON text (for
   example code-tuned Qwen 2.5 variants on Ollama). The test then says
   *"Inference works, but not tool calling"*. LeForge runs a call written as
   text when it names an offered tool, but such models are unreliable for long
   tasks; prefer models whose test passes.
2. **A large enough context window.** A turn sends the whole conversation
   (task, tool results, file contents) on every model call. LeForge asks
   Ollama for 32768 tokens (`num_ctx`) by default — Ollama's own default is
   4096 and it silently drops the beginning of longer conversations, task
   included. Change it per provider (**Context window** under *Limits*, or
   `OLLAMA_CONTEXT_WINDOW` for the provider from `OLLAMA_BASE_URL`). Larger
   windows need more memory on the machine running the model.

When something goes wrong, the agent's turn fails with the reason, visible in
**Events** (`execution.failed`) and the agent's **Recent turns**:

| Turn error | Meaning | What to do |
|---|---|---|
| `prompt exceeds the model context window` | The conversation no longer fits the provider's context window | Raise the provider's context window, or give the agent a smaller task |
| `the model wrote a tool call as text instead of calling the tool` | The model answered with JSON that is not a usable call | Use a model whose connection test passes tool calling |
| `the model ended the turn without an answer or a tool call` | Empty answer, nothing done | Check the model and the prompt; this often follows a truncated context |
| `the model's answer was cut off by its output limit` | The answer hit the output token limit | Use a model or provider with a larger output limit |
| `model call timed out: … gave no answer within 5m0s` | The model server did not answer within the provider's timeout — typical for a large local model that is still loading or a machine short of memory (heavy swapping) | Use a smaller model, close memory-hungry apps, or raise the provider's timeout (**Timeout** under *Limits*, or `OLLAMA_TIMEOUT_SEC` for the provider from `OLLAMA_BASE_URL`) |
| `the turn used every model round without finishing` | The agent kept calling tools until its budget ran out | Raise **Max tool calls** in the agent's *Limits per turn*, or split the task |

### Ollama (local, offline after the first pull)

```bash
docker compose --profile ollama up -d ollama
docker compose exec ollama ollama pull llama3.2
LEFORGE_DEFAULT_PROVIDER=ollama docker compose --profile ollama up -d
```

`OLLAMA_BASE_URL` (default `http://ollama:11434`) and `OLLAMA_MODEL` (default
`llama3.2`) control the provider. Pick a model with tool-calling support for
native agents (e.g. `llama3.1`, `qwen2.5`).

### Ollama installed on the host

If Ollama already runs natively on your machine, use it instead of the
container: it can use the host's GPU and is not limited by the memory given to
Docker.

1. Make Ollama listen on an address containers can reach. By default it only
   accepts connections from `127.0.0.1`; set `OLLAMA_HOST=0.0.0.0` where Ollama
   is started (for `ollama serve`, the systemd unit, or the desktop app's
   environment):

   ```bash
   OLLAMA_HOST=0.0.0.0 ollama serve
   ```

   This exposes port 11434 to your network while it runs, without
   authentication. Restrict it with a firewall on shared or public networks.

2. Pull a model:

   ```bash
   ollama pull llama3.2
   ```

3. Point LeForge at the host. Add to `.env`:

   ```dotenv
   OLLAMA_BASE_URL=http://host.docker.internal:11434
   ```

   then recreate `forged` so it reads the variable:

   ```bash
   docker compose up -d forged
   ```

`host.docker.internal` resolves out of the box on Docker Desktop and Colima.
With Docker Engine on Linux, map it to the host gateway in a
`docker-compose.override.yml` next to `docker-compose.yml` (Compose merges it
automatically):

```yaml
services:
  forged:
    extra_hosts: ["host.docker.internal:host-gateway"]
```

Instead of the environment variable, you can also add the host Ollama as a
provider in **Integrations** with the same URL; it is then editable and
testable from the console.

LeForge sends the context window (`num_ctx`, default 32768) with every
request, so `OLLAMA_CONTEXT_LENGTH` on the Ollama side is not needed; set
`OLLAMA_CONTEXT_WINDOW` in `.env` (and `docker compose up -d forged`) to change
it for this provider.

Check the connection with **Test connection** on the `ollama` provider in
**Integrations**: it sends a real prompt, so success means agents can use the
model. A `no such host` error means the URL still points at the bundled
service (`http://ollama:11434`); `connection refused` usually means Ollama is
still listening only on `127.0.0.1`.

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

### Build and configure a graph in the browser

Everything above can be done from the console instead of YAML:

1. **Graphs → New graph**, keep or adjust the template, **Create**. The graph
   opens on the **Graph** tab.
2. **Graph settings** (toolbar) edits graph-wide values:
   - **Name** and **Description**;
   - **Repository URL**: a remote repository (`https://…`, `ssh://…` or
     `user@host:path`) cloned into the graph workspace the first time the
     graph starts, while the workspace is empty. Changing it later does not
     replace an existing workspace. Local paths and `file://` are refused, so a
     graph cannot copy another graph's workspace or the server's files.
     Private repositories need credentials, which are not supported yet;
   - **Parameters**: values used in prompts as `@name`;
   - **Max hops**: how long a chain of agent-to-agent messages may get.
3. **Add agent**, then select it to configure it in **Agent configuration**:
   name, runtime, provider and model, prompt (**Edit → Apply to draft**), loop
   and stop policy, replicas, **Command sandbox** (see below), **Limits per
   turn**, approval gates, and tools in **Tools**.
4. Drag from one agent's right handle to another's left handle to add a
   channel, then select it to set its type, message types, route condition
   and **Require approval before delivery**. A channel with approval and no consumer is a
   convenient "notify me" endpoint: its messages wait in **Approvals**.
5. **Save graph**. Problems are listed above the canvas and next to the field
   that caused them; fix them and they clear on their own.
6. **Run all** (or **Start** on one agent) and follow **Live output**,
   **Messages**, **Events** and **Approvals**.

**Limits per turn** bound one agent turn: **Turn timeout** (default `30m`),
**Max tool calls** (default 50) and **Token budget** (unlimited by default). A
turn that exceeds one fails and its input is redelivered, so raise them for
long tasks such as dependency upgrades (for example `2h` and 300 calls).

### Dry-run a topology for free

Set `runtime: {kind: echo}` (per node or in `spec.defaults`) to run the whole
machinery — queues, routing, clusters, approvals, persistence, tracing —
without calling a model. Echo agents forward each input to their outputs, and
source nodes emit a `tick` per turn. `config: {fail: true}` makes a node fail
(to test retries and dead-lettering); `config: {stopAfter: N}` exercises the
auto-stop handshake. See `examples/graphs/hello-echo.yaml`.

### Run agent commands in containers

Agents that hold `shell.exec` or the git tools should run their commands in a
container: select the agent, open **Agent configuration → Command sandbox**,
set **Isolation** to `docker`, and choose **Network** `none` (default) or
`bridge` when it must download dependencies. Commands then run on
`sandbox-daemon`, can reach the internet only when bridged, and never reach
LeForge's services or your network. Details, reset and troubleshooting:
[sandbox runtime](sandbox-runtime.md).

### Prompt tokens

| Token | Meaning |
|---|---|
| `@input` | the messages claimed this turn (listed in the system preamble) |
| `@output` | the node's outgoing channels; hand work on with the `emit` tool |
| `@workspace` / `@project` | the graph workspace (a git repository when `spec.workspace.git`) |
| `@loop` | the replica's private scratch folder (`.leforge/<node>/r<n>`) |
| `@<parameter>` | substituted with the parameter's value each turn |

## 5. Run without Docker

You need Go 1.26+, Python 3.10+, git and (for the console) Node 22.22.2+
within the Node 22 line. For development aligned with the container versions,
use Go 1.26, Python 3.12 and Node 22; see the
[asdf or mise setup instructions](../CONTRIBUTING.md#local-toolchain-versions).

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
- [Sandbox runtime](sandbox-runtime.md) to run agent commands in containers and reset them.
- [Project environments](environments.md) to give agents your project's toolchain (Go, Node, Python, ...).
- [Deployment](deployment.md) and [production hardening](production-hardening.md) before going live.

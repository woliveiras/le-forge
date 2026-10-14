# Configuration reference

`forged` reads configuration in three layers; later layers win:

1. **Built-in defaults** — a laptop setup with no services: SQLite, filesystem
   queues, the offline `mock` model provider (`config.Default()`).
2. **Config file** — YAML, given by `--config <path>` or `LEFORGE_CONFIG`.
   `${VAR}` references in the file are expanded from the environment before
   parsing. The container image ships `deploy/config/forge.yaml` at
   `/etc/leforge/forge.yaml`.
3. **Environment variables** — the overrides listed below.

Secrets are never written in the file: the file names the environment variable
(`tokenEnv`, `apiKeyEnv`) or mounted file (`tokenFile`, `apiKeyFile`) that
holds them. Startup fails fast on an invalid configuration (unknown default
provider, duplicate provider names, invalid roles, no API credentials).

## Reference

| Key | Env var | Default | Description |
|---|---|---|---|
| `instance` | `LEFORGE_INSTANCE` | hostname | Unique instance name; holder of replica leases and recorded on executions. Must differ per instance. |
| `server.addr` | `LEFORGE_ADDR` | `:8080` | Listen address (API, console, `/metrics`, `/internal`). |
| `server.internalUrl` | `LEFORGE_INTERNAL_URL` | `http://127.0.0.1:8080` | URL external agent runtimes use to call back **this** instance. Per instance (pod IP), never a load balancer. |
| `server.webDir` | `LEFORGE_WEB_DIR` | — (image: `/app/web`) | Serve the built console from this directory. |
| `server.corsOrigins` | — | `[]` | Allowed CORS origins (`*` allowed; avoid in production). |
| `server.maxBodyBytes` | — | `8388608` | Request body limit. |
| `server.internalAddr` | `LEFORGE_INTERNAL_ADDR` | — | Serve the callback API (`/internal/v1`) on its own listener (e.g. `:8081`) and remove it from `server.addr`; point `server.internalUrl` at that port. |
| `server.protectMetrics` | `LEFORGE_PROTECT_METRICS` | `false` | Require a viewer token on `/metrics` (configure Prometheus with a bearer token). |
| `auth.disabled` | `LEFORGE_AUTH_DISABLED` | `false` | Disable authentication entirely (local development only). |
| `auth.tokens[]` | `LEFORGE_ADMIN_TOKEN` adds `{name: admin, role: admin}` | `[]` | Static bearer tokens: `name`, `role` (`viewer`/`operator`/`admin`), and one of `tokenEnv`, `tokenFile`, `token` (inline, dev only). Minimum 12 characters. |
| `auth.oidc` | — | unset | `issuer`, `audience`, `roleClaim` (default `leforge_role`). Accepts RS256/ES256 JWTs verified against the issuer's JWKS. |
| `storage.type` | `LEFORGE_STORAGE_TYPE` | `sqlite` | `sqlite`, `postgres`, `fs`. |
| `storage.dsn` | `LEFORGE_STORAGE_DSN` | `file:data/leforge.db` | SQLite path/URI or PostgreSQL URL. |
| `storage.dir` | `LEFORGE_STORAGE_DIR` | `data` | Data directory for `fs` and default SQLite location. |
| `queue.type` | `LEFORGE_QUEUE_TYPE` | `fs` | `nats`, `redis`, `rabbitmq`, `kafka`, `fs`, `memory`. |
| `queue.url` | `LEFORGE_QUEUE_URL` | — | Broker URL (see [brokers](#brokers)). |
| `queue.dir` | `LEFORGE_QUEUE_DIR` | `data/queues` | Directory for the `fs` broker. |
| `queue.options` | — | `{}` | Adapter options (map of strings). |
| `workspaces.root` | `LEFORGE_WORKSPACES` | `data/workspaces` | One directory per graph; worktrees under `.worktrees/`. |
| `workspaces.hostRoot` | `LEFORGE_WORKSPACES_HOST` | = root | The same directory as the Docker daemon sees it (bind mounts for docker sandboxes when forged runs in a container). Not needed with Compose: `sandbox-daemon` mounts the workspaces volume at the same path. |
| `workspaces.allowAbsolute` | — | `false` | Allow graphs to set an absolute `spec.workspace.path`. |
| `llm.default` | `LEFORGE_DEFAULT_PROVIDER` | `mock` | Provider used when a node names none. Must be configured. |
| `llm.providers[]` | see [below](#providers-from-environment-variables) | `[{name: mock, type: mock}]` | Model providers. |
| `secrets.keyEnv` | `LEFORGE_SECRET_KEY` (the variable it names) | `LEFORGE_SECRET_KEY` | Variable holding the installation key (32 bytes, base64 or hex) that encrypts credentials entered in the console. Wins over `keyFile`. |
| `secrets.keyFile` | `LEFORGE_SECRET_KEY_FILE` | `data/secret.key` (image: `/var/lib/leforge/data/secret.key`) | File holding the key when the variable is empty. |
| `secrets.generateKeyFile` | `LEFORGE_SECRET_KEY_GENERATE` | `true` | Create `keyFile` (mode 0600) with a random key on first start when no key exists. Turn off when several instances run (they must share one key). |
| `secrets.previousKeysEnv` | `LEFORGE_SECRET_KEY_PREVIOUS` (the variable it names) | `LEFORGE_SECRET_KEY_PREVIOUS` | Comma-separated older keys that still decrypt; values are re-encrypted with the current key at startup (rotation). |
| `runtimes.sharedSecretEnv` | — (image: `LEFORGE_RUNTIME_SECRET`) | — | Env var whose value is sent to external runtimes as `X-Leforge-Runtime-Secret`. |
| — | `LEFORGE_RUNTIME_NETWORK` | — | Docker network for runtimes launched from `runtime.image` with docker isolation. |
| `isolation.docker.bin` | — | `docker` | Docker CLI used by docker sandboxes and launched runtimes. |
| `isolation.docker.image` | `LEFORGE_SANDBOX_IMAGE` | `leforge/sandbox:latest` | Default sandbox image. |
| `isolation.docker.network` | — | `none` | Default sandbox network. |
| `isolation.docker.host` | `LEFORGE_SANDBOX_DOCKER_HOST` | — | Docker daemon for sandboxes (`DOCKER_HOST` syntax). Compose sets `tcp://sandbox-daemon:2376` with `DOCKER_TLS_VERIFY`/`DOCKER_CERT_PATH`. Setting it enables the `sandbox` readiness check. See [sandbox-runtime.md](sandbox-runtime.md). |
| `tools.allowPrivateNetworks` | — | `false` | Let `http.fetch` reach loopback/private/link-local addresses (SSRF guard off). |
| `observability.serviceName` | `OTEL_SERVICE_NAME` | `leforge` | OTel service name. |
| `observability.otlpEndpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP/HTTP base URL; empty disables trace/log export. |
| `observability.traceSampleRatio` | — | `1` | Parent-based ratio sampler. |
| `observability.logLevel` | `LEFORGE_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `observability.logFormat` | `LEFORGE_LOG_FORMAT` | `json` | `json` or `text`. |
| `observability.grafanaUrl` | `LEFORGE_GRAFANA_URL` | — | Console links to dashboards and traces. |
| `engine.leaseTTL` | — | `20s` | Replica lease lifetime (min 3s). Failover time ≈ TTL. |
| `engine.reconcileInterval` | — | `2s` | Reconcile/renew period; clamped to ≤ TTL/2. |
| `engine.retryBackoff` | — | `2s` | Base delay for nacked inputs (doubles per attempt, max 5m). |
| `engine.maxConcurrentTurns` | — | `0` (unlimited) | Cap on simultaneous turns per instance. |
| `bootstrap.graphs` | `LEFORGE_BOOTSTRAP_GRAPHS` (comma list) | `[]` | Graph files or directories loaded at startup (saved only when changed). |
| `bootstrap.start` | `LEFORGE_BOOTSTRAP_START` (comma list) | `[]` | Graph ids started after bootstrapping. |

### Providers from environment variables

Each variable below adds a provider (unless one with that name exists in the
file), so a single variable enables a backend:

| Trigger | Provider name / type | Base URL | Model (env, default) |
|---|---|---|---|
| `OLLAMA_BASE_URL` | `ollama` / `ollama` | the variable | `OLLAMA_MODEL`, `llama3.2` |
| `OPENAI_API_KEY` | `openai` / `openai` | `OPENAI_BASE_URL`, `https://api.openai.com/v1` | `OPENAI_MODEL`, `gpt-4o-mini` |
| `ANTHROPIC_API_KEY` | `anthropic` / `anthropic` | `ANTHROPIC_BASE_URL`, `https://api.anthropic.com` | `ANTHROPIC_MODEL`, `claude-sonnet-5-5` |
| `OPENROUTER_API_KEY` | `openrouter` / `openrouter` | `https://openrouter.ai/api/v1` | `OPENROUTER_MODEL`, `meta-llama/llama-3.3-70b-instruct` |
| `VLLM_BASE_URL` | `vllm` / `vllm` | the variable | `VLLM_MODEL` |
| `HF_BASE_URL` | `huggingface` / `huggingface` | the variable | `HF_MODEL`, `tgi` (key from `HF_API_KEY` if set) |

### Provider configuration

```yaml
llm:
  default: local-ollama
  providers:
    - {name: offline, type: mock}
    - {name: local-ollama, type: ollama, baseUrl: http://ollama:11434, defaultModel: llama3.1, concurrency: 2}
    - {name: vllm, type: vllm, baseUrl: http://vllm:8000/v1, defaultModel: Qwen/Qwen2.5-7B-Instruct}
    - {name: lmstudio, type: lmstudio, baseUrl: http://host.docker.internal:1234/v1}
    - {name: hf-local, type: huggingface, baseUrl: http://tgi:80/v1}          # TGI / any OpenAI-compatible HF server
    - {name: openai, type: openai, apiKeyEnv: OPENAI_API_KEY, defaultModel: gpt-4o-mini}
    - {name: anthropic, type: anthropic, apiKeyEnv: ANTHROPIC_API_KEY, defaultModel: claude-sonnet-5-5}
    - {name: openrouter, type: openrouter, apiKeyFile: /run/secrets/openrouter, defaultModel: meta-llama/llama-3.1-70b-instruct,
       headers: {X-Title: LeForge}}
```

Fields: `name`, `type` (`openai`, `vllm`, `lmstudio`, `anthropic`, `ollama`,
`openrouter`, `huggingface`/`tgi`, `mock`), `baseUrl`, `apiKeyEnv` /
`apiKeyFile`, `defaultModel`, `headers`, `timeoutSec` (default 300),
`maxRetries` (default 3; 429/408/5xx/network errors with backoff and
`Retry-After`), `concurrency` (per-provider in-flight cap, 0 = unlimited).

Nodes select models with `model: {provider, name, temperature, maxTokens}`;
an empty provider means `llm.default`, an empty name the provider's
`defaultModel`.

### Providers managed in the console

Besides the providers above (shown read-only in the console as *server
configuration*), administrators add, edit, test and remove providers under
**Integrations → Model providers**, and may choose a different default
provider there. Those providers are stored in the database with their API
key encrypted by the installation key (`secrets.*`); keys are never returned
by the API after saving. Their names cannot clash with configured providers,
and `default` is reserved. Changes apply immediately on the instance that
made them and within 5 seconds on other instances.

A node that selects a provider that does not exist or cannot be used (for
example, its credential cannot be decrypted) is reported as a warning when
the graph is saved, and the graph or node is refused with `422` when started.

### Installation key

Credentials entered in the console are encrypted (AES-256-GCM) with the
installation key before they reach storage. Without a key, the console
refuses to store credentials (`409 secret_key_missing`) and keyless
providers (Ollama, vLLM, ...) still work.

* **Single instance (Docker Compose):** by default a key is generated on
  first start in the `forged-data` volume. Back up that volume (or set
  `LEFORGE_SECRET_KEY` yourself with `openssl rand -base64 32`): without the
  key, stored credentials must be entered again.
* **Several instances (Kubernetes):** set `LEFORGE_SECRET_KEY` from a Secret
  on every instance and set `secrets.generateKeyFile: false`.
* **Rotation:** set the new key as `LEFORGE_SECRET_KEY`, move the old one to
  `LEFORGE_SECRET_KEY_PREVIOUS`, restart (values are re-encrypted and the log
  reports how many), then remove the previous key.
* **Lost key:** affected providers show *Unavailable* with the reason; agents
  selecting them fail with that reason instead of using another provider.
  Re-enter the credential in the console to repair them.

### Brokers

| Type | URL | Options |
|---|---|---|
| `nats` (default when type is empty) | `nats://host:4222` | `replicas` (stream replicas), `creds` (credentials file), `name`, `advisories=false` |
| `redis` | `redis://host:6379/0` (Redis ≥ 7) | — |
| `rabbitmq` | `amqp://user:pass@host:5672/` | `ackWait`, `maxDeliver` |
| `kafka` | `kafka://h1:9092,h2:9092` or host list | `partitions` (default 12), `replicationFactor`, `ackWait`, `maxDeliver`, `clientId` |
| `fs` | `file:///path` or `queue.dir` | — |
| `memory` | — | — (not durable; tests) |

### Storage

| Type | DSN | Notes |
|---|---|---|
| `sqlite` (default) | path or `file:` URI | WAL, single writer connection; single instance only. |
| `postgres` | `postgres://user:pass@host:5432/db?sslmode=require` | Required for multiple instances. Migrations run at startup under an advisory lock. |
| `fs` | `storage.dir` | JSON files + JSONL event log; one process per directory (lock file). |

## Example: production-style file

```yaml
instance: ${HOSTNAME}
server:
  addr: ":8080"
  internalUrl: "http://${POD_IP}:8080"
storage: {type: postgres}                      # DSN from LEFORGE_STORAGE_DSN
queue: {type: nats, url: "nats://nats:4222", options: {replicas: "3"}}
workspaces: {root: /var/lib/leforge/workspaces}
auth:
  tokens:
    - {name: ci, role: operator, tokenEnv: LEFORGE_CI_TOKEN}
    - {name: grafana, role: viewer, tokenFile: /run/secrets/viewer-token}
  oidc: {issuer: "https://sso.example.com/realms/eng", audience: leforge, roleClaim: leforge_role}
llm:
  default: vllm
  providers:
    - {name: vllm, type: vllm, baseUrl: "http://vllm.models:8000/v1", defaultModel: "Qwen/Qwen2.5-32B-Instruct", concurrency: 16}
runtimes: {sharedSecretEnv: LEFORGE_RUNTIME_SECRET}
observability: {otlpEndpoint: "http://otel-collector.observability:4318", traceSampleRatio: 0.2}
engine: {leaseTTL: 30s, reconcileInterval: 5s, maxConcurrentTurns: 64}
```

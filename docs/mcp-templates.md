# MCP template candidates

Research date: 2026-10-17. This is a proposed catalogue, not a list of shipped
integrations. The four Release 1 integrations remain GitHub, Bitbucket Cloud,
Jira Cloud and Confluence Cloud; see [their contracts](mcp-integrations.md).

A template should prefill reviewed configuration and explain prerequisites. It
must not install a server, create credentials or grant an agent access implicitly.
Compatibility below is a planning assessment against [the current client](mcp.md),
not a live certification. DeepWiki has previously been exercised in the installed
LeForge browser/gateway flow; no other candidate is certified by this survey.

## Software engineering

| Candidate / official source | Useful work | Connection and authentication | LeForge delivery |
| --- | --- | --- | --- |
| [DeepWiki](https://docs.devin.ai/work-with-devin/deepwiki-mcp) | Understand public repositories and their architecture | `https://mcp.deepwiki.com/mcp`; remote Streamable HTTP, no credential | First template and repeatable public smoke test. Discover actual tool names rather than copying examples. |
| [Context7](https://context7.com/docs/resources/all-clients) | Library documentation and code examples | `https://mcp.context7.com/mcp`; HTTP with Bearer API key; separate `/mcp/oauth` OAuth endpoint | First wave, using encrypted Bearer credentials; live compatibility test pending. |
| [Sentry](https://mcp.sentry.dev/) | Investigate errors and performance | `https://mcp.sentry.dev/mcp`; remote OAuth; optional organization/project path | After OAuth support; expose project selection and read permissions first. |
| [Playwright](https://github.com/microsoft/playwright-mcp) | Browser interaction and UI investigation | Official server launched locally or in a container; no hosted public endpoint selected | Isolated managed-server track. Browser lifecycle, inbound access and resource limits must be designed before offering an executable template. |

## Data engineering

| Candidate / official source | Useful work | Connection and authentication | LeForge delivery |
| --- | --- | --- | --- |
| [dbt](https://docs.getdbt.com/docs/dbt-ai/setup-remote-mcp) | Model metadata, lineage and semantic-layer context | `https://<dbt-host>/api/ai/v1/mcp/`; remote OAuth or token (`Authorization: Token` **or** `Bearer`); environment headers such as `x-dbt-prod-environment-id` | Typed additional headers, project/environment selection and per-tool credential requirements. Remote service does not replace local dbt CLI execution. |
| [Databricks](https://docs.databricks.com/aws/en/generative-ai/mcp/custom-mcp-usage) | Governed workspace data and managed services | `https://<workspace-host>/api/2.0/mcp/<service>/<path>`; OAuth user/service-principal credentials; privileges depend on selected service | Workspace/service templates, token lifecycle and service-specific acceptance. Do not assume one universal scope or endpoint. |
| [BigQuery](https://docs.cloud.google.com/bigquery/docs/use-bigquery-mcp) | Dataset/table exploration and SQL | `https://bigquery.googleapis.com/mcp`; Google identity/IAM | Protocol qualification first: current documentation describes stateless 2026-07-28 without initialize/session lifecycle, unlike LeForge 2025-06-18. Add IAM guidance and query-cost controls before SQL grants. |
| [Snowflake](https://docs.snowflake.com/en/user-guide/snowflake-cortex/cortex-agents-mcp) | Governed Cortex/data tools | Account-scoped `/api/v2/databases/<db>/schemas/<schema>/mcp-servers/<server>` endpoint; PAT/OAuth and role privileges | Qualify lifecycle/version differences documented by Snowflake before compatibility claims; then account/server/role setup and least-privilege profiles. |

## Machine learning and AI engineering

| Candidate / official source | Useful work | Connection and authentication | LeForge delivery |
| --- | --- | --- | --- |
| [Hugging Face Hub](https://github.com/huggingface/hf-mcp-server) | Discover models, datasets, papers and Spaces | `https://huggingface.co/mcp`; remote HTTP, optional Bearer HF token | First wave with discovery/search profile; separately review extra Space tools, data transfer and compute capabilities. |
| [W&B Models / Weave](https://github.com/wandb/wandb-mcp-server) | Compare experiments, inspect metrics, traces and evaluations | `https://mcp.withwandb.com/mcp`; Bearer W&B API key | First wave, subject to protocol tests; start with read tools and project-scoped credentials. |
| [MLflow](https://mlflow.org/docs/latest/genai/mcp/) | Analyze GenAI traces and feedback | Documented local `mlflow mcp run`; tracking-server configuration; no hosted MCP URL selected | Managed isolated-server track with separate upstream authentication. Do not confuse the tracking URL with an MCP endpoint. |
| [LangSmith](https://github.com/langchain-ai/langsmith-mcp-server) | Inspect traces, datasets, prompts and evaluations | `https://langsmith-mcp-server.onrender.com/mcp`; `LANGSMITH-API-KEY`, optional workspace/endpoint headers | Encrypted custom-header support and workspace selection. Confirm each tool's actual capability; documentation tools do not imply remote execution of evaluations. |
| [Qdrant](https://qdrant.tech/documentation/qdrant-mcp-server/) | Semantic retrieval and agent memory | [Official server](https://github.com/qdrant/mcp-server-qdrant), stdio or Streamable HTTP; upstream Qdrant URL/API key | Managed isolated-server track. Database credentials do not authenticate inbound MCP clients; provision TLS/auth separately. Retrieval first, storage writes separately granted. |

## Proposed delivery order

1. Versioned template schema and UI, then DeepWiki, Context7, Hugging Face and W&B.
   These exercise no-auth/Bearer configurations without requiring new credential
   mechanisms. Only DeepWiki has existing installed-flow evidence.
2. Complete the four priority vendor contracts and guided authentication, then
   Sentry, dbt, LangSmith and Databricks as their dependencies become available.
3. Qualify BigQuery and Snowflake protocol differences; implement explicit
   compatibility profiles only after tests establish their contracts.
4. Design isolated MCP hosting for Playwright, MLflow and Qdrant. Keep arbitrary
   startup commands out of the LeForge main process; private networking remains
   a separate explicit policy, not a template exception.

Each template needs an owner/source URL, review date, version, endpoint parameters,
transport/protocol profile, credential fields, scope guidance, data boundary,
read/write/cost classification and validation status. New discoveries never
silently expand existing agent grants. Template updates must show a configuration
diff before changing an existing connection.

Acceptance for each implemented template: unit tests for validation/defaults and
invalid inputs; integration tests with TLS, storage and gateway authorization;
browser E2E for setup, discovery, exact grants, invocation and revocation; an
opt-in live smoke test with authorized test resources. Cover expired/insufficient
credentials, timeouts, pagination, schema drift and redaction. A fixture success
is not proof of live vendor compatibility. Live write tests need explicit approved
test targets; no automatic production mutations or paid jobs.

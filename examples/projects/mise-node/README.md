# mise sandbox example

A small repository with `mise.toml`, Node 22.14.0 and one locked npm dependency.
It intentionally has no LeForge, devcontainer, Dockerfile or Compose definition.
Copy these files into a repository used by a LeForge graph. In **Graph settings →
Environment**, discover, analyze and review the initial proposal in **Approvals**.
Analysis requires a configured model; its output must be reviewed, not assumed
to match the deterministic test proposal below.

The validated proposal uses the official
[mise Debian image](https://mise.jdx.dev/mise-cookbook/docker.html), pinned to
`ghcr.io/jdx/mise:2026.10.4-debian@sha256:2fb7fa9a4120333531a8a70296d4ea152ffd3e7658dcf29ac75774a7d78dec24`.
Use an inline Dockerfile with that `FROM` and:

```dockerfile
ENV MISE_DATA_DIR=/home/sandbox/.local/share/mise MISE_CONFIG_DIR=/home/sandbox/.config/mise MISE_CACHE_DIR=/home/sandbox/.cache/mise
```

Review these setup commands before approving:

```sh
mise trust /workspace/mise.toml
mise install
mise exec -- npm ci --ignore-scripts
```

Keep the supplied `.dockerignore`: it excludes installed dependencies and task
outputs from the approved Docker build context, so setup does not invalidate
its own accepted inputs.

Setup uses `network.setup: bridge` to download tools and packages; execution uses
`network.run: none`. The installation must allow bridge setup. This example does
not bypass a stricter policy or expand any host allowlist. Declare only
`/home/sandbox/.npm/_cacache` as a reusable cache; do not cache credentials or task
outputs. mise tool directories remain private. See the official
[mise installation documentation](https://mise.jdx.dev/installing-mise.html).

After approval and build, ask the agent to run these commands with `shell.exec`
inside its Docker sandbox:

```sh
mise exec -- npm run build
mise exec -- npm test
```

The build copies the module into `built.cjs`; two tests exercise its dependency
and invalid-input handling. npm lifecycle scripts are intentionally disabled
for this example. Nothing needs to be installed on the developer's host.

`make test-e2e-real` automates this journey with scripted inference but real
approval, downloads, build, setup, execution and file checks. A second task for
the same repository reuses the prepared image and compatible npm seed while
receiving a different writable cache, final setup image and workspace. Setup
runs again in that workspace; this is not reuse of another task's node_modules
or installed toolchain. The test requires internet access to GHCR, Node's
release distribution and npm during preparation; execution is offline.

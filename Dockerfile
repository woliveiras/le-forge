# LeForge control plane: forged (API + engine + console) and the forge CLI.
#   docker build -t leforge/forged .
# Multi-stage: the console is built with Node, the binaries with Go, and the
# runtime image carries only what agents' tools need (git, python3, a shell).

FROM node:22-alpine AS web
RUN corepack enable
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS build
WORKDIR /src/server
# -p=2 bounds compiler memory so the image builds on small Docker VMs (2 GB).
ENV CGO_ENABLED=0 GOFLAGS="-trimpath -p=2"
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
ARG VERSION=dev
RUN go build -ldflags "-s -w -X github.com/woliveiras/le-forge/server/internal/api.Version=${VERSION}" -o /out/forged ./cmd/forged \
 && go build -ldflags "-s -w" -o /out/forge ./cmd/forge \
 && go build -ldflags "-s -w" -o /out/forge-preview-linux ./cmd/forge-preview

FROM alpine:3.22
# git: workspaces and worktrees; python3: QA agents run test suites through
# shell.exec in the process sandbox; docker-cli: docker isolation, talking to
# the dedicated sandbox daemon (LEFORGE_SANDBOX_DOCKER_HOST); tini: signal
# handling for children.
RUN apk add --no-cache ca-certificates git python3 tini docker-cli docker-cli-buildx tzdata \
 && addgroup -S -g 10001 leforge && adduser -S -u 10001 -G leforge -h /var/lib/leforge leforge \
 && mkdir -p /var/lib/leforge/data /var/lib/leforge/workspaces /app \
 && chown -R leforge:leforge /var/lib/leforge
COPY --from=build /out/forged /out/forge /usr/local/bin/
COPY --from=build /out/forge-preview-linux /usr/local/lib/leforge/forge-preview-linux
COPY --from=web /web/dist /app/web
COPY examples /app/examples
COPY deploy/config/forge.yaml /etc/leforge/forge.yaml
ENV LEFORGE_WEB_DIR=/app/web \
    LEFORGE_PREVIEW_RELAY=/usr/local/lib/leforge/forge-preview-linux \
    LEFORGE_STORAGE_DIR=/var/lib/leforge/data \
    LEFORGE_WORKSPACES=/var/lib/leforge/workspaces \
    LEFORGE_QUEUE_DIR=/var/lib/leforge/data/queues \
    HOME=/var/lib/leforge
USER 10001
WORKDIR /var/lib/leforge
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=6 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["/sbin/tini", "--", "forged"]
CMD ["--config", "/etc/leforge/forge.yaml"]

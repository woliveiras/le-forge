# The collector uses only the dedicated daemon's existing privileges.
FROM golang:1.26-alpine AS build
ENV CGO_ENABLED=0
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN go build -o /forge-workspace-journal ./cmd/forge-workspace-journal

FROM docker:28.5.2-dind
COPY --from=build /forge-workspace-journal /usr/local/bin/forge-workspace-journal

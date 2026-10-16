#!/usr/bin/env sh
# Run the Go toolchain inside a container so contributors only need Docker.
# Module and build caches live in named volumes so repeated runs are fast.
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
GO_IMAGE=${GO_IMAGE:-golang:1.26-alpine}
if [ "${1:-}" = test ]; then
  # Real workspace integration tests need Git. Build a cached toolchain image;
  # never install it on the host or silently skip these tests.
  GO_IMAGE=$(docker build -q --build-arg "GO_IMAGE=$GO_IMAGE" -f "$ROOT/scripts/Dockerfile.go-test" "$ROOT/scripts")
fi
exec docker run --rm \
  -v "$ROOT:/repo" \
  -v leforge-gomod:/go/pkg/mod \
  -v leforge-gocache:/root/.cache/go-build \
  -e CGO_ENABLED=0 -e GOFLAGS="${GOFLAGS:-}" \
  -w /repo/server "$GO_IMAGE" go "$@"

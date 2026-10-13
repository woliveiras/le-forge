# Contributing to LeForge

Thanks for helping. LeForge is a vendor-neutral platform; contributions that add
coupling to a specific cloud, model vendor or proprietary runtime will not be
merged into core — put them behind one of the extension interfaces instead
(see [docs/extensibility.md](docs/extensibility.md)).

## Development setup

Only Docker is required. Go runs in a container through `scripts/go.sh`; Python
3.10+ and Node 20+ are used for the SDK and the console if installed locally.

```bash
make build        # forged + forge into server/bin
make test         # Go, Python and web type-check
make lint         # gofmt + go vet
make up           # full stack via docker compose
```

Live broker/database suites are skipped unless their environment variables are
set: `LEFORGE_TEST_NATS_URL`, `LEFORGE_TEST_REDIS_URL`,
`LEFORGE_TEST_RABBITMQ_URL`, `LEFORGE_TEST_KAFKA_BROKERS`,
`LEFORGE_TEST_POSTGRES_DSN`.

## Ground rules

- **Contracts first.** The wire contracts (`docs/dev/contracts.md`, the JSON
  tags in `server/internal/domain`, `runtime`, `llm`, `tools`) are public API.
  Additive changes only; anything else needs an ADR in `docs/adr/`.
- **Adapters pass conformance.** A new broker must pass
  `queue/conformance`; a new store must pass `storage/conformance`.
- **Metric names are contracts.** Add metrics, never rename them
  (`server/internal/observability/metrics.go`).
- **Security defaults stay closed.** Tool grants are default-deny; new tools
  declare a capability and enforce their own constraints (paths, commands,
  hosts) through the helpers in `server/internal/tools/policy.go`.
- **Tests** accompany every behaviour change. Engine behaviour is tested
  end-to-end in `server/internal/engine/engine_test.go`.
- Run `make lint test` before opening a pull request. Keep commits focused and
  describe the *why* in the message.

## Architecture decisions

Significant decisions are recorded as ADRs in `docs/adr/`. Propose a new one
(next number, status "Proposed") in the same pull request as the change.

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0.

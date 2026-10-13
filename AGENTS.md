# Engineering Guidelines

These guidelines apply to the LeForge project. Before changing code, identify
the relevant component, language, architecture, conventions, tests, and
available commands. Follow existing patterns and consult the project
documentation if instructions conflict.

## Required Testing

- For every change that modifies or adds behavior, write or update **both**
  unit tests and integration tests. Do not consider the implementation complete
  without both types of tests.
- Unit tests must cover the unit's behavior and error cases, isolating external
  dependencies according to the project's capabilities and conventions.
- Integration tests must exercise real collaboration between components or
  relevant boundaries (for example, APIs, storage, queues, or UI), without
  replacing the integration with mocks that hide the behavior under test.
- Include success cases, invalid inputs, dependency failures, and important
  boundary conditions. Prefer assertions about results and observable effects
  over tests coupled to internal details.
- Run relevant tests after the change and, when feasible, the full suite and
  build, type-checking, and lint checks documented by the project. Do not claim
  that checks passed unless you ran them. If a test cannot be run, explain which
  one, why, and what still needs validation.
- If one type of test is genuinely inapplicable, explain why objectively and
  propose the closest alternative validation; do not silently omit coverage.
- Changes that are exclusively documentation or formatting, with no behavior
  change, do not require new tests.

## Error Handling

- Identify expected failure modes at code boundaries and handle them
  explicitly, preserving useful context for callers and diagnosis.
- Propagate errors when the current layer cannot resolve them; convert them
  into appropriate responses or states at the boundary responsible for
  presenting them.
- Do not ignore errors, use fallbacks that appear successful, or introduce
  broad catch/recovery blocks without need. Do not use panic, force unwrap, or
  equivalents for recoverable failures.
- Avoid logging the same error in every layer. Record or present the error at
  the point with the context and responsibility to act on it.
- Add tests for error paths and verify that resources, transactions, and state
  remain consistent after failures.

## Architecture and Design Patterns

- Preserve existing boundaries, contracts, and abstractions. Prefer simple,
  cohesive solutions aligned with the documented architecture.
- Use design patterns when they solve a concrete problem involving coupling,
  creation, extensibility, or coordination. Choose a pattern suited to the
  context and explain the decision when it is not obvious.
- Do not introduce patterns, layers, interfaces, or abstractions just for
  formality. Avoid duplication and reuse existing components when doing so does
  not increase coupling.
- Update contracts and documentation directly affected by the change.

## Observability

- Make important flows diagnosable using the mechanisms already adopted by the
  project: structured logs, metrics, and distributed tracing, as applicable.
- Record important events and failures with appropriate severity, context, and
  correlation identifiers. Prefer structured logs over text that is difficult
  to query.
- Instrument outcome, failure, and latency metrics at critical boundaries and
  operations; propagate spans and tracing context through relevant calls when
  the project already supports them.
- Do not log secrets, credentials, tokens, personal data, or sensitive
  payloads. Avoid unbounded cardinality in metric labels and unnecessary log
  volume.
- Maintain compatibility with existing telemetry names and contracts. When
  adding or changing instrumentation, update related tests and documentation.

## Before Completing a Change

- Review the diff to confirm that the change is limited to the request, that
  unit and integration tests cover the changed behavior, and that relevant
  errors and observability signals have been considered.
- Report the commands run, their results, and any pending checks. Never declare
  the task complete if required tests were omitted without justification.

## Known Commands

- Run `make test`, `make lint`, and `make build` from the LeForge project root.
- Consult the project's `Makefile` and documentation before running commands;
  some tests or builds may depend on local services or tools.
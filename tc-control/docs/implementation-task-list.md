# tc-control Implementation Task List

## Purpose

This document turns the `tc-control` Definition of Done into an implementation order for the control plane backend.

## Rules

- Work top-down unless a dependency forces a change.
- Keep the message hot path out of `tc-control`.
- Treat server-accepted state and projections derived from it as truth.
- Do not write broker or storage internals directly from handlers.
- Every mutation must be authorized, idempotent where needed, and audited.
- Every behavior change must add or update tests in the same milestone.

## Status Legend

```text
[ ] not started
[~] in progress
[x] done
```

## Current Implementation Checkpoint

As of the CLI-first implementation slice, `tc-control` has a runnable HTTP API for:

- health, readiness, and version
- server-backed snapshot projection
- endpoint, capability, task, message, artifact, approval, DLQ, and side-effect queries
- message send
- task cancel and retry
- approval decision forwarding
- artifact finalization
- DLQ replay

Known remaining gaps:

- authn/authz is not enforced yet
- mutation audit records are not persisted yet
- read models are server-backed projections, not a dedicated projection store
- admin compatibility is intentionally deferred while UI is excluded

## Milestone 0: Freeze Control-Plane Decisions

Tasks:

- [ ] TCCP-0001 Choose Control API style.
- [ ] TCCP-0002 Freeze API versioning and compatibility policy for `tcctl` and `admin`.
- [ ] TCCP-0003 Freeze server command port shape.
- [ ] TCCP-0004 Freeze read-model/projection storage adapter.
- [ ] TCCP-0005 Freeze authn/authz mode.
- [ ] TCCP-0006 Freeze audit record schema.
- [ ] TCCP-0007 Freeze local development stack.

Exit checks:

- API, compatibility, server command, read-model, auth, audit, and local stack decisions are documented.

## Milestone 1: Service Skeleton

Tasks:

- [ ] TCCP-0101 Initialize the Go module if it does not exist.
- [ ] TCCP-0102 Add `tc-control/cmd/tc-control/main.go`.
- [ ] TCCP-0103 Add package layout for API, application, ports, adapters, config, and observability.
- [ ] TCCP-0104 Add config loader with environment variable support.
- [ ] TCCP-0105 Add config validation and redaction.
- [ ] TCCP-0106 Add structured logging baseline.
- [ ] TCCP-0107 Add health, readiness, and version endpoints.
- [ ] TCCP-0108 Add graceful shutdown.
- [ ] TCCP-0109 Add build/test commands.

Suggested package boundaries:

```text
tc-control/cmd/tc-control
tc-control/internal/control
tc-control/internal/app
tc-control/internal/domain
tc-control/internal/ports
tc-control/internal/adapters/serverapi
tc-control/internal/adapters/readmodel
tc-control/internal/adapters/api
tc-control/internal/config
tc-control/internal/observability
```

Exit checks:

- `tc-control` starts with valid config.
- invalid config fails fast.
- readiness reports `tc-server` and read-model status.
- `go test ./...` passes.

## Milestone 2: API Schema and Server Client Boundary

Tasks:

- [ ] TCCP-0201 Define request and response envelope.
- [ ] TCCP-0202 Define stable domain error envelope.
- [ ] TCCP-0203 Define Control API schemas for all groups.
- [ ] TCCP-0204 Define server command port.
- [ ] TCCP-0205 Define read-model query port.
- [ ] TCCP-0206 Implement server API compatibility check.
- [ ] TCCP-0207 Implement timeout and retry policy for server calls.
- [ ] TCCP-0208 Add API schema and error mapping tests.

Exit checks:

- handler tests pass without real `tc-server`.
- server unavailable and server rejection states map to stable errors.

## Milestone 3: Read-Only Query APIs

Tasks:

- [ ] TCCP-0301 Implement endpoint list and inspect.
- [ ] TCCP-0302 Implement capability list.
- [ ] TCCP-0303 Implement task status and history.
- [ ] TCCP-0304 Implement message inspect and task message history.
- [ ] TCCP-0305 Implement artifact list and inspect.
- [ ] TCCP-0306 Implement approval list and inspect.
- [ ] TCCP-0307 Implement DLQ list and inspect.
- [ ] TCCP-0308 Implement pagination, sorting, and filtering.
- [ ] TCCP-0309 Implement projection freshness metadata.

Exit checks:

- query APIs return exact ids and refs.
- raw artifact bodies, secrets, and full prompts are not exposed by default.

## Milestone 4: Mutation APIs and Audit

Tasks:

- [ ] TCCP-0401 Implement actor identity extraction.
- [ ] TCCP-0402 Implement authorization checks.
- [ ] TCCP-0403 Implement command idempotency handling.
- [ ] TCCP-0404 Implement audit record creation.
- [ ] TCCP-0405 Implement task create.
- [ ] TCCP-0406 Implement message send.
- [ ] TCCP-0407 Implement task retry.
- [ ] TCCP-0408 Implement task cancel.
- [ ] TCCP-0409 Implement artifact finalize.

Exit checks:

- every mutation produces an audit record.
- local success is reported only after server acceptance.
- duplicate protected commands are deterministic.

## Milestone 5: Approval and DLQ Workflows

Tasks:

- [ ] TCCP-0501 Implement approval approve.
- [ ] TCCP-0502 Implement approval reject.
- [ ] TCCP-0503 Implement approval hash validation path.
- [ ] TCCP-0504 Implement expired approval rejection mapping.
- [ ] TCCP-0505 Implement self-approval policy mapping.
- [ ] TCCP-0506 Implement DLQ replay command.
- [ ] TCCP-0507 Implement replay audit and idempotency.

Exit checks:

- approval decisions preserve actor identity.
- replay never edits the original DLQ record.
- approval and replay tests pass.

## Milestone 6: Client Compatibility and Scenario Support

Tasks:

- [ ] TCCP-0601 Add compatibility tests for `tcctl`.
- [ ] TCCP-0602 Add compatibility tests for `admin`.
- [ ] TCCP-0603 Implement scenario run support APIs.
- [ ] TCCP-0604 Implement scenario verify support APIs.
- [ ] TCCP-0605 Add canonical scenario integration test harness.

Exit checks:

- `tcctl` and `admin` can use the same Control API contracts.
- canonical scenario can be verified through `tc-control`.

## Milestone 7: Hardening and Release Gate

Tasks:

- [ ] TCCP-0701 Add metrics for request, query, command, approval, retry/replay, and auth paths.
- [ ] TCCP-0702 Add trace-compatible context propagation.
- [ ] TCCP-0703 Add no-secret log and response tests.
- [ ] TCCP-0704 Add auth and authorization tests.
- [ ] TCCP-0705 Add graceful shutdown tests.
- [ ] TCCP-0706 Run API compatibility tests.
- [ ] TCCP-0707 Run full canonical scenario with `tc-server` and `tc-worker`.
- [ ] TCCP-0708 Verify docs links.

Exit checks:

- all unit and integration tests pass.
- canonical scenario passes through split control/data plane.
- documentation links from `tc-control/README.md` are current.

## Related Docs

- `tc-control/docs/implementation-contract.md`
- `tc-control/docs/definition-of-done.md`
- `tc-server/docs/implementation-contract.md`
- `tcctl/docs/implementation-contract.md`
- `admin/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`

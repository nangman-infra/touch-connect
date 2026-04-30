# tc-server Implementation Task List

## Purpose

This document turns the `tc-server` Definition of Done into an implementation order for the message routing data plane.

## Rules

- Work top-down unless a dependency forces a change.
- Do not let `tc-control` calls enter the message hot path.
- Do not let NATS, JetStream, or a database adapter define domain semantics.
- Every behavior change must add or update tests in the same milestone.

## Status Legend

```text
[ ] not started
[~] in progress
[x] done
```

## Milestone 0: Freeze Data-Plane Decisions

Tasks:

- [ ] TCS-0001 Choose Worker API and message ingress API style.
- [ ] TCS-0002 Choose MVP data-plane storage adapter.
- [ ] TCS-0003 Freeze NATS subject and JetStream naming scheme.
- [ ] TCS-0004 Freeze MVP latency budgets.
- [ ] TCS-0005 Freeze local development stack for `tc-server`, NATS/JetStream, and storage.
- [ ] TCS-0006 Freeze data-plane auth mode.
- [ ] TCS-0007 Freeze API compatibility policy with `tc-worker` and `tc-control`.

Exit checks:

- API, storage, transport, auth, performance, and compatibility decisions are documented.

## Milestone 1: Server Skeleton

Tasks:

- [ ] TCS-0101 Initialize the Go module if it does not exist.
- [ ] TCS-0102 Add `tc-server/cmd/tc-server/main.go`.
- [ ] TCS-0103 Add package layout for data-plane services, domain, ports, and adapters.
- [ ] TCS-0104 Add config loader with environment variable support.
- [ ] TCS-0105 Add config validation and redaction.
- [ ] TCS-0106 Add structured logging baseline.
- [ ] TCS-0107 Add health and readiness endpoints.
- [ ] TCS-0108 Add graceful shutdown.
- [ ] TCS-0109 Add build/test commands.

Suggested package boundaries:

```text
tc-server/cmd/tc-server
tc-server/internal/server
tc-server/internal/app/dataplane
tc-server/internal/domain
tc-server/internal/ports
tc-server/internal/adapters/storage
tc-server/internal/adapters/transport
tc-server/internal/adapters/api
tc-server/internal/config
tc-server/internal/observability
```

Exit checks:

- `tc-server` starts with valid config.
- invalid config fails fast.
- readiness reports dependency status.
- `go test ./...` passes.

## Milestone 2: Domain Model and Validation

Tasks:

- [ ] TCS-0201 Implement ids and refs for message, endpoint, attempt, checkpoint, artifact version, side effect execution, and correlation.
- [ ] TCS-0202 Implement canonical message envelope types.
- [ ] TCS-0203 Implement delivery classes.
- [ ] TCS-0204 Implement task revision and thread sequence guards.
- [ ] TCS-0205 Implement endpoint and capability declarations.
- [ ] TCS-0206 Implement attempt, claim, lease, and checkpoint domain types.
- [ ] TCS-0207 Implement side effect intent and execution types.
- [ ] TCS-0208 Implement stable domain error codes.
- [ ] TCS-0209 Implement validators for all data-plane records.

Exit checks:

- invalid state transitions are rejected.
- domain package imports no storage, NATS, HTTP, CLI, or admin packages.

## Milestone 3: Data-Plane Storage Ports

Tasks:

- [ ] TCS-0301 Define storage port interfaces.
- [ ] TCS-0302 Define transaction or unit-of-work boundary.
- [ ] TCS-0303 Define outbox or publish coordination port.
- [ ] TCS-0304 Define schema or record model for selected MVP adapter.
- [ ] TCS-0305 Implement endpoint and capability stores.
- [ ] TCS-0306 Implement message and delivery stores.
- [ ] TCS-0307 Implement attempt and checkpoint stores.
- [ ] TCS-0308 Implement artifact metadata store.
- [ ] TCS-0309 Implement side effect execution and idempotency stores.
- [ ] TCS-0310 Implement dead-letter store.
- [ ] TCS-0311 Add storage conformance tests.

Exit checks:

- atomic claim and atomic idempotency guard are proven by tests.
- durable write and publish intent cannot silently diverge.

## Milestone 4: Worker API and Ingress Skeletons

Tasks:

- [ ] TCS-0401 Define API version prefix or method namespace.
- [ ] TCS-0402 Define request and response envelope.
- [ ] TCS-0403 Define error response envelope.
- [ ] TCS-0404 Add auth and actor identity middleware.
- [ ] TCS-0405 Add endpoint identity middleware for Worker API.
- [ ] TCS-0406 Add Worker API route or method skeletons.
- [ ] TCS-0407 Add message ingress route or method skeleton.
- [ ] TCS-0408 Add schema validation for every request.
- [ ] TCS-0409 Add API tests for validation, auth, and error mapping.

Worker API skeleton must include:

- register endpoint
- refresh endpoint heartbeat
- advertise capabilities
- pull or receive routable message
- claim message processing
- refresh processing lease
- acknowledge message receipt
- submit readback
- submit checkpoint
- submit processing completion
- submit processing failure
- register artifact version metadata
- claim side effect execution
- report side effect execution result

Exit checks:

- every data-plane endpoint returns a stable response envelope.
- API tests pass without real NATS.

## Milestone 5: Transport Adapter and Bootstrap

Tasks:

- [ ] TCS-0501 Define transport port interfaces.
- [ ] TCS-0502 Implement subject builder and validator.
- [ ] TCS-0503 Implement transport-safe alias mapping.
- [ ] TCS-0504 Implement NATS connection lifecycle.
- [ ] TCS-0505 Implement JetStream stream bootstrap.
- [ ] TCS-0506 Implement durable consumer bootstrap.
- [ ] TCS-0507 Implement live signal publish.
- [ ] TCS-0508 Implement durable message publish.
- [ ] TCS-0509 Implement durable pull receive and broker ack/nack/terminate.
- [ ] TCS-0510 Implement DLQ event publish.
- [ ] TCS-0511 Add integration tests against local NATS/JetStream.

Exit checks:

- stream and consumer bootstrap are idempotent.
- redelivery after missing ack is verified.
- max redelivery produces a DLQ path.
- domain and application code import no NATS client types.

## Milestone 6: Message Routing and Delivery Records

Tasks:

- [ ] TCS-0601 Implement endpoint registration.
- [ ] TCS-0602 Implement capability advertisement.
- [ ] TCS-0603 Implement capability-first routing resolver.
- [ ] TCS-0604 Implement direct endpoint routing.
- [ ] TCS-0605 Implement message ingress flow.
- [ ] TCS-0606 Implement delivery record creation.
- [ ] TCS-0607 Implement transactional outbox or equivalent publish coordination.
- [ ] TCS-0608 Implement outbox publisher.
- [ ] TCS-0609 Implement accepted delivery projection for `tc-control` reads.

Message ingress flow:

```text
validate envelope
resolve routing target
write immutable message
write delivery record
write publish intent
publish through transport adapter
mark publish result
```

Exit checks:

- unsupported capability is rejected before publish.
- expired message cannot start new work.
- duplicate message id does not create a second logical message.
- send path has latency measurement.

## Milestone 7: Worker Processing, Claim, Lease, and Checkpoint

Tasks:

- [ ] TCS-0701 Implement worker heartbeat refresh.
- [ ] TCS-0702 Implement message claim.
- [ ] TCS-0703 Implement attempt creation.
- [ ] TCS-0704 Implement lease refresh.
- [ ] TCS-0705 Implement ack receipt record.
- [ ] TCS-0706 Implement readback submission.
- [ ] TCS-0707 Implement checkpoint submission.
- [ ] TCS-0708 Implement processing completion.
- [ ] TCS-0709 Implement processing failure.
- [ ] TCS-0710 Implement lease scanner.
- [ ] TCS-0711 Implement checkpoint stall scanner.
- [ ] TCS-0712 Implement takeover candidate creation.
- [ ] TCS-0713 Implement redelivery exhaustion to DLQ.

Exit checks:

- two workers racing for the same message produce one owner.
- stale attempt cannot checkpoint or complete.
- heartbeat does not replace lease ownership.
- takeover starts from latest valid checkpoint.

## Milestone 8: Protected Side Effect Execution Path

Tasks:

- [ ] TCS-0801 Implement side effect execution claim.
- [ ] TCS-0802 Implement side effect execution result reporting.
- [ ] TCS-0803 Implement duplicate side effect request handling.
- [ ] TCS-0804 Implement uncertain result handling.
- [ ] TCS-0805 Implement side effect execution state projection.

Side effect guard order:

```text
validate approved decision
validate approval_hash
validate approval expiry
validate lease ownership for attempt_ref
atomic insert-or-get execution record
return execution permission or existing state
```

Exit checks:

- protected side effect cannot run without approval.
- duplicate side effect request does not create a second external call.
- uncertain external result is not marked succeeded.

## Milestone 9: Recovery and Reconciliation

Tasks:

- [ ] TCS-0901 Implement DLQ creation.
- [ ] TCS-0902 Implement replay command intake from `tc-control`.
- [ ] TCS-0903 Implement retry command intake from `tc-control`.
- [ ] TCS-0904 Implement outbox replay after broker failure.
- [ ] TCS-0905 Implement startup reconciliation for unfinished publish intents.
- [ ] TCS-0906 Implement storage and broker readiness diagnostics.

Exit checks:

- replay is explicit and audited through accepted records.
- outbox recovery is idempotent.
- broker restart does not create false completion.

## Milestone 10: Hardening and Release Gate

Tasks:

- [ ] TCS-1001 Add data-plane authorization checks.
- [ ] TCS-1002 Add metrics for publish, routing, claim, redelivery, DLQ, checkpoint freshness, and side effect dedupe.
- [ ] TCS-1003 Add trace-compatible context propagation.
- [ ] TCS-1004 Add slow-path logs separating storage, broker, routing, and projection time.
- [ ] TCS-1005 Add benchmarks for latency budgets.
- [ ] TCS-1006 Add load tests for burst, reconnect, redelivery, and DLQ behavior.
- [ ] TCS-1007 Add graceful shutdown tests.
- [ ] TCS-1008 Verify canonical scenario with `tc-control` and `tc-worker`.

Exit checks:

- all unit tests pass.
- storage conformance tests pass.
- NATS/JetStream integration tests pass.
- claim and side effect race tests pass.
- canonical scenario passes through the split data/control plane.

## Related Docs

- `tc-server/docs/implementation-contract.md`
- `tc-server/docs/definition-of-done.md`
- `tc-control/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/delivery-semantics.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`

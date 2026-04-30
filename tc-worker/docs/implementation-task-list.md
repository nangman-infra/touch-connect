# tc-worker Implementation Task List

## Purpose

This document turns the `tc-worker` Definition of Done into an implementation order.

Use this as the development checklist for the MVP worker. The DoD says when the worker is acceptable; this task list says what to build first, what each step should produce, and which gates it closes.

## Rules

- Work top-down unless a dependency forces a change.
- Treat server-side accepted records from `tc-server` as truth for runtime state.
- Do not execute local work before claim acceptance.
- Do not run protected side effects before approval, execution ledger claim, and lease ownership are verified.
- Add or update tests in the same milestone as behavior changes.

## Status Legend

```text
[ ] not started
[~] in progress
[x] done
```

## Milestone 0: Freeze Worker Decisions

Goal: remove decisions that would otherwise be made inside worker code.

Tasks:

- [ ] TCW-0001 Choose worker server communication mode.
  - Output: Worker API client choice and direct transport usage decision.
- [ ] TCW-0002 Freeze server API compatibility policy.
  - Output: compatible server API version range and failure behavior.
- [ ] TCW-0003 Choose local execution policy shape.
  - Output: workspace root rule, command policy model, timeout model, stdout/stderr capture policy.
- [ ] TCW-0004 Choose artifact body storage mode.
  - Output: local path, server upload, object store, or adapter plan.
- [ ] TCW-0005 Freeze capability naming for MVP workers.
  - Output: stable capability names and execution hints.
- [ ] TCW-0006 Freeze checkpoint cadence.
  - Output: heartbeat, lease refresh, and checkpoint interval defaults.
- [ ] TCW-0007 Freeze side effect execution policy.
  - Output: protected operation kinds, external target shape, and uncertain result policy.

Exit checks:

- server communication, API compatibility, local execution, artifact storage, capability, checkpoint, and side effect decisions are documented.
- no runtime boundary is left to implicit code decisions.

Closes DoD areas:

- Startup and Registration
- Message Receive, Claim, and Lease
- Local Execution Boundary
- Artifact
- Protected Side Effect
- Config and Environment

## Milestone 1: Worker Skeleton and Config

Goal: create the runnable worker shell without message processing.

Tasks:

- [ ] TCW-0101 Add `tc-worker/cmd/tc-worker/main.go`.
- [ ] TCW-0102 Add worker package layout.
- [ ] TCW-0103 Add config loader with environment variable support.
- [ ] TCW-0104 Add config validation and redaction.
- [ ] TCW-0105 Add structured logging baseline.
- [ ] TCW-0106 Add signal handling and graceful shutdown.
- [ ] TCW-0107 Add build/test commands.

Suggested package boundaries:

```text
tc-worker/cmd/tc-worker
tc-worker/internal/worker
tc-worker/internal/app
tc-worker/internal/domain
tc-worker/internal/ports
tc-worker/internal/adapters/serverapi
tc-worker/internal/adapters/transport
tc-worker/internal/adapters/execution
tc-worker/internal/adapters/artifact
tc-worker/internal/config
tc-worker/internal/observability
```

Exit checks:

- worker starts with valid config.
- invalid config fails fast.
- secrets are redacted from diagnostics.
- shutdown path works while idle.
- `go test ./...` passes.

Closes DoD areas:

- Startup and Registration
- Config and Environment
- Observability and Release

## Milestone 2: Worker Domain and Validation

Goal: encode worker-side invariants before adapters exist.

Tasks:

- [ ] TCW-0201 Implement endpoint identity types.
- [ ] TCW-0202 Implement capability declaration types.
- [ ] TCW-0203 Implement message envelope validation.
- [ ] TCW-0204 Implement claim and lease state types.
- [ ] TCW-0205 Implement readback and checkpoint payload types.
- [ ] TCW-0206 Implement artifact metadata payload types.
- [ ] TCW-0207 Implement side effect intent and result types.
- [ ] TCW-0208 Implement worker error codes and retry classes.

Required invariants:

- processing starts only after claim acceptance.
- checkpoints reference `attempt_ref`.
- ack is not completion.
- unsupported capability is rejected before execution.
- protected side effects require approval, execution ledger claim, and lease ownership.

Exit checks:

- validation tests cover all worker-submitted records.
- invalid transitions are rejected.
- domain package imports no HTTP, NATS, storage, or shell packages.

Closes DoD areas:

- Message Receive, Claim, and Lease
- Readback and Checkpoint
- Protected Side Effect

## Milestone 3: Server API Client and Registration

Goal: connect the worker to `tc-server` without processing work yet.

Tasks:

- [ ] TCW-0301 Define Worker API client port.
- [ ] TCW-0302 Implement selected Worker API client adapter.
- [ ] TCW-0303 Implement server API compatibility check.
- [ ] TCW-0304 Implement endpoint registration.
- [ ] TCW-0305 Implement capability advertisement.
- [ ] TCW-0306 Implement heartbeat loop.
- [ ] TCW-0307 Implement server rejection handling.
- [ ] TCW-0308 Implement bounded retry with backoff.

Exit checks:

- registration succeeds against test server.
- incompatible server API version prevents processing.
- registration rejection prevents processing.
- heartbeat failure is logged and measured.
- capability changes can be re-advertised.

Closes DoD areas:

- Startup and Registration
- Server and Transport Integration
- Observability and Release

## Milestone 4: Message Receive and Claim Loop

Goal: receive or pull messages and claim them before execution.

Tasks:

- [ ] TCW-0401 Define receive source port.
- [ ] TCW-0402 Implement server-poll or transport-pull receive adapter.
- [ ] TCW-0403 Implement message envelope validation before ack.
- [ ] TCW-0404 Implement claim request.
- [ ] TCW-0405 Implement ack receipt submission after claim acceptance.
- [ ] TCW-0406 Implement lease refresh loop.
- [ ] TCW-0407 Implement max in-flight message limit.
- [ ] TCW-0408 Implement duplicate delivery handling.

Exit checks:

- receive without claim does not execute.
- expired message does not execute.
- duplicate delivery is safe.
- lease refresh failure moves work to lease-lost handling.

Closes DoD areas:

- Server and Transport Integration
- Message Receive, Claim, and Lease
- Performance and Backpressure

## Milestone 5: Readback, Checkpoint, Completion, and Failure

Goal: report worker understanding and progress through server-owned records.

Tasks:

- [ ] TCW-0501 Implement readback generation.
- [ ] TCW-0502 Implement readback submission.
- [ ] TCW-0503 Implement checkpoint builder.
- [ ] TCW-0504 Implement checkpoint submission.
- [ ] TCW-0505 Implement blocked missing fields checkpoint.
- [ ] TCW-0506 Implement processing completion submission.
- [ ] TCW-0507 Implement processing failure submission.
- [ ] TCW-0508 Implement lease-lost guard for all attempt-scoped writes.

Exit checks:

- readback-required messages emit readback before substantial work.
- checkpoint state validation works.
- completion after lease loss is rejected locally or by server.
- failed checkpoint requires failure reason code.

Closes DoD areas:

- Readback and Checkpoint
- Message Receive, Claim, and Lease
- Observability and Release

## Milestone 6: Local Execution Adapter

Goal: run local work inside a constrained execution boundary.

Tasks:

- [ ] TCW-0601 Define execution adapter port.
- [ ] TCW-0602 Implement workspace root validation.
- [ ] TCW-0603 Implement command allowlist, denylist, or policy adapter.
- [ ] TCW-0604 Implement command timeout.
- [ ] TCW-0605 Implement stdout/stderr capture policy.
- [ ] TCW-0606 Implement output size limits.
- [ ] TCW-0607 Implement redaction before checkpoint or message submission.
- [ ] TCW-0608 Implement cancellation after lease loss.

Exit checks:

- path traversal outside workspace root is rejected.
- disallowed command is rejected.
- timeout cancels command.
- captured output is bounded and redacted.
- lease loss cancels protected execution path.

Closes DoD areas:

- Local Execution Boundary
- Security
- Performance and Backpressure

## Milestone 7: Artifact Handling

Goal: create artifact refs that the server can trust.

Tasks:

- [ ] TCW-0701 Define artifact storage adapter port.
- [ ] TCW-0702 Implement artifact body write for selected storage mode.
- [ ] TCW-0703 Implement checksum generation.
- [ ] TCW-0704 Implement artifact metadata registration.
- [ ] TCW-0705 Implement artifact refs in checkpoints.
- [ ] TCW-0706 Implement lineage and provenance fields.
- [ ] TCW-0707 Implement explicit artifact final designation request.
- [ ] TCW-0708 Implement artifact failure handling.

Exit checks:

- large payload is not sent in message body.
- artifact version ref is immutable.
- final designation is not inferred from latest version.
- checksum is required.
- registration failure prevents fake completion.

Closes DoD areas:

- Artifact
- Readback and Checkpoint
- Observability and Release

## Milestone 8: Protected Side Effects

Goal: make external effects safe, idempotent, and auditable.

Tasks:

- [ ] TCW-0801 Implement approval decision lookup or server-provided approval validation.
- [ ] TCW-0802 Implement approval hash validation.
- [ ] TCW-0803 Implement approval expiry validation.
- [ ] TCW-0804 Implement side effect execution claim call.
- [ ] TCW-0805 Implement external operation adapter boundary.
- [ ] TCW-0806 Implement side effect result reporting.
- [ ] TCW-0807 Implement duplicate execution response handling.
- [ ] TCW-0808 Implement uncertain result reporting.
- [ ] TCW-0809 Implement lease-lost guard before external call.

Exit checks:

- no external protected call starts before server execution claim.
- duplicate request returns existing execution state.
- approval hash mismatch blocks execution.
- uncertain result is not reported as success.

Closes DoD areas:

- Protected Side Effect
- Security
- Observability and Release

## Milestone 9: Reconnect, Crash, and Recovery Behavior

Goal: survive dependency interruption without stale ownership assumptions.

Tasks:

- [ ] TCW-0901 Implement server reconnect behavior.
- [ ] TCW-0902 Implement broker reconnect behavior when direct broker use is enabled.
- [ ] TCW-0903 Re-register endpoint after reconnect.
- [ ] TCW-0904 Re-advertise capabilities after reconnect.
- [ ] TCW-0905 Implement recoverable attempt lookup only through server.
- [ ] TCW-0906 Implement local partial work cleanup policy.
- [ ] TCW-0907 Implement duplicate delivery after reconnect handling.

Exit checks:

- reconnect does not assume old lease ownership.
- protected side effects do not resume without execution ledger state.
- duplicate delivery remains safe.
- local partial work is handled by policy.

Closes DoD areas:

- Reconnect and Crash
- Message Receive, Claim, and Lease
- Protected Side Effect

## Milestone 10: Hardening and Canonical Scenario

Goal: make the worker operable in the MVP flow.

Tasks:

- [ ] TCW-1001 Add metrics for registration, heartbeat, claim, lease refresh, checkpoint, local execution, artifact, and side effect paths.
- [ ] TCW-1002 Add trace-compatible context propagation.
- [ ] TCW-1003 Add benchmarks for validation, claim, checkpoint, artifact registration, and local command startup overhead.
- [ ] TCW-1004 Add load tests for burst delivery and repeated checkpoints.
- [ ] TCW-1005 Add security tests for workspace, command policy, redaction, and side effect guard.
- [ ] TCW-1006 Add integration test against `tc-server`.
- [ ] TCW-1007 Verify canonical scenario without fake checkpoints.

Exit checks:

- all unit tests pass.
- server client tests pass.
- transport integration tests pass when direct transport is enabled.
- local execution policy tests pass.
- reconnect and lease-loss tests pass.
- canonical scenario can use the worker.

Closes DoD areas:

- Performance and Backpressure
- Security
- Observability and Release

## DoD Coverage Map

```text
Startup and Registration:              M0, M1, M3
Server and Transport Integration:      M3, M4
Message Receive, Claim, and Lease:     M2, M4, M5, M9
Readback and Checkpoint:               M2, M5, M7
Local Execution Boundary:              M0, M6
Artifact:                              M0, M7
Protected Side Effect:                 M0, M2, M8, M9
Reconnect and Crash:                   M9
Config and Environment:                M0, M1
Performance and Backpressure:          M4, M6, M10
Security:                              M6, M8, M10
Observability and Release:             M1, M3, M5, M7, M8, M10
```

## Not In This List

These belong to other app task lists:

- server source-of-truth ledgers.
- global routing policy.
- approval decision authority.
- operator command UX.
- admin UI.
- full workflow planning.

## Related Docs

- `tc-worker/docs/implementation-contract.md`
- `tc-worker/docs/definition-of-done.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`
- `docs/active/contracts/delivery-semantics.md`

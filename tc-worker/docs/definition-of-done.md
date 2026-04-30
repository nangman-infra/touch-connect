# tc-worker Definition of Done

## Purpose

This document defines what must be true before `tc-worker` can be considered implementation-complete for the MVP execution endpoint.

The checklist combines backend worker best practices with the touch-connect product contract:

- `worker = execution`.
- server-side accepted records are truth.
- delivery is not processing.
- ack is not completion.
- capability advertisement is public; skill selection is local.
- checkpoints are emitted by the worker, not inferred by the server.
- protected side effects require approval, execution ledger claim, and current lease ownership.

## Completion Gate

`tc-worker` is not done until all items below are true:

- startup, endpoint registration, capability advertisement, heartbeat, and shutdown are implemented and tested.
- message receive, claim, lease refresh, ack, readback, checkpoint, completion, and failure paths are implemented.
- local execution is constrained by workspace, command policy, timeout, and output capture rules.
- artifact content and metadata handling preserve immutable version refs.
- protected side effects cannot run without approval, execution ledger claim, and lease ownership.
- reconnect and crash behavior never assumes stale ownership.
- config is environment-driven, validated at startup, and redacted in logs.
- performance, security, observability, and release checks are repeatable.

## 1. Startup and Registration DoD

Done means:

- config is loaded before any server or broker connection is attempted.
- required config is validated and unsafe production defaults are rejected.
- server API compatibility is verified before endpoint registration.
- endpoint identity is registered with `tc-server`.
- registration includes endpoint ref, display name, actor id, workspace id, capabilities, execution hints, worker version, and started time.
- registration excludes secrets, internal prompts, local path inventories, and credential material.
- capability advertisement happens after endpoint registration and before message processing.
- heartbeat starts only after successful registration.
- message processing never starts when registration fails.
- graceful shutdown stops receiving new work before local execution is interrupted.

Tests must cover:

- missing config.
- invalid workspace root.
- server API compatibility rejection.
- registration rejection.
- capability advertisement rejection.
- heartbeat start and stop.
- shutdown during idle state.

## 2. Server and Transport Integration DoD

Done means:

- Worker API client is isolated behind an application port.
- NATS or JetStream client code is isolated behind a transport adapter when direct transport is used.
- worker code does not write storage directly.
- worker code does not mutate server-owned state locally.
- server-accepted state is treated as truth.
- transient server or broker failures use bounded retry with backoff.
- permanent contract rejection stops the current operation and emits a clear error.

Required Worker API calls:

- register endpoint
- refresh endpoint heartbeat
- advertise capabilities
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

Tests must cover:

- unavailable server.
- unavailable broker when broker is required.
- server rejection.
- retryable failure with backoff.
- no duplicate state mutation after retry.

## 3. Message Receive, Claim, and Lease DoD

Done means:

- processing starts only after server claim acceptance.
- claim unit is `message`.
- claim response binds the worker to an `attempt_ref`.
- ack means envelope received, parsed, and accepted for processing.
- ack is never treated as completion.
- expired messages do not start new work.
- unsupported capabilities are rejected before local execution.
- lease refresh is periodic and tied to the accepted `attempt_ref`.
- lease loss prevents checkpoint, completion, and protected side effect execution.
- worker heartbeat does not imply processing lease ownership.

Tests must cover:

- message received but claim rejected.
- message expired before claim.
- unsupported capability.
- ack before completion.
- lease refresh failure.
- lease lost during local execution.
- duplicate delivery of the same message.

## 4. Readback and Checkpoint DoD

Done means:

- `readback_required=true` always produces readback before substantial work.
- readback includes goal, constraints, and next action.
- checkpoints are emitted by the worker directly.
- checkpoints reference `attempt_ref`.
- checkpoint states are validated before submission.
- checkpoint summaries are operational and short.
- `blocked_missing_fields` includes missing fields and reasons.
- `failed` includes failure reason code.
- checkpoint artifact progress uses artifact refs, not large inline payloads.
- completion is reported separately from ack.

Minimum checkpoint states:

```text
claimed
validating
blocked_missing_fields
in_progress
retrying
completed
failed
```

Tests must cover:

- readback required path.
- missing fields checkpoint.
- invalid checkpoint state.
- checkpoint after lease loss.
- completion after lease loss.
- failed checkpoint without reason code.

## 5. Local Execution Boundary DoD

Done means:

- all local execution is scoped to configured workspace root.
- path traversal outside workspace root is rejected.
- executable tools are controlled by allowlist, denylist, or policy adapter.
- every local command has a timeout.
- stdout and stderr capture policy is explicit.
- command output is size-limited.
- sensitive output is redacted before checkpoint or message submission.
- environment variables passed to child processes are explicit.
- worker does not expose credential material in logs, messages, checkpoints, or artifacts.
- skill selection remains local and is not registered as routing data.

Tests must cover:

- command outside workspace root.
- disallowed executable.
- command timeout.
- oversized output.
- redaction.
- local execution cancellation after lease loss.

## 6. Artifact DoD

Done means:

- artifact body storage mode is explicit.
- large payloads do not move through message bodies.
- artifact metadata is registered with the server.
- artifact refs point to exact immutable versions.
- checksum is provided for stored content.
- lineage and provenance are recorded when derived from earlier artifacts.
- final designation is requested explicitly and accepted by the server.
- latest artifact pointer is never used as a domain contract.
- artifact registration failure is visible and does not fake completion.

Tests must cover:

- artifact version metadata payload.
- checksum requirement.
- immutable version ref.
- final designation request.
- oversized inline payload rejection.
- artifact registration failure.

## 7. Protected Side Effect DoD

Done means protected side effects cannot execute unless:

- approval decision is approved.
- `approval_hash` matches current intent.
- approval is not expired.
- worker has current lease ownership.
- worker has claimed or received the side effect execution record.
- `idempotency_key` and `protected_scope` are present.

Execution rules:

- external call starts only after server accepts execution claim.
- duplicate request returns existing execution state.
- uncertain external result is reported as uncertain or failed, not succeeded.
- completion is reported to server after external call.
- side effect result does not mutate original message or approval request.

Tests must cover:

- approval missing.
- approval hash mismatch.
- expired approval.
- lease lost before external call.
- duplicate side effect request.
- external result unknown.
- server unavailable after external call.

## 8. Reconnect and Crash DoD

Done means:

- reconnect refreshes endpoint heartbeat.
- reconnect re-advertises capabilities.
- worker does not assume it still owns prior leases.
- recoverable attempt lookup is server-driven.
- protected side effects are not resumed without execution ledger state.
- local partial work is either discarded, checkpointed, or converted to artifact according to policy.
- duplicate delivery after reconnect is safe.

Tests must cover:

- reconnect after server restart.
- reconnect after broker restart.
- local process interrupted during work.
- worker restart with previous local state.
- duplicate delivery after reconnect.

## 9. Config and Environment DoD

Done means:

- deployment config is separated from code.
- environment variables are supported.
- optional config file support is explicit.
- command flags may override config for local development.
- config precedence is documented.
- durations, byte sizes, and capability lists use typed parsing.
- secret values are redacted from logs and diagnostics.

Required environment variable namespace:

```text
TC_WORKER_SERVER_URL
TC_WORKER_NATS_URL
TC_WORKER_ENDPOINT_DISPLAY_NAME
TC_WORKER_ACTOR_ID
TC_WORKER_WORKSPACE_ID
TC_WORKER_WORKSPACE_ROOT
TC_WORKER_CAPABILITIES
TC_WORKER_CREDENTIAL_REF
TC_WORKER_AUTH_TOKEN
TC_WORKER_HEARTBEAT_INTERVAL
TC_WORKER_CHECKPOINT_INTERVAL
TC_WORKER_LEASE_REFRESH_INTERVAL
TC_WORKER_LOCAL_COMMAND_TIMEOUT
TC_WORKER_ARTIFACT_STORAGE_MODE
TC_WORKER_MAX_MESSAGE_PAYLOAD_BYTES
TC_WORKER_LOG_LEVEL
TC_WORKER_OTEL_EXPORTER_OTLP_ENDPOINT
```

`TC_WORKER_AUTH_TOKEN` is credential material and must be redacted from logs. When production auth uses an external credential store, `TC_WORKER_CREDENTIAL_REF` should be preferred.

Tests must cover:

- missing required values.
- invalid durations.
- invalid byte sizes.
- unsafe production defaults.
- secret redaction.
- precedence order.

## 10. Performance and Backpressure DoD

The project is speed-sensitive.
Done means worker behavior is measurable and bounded.

Required benchmarks:

- message envelope validation latency.
- claim request latency.
- readback submission latency.
- checkpoint submission latency.
- artifact metadata registration latency.
- side effect claim latency.
- local command startup overhead.

Required load tests:

- burst of durable handoff messages.
- repeated checkpoint submission.
- server reconnect storm.
- broker reconnect storm when broker is used directly.
- duplicate delivery under retry.

Done means:

- local concurrency limit is configurable.
- max in-flight messages is bounded.
- lease refresh has priority over new work.
- checkpoints are not delayed behind long artifact operations.
- large artifacts are moved outside message bodies.

## 11. Security DoD

Done means:

- endpoint identity cannot impersonate another endpoint.
- actor identity is preserved for worker-submitted records.
- workspace boundary is enforced for file and process access.
- child process environment is minimized.
- logs never include secrets, credentials, full prompts, raw protected payloads, or raw artifact bodies.
- approval and lease checks happen before protected side effects.
- command policy is explicit and testable.
- network endpoints and TLS or trusted network assumptions are documented.

## 12. Observability and Release DoD

Done means:

- structured logs include `endpoint_ref`, `message_id`, `task_id`, `attempt_ref`, and `correlation_id` when available.
- metrics include registration failures, heartbeat failures, claim latency, lease refresh failures, checkpoint latency, command duration, artifact registration failures, and side effect outcomes.
- traces or trace-compatible context are propagated through API and transport calls.
- all unit tests pass.
- server client tests pass.
- server API compatibility tests pass.
- transport integration tests pass when direct transport is enabled.
- local execution policy tests pass.
- reconnect and lease-loss tests pass.
- canonical scenario can use the worker without fake checkpoints.

## Reference Baselines

- NATS JetStream consumer ack and redelivery behavior: https://docs.nats.io/nats-concepts/jetstream/consumers
- Twelve-Factor App config principle: https://www.12factor.net/config
- OWASP API Security project: https://owasp.org/API-Security/
- OpenTelemetry log correlation and context propagation: https://opentelemetry.io/docs/specs/otel/logs/

## Related Contracts

- `tc-worker/docs/implementation-contract.md`
- `tc-worker/docs/implementation-task-list.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`
- `docs/active/contracts/delivery-semantics.md`

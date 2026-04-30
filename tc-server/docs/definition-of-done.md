# tc-server Definition of Done

## Purpose

This document defines what must be true before `tc-server` can be considered implementation-complete for the MVP message routing data plane.

The checklist combines messaging backend best practices with the touch-connect product contract:

- `tc-server` is the data plane.
- `tc-control` is the control plane.
- NATS and JetStream are transport adapters, not the domain model.
- delivery is not processing.
- ack is not completion.
- routing is capability-first.
- claim unit is message.
- protected side effects are guarded by approval and an execution ledger.

## Completion Gate

`tc-server` is not done until all items below are true:

- Worker API and message ingress schemas are explicit, versioned, and tested.
- NATS subjects, JetStream streams, and durable consumers are named by documented rules.
- data-plane storage ports and minimum records are defined without binding the domain to one database.
- claim, lease, takeover, and DLQ paths work under concurrency.
- protected side effects are idempotent through the execution ledger.
- config is environment-driven, validated at startup, and safe to log in redacted form.
- performance budgets are documented and covered by repeatable benchmarks.
- control-plane queries cannot block hot-path routing or worker processing.

## 1. Worker API and Message Ingress DoD

Done means:

- API style is selected and documented before implementation starts.
- APIs are versioned from the first implementation.
- every request and response has a stable schema.
- every write operation returns an accepted record id or a structured domain error.
- domain error codes are stable and machine-readable.
- request cancellation and timeout behavior are defined.
- API handlers contain no direct NATS or database-specific logic.

Minimum Worker API operations:

- get server health
- get server readiness
- get server version
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

Minimum message ingress operations:

- submit message envelope
- validate delivery class
- resolve routing target
- create delivery record
- publish durable or live signal
- return accepted message and delivery refs

Tests must cover:

- schema validation for every API operation
- authn/authz failure
- duplicate submission
- expired message
- lease lost during checkpoint or completion
- unavailable broker
- unavailable storage

## 2. NATS Subject and JetStream DoD

Done means subject and JetStream names follow documented rules:

- NATS subjects are ASCII, lowercase, dot-separated, and semantic.
- subject tokens use only alphanumeric characters, dash, and underscore.
- subjects do not contain secrets, prompts, local paths, or human message text.
- raw user input is never used directly as a subject token.
- workspace, endpoint, and capability values use stable transport-safe aliases in subjects.
- publishers never publish to wildcard subjects.
- JetStream stream and durable consumer names avoid dots, wildcards, spaces, and path separators.

Required subject families:

```text
tc.<workspace>.live.endpoint.<endpoint_ref>
tc.<workspace>.msg.<delivery_class>.<target_capability>
tc.<workspace>.msg.direct.<endpoint_ref>
tc.<workspace>.state.<record_kind>
tc.<workspace>.sidefx.<event_kind>
tc.<workspace>.dlq.<reason>
```

Durable delivery paths must use JetStream with:

- explicit ack policy for durable processing
- `AckWait` mapped to server `ack_timeout`
- `MaxDeliver` mapped to server `max_redelivery`
- `MaxAckPending` configured for backpressure
- pull consumers preferred for worker processing loops
- max-deliver advisories or equivalent handling connected to DLQ creation

Core NATS may be used only for live signals or non-critical informational delivery.
Protected side effects must never depend on Core NATS-only delivery.

## 3. Data-Plane Storage DoD

Done means:

- domain and application code depend on storage ports.
- storage adapters implement those ports.
- no domain type imports database driver types.
- every adapter has conformance tests.
- migration or schema initialization is repeatable.
- storage failures are surfaced as domain-visible failures where relevant.

Minimum stores:

- endpoint registry
- capability registry
- message ledger
- delivery record store
- attempt store
- checkpoint store
- artifact metadata store
- side effect execution store
- idempotency store
- dead-letter store
- outbox or publish coordination store

Minimum constraints:

```text
message_id unique
endpoint_ref unique within workspace
artifact_version_id unique
attempt_ref unique
side_effect_execution_id unique
idempotency_key + protected_scope unique
task_id + task_revision unique
thread_id + thread_sequence unique
```

Transaction rules:

- ledger write and publish intent cannot silently diverge.
- a transactional outbox or equivalent publish coordination pattern is used for durable events.
- duplicate publish after retry is safe through message ids and idempotency keys.
- projection updates cannot rollback on late arrival.
- server time is used for authoritative timestamps.

## 4. Claim, Lease, and Takeover DoD

Done means the processing state machine is implemented and tested.

Minimum state flow:

```text
available -> claimed -> in_progress -> completed
available -> claimed -> in_progress -> failed
available -> claimed -> expired -> takeover_candidate -> claimed
available -> claimed -> redelivery_exhausted -> dlq
```

Rules:

- claim unit is `message`.
- claim is atomic compare-and-set.
- exactly one live attempt owns a message at a time.
- successful claim creates or binds `attempt_ref`.
- retry or reassignment creates a new `attempt_ref`.
- lease refresh requires the current `attempt_ref`.
- stale attempts cannot checkpoint, complete, or start protected side effects.
- takeover starts from latest valid checkpoint.
- raw history replay alone is not a valid recovery path.

Tests must cover:

- two workers racing for the same message
- heartbeat present but lease expired
- checkpoint after lease loss
- completion after lease loss
- takeover from latest checkpoint
- redelivery exhaustion to DLQ

## 5. Idempotency and Side Effect Execution Ledger DoD

Done means protected side effects cannot execute without:

- approved approval decision
- matching `approval_hash`
- non-expired approval
- `idempotency_key`
- `protected_scope`
- side effect execution record
- current processing lease

Execution ledger rules:

- `idempotency_key + protected_scope` is the uniqueness boundary.
- insert-or-get for the uniqueness boundary is atomic.
- duplicate requests return existing execution state or a linked `deduped` record.
- uncertain external result is never marked `succeeded`.
- retries are explicit and auditable.

## 6. Config and Environment DoD

Done means configuration follows deployable service practice:

- deployment config is separated from code.
- environment variables are supported.
- optional config file support is explicit.
- all required config is validated at startup.
- invalid config fails fast.
- durations and sizes use typed parsing.
- production mode rejects unsafe defaults.
- secret values are redacted from logs and diagnostics.

Required environment variable namespace:

```text
TC_SERVER_BIND_ADDR
TC_SERVER_NATS_URL
TC_SERVER_JETSTREAM_ENABLED
TC_SERVER_STORAGE_DRIVER
TC_SERVER_STORAGE_DSN
TC_SERVER_ARTIFACT_STORAGE_DRIVER
TC_SERVER_ARTIFACT_STORAGE_ROOT
TC_SERVER_ACK_TIMEOUT
TC_SERVER_MESSAGE_EXPIRY
TC_SERVER_MAX_REDELIVERY
TC_SERVER_LEASE_TIMEOUT
TC_SERVER_CHECKPOINT_STALL_AFTER
TC_SERVER_MAX_MESSAGE_PAYLOAD_BYTES
TC_SERVER_AUTH_MODE
TC_SERVER_LOG_LEVEL
TC_SERVER_OTEL_EXPORTER_OTLP_ENDPOINT
```

## 7. Performance and Backpressure DoD

The project is speed-sensitive.
Done means the server has measurable performance gates instead of vague claims.

Required benchmarks:

- message validation latency
- routing resolution latency
- durable publish latency
- claim latency under concurrent workers
- checkpoint write latency
- side effect idempotency guard latency

Required load tests:

- burst of informational messages
- burst of durable handoff messages
- worker reconnect storm
- redelivery storm
- DLQ creation under repeated failure

Done means:

- MVP latency budgets are documented before release.
- slow-path logs identify storage, broker, routing, and projection time separately.
- backpressure uses `MaxAckPending`, queue depth, or server-side admission checks.
- payload size limits prevent large artifacts from moving through message bodies.

## 8. Security DoD

Done means:

- every data-plane write path authenticates the caller.
- worker identity cannot impersonate another endpoint.
- actor identity is preserved for worker-submitted records.
- logs never include secrets, credential material, full prompts, or raw artifact bodies.
- TLS or trusted network assumptions are documented for every deployment mode.

## 9. Observability and Operations DoD

Done means:

- health endpoint reports process liveness.
- readiness endpoint reports storage and broker readiness.
- graceful shutdown stops accepting new work before closing broker and storage clients.
- structured logs include `message_id`, `task_id`, `attempt_ref`, `endpoint_ref`, and `correlation_id` when available.
- metrics include routing latency, publish latency, ack timeout count, redelivery count, DLQ count, claim latency, checkpoint freshness, and side effect dedupe count.
- traces or trace-compatible context are propagated across API, storage, and transport boundaries.

## 10. Release Readiness DoD

Done means:

- all unit tests pass.
- adapter conformance tests pass.
- API compatibility tests pass.
- NATS/JetStream integration tests pass.
- storage integration tests pass for the selected MVP adapter.
- race/concurrency tests pass for claim and side effect guards.
- canonical scenario can complete through `tc-control`, `tc-server`, and `tc-worker`.
- documentation links from `tc-server/README.md` are current.
- no unresolved placeholder is allowed in source code for contract-critical behavior.

## Reference Baselines

- NATS subject naming and wildcard rules: https://docs.nats.io/nats-concepts/subjects
- NATS JetStream consumer ack and redelivery behavior: https://docs.nats.io/nats-concepts/jetstream/consumers
- NATS JetStream stream, consumer, and account naming: https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/naming
- Twelve-Factor App config principle: https://www.12factor.net/config
- OWASP API Security project: https://owasp.org/API-Security/
- OpenTelemetry log correlation and context propagation: https://opentelemetry.io/docs/specs/otel/logs/

## Related Contracts

- `tc-server/docs/implementation-contract.md`
- `tc-server/docs/implementation-task-list.md`
- `tc-control/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/delivery-semantics.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`

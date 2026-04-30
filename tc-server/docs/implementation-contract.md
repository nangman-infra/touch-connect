# tc-server Implementation Contract

## Purpose

`tc-server` is the touch-connect message routing and delivery data plane.

It is latency-sensitive. It owns the fast path for AI endpoint communication:

- message ingress
- capability-based routing
- delivery class mapping
- NATS/Core and JetStream integration
- ack, readback, redelivery, and DLQ handoff
- worker registration, claim, lease, checkpoint, completion, and failure submission

`tc-server` does not expose the operator/admin Control API. `tc-control` owns that plane.

## Runtime Role

`tc-server` is a long-running backend process.

It owns:

- endpoint registration write path
- capability advertisement write path
- message routing and delivery write path
- delivery record write path
- attempt and checkpoint write path
- worker-facing API
- transport adapter bootstrap
- data-plane metrics and backpressure

It does not own:

- admin web UI
- `tcctl` command handling
- operator query API
- approval decision UI/API
- retry/replay/cancel operator API
- local workspace execution
- endpoint-internal skill selection
- workflow planning or task decomposition

## Plane Boundary

The data plane and control plane are separate.

- `tc-server`
  - handles worker and message hot paths
  - optimizes latency and backpressure
  - validates data-plane contracts before accepting records
- `tc-control`
  - handles operator/admin API
  - handles approval decisions, retry, replay, cancel, and inspection
  - may issue accepted control commands that `tc-server` observes or executes

`tc-server` must not call `tc-control` on the message hot path.

## Worker API

The worker-facing API is used by `tc-worker`.

Minimum operations:

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
- register artifact version metadata
- submit processing failure
- claim side effect execution
- report side effect execution result

Worker APIs must authenticate actor identity and endpoint identity before accepting state changes.

## Message Ingress API

Message ingress may be called by trusted producers, `tc-control`, or internal automation.

Minimum operations:

- submit message envelope
- validate delivery class
- resolve routing target
- create delivery record
- publish durable or live signal
- return accepted message and delivery refs

Message ingress is not an operator Control API. It accepts communication events and returns data-plane acceptance.

## Transport Adapter

The transport adapter hides NATS/Core and JetStream from domain/application code.

Required ports:

- publish live signal
- publish durable message
- create durable consumer
- pull durable messages
- ack durable message
- nack or terminate durable message
- publish DLQ event

Domain/application code must not import NATS client types.

## Storage Adapter

Storage is a replaceable layer.

Required data-plane ports:

- endpoint registry store
- capability registry store
- message ledger store
- delivery record store
- attempt store
- checkpoint store
- artifact metadata store
- side effect execution store
- idempotency store
- dead-letter store
- outbox or publish coordination store

Storage implementation may be SQLite, MongoDB, Postgres, JetStream KV, or another adapter.
Domain logic must depend on ports, not a specific database.

## Communication Contract

`tc-server` implements the AI communication layer data plane.

Required invariants:

- `worker = execution`
- `ack != completion`
- `delivery != processing`
- routing key is `target_capability`
- claim unit is `message`
- checkpoint first, timeout second
- side effect exactly-once is verified by execution ledger
- control-plane queries must not block hot-path routing

## Routing Contract

Routing is capability-first.

Required behavior:

- validate canonical message envelope
- resolve eligible endpoints by `target_capability`
- publish to the correct transport path by delivery class
- support direct routing when an endpoint is explicitly targeted
- support broadcast only for delivery classes that permit it
- preserve correlation ids without interpreting their business meaning

`tc-server` must not inspect endpoint-internal skill lists or prompts.

## Delivery Class Mapping

Default mapping:

- `informational`
  - Core NATS or lightweight durable path when configured
- `handoff`
  - JetStream durable path
- `approval_request`
  - JetStream durable path
- `state_update`
  - JetStream durable path

Protected side effects must not execute from Core NATS-only delivery.

## Ledger Contract

Data-plane ledger writes must be append-oriented where possible.

Rules:

- messages are immutable
- delivery records are append-oriented
- attempts are append-only
- checkpoints are append-only
- artifact versions are immutable
- side effect execution records are the only source for protected execution outcome
- task state is a projection derived from accepted records

Task projection must use `task_revision` as its monotonic update boundary.

## Idempotency Contract

`tc-server` must enforce idempotency for protected side effects on the execution path.

Rules:

- `idempotency_key + protected_scope` is the uniqueness boundary
- duplicate side effect requests must not create new external calls
- material change that changes `approval_hash` requires a new idempotency key
- uncertain side effect results must not be treated as success

The storage adapter must expose atomic insert-or-get behavior for idempotency guards.

## Failure Handling

Required server behavior:

- expired messages must not start new work
- lower `task_revision` updates must not rollback current task state
- missing required message fields must be rejected or marked blocked by contract
- repeated redelivery beyond policy must create a DLQ record
- orphaned claims must become takeover candidates
- broker failure must not be hidden as successful delivery

## Configuration

Minimum configuration keys:

```text
tc_server_bind_addr
nats_url
jetstream_enabled
storage_driver
storage_dsn
artifact_storage_driver
artifact_storage_root
ack_timeout
message_expiry
max_redelivery
lease_timeout
checkpoint_stall_after
max_message_payload_bytes
auth_mode
log_level
otel_exporter_otlp_endpoint
```

Environment variables should use the `TC_SERVER_` prefix defined in `tc-server/docs/definition-of-done.md`.

## Observability

`tc-server` must emit metrics or structured logs for:

- message publish latency
- routing resolution latency
- ack timeout count
- redelivery count
- DLQ count
- endpoint online/offline transitions
- claim latency
- checkpoint freshness
- side effect dedupe count
- side effect execution outcome

Every log related to message processing should include `message_id`, `task_id`, `attempt_ref`, `endpoint_ref`, and `correlation_id` when available.

## Test Contract

Minimum tests:

- message envelope validation
- capability routing
- delivery class mapping
- duplicate message dedupe
- side effect idempotency uniqueness
- checkpoint append and latest checkpoint projection
- DLQ creation after max redelivery
- worker heartbeat expiry and takeover eligibility
- storage adapter conformance for data-plane stores
- outbox publish recovery
- server API compatibility checks

## Related Contracts

- `tc-server/docs/definition-of-done.md`
- `tc-server/docs/implementation-task-list.md`
- `tc-control/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/delivery-semantics.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`

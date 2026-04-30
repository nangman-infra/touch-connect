# tc-control Implementation Contract

## Purpose

`tc-control` is the control plane backend for touch-connect.

It exposes the operator/admin API used by `tcctl` and `admin`.
It is optimized for correctness, auditability, and stable query/mutation contracts, not message hot-path latency.

## Runtime Role

`tc-control` is a long-running backend process.

It owns:

- Control API versioning
- operator/admin authentication and authorization
- task, message, endpoint, artifact, approval, and DLQ query APIs
- task creation and cancellation requests
- message send requests from human or admin surfaces
- approval approve/reject decisions
- retry, replay, and artifact finalization requests
- audit records for every protected mutation
- read-model/projection access for operator workflows

It does not own:

- data-plane routing loops
- worker registration or heartbeat hot paths
- claim, lease, checkpoint, completion, or failure hot paths
- direct worker execution
- NATS or JetStream as a domain dependency
- admin web rendering
- local workspace access

## Plane Boundary

`tc-control` talks to `tc-server` only through explicit ports:

- read/projection ports for server-accepted state
- control command ports for retry, replay, cancel, and message ingress requests
- health/version compatibility checks

`tc-server` must not call `tc-control` on the message hot path.
If `tc-control` is unavailable, already accepted data-plane routing and worker processing must continue.

## Control API

Minimum API groups:

```text
health
version
endpoint
task
message
artifact
approval
dlq
scenario
```

Minimum operations:

- get service health, readiness, and version
- list and inspect endpoints
- list endpoint capabilities
- create, inspect, retry, and cancel tasks
- inspect task history
- send, inspect, and query messages
- list, inspect, and finalize artifact versions
- list, inspect, approve, and reject approval requests
- list, inspect, and replay DLQ records
- run and verify canonical scenario support operations

Every mutation must return server-accepted state or a structured domain rejection.

## Query and Projection Contract

Control-plane reads may use projections, but projections must be traceable to accepted records.

Rules:

- projections are not the source event.
- stale projections must expose freshness metadata when relevant.
- pagination and sorting are stable.
- read APIs must never expose secrets, full prompts, raw artifact bodies, or credential material.
- ids and refs are returned exactly as accepted by server records.

## Mutation Contract

Control-plane mutations are command requests, not direct storage rewrites.

Rules:

- command request validation happens before sending to `tc-server`.
- accepted commands are audited with actor identity, workspace, request id, and target refs.
- retry, replay, cancel, approve, reject, and finalize operations are explicit.
- protected operations require actor identity.
- command idempotency keys are required for protected or retryable mutations.
- local success must never be reported before server acceptance.

## Approval Contract

Approval decisions preserve human accountability.

Rules:

- approve and reject require actor identity.
- self-approval policy is explicit.
- approval decision includes the current `approval_hash`.
- expired approval requests cannot be approved.
- approval decision does not mutate the original message.

## DLQ and Recovery Contract

DLQ replay and task retry are operator commands.

Rules:

- replay creates a new accepted replay request or delivery attempt according to server policy.
- replay does not edit the original DLQ record.
- retry and cancel use monotonic task revision guards.
- commands are audited before they reach data-plane execution.

## Configuration

Minimum configuration keys:

```text
bind_addr
server_url
storage_driver
storage_dsn
read_model_driver
read_model_dsn
auth_mode
admin_origin_allowlist
request_timeout
log_level
otel_exporter_otlp_endpoint
```

Environment variables must use the `TC_CONTROL_` prefix.

## Observability

`tc-control` must emit structured logs or metrics for:

- API request latency
- server command latency
- query latency
- command rejection count
- approval decision count
- retry/replay/cancel count
- DLQ replay count
- authorization rejection count

Logs for commands should include `request_id`, `actor_id`, `workspace_id`, `task_id`, `message_id`, `approval_id`, `dead_letter_id`, and `correlation_id` when available.

## Test Contract

Minimum tests:

- Control API schema validation
- API version compatibility with `tcctl` and `admin`
- server command mapping
- query pagination and sorting
- projection freshness handling
- approval approve/reject behavior
- retry, cancel, and DLQ replay command behavior
- artifact finalization behavior
- authn/authz rejection
- unavailable `tc-server` behavior
- audit record creation for mutations

## Related Contracts

- `tc-control/docs/definition-of-done.md`
- `tc-control/docs/implementation-task-list.md`
- `tc-server/docs/implementation-contract.md`
- `tcctl/docs/implementation-contract.md`
- `admin/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/delivery-semantics.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`

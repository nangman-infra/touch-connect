# tc-worker Implementation Contract

## Purpose

`tc-worker` is the touch-connect execution endpoint runtime.

It registers capabilities, receives or claims messages, performs local work, and reports progress through readback, checkpoints, artifact metadata, completion, or failure updates.

## Runtime Role

`tc-worker` is a long-running process.

It owns:

- endpoint registration lifecycle
- capability advertisement
- local workspace access
- local shell/process execution
- endpoint-internal skill selection
- readback generation
- checkpoint emission
- artifact creation and registration
- side effect execution when explicitly approved and claimed

It does not own:

- global source of truth
- task projection
- durable ledger policy
- routing policy
- approval decision authority
- other endpoint orchestration

## Startup Contract

On startup, a worker must:

1. load local configuration
2. establish server connection
3. verify server API compatibility
4. establish NATS/JetStream connection when required
5. register endpoint identity
6. advertise capabilities
7. start heartbeat
8. start receive or claim loop

If endpoint registration fails, the worker must not process messages.

## Endpoint Registration

Minimum endpoint registration fields:

```text
endpoint_ref
display_name
actor_id
workspace_id
connection_state
capabilities[]
execution_hints
worker_version
started_at
```

The worker must not register:

- local secrets
- internal prompts
- full local path inventory
- endpoint-internal skill selection logic
- credential material

## Capability Contract

Capabilities are public routing declarations.

Rules:

- capability names are stable strings
- one message targets one `target_capability`
- skill selection remains internal to the worker
- capability changes must be re-advertised
- unsupported capability messages must be rejected before execution

## Message Receive Contract

The worker may receive messages through pull or push transport, but processing starts only after claim is accepted.

Required behavior:

- validate message envelope before ack/readback
- ack means the envelope was received, parsed, and accepted for processing
- ack does not mean completion
- if `readback_required=true`, emit readback before substantial work
- if message is expired, do not start new work
- if required fields are missing, emit `blocked_missing_fields` checkpoint

## Claim and Attempt Contract

Processing is attempt-based.

Rules:

- the claim unit is `message`
- successful claim creates or binds an `attempt_ref`
- retry or reassignment creates a new `attempt_ref`
- `attempt_no` is a task-local retry projection
- checkpoints must reference the current `attempt_ref`
- stale attempts must not continue external side effects after losing lease

## Checkpoint Contract

The worker must send checkpoints directly.
The server must not infer them on the worker's behalf.

Minimum checkpoint states:

- `claimed`
- `validating`
- `blocked_missing_fields`
- `in_progress`
- `retrying`
- `completed`
- `failed`

Rules:

- checkpoints are append-only
- checkpoint summaries must be short and operational
- `failed` checkpoint requires `failure_reason_code`
- `blocked_missing_fields` requires `missing_fields` and `missing_reasons`
- artifact progress should be reported by artifact refs, not large inline payloads

## Artifact Contract

Workers create artifact content and register artifact metadata.

Rules:

- large payloads must not be sent through the message body
- artifact body goes to artifact storage
- message/checkpoint references exact artifact versions
- artifact version content is immutable
- checksum must be provided for stored content
- final designation is requested explicitly and accepted by the server
- final designation is not implied by latest version

## Side Effect Contract

Protected side effects require approval and execution ledger claim.

Rules:

- worker must not execute protected side effects without approved approval record
- worker must not execute when `approval_hash` does not match current intent
- worker must claim or create side effect execution record before external call
- duplicate side effect request must return existing execution state
- uncertain external result must not be reported as success

## Local Execution Boundary

The worker may access local workspace and shell only within configured scope.

Required safeguards:

- configured workspace root
- allowlist or policy for executable tools
- timeout for local commands
- captured stdout/stderr policy
- artifact extraction policy
- no credential material in checkpoints or messages

## Reconnect and Crash Contract

On reconnect, the worker must:

- refresh endpoint heartbeat
- re-advertise capabilities
- ask server for recoverable attempts only if supported
- not assume it still owns previous leases
- not resume protected side effects without execution ledger state

If the worker crashes, server-side lease expiry and takeover rules apply.

## Configuration

Minimum configuration keys:

```text
server_url
nats_url
endpoint_display_name
actor_id
workspace_id
workspace_root
capabilities[]
credential_ref
auth_token
heartbeat_interval
checkpoint_interval
lease_refresh_interval
local_command_timeout
artifact_storage_mode
max_message_payload_bytes
log_level
otel_exporter_otlp_endpoint
```

`auth_token` is credential material and must be redacted from logs. When production auth uses an external credential store, `credential_ref` should be preferred.

## Observability

The worker must log:

- endpoint registration
- capability advertisement
- message received
- claim accepted/rejected
- readback sent
- checkpoint sent
- artifact registered
- local command started/finished
- side effect execution claimed/completed/failed
- reconnect events

Logs should include `endpoint_ref`, `message_id`, `task_id`, `attempt_ref`, and `correlation_id` when available.

## Test Contract

Minimum tests:

- endpoint registration payload
- capability advertisement
- message envelope validation
- readback requirement
- checkpoint state validation
- blocked missing fields checkpoint
- artifact metadata registration
- artifact final designation request
- side effect execution guard
- lease loss stops protected execution
- reconnect does not assume claim ownership
- server API compatibility failure

## Related Contracts

- `tc-worker/docs/definition-of-done.md`
- `tc-worker/docs/implementation-task-list.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/checkpoint-and-takeover-model.md`
- `docs/active/contracts/artifact-model.md`
- `docs/active/contracts/approval-identity-policy.md`
- `docs/active/contracts/delivery-semantics.md`

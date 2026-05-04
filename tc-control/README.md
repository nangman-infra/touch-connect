# tc-control

`tc-control` is the touch-connect control plane backend.

Responsibilities:

- expose the Control API used by `tcctl` and `admin`
- provide read models for endpoints, tasks, messages, artifacts, approvals, and DLQ records
- accept operator commands for task creation, message send, approval decisions, retry, cancel, artifact finalization, and DLQ replay
- validate control-plane authorization and audit every mutation
- forward accepted data-plane commands to `tc-server` through an explicit command path

It does not own message hot-path routing, worker execution, NATS/JetStream transport internals, or the web UI.

## Current API Surface

```text
GET  /healthz
GET  /readyz
GET  /version

GET  /v1/snapshot
GET  /v1/endpoints
GET  /v1/endpoints/inspect?ref=<endpoint_ref>
GET  /v1/capabilities

GET  /v1/tasks/status?task=<task_ref>
GET  /v1/tasks/history?task=<task_ref>
POST /v1/tasks/cancel
POST /v1/tasks/retry

GET  /v1/messages
GET  /v1/messages?task=<task_ref>
GET  /v1/messages/inspect?ref=<message_ref>
GET  /v1/messages/history?task=<task_ref>
POST /v1/messages

GET  /v1/artifacts
GET  /v1/artifacts?task=<task_ref>
GET  /v1/artifacts/inspect?ref=<artifact_version_ref>
POST /v1/artifacts/finalize

GET  /v1/approvals
GET  /v1/approvals/inspect?ref=<approval_ref>
POST /v1/approvals/decide

GET  /v1/dlq
GET  /v1/dlq/inspect?ref=<dead_letter_ref>
POST /v1/dlq/replay

GET  /v1/side-effects
GET  /v1/side-effects?task=<task_ref>
```

## Current Gaps

- Authn/authz is not enforced yet.
- Audit records are not persisted yet.
- The read model is currently backed by `tc-server` projection reads, not a separate projection store.
- Admin web UI is intentionally out of scope for the CLI-first phase.

Detailed implementation docs are maintained as local living contracts and are intentionally not tracked in the public Git repository.

# tc-control Definition of Done

## Purpose

This document defines what must be true before `tc-control` can be considered implementation-complete for the MVP control plane backend.

The checklist combines backend API best practices with the touch-connect product contract:

- `tc-control` is the control plane.
- `tc-server` is the data plane.
- `tcctl` and `admin` talk to `tc-control`, not directly to storage or broker internals.
- accepted state comes from server records or projections derived from them.
- protected mutations are explicit, authorized, idempotent, and audited.

## Completion Gate

`tc-control` is not done until all items below are true:

- Control API schemas are explicit, versioned, and tested.
- every command maps to a server command port or accepted read-model query.
- no message hot-path routing depends on `tc-control`.
- every protected mutation has actor identity, authorization, idempotency, and audit records.
- approval, retry, cancel, DLQ replay, and artifact finalization workflows are implemented.
- pagination, sorting, filtering, and projection freshness are stable.
- config is environment-driven, validated at startup, and redacted in diagnostics.
- `tcctl`, `admin`, `tc-control`, `tc-server`, and `tc-worker` can pass the canonical scenario.

## 1. Control API DoD

Done means:

- API style is selected and documented before implementation starts.
- APIs are versioned from the first implementation.
- every request and response has a stable schema.
- every mutation returns accepted state or a structured domain rejection.
- domain error codes are stable and machine-readable.
- request cancellation and timeout behavior are defined.
- API handlers contain no direct broker-specific logic.

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

## 2. Server Boundary DoD

Done means:

- `tc-server` client access is isolated behind application ports.
- data-plane commands are explicit and audited.
- `tc-control` does not write server-owned storage tables directly unless the selected storage adapter defines a shared read-model boundary.
- unavailable `tc-control` does not stop already accepted data-plane work.
- unavailable `tc-server` is surfaced as a control-plane failure, not local success.

Tests must cover:

- server success response.
- server domain rejection.
- incompatible server API version.
- unavailable server.
- command timeout.

## 3. Query and Projection DoD

Done means:

- list and inspect APIs return server-accepted ids and refs exactly.
- pagination is stable.
- sorting is deterministic.
- filters are documented.
- projection freshness metadata is exposed when stale reads are possible.
- raw artifact bodies, secrets, credentials, and full prompts are not returned through default query APIs.

Required query coverage:

- endpoints and capabilities
- task status and history
- messages and task message history
- artifact versions
- approval requests and decisions
- DLQ records

## 4. Mutation and Audit DoD

Done means:

- every mutation requires actor identity.
- protected mutations require authorization.
- retryable protected mutations require idempotency keys.
- audit records include request id, actor id, workspace id, operation, target refs, accepted/rejected outcome, and timestamp.
- local success is never emitted before server acceptance.
- duplicate command handling is deterministic.

Required mutation coverage:

- task create
- task retry
- task cancel
- message send
- artifact finalize
- approval approve
- approval reject
- DLQ replay

## 5. Approval and Protected Operation DoD

Done means:

- approve and reject include current `approval_hash`.
- expired approvals cannot be approved.
- self-approval policy is explicit.
- approval decisions do not mutate original messages.
- protected command responses expose enough context for `tcctl` and `admin` without exposing secrets.

## 6. Config and Environment DoD

Done means:

- deployment config is separated from code.
- environment variables are supported.
- optional config file support is explicit.
- all required config is validated at startup.
- invalid config fails fast.
- durations and sizes use typed parsing.
- secret values are redacted from logs and diagnostics.

Required environment variable namespace:

```text
TC_CONTROL_BIND_ADDR
TC_CONTROL_SERVER_URL
TC_CONTROL_STORAGE_DRIVER
TC_CONTROL_STORAGE_DSN
TC_CONTROL_READ_MODEL_DRIVER
TC_CONTROL_READ_MODEL_DSN
TC_CONTROL_AUTH_MODE
TC_CONTROL_ADMIN_ORIGIN_ALLOWLIST
TC_CONTROL_REQUEST_TIMEOUT
TC_CONTROL_LOG_LEVEL
TC_CONTROL_OTEL_EXPORTER_OTLP_ENDPOINT
```

## 7. Security and Observability DoD

Done means:

- every mutation authenticates the caller.
- authorization does not trust object ids as proof.
- CORS/origin policy for `admin` is explicit.
- logs never include secrets, credential material, full prompts, or raw artifact bodies.
- health reports process liveness.
- readiness reports `tc-server` and storage/read-model readiness.
- graceful shutdown stops accepting new API requests before closing dependencies.
- metrics include request latency, query latency, command latency, rejection count, approval count, retry/replay count, and auth failure count.

## 8. Release Readiness DoD

Done means:

- all unit tests pass.
- API compatibility tests pass for `tcctl` and `admin`.
- server command mapping tests pass.
- query/projection tests pass.
- approval, retry, cancel, DLQ replay, and artifact finalization tests pass.
- auth and audit tests pass.
- canonical scenario can complete through `tcctl` or `admin`, `tc-control`, `tc-server`, and `tc-worker`.
- documentation links from `tc-control/README.md` are current.

## Reference Baselines

- Twelve-Factor App config principle: https://www.12factor.net/config
- OWASP API Security project: https://owasp.org/API-Security/
- OpenTelemetry log correlation and context propagation: https://opentelemetry.io/docs/specs/otel/logs/

## Related Contracts

- `tc-control/docs/implementation-contract.md`
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

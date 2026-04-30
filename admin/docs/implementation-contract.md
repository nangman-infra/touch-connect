# admin Implementation Contract

## Purpose

`admin` is the web admin frontend for touch-connect.

It gives human operators a browser-based control surface for inspection and protected operations.
It is a client of `tc-control`, not a direct data-plane component.

## Runtime Role

`admin` owns:

- dashboard views
- endpoint and capability inspection
- task and message inspection
- artifact inspection and finalization workflow
- approval review and decision workflow
- DLQ inspection and replay workflow
- retry and cancel workflow
- canonical scenario operation views when selected for MVP development

It does not own:

- Control API business logic
- message routing
- worker execution
- server source-of-truth records
- broker or storage access
- authorization decisions beyond client-side affordances

## Backend Boundary

`admin` must call only `tc-control`.

Rules:

- no direct `tc-server` API calls
- no direct `tc-worker` API calls
- no direct NATS/JetStream access
- no direct database access
- no local mutation treated as accepted state
- optimistic UI, if used, must be visibly pending until `tc-control` acceptance

## Required Views

Minimum MVP views:

- dashboard
- endpoint list and endpoint detail
- task list and task detail
- message detail and task message history
- artifact list and artifact detail
- approval queue and approval detail
- DLQ list and DLQ detail
- system health/version

Protected operation views:

- approve approval request
- reject approval request
- retry task
- cancel task
- finalize artifact version
- replay DLQ record

## UI State Contract

Every view must distinguish:

- loading
- empty
- stale projection
- permission denied
- unavailable `tc-control`
- server rejected command
- command pending
- command accepted

Accepted state is whatever `tc-control` returns from server records or projections.

## Data Display Contract

Rules:

- exact ids and refs must be copyable.
- timestamps must include timezone or use a consistent UTC display policy.
- destructive/protected actions must show target refs before submission.
- secrets, credential material, full prompts, raw artifact bodies, and raw protected payloads are not rendered by default.
- large lists must use pagination or incremental loading.

## Configuration

Minimum configuration keys:

```text
api_base_url
auth_mode
default_workspace_id
request_timeout
log_level
```

Build-time or runtime environment variables must use the `TC_ADMIN_` prefix when the chosen frontend stack supports it.

## Test Contract

Minimum tests:

- route rendering
- API client request mapping
- loading, empty, error, and permission states
- approval approve/reject workflow
- retry, cancel, artifact finalize, and DLQ replay workflow
- no direct `tc-server` or storage client usage
- no secret rendering in default views
- accessibility checks for protected operation controls

## Related Contracts

- `admin/docs/definition-of-done.md`
- `admin/docs/implementation-task-list.md`
- `tc-control/docs/implementation-contract.md`
- `tc-server/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`

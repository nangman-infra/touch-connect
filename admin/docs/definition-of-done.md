# admin Definition of Done

## Purpose

This document defines what must be true before `admin` can be considered implementation-complete for the MVP web admin frontend.

The checklist combines frontend product quality with the touch-connect control/data plane split:

- `admin` is a human control surface.
- `admin` calls `tc-control`.
- `admin` does not talk directly to `tc-server`, `tc-worker`, broker, or storage.
- protected operations remain explicit, reviewable, and server-accepted.

## Completion Gate

`admin` is not done until all items below are true:

- required views exist and are navigable.
- every API call goes through the `tc-control` client.
- loading, empty, error, stale, permission, pending, accepted, and rejected states are implemented.
- protected operations show target refs and require explicit user action.
- exact ids and refs are visible or copyable where operators need them.
- secrets, credential material, full prompts, raw artifact bodies, and raw protected payloads are not rendered by default.
- UI tests, API mapping tests, and protected workflow tests are repeatable.

## 1. App Shell and Navigation DoD

Done means:

- the app has stable routes for all required views.
- navigation supports dashboard, endpoints, tasks, messages, artifacts, approvals, DLQ, and system health.
- layout works on supported desktop widths.
- page titles and loading states are specific to the current route.
- unavailable `tc-control` is shown as an operational state, not a blank screen.

## 2. API Client DoD

Done means:

- API access is isolated in a `tc-control` client module.
- no direct `tc-server`, `tc-worker`, broker, or storage client exists in frontend code.
- request timeout behavior is explicit.
- server domain errors map to stable UI states.
- auth failure maps to a permission state.
- accepted command responses refresh or invalidate the affected view.

## 3. Inspection Views DoD

Done means these views are implemented:

- dashboard
- endpoint list and detail
- task list and detail
- message detail and task message history
- artifact list and detail
- approval queue and detail
- DLQ list and detail
- system health/version

Each list view must support pagination or incremental loading before release.

## 4. Protected Workflow DoD

Done means these workflows are implemented:

- approve approval request
- reject approval request
- retry task
- cancel task
- finalize artifact version
- replay DLQ record

Rules:

- target refs are shown before submission.
- command pending state is visible.
- rejected commands show the server-provided reason.
- local success is never displayed before `tc-control` acceptance.

## 5. Data Safety and Accessibility DoD

Done means:

- secrets and credential material are not rendered.
- raw artifact bodies are not displayed by default.
- full prompts and protected payloads are not displayed by default.
- keyboard navigation works for protected actions.
- buttons and controls have accessible names.
- destructive or protected actions are visually distinct from read-only actions.

## 6. Config and Release DoD

Done means:

- API base URL is configurable.
- auth mode is configurable.
- build/runtime environment handling is documented for the selected stack.
- unit tests pass.
- API mapping tests pass.
- protected workflow tests pass.
- no direct data-plane or storage access is present.
- canonical scenario state can be inspected through the UI.

## Related Contracts

- `admin/docs/implementation-contract.md`
- `admin/docs/implementation-task-list.md`
- `tc-control/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`

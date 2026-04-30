# admin Implementation Task List

## Purpose

This document turns the `admin` Definition of Done into an implementation order for the web admin frontend.

## Rules

- Work top-down unless a dependency forces a change.
- Call only `tc-control`.
- Do not create hidden local source-of-truth state.
- Every protected workflow must show target refs and wait for server acceptance.
- Add or update tests in the same milestone as behavior changes.

## Status Legend

```text
[ ] not started
[~] in progress
[x] done
```

## Milestone 0: Freeze Frontend Decisions

Tasks:

- [ ] TCA-0001 Choose frontend stack and build tooling.
- [ ] TCA-0002 Freeze route map and navigation model.
- [ ] TCA-0003 Freeze API client generation or hand-written client policy.
- [ ] TCA-0004 Freeze auth mode and session handling.
- [ ] TCA-0005 Freeze supported viewport range.
- [ ] TCA-0006 Freeze protected action confirmation pattern.

Exit checks:

- stack, routes, API client, auth, viewport, and protected action decisions are documented.

## Milestone 1: App Shell and API Client

Tasks:

- [ ] TCA-0101 Initialize frontend project structure.
- [ ] TCA-0102 Add app shell and navigation.
- [ ] TCA-0103 Add configuration loading for `TC_ADMIN_` values or selected stack equivalent.
- [ ] TCA-0104 Add `tc-control` API client boundary.
- [ ] TCA-0105 Add request timeout and error mapping.
- [ ] TCA-0106 Add loading, empty, error, permission, pending, accepted, and rejected state primitives.
- [ ] TCA-0107 Add basic route tests.

Exit checks:

- app renders without `tc-control`.
- unavailable `tc-control` shows a stable state.
- no direct `tc-server` or storage client exists.

## Milestone 2: Read-Only Operational Views

Tasks:

- [ ] TCA-0201 Implement dashboard.
- [ ] TCA-0202 Implement system health/version view.
- [ ] TCA-0203 Implement endpoint list and detail.
- [ ] TCA-0204 Implement task list and detail.
- [ ] TCA-0205 Implement message detail and task message history.
- [ ] TCA-0206 Implement artifact list and detail.
- [ ] TCA-0207 Implement approval queue and detail.
- [ ] TCA-0208 Implement DLQ list and detail.
- [ ] TCA-0209 Add pagination or incremental loading for list views.

Exit checks:

- exact ids and refs are visible or copyable.
- stale projection state can be shown when API supplies freshness metadata.
- secrets and raw artifact bodies are not rendered by default.

## Milestone 3: Protected Workflows

Tasks:

- [ ] TCA-0301 Implement approval approve workflow.
- [ ] TCA-0302 Implement approval reject workflow.
- [ ] TCA-0303 Implement task retry workflow.
- [ ] TCA-0304 Implement task cancel workflow.
- [ ] TCA-0305 Implement artifact finalize workflow.
- [ ] TCA-0306 Implement DLQ replay workflow.
- [ ] TCA-0307 Add pending and rejected command rendering.
- [ ] TCA-0308 Add workflow tests.

Exit checks:

- protected commands show target refs before submission.
- local success is displayed only after `tc-control` acceptance.
- server rejection reason is visible.

## Milestone 4: Hardening and Release Gate

Tasks:

- [ ] TCA-0401 Add no-secret rendering tests.
- [ ] TCA-0402 Add API mapping tests.
- [ ] TCA-0403 Add accessibility checks for protected controls.
- [ ] TCA-0404 Add responsive layout checks for supported viewport range.
- [ ] TCA-0405 Add canonical scenario inspection walkthrough.
- [ ] TCA-0406 Verify docs links.

Exit checks:

- all unit and UI tests pass.
- no direct data-plane or storage access is present.
- canonical scenario state can be inspected through `admin`.

## Related Docs

- `admin/docs/implementation-contract.md`
- `admin/docs/definition-of-done.md`
- `tc-control/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`

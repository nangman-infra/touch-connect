# tcctl Implementation Task List

## Purpose

This document turns the `tcctl` Definition of Done into an implementation order.

Use this as the development checklist for the MVP operator CLI. The DoD says when the CLI is acceptable; this task list says what to build first, what each step should produce, and which gates it closes.

## Rules

- Work top-down unless a dependency forces a change.
- Treat `tc-control` as the only Control API boundary.
- Treat server-accepted records and projections returned by `tc-control` as truth.
- Do not write storage or JetStream directly.
- Do not execute AI task work locally.
- Preserve stable JSON output once released.
- Add or update tests in the same milestone as behavior changes.

## Status Legend

```text
[ ] not started
[~] in progress
[x] done
```

## Milestone 0: Freeze CLI Decisions

Goal: remove decisions that would otherwise be made while adding commands.

Tasks:

- [ ] TCC-0001 Choose CLI framework.
  - Output: framework decision and rationale.
  - Default candidate: Cobra when subcommand/help/completion needs justify it; standard `flag` when scope stays small.
- [ ] TCC-0002 Freeze command naming and grouping.
  - Output: command tree for server, endpoint, task, message, artifact, approval, DLQ, and scenario.
- [ ] TCC-0003 Freeze config precedence.
  - Output: flags, env vars, config file, default order.
- [ ] TCC-0004 Freeze output modes.
  - Output: human output policy and JSON output policy.
- [ ] TCC-0005 Freeze exit code mapping.
  - Output: exit code table.
- [ ] TCC-0006 Freeze auth material handling.
  - Output: actor id, workspace id, token source, and redaction rules.
- [ ] TCC-0007 Freeze Control API compatibility policy.
  - Output: compatible Control API version range and failure behavior.

Exit checks:

- CLI framework, command tree, config, output, exit code, auth, and Control API compatibility decisions are documented.
- no command is implemented with unresolved UX or API mapping.

Closes DoD areas:

- Command Model
- Config and Environment
- Output and Exit Code
- Security

## Milestone 1: CLI Skeleton and Config

Goal: create the runnable CLI shell without server behavior.

Tasks:

- [ ] TCC-0101 Add `tcctl/cmd/tcctl/main.go`.
- [ ] TCC-0102 Add CLI package layout.
- [ ] TCC-0103 Add root command.
- [ ] TCC-0104 Add command groups.
- [ ] TCC-0105 Add config loader.
- [ ] TCC-0106 Add config precedence.
- [ ] TCC-0107 Add config validation and redaction.
- [ ] TCC-0108 Add `--help`, `--version`, and `--json`.
- [ ] TCC-0109 Add build/test commands.

Suggested package boundaries:

```text
tcctl/cmd/tcctl
tcctl/internal/cli
tcctl/internal/app
tcctl/internal/ports
tcctl/internal/adapters/controlapi
tcctl/internal/config
tcctl/internal/output
```

Exit checks:

- root help works.
- group help works.
- version output works.
- invalid config fails before API call.
- `go test ./...` passes.

Closes DoD areas:

- Command Model
- Config and Environment
- Observability and Release

## Milestone 2: Control API Client and Error Mapping

Goal: make all commands call `tc-control` through one client boundary.

Tasks:

- [ ] TCC-0201 Define Control API client port.
- [ ] TCC-0202 Implement selected Control API client adapter.
- [ ] TCC-0203 Implement Control API compatibility check.
- [ ] TCC-0204 Implement request timeout.
- [ ] TCC-0205 Implement auth and actor identity attachment.
- [ ] TCC-0206 Implement control/server domain error mapping.
- [ ] TCC-0207 Implement unavailable control plane mapping.
- [ ] TCC-0208 Implement response envelope decoding.
- [ ] TCC-0209 Add fake `tc-control` test harness.

Exit checks:

- control success response maps to command result.
- incompatible Control API version maps to command failure.
- domain rejection maps to exit code `2`.
- unavailable control plane maps to exit code `3`.
- auth failure maps to exit code `4`.
- diagnostics go to stderr.

Closes DoD areas:

- Control API Mapping
- Output and Exit Code
- Security

## Milestone 3: Read-Only Commands

Goal: make inspection workflows stable before mutation workflows.

Tasks:

- [ ] TCC-0301 Implement `tcctl server health`.
- [ ] TCC-0302 Implement `tcctl server version`.
- [ ] TCC-0303 Implement `tcctl endpoint list`.
- [ ] TCC-0304 Implement `tcctl endpoint inspect <endpoint_ref>`.
- [ ] TCC-0305 Implement `tcctl endpoint capabilities`.
- [ ] TCC-0306 Implement `tcctl task status <task_id>`.
- [ ] TCC-0307 Implement `tcctl task history <task_id>`.
- [ ] TCC-0308 Implement `tcctl message inspect <message_id>`.
- [ ] TCC-0309 Implement `tcctl message history --task <task_id>`.
- [ ] TCC-0310 Implement `tcctl artifact list --task <task_id>`.
- [ ] TCC-0311 Implement `tcctl artifact inspect <artifact_version_ref>`.

Exit checks:

- human output is concise.
- JSON output preserves exact ids and refs.
- pagination behavior is stable.
- no raw secret or artifact body is printed.

Closes DoD areas:

- Command Model
- Control API Mapping
- Output and Exit Code

## Milestone 4: Task and Message Mutation Commands

Goal: create work through canonical server-accepted records.

Tasks:

- [ ] TCC-0401 Implement `tcctl task create`.
- [ ] TCC-0402 Implement `tcctl task cancel <task_id>`.
- [ ] TCC-0403 Implement `tcctl message send`.
- [ ] TCC-0404 Implement canonical message envelope builder.
- [ ] TCC-0405 Implement delivery class validation.
- [ ] TCC-0406 Implement readback-required flag handling.
- [ ] TCC-0407 Implement input from flags.
- [ ] TCC-0408 Implement optional stdin or file input if selected in Milestone 0.
- [ ] TCC-0409 Implement oversized inline payload rejection.

Exit checks:

- task create returns `tc-control` accepted task id backed by server records.
- message send returns `tc-control` accepted message id or structured rejection.
- invalid message envelope fails before API call when possible.
- protected side effect intent requires idempotency data or server policy delegation.

Closes DoD areas:

- Input Validation
- Control API Mapping
- Output and Exit Code

## Milestone 5: Approval, Retry, and DLQ Commands

Goal: make protected operator actions explicit and auditable.

Tasks:

- [ ] TCC-0501 Implement `tcctl approval list`.
- [ ] TCC-0502 Implement `tcctl approval inspect <approval_id>`.
- [ ] TCC-0503 Implement `tcctl approval approve <approval_id>`.
- [ ] TCC-0504 Implement `tcctl approval reject <approval_id>`.
- [ ] TCC-0505 Implement approval hash inclusion.
- [ ] TCC-0506 Implement actor identity requirement for approval decisions.
- [ ] TCC-0507 Implement `tcctl task retry <task_id>`.
- [ ] TCC-0508 Implement `tcctl artifact finalize <artifact_version_ref>`.
- [ ] TCC-0509 Implement `tcctl dlq list`.
- [ ] TCC-0510 Implement `tcctl dlq inspect <dead_letter_id>`.
- [ ] TCC-0511 Implement `tcctl dlq replay <dead_letter_id>`.
- [ ] TCC-0512 Add explicit intent flags for replay or protected commands when selected.

Exit checks:

- approval commands preserve actor identity.
- approval hash mismatch is surfaced.
- expired approval rejection is surfaced.
- retry and replay output reflects `tc-control` accepted state.
- artifact finalization output reflects `tc-control` accepted state.
- no command fakes success locally.

Closes DoD areas:

- Approval and Protected Operation
- Retry, DLQ, and Scenario
- Security

## Milestone 6: Canonical Scenario Commands

Goal: make MVP scenario execution and verification repeatable.

Tasks:

- [ ] TCC-0601 Implement `tcctl scenario run canonical`.
- [ ] TCC-0602 Implement scenario input options.
- [ ] TCC-0603 Implement room creation step.
- [ ] TCC-0604 Implement task creation step.
- [ ] TCC-0605 Implement initial handoff message step.
- [ ] TCC-0606 Implement optional approval decision step.
- [ ] TCC-0607 Implement fake-worker mode only if explicitly selected.
- [ ] TCC-0608 Implement `tcctl scenario verify canonical`.
- [ ] TCC-0609 Implement verification report output.
- [ ] TCC-0610 Implement validation report artifact access for scenario verification.

Scenario verify checks:

```text
readback-required handoff succeeded
at least two artifact versions exist
validation report includes SonarQube gate pass
approval decision is approved
side effect execution record is succeeded
side effect uniqueness boundary executed once
final task state is completed
```

Exit checks:

- scenario run creates only `tc-control` accepted records.
- fake-worker mode cannot be mistaken for real worker execution.
- scenario verify fails when any required condition is missing.
- scenario verify JSON output is stable.

Closes DoD areas:

- Retry, DLQ, and Scenario
- Control API Mapping
- Observability and Release

## Milestone 7: Output, Docs, and Hardening

Goal: make the CLI reliable for humans and automation.

Tasks:

- [ ] TCC-0701 Add stable JSON output tests for every command.
- [ ] TCC-0702 Add human output tests for key commands.
- [ ] TCC-0703 Add stderr tests for errors.
- [ ] TCC-0704 Add no-secret output tests.
- [ ] TCC-0705 Add verbose mode or debug output if selected.
- [ ] TCC-0706 Add generated command docs or help snapshots if selected.
- [ ] TCC-0707 Add command examples.
- [ ] TCC-0708 Add shell completion if selected.

Exit checks:

- stdout and stderr are separated correctly.
- JSON output is stable.
- help output is current.
- no secret appears in diagnostics.

Closes DoD areas:

- Output and Exit Code
- Command Model
- Observability and Release

## Milestone 8: Release Gate

Goal: prove the CLI is ready to drive the MVP flow.

Tasks:

- [ ] TCC-0801 Run command parsing tests.
- [ ] TCC-0802 Run Control API mapping tests.
- [ ] TCC-0803 Run JSON output tests.
- [ ] TCC-0804 Run control/server rejection tests.
- [ ] TCC-0805 Run unavailable control plane tests.
- [ ] TCC-0806 Run approval command tests.
- [ ] TCC-0807 Run DLQ replay command tests.
- [ ] TCC-0808 Run canonical scenario tests.
- [ ] TCC-0809 Verify docs links.

Exit checks:

- all tests pass.
- canonical scenario commands work against `tc-control`.
- command help and docs match implemented commands.
- release notes can list supported command groups.

Closes DoD areas:

- Observability and Release
- all previous DoD areas

## DoD Coverage Map

```text
Command Model:                         M0, M1, M3, M7
Control API Mapping:                   M2, M3, M4, M6
Input Validation:                      M4
Output and Exit Code:                  M0, M2, M3, M4, M7
Approval and Protected Operation:      M5
Retry, DLQ, and Scenario:              M5, M6
Config and Environment:                M0, M1
Security:                              M0, M2, M5
Observability and Release:             M1, M2, M6, M7, M8
```

## Not In This List

These belong to other app task lists:

- server ledger implementation.
- worker endpoint registration automation.
- worker local execution.
- worker checkpoint generation.
- direct broker or storage mutation.
- admin UI.

## Related Docs

- `tcctl/docs/implementation-contract.md`
- `tcctl/docs/definition-of-done.md`
- `tc-control/docs/implementation-contract.md`
- `tc-server/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/approval-identity-policy.md`
- `docs/active/contracts/delivery-semantics.md`

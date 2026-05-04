# tcctl

`tcctl` is the operator and admin CLI.

It talks to `tc-control` as its only Control API backend.

Responsibilities:

- create tasks and messages
- inspect endpoints, tasks, messages, checkpoints, artifacts, approvals, and DLQ state
- approve or reject approval requests
- request retry or DLQ replay
- run the canonical MVP scenario during development

It is a human control surface, not an execution worker.
It does not write storage, broker, or `tc-server` internals directly.

## Current Commands

```text
tcctl server health
tcctl server version

tcctl endpoint list
tcctl endpoint inspect <endpoint_ref>
tcctl endpoint capabilities

tcctl task create <task_ref> --capability <name> --summary <text> --body <text>
tcctl task status <task_ref>
tcctl task history <task_ref>
tcctl task cancel <task_ref>
tcctl task retry <task_ref>

tcctl message send --capability <name> --summary <text> --body <text> [--task <task_ref>]
tcctl message list [--task <task_ref>]
tcctl message inspect <message_ref>
tcctl message history [--task <task_ref>]

tcctl artifact list [--task <task_ref>]
tcctl artifact inspect <artifact_version_ref>
tcctl artifact finalize <artifact_version_ref> --actor <actor_id>

tcctl approval list
tcctl approval inspect <approval_ref>
tcctl approval approve <approval_ref> --attempt-ref <attempt_ref> --target-ref <target_ref> --requested-by <actor_id> --approvers <role> --scope <scope> --hash <hash> --decided-by <actor_id>
tcctl approval reject <approval_ref> --attempt-ref <attempt_ref> --target-ref <target_ref> --requested-by <actor_id> --approvers <role> --scope <scope> --hash <hash> --decided-by <actor_id> --reason <reason>

tcctl dlq list
tcctl dlq inspect <dead_letter_ref>
tcctl dlq replay <dead_letter_ref>

tcctl scenario run canonical [--task <task_ref>]
tcctl scenario verify canonical [--task <task_ref>]
```

## Config

```text
--control-url / TCCTL_CONTROL_URL
--timeout / TCCTL_TIMEOUT
--contract-version / TCCTL_CONTRACT_VERSION
--json
```

`tcctl` checks the `tc-control` reported contract version before executing commands, except local `--version` and `server version` inspection.

## Current Gaps

- Auth material is not yet enforced as middleware.
- Mutation audit is not yet persisted by `tc-control`.
- `scenario verify canonical` reports missing evidence as `passed=false`; it does not fake scenario success.

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)

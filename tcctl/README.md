# tcctl

`tcctl` is the operator and admin CLI.

It talks to `tc-control` as its only Control API backend.

Responsibilities:

- create tasks and messages
- inspect endpoints, tasks, messages, checkpoints, artifacts, approvals, and DLQ state
- approve or reject approval requests
- request retry or DLQ replay
- register local AI `SKILL.md` guidance documents
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

tcctl task create <task_ref> --capability <name> --summary <text> (--body <text>|--body-file <path>)
tcctl task status <task_ref>
tcctl task history <task_ref>
tcctl task watch <task_ref> [--interval 1s]
tcctl task cancel <task_ref>
tcctl task retry <task_ref>

tcctl message send --capability <name> --summary <text> (--body <text>|--body-file <path>) [--task <task_ref>]
tcctl message list [--task <task_ref>]
tcctl message inspect <message_ref>
tcctl message history [--task <task_ref>]
tcctl message tail [--task <task_ref>] [--capability <cap>] [--interval 1s]

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

tcctl skill register /absolute/path/to/SKILL.md
tcctl skill list
tcctl skill inspect <skill_ref_or_name>

tcctl manager [--once]
tcctl manager --watch [--task <task_ref>] [--capability <cap>]
tcctl manager --send --capability <cap> --summary <text> (--body <text>|--body-file <path>) [--task <task_ref>] [--watch]

tcctl monitor [--interval 1s] [--once]

tcctl scenario run canonical [--task <task_ref>] [--wait=true] [--wait-timeout 10s]
tcctl scenario verify canonical [--task <task_ref>]
```

## Config

```text
--control-url / TCCTL_CONTROL_URL
--timeout / TCCTL_TIMEOUT
--contract-version / TCCTL_CONTRACT_VERSION
--json
```

`tcctl skill` commands use a local registry and do not require a running control plane.

```text
--registry / TCCTL_SKILL_REGISTRY / TC_SKILL_REGISTRY
```

`tcctl` checks the `tc-control` reported contract version before executing commands, except local `--version`, local help output, `server version` inspection, and local `skill` registry commands.

Use command help without a running control plane:

```text
tcctl help message
tcctl message send -h
tcctl task create -h
```

## Live Watch

`manager` is the primary human operator surface. It combines send, worker visibility, task state, timeline, and next actions in one cockpit:

```text
tcctl manager --once
tcctl manager --task tc://task/live_ai_tikitaka --watch
tcctl manager --send --capability ai.review --summary "Review handoff" --body-file /absolute/path/body.md --watch
```

Use `manager` when a human is actively driving AI-to-AI handoffs. Use the lower-level `message`, `task`, `endpoint`, and `artifact` commands when scripts need precise machine-readable operations.

`message tail` and `task watch` are polling-based operator views for local standalone runs. They print one line for each new or changed message, attempt, checkpoint, readback, and artifact:

```text
tcctl message tail --capability ai.review
tcctl task watch tc://task/live_ai_tikitaka
```

Use `--once` in scripts or tests to print the current matching state and exit.

`monitor` is the standalone operator entrypoint. It prints workers, message states, tasks, quality decisions, and recent artifacts in one frame:

```text
tcctl monitor --once
tcctl monitor --interval 1s
```

## Current Gaps

- Auth material is not yet enforced as middleware.
- Mutation audit is not yet persisted by `tc-control`.
- `scenario verify canonical` reports missing evidence as `passed=false`; `scenario run canonical` waits for real worker evidence instead of writing worker checkpoints locally.

Detailed implementation docs are maintained as local living contracts and are intentionally not tracked in the public Git repository.

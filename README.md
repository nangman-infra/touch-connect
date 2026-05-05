# touch-connect

`touch-connect` is a message-quality and handoff-governance layer for heterogeneous AI agents, compatible with A2A and AGNTCY.

A2A and AGNTCY move agent messages across systems. `touch-connect` makes those handoffs sufficient, correctly understood, approval-aware, and auditable.

It is not a queue, transport, workflow engine, or new wire protocol. Production transport, durability, and replay belong behind adapters such as NATS JetStream, Temporal, A2A, and AGNTCY-compatible bindings. The built-in memory and SQLite paths exist for local development and tests.

The project is still contract-first, but living contract docs are maintained locally and are intentionally not tracked in the public Git repository.

Root app units:

- `tc-server`
  - message governance records and adapter-backed delivery data plane
- `tc-control`
  - control plane backend API for `tcctl` and `admin`
- `tc-worker`
  - execution endpoint runtime, including skill-guided AI workers that read local `SKILL.md` guidance and run local AI CLIs such as Codex or Claude Code
- `tcctl`
  - operator/admin CLI
- `admin`
  - web admin frontend

Runtime relationship:

```text
tc-worker -> tc-server
tcctl    -> tc-control
admin    -> tc-control
tc-control -> tc-server control command path
tc-server  -> NATS/JetStream + Temporal + A2A/AGNTCY adapters
```

`tc-server` must stay focused on accepted message records, quality policy enforcement, governance state, and adapter-backed dispatch.
Operator APIs, admin workflows, approvals, retries, DLQ replay, and inspection belong to `tc-control`.

## Standalone Compose

The standalone local stack is Docker Compose first. It starts `tc-server`, `tc-control`, a basic `code.change` worker, SQLite-backed state, artifact storage, and the dev NATS service:

```sh
docker compose -f docker-compose.dev.yml up -d --build
```

Useful commands:

```sh
docker compose -f docker-compose.dev.yml ps
docker compose -f docker-compose.dev.yml logs -f tc-server tc-control tc-worker-echo
docker compose -f docker-compose.dev.yml run --rm tcctl endpoint list
docker compose -f docker-compose.dev.yml run --rm tcctl message send \
  --capability code.change \
  --summary "compose smoke" \
  --body "Verify compose echo worker can receive and complete a message." \
  --quality-gate=skip
```

Or use the Makefile wrapper:

```sh
make dev-up
make smoke
make dev-logs
make dev-down
```

Local AI CLI workers remain host-side by default because Codex, Claude Code, Gemini, and Kiro rely on the user's local installation and authentication state. Attach a host Codex worker to the Compose server with:

```sh
make host-codex-worker
```

For Claude Max users, the easiest worker path is:

```sh
make host-claude-worker
```

That target uses `--backend claude --model 'opus[1m]'` by default. On Claude Code Max plans, this targets Opus 4.7 with the 1M context window. Worker join presets are non-interactive by default: Claude uses `--permission-mode bypassPermissions`, Codex uses `approval_policy="never"`, Gemini uses `--approval-mode yolo`, and Kiro uses `--trust-all-tools`. This is intentionally powerful and risky; run it only in a trusted local workspace.

## Local AI Handoff Roles

The canonical worker onboarding contract is [WORKER.md](./WORKER.md). If an AI session is asked to become a worker, it should read that file first. The project contract, not a one-off user prompt, defines what "worker" means.

For a real heterogeneous-AI handoff demo, do not ask the second AI to answer in its chat window. That proves only that a human copied the message. The second AI should join as a `tc-worker` backend. `tc-worker join` waits for matching messages, wakes the selected local AI CLI, and writes the result back as readback, checkpoints, completion, and an artifact.

There are two roles:

- Manager/operator: decides what should happen, watches the system, and sends messages with `tcctl`.
- Worker AI: runs `tc-worker join`, stays registered as an endpoint, advertises capabilities from `SKILL.md`, waits for messages, executes received tasks, and records evidence. It does not send its own test message or grade its own result.

The compose `tc-worker-echo` service is only a smoke-test worker. The manager should stop it when a host AI worker should receive `code.change`, otherwise both workers can race for the same capability:

```sh
cd /absolute/path/to/touch-connect
docker compose -f docker-compose.dev.yml stop tc-worker-echo
```

### Worker AI Terminal

Preferred worker start commands:

```sh
cd /absolute/path/to/touch-connect
go run ./tc-worker/cmd/tc-worker join \
  --backend claude \
  --model 'opus[1m]' \
  --skills-dir /absolute/path/to/touch-connect/examples/skills \
  --capabilities code.change
```

After this starts, the worker stays in the foreground and waits. It should not run `tcctl message send`, `tcctl task watch`, or `tcctl task history` to verify itself. Those commands belong to the manager terminal.

For a Codex worker:

```sh
go run ./tc-worker/cmd/tc-worker join \
  --backend codex \
  --model gpt-5.4-mini \
  --skills-dir /absolute/path/to/touch-connect/examples/skills \
  --capabilities code.change
```

Supported backend presets are `claude`, `codex`, `gemini`, and `kiro`. `--model` is passed to the selected backend when that CLI supports model selection. Use `--command` and `--args` only for debugging a custom local AI CLI.

### Manager Terminal

Watch the live flow first:

```sh
cd /absolute/path/to/touch-connect
docker compose -f docker-compose.dev.yml run --rm tcctl monitor --once
docker compose -f docker-compose.dev.yml run --rm tcctl message tail --capability code.change
```

In another manager terminal, send the task:

```sh
cd /absolute/path/to/touch-connect

TASK_REF=tc://task/local_ai_handoff_demo

docker compose -f docker-compose.dev.yml run --rm tcctl message send \
  --capability code.change \
  --summary "Manager requests worker result" \
  --body "Role split: the sender is the manager/operator and the receiver is the worker AI. Inspect the current standalone compose + watch/tail flow. Return WORKER_READBACK, WORKER_ACTION, and WORKER_RESULT_READY. Do not modify files." \
  --task "$TASK_REF" \
  --readback-required \
  --quality-gate=skip

docker compose -f docker-compose.dev.yml run --rm tcctl task watch "$TASK_REF" --once
docker compose -f docker-compose.dev.yml run --rm tcctl task history "$TASK_REF"
```

The demo passes only when all of these are true:

- `endpoint list` shows the worker AI as `online`.
- `task watch` shows `message`, `attempt`, `checkpoint`, `readback`, and `artifact` lines.
- `task history` has at least one artifact for the worker attempt.
- The artifact JSON contains the worker output, including `WORKER_READBACK`, `WORKER_ACTION`, and `WORKER_RESULT_READY`.

If the other AI writes the answer only in its normal chat UI and no artifact appears in `task watch`, the handoff did not complete as a touch-connect worker execution.

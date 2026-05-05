# touch-connect Worker Onboarding Contract

This file is the first document an AI session should read when it is asked to become a touch-connect worker.

If you are an AI reading this file, do not treat "you are the worker" as a request to answer in chat. A worker is a local `tc-worker join` process that joins the touch-connect server as an endpoint, waits for messages, wakes your local AI CLI when work arrives, and leaves auditable evidence in touch-connect.

## Roles

- Manager/operator: sends tasks with `tcctl`, watches live state, and decides the next step.
- Worker AI: starts `tc-worker join`, advertises capabilities from `SKILL.md`, waits for matching messages, executes the task only when the server delivers one, and records readback, checkpoints, completion, and artifacts.

## Non-Negotiable Rules

1. Chat-only answers do not count as worker execution.
2. A completed handoff must create server-visible evidence.
3. `task watch` must show the worker message, attempt, checkpoint, readback, and artifact.
4. `task history` must show at least one artifact for the worker attempt.
5. The artifact must contain the actual worker output.
6. If file edits are not explicitly requested by the message, do not modify files.
7. A worker must not send its own test message.
8. A worker must not decide whether the handoff passed or failed.
9. A worker must not run manager verification commands such as `message send`, `task watch`, or `task history` unless it has been explicitly reassigned as the manager/operator.

## Worker Startup Flow

Use this flow when a human or manager says "you are the worker":

1. Read `README.md`, `tc-worker/README.md`, `tcctl/README.md`, and the selected `SKILL.md`.
2. Start `tc-worker join --wizard` and choose an installed backend/model, or use an explicit backend command if the manager already chose one.
3. Keep the worker process running until the manager stops it.
4. Wait for server-delivered messages.
5. When a message arrives, `tc-worker` wakes the configured AI CLI and processes the message.
6. Do not send a message to yourself.
7. Do not run `task watch` or `task history` to grade yourself.
8. Do not answer the task only in chat.

Server readiness, conflicting worker shutdown, message sending, and pass/fail verification are manager/operator responsibilities.

## Preferred Worker Join Command

From the repository root:

```sh
go run ./tc-worker/cmd/tc-worker join --wizard \
  --skills-dir /absolute/path/to/touch-connect/examples/skills
```

The wizard detects installed AI CLIs and marks their current state:

```text
Detected AI CLIs:
  1. Claude       ready        model=opus[1m] path=/opt/homebrew/bin/claude
  2. Codex        auth_unknown model=default path=/opt/homebrew/bin/codex
  3. Gemini       missing      command=gemini
  4. Kiro         missing      command=kiro-cli
```

If Claude Code is installed, the recommended Claude Max default is `opus[1m]`.

Direct Claude command:

```sh
go run ./tc-worker/cmd/tc-worker join \
  --backend claude \
  --model 'opus[1m]' \
  --skills-dir /absolute/path/to/touch-connect/examples/skills \
  --capabilities code.change
```

After this command starts, the worker should stay in the foreground and wait. Seeing no worker chat output is normal. The manager verifies endpoint registration from another terminal.

Claude joins with `--permission-mode bypassPermissions` by default. That prevents Claude Code from stopping mid-handoff for local permission prompts. Use this only in a trusted local workspace.

For a Codex worker:

```sh
go run ./tc-worker/cmd/tc-worker join \
  --backend codex \
  --model gpt-5.4-mini \
  --skills-dir /absolute/path/to/touch-connect/examples/skills \
  --capabilities code.change
```

For a Gemini worker:

```sh
GEMINI_MODEL=your-gemini-model
go run ./tc-worker/cmd/tc-worker join \
  --backend gemini \
  --model "$GEMINI_MODEL" \
  --skills-dir /absolute/path/to/touch-connect/examples/skills \
  --capabilities code.change
```

Kiro is exposed as a backend preset too, but it depends on a local headless Kiro CLI being installed and authenticated.

Default join presets must not wait for interactive approval:

- Claude: `--permission-mode bypassPermissions`
- Codex: `approval_policy="never"`
- Gemini: `--approval-mode yolo`
- Kiro: `--trust-all-tools`

## Raw Worker Environment

Use this only when debugging `tc-worker join` itself:

```sh
mkdir -p /tmp/tc-worker-ai/artifacts

TC_WORKER_SERVER_URL=http://127.0.0.1:8080 \
TC_WORKER_ENDPOINT_REF=tc://endpoint/local_ai_worker \
TC_WORKER_DISPLAY_NAME="Local AI worker" \
TC_WORKER_ACTOR_ID=actor.local-ai-worker \
TC_WORKER_WORKSPACE_ID=workspace.local \
TC_WORKER_EXECUTOR=skill \
TC_WORKER_SKILL_BACKEND=ai-cli \
TC_WORKER_SKILLS_DIR="$(pwd)/examples/skills" \
TC_WORKER_CAPABILITIES=code.change \
TC_WORKER_AI_CLI_COMMAND="$(command -v claude)" \
TC_WORKER_AI_CLI_ARGS="-p,--permission-mode,bypassPermissions,--model,opus[1m]" \
TC_WORKER_AI_CLI_WORKDIR="$(pwd)" \
TC_WORKER_AI_CLI_TIMEOUT=180s \
TC_WORKER_ARTIFACT_DIR=/tmp/tc-worker-ai/artifacts \
TC_WORKER_POLL_INTERVAL=500ms \
TC_WORKER_HEARTBEAT_INTERVAL=5s \
go run ./tc-worker/cmd/tc-worker
```

For a Codex worker, keep the same contract and replace the AI CLI command settings:

```sh
TC_WORKER_AI_CLI_COMMAND="$(command -v codex)"
TC_WORKER_AI_CLI_ARGS='exec,--skip-git-repo-check,--sandbox,read-only,-c,approval_policy="never",-'
```

## Expected Worker Output Contract

Every successful worker result should include these sections in the execution output:

```text
WORKER_READBACK
<what you understood>

WORKER_ACTION
<what you did>

WORKER_RESULT_READY
<final concise result>
```

The output must be produced by the worker execution path so it is captured in the execution artifact. If these sections appear only in the chat UI and not in the artifact, the handoff failed.

## Manager-Owned Verification

This section is for the manager/operator, not for the worker. A worker should not run these commands as part of onboarding.

The manager verifies worker readiness and execution with:

```sh
docker compose -f docker-compose.dev.yml run --rm tcctl monitor --once
docker compose -f docker-compose.dev.yml run --rm tcctl endpoint list
docker compose -f docker-compose.dev.yml run --rm tcctl message tail --capability code.change
docker compose -f docker-compose.dev.yml run --rm tcctl task watch <task_ref> --once
docker compose -f docker-compose.dev.yml run --rm tcctl task history <task_ref>
```

Pass condition:

- The worker endpoint is `online`.
- The intended worker endpoint claims the message.
- `task watch` includes `readback` and `artifact`.
- The artifact contains `WORKER_READBACK`, `WORKER_ACTION`, and `WORKER_RESULT_READY`.

# tc-worker

`tc-worker` is the installable local AI worker runtime for touch-connect.

It is the part a user runs on their own machine to let an authenticated local AI CLI, such as Claude Code, Codex, Gemini, or Kiro, subscribe to touch-connect messages and execute skill-guided work.

## Normal Lifecycle

Install or update only the worker binary:

```sh
curl -fsSL https://raw.githubusercontent.com/nangman-infra/touch-connect/main/scripts/install-worker.sh | sh
```

For the current alpha prerelease, install with an explicit version because GitHub does not expose prereleases through the `latest` download URL:

```sh
curl -fsSL https://raw.githubusercontent.com/nangman-infra/touch-connect/main/scripts/install-worker.sh \
  | VERSION=worker-v0.1.0-alpha.2 sh
```

Then configure once:

```sh
tc-worker setup
```

Start the worker:

```sh
tc-worker join
```

The setup command writes:

```text
~/.touch-connect/worker/config.json
~/.touch-connect/worker/skills/local-ai-worker/SKILL.md
~/.touch-connect/worker/artifacts/
```

`tc-worker join` loads that config. If the config is missing and the terminal is interactive, `join` runs setup once and then starts the worker.

## Installed Command Surface

```text
tc-worker install       install the latest released worker binary
tc-worker setup         create or refresh local worker config
tc-worker join          start the configured worker cockpit
tc-worker doctor        inspect config and installed AI CLIs
tc-worker update        update the installed worker binary
tc-worker uninstall     remove the installed worker binary
tc-worker version       print build version
```

`install` and `update` use GitHub release assets named:

```text
tc-worker_Darwin_x86_64.tar.gz
tc-worker_Darwin_arm64.tar.gz
tc-worker_Linux_x86_64.tar.gz
tc-worker_Linux_arm64.tar.gz
checksums.txt
```

## Development Shortcut

From the repository root:

```sh
make worker
```

This keeps the same product contract as `tc-worker join`, but runs through `go run` for local development.

## Worker Contract

The worker does not wait for chat input after it starts. It registers with `tc-server`, advertises capabilities, polls for messages, and writes readback/checkpoint/artifact/completion records through the server.

Default setup values:

```text
server      http://127.0.0.1:8080
role        code-worker
capability  code.change,ai.review
permission  auto-approve
workspace   current directory
```

`permission=auto-approve` is intentional for trusted local AI-to-AI handoff demos. It prevents Claude/Codex/Gemini/Kiro from stalling on local permission prompts. Use this only inside a workspace you trust.

## Advanced Join

The long-form command remains supported for CI, automation, and explicit role assignment:

```sh
tc-worker join \
  --backend claude \
  --model 'opus[1m]' \
  --server http://127.0.0.1:8080 \
  --endpoint tc://endpoint/claude_worker \
  --role code-worker \
  --capabilities code.change,ai.review \
  --permission auto-approve
```

Useful backend presets:

```text
claude  claude -p --permission-mode bypassPermissions --model <model>
codex   codex exec --skip-git-repo-check --sandbox danger-full-access -c approval_policy="never" -
gemini  gemini -p {{prompt}} --approval-mode yolo
kiro    kiro-cli chat --no-interactive --trust-all-tools {{prompt}}
```

## Worker Cockpit

When attached to a terminal, `tc-worker join` opens the worker cockpit TUI. It shows the current message, worker status, readback, result, artifacts, and event log.

Keys:

```text
1..5             switch Body, Readback, Result, Artifacts, and Log tabs
tab / shift+tab  cycle worker tabs
j/k or arrows    scroll the active tab
enter            open the selected artifact
r                refresh snapshot now
?                show help
q                stop the worker
```

Use plain mode for background scripts:

```sh
tc-worker join --plain
```

## Legacy Environment Contract

Running `tc-worker` without a command keeps the legacy `TC_WORKER_*` environment contract for tests and old deployments.

Key env variables:

```text
TC_WORKER_SERVER_URL
TC_WORKER_ENDPOINT_REF
TC_WORKER_EXECUTOR
TC_WORKER_SKILLS_DIR
TC_WORKER_AI_CLI_COMMAND
TC_WORKER_AI_CLI_ARGS
TC_WORKER_AI_CLI_WORKDIR
TC_WORKER_ARTIFACT_DIR
```

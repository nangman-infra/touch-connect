# tc-worker

`tc-worker` is the execution endpoint runtime.

Responsibilities:

- register itself as an endpoint
- advertise capabilities
- receive or claim messages
- run local CLI, shell, process, or skill-backed work
- send readback, checkpoint, artifact, completion, and failure updates

It owns execution, not the source of truth.

Executor modes:

- default echo mode when no executor env is set
- command mode when `TC_WORKER_ALLOWED_COMMANDS` is set, or `TC_WORKER_EXECUTOR=command`
- local AI CLI mode with `TC_WORKER_EXECUTOR=ai-cli`
- LLM mode with `TC_WORKER_EXECUTOR=llm`
- skill mode with `TC_WORKER_EXECUTOR=skill`

Local AI CLI mode is the primary AI worker path. It passes the touch-connect execution context to an already authenticated CLI through stdin:

```sh
TC_WORKER_EXECUTOR=ai-cli \
TC_WORKER_AI_CLI_COMMAND=codex \
TC_WORKER_AI_CLI_ARGS='exec,--skip-git-repo-check,--sandbox,read-only,-c,approval_policy="never",-' \
TC_WORKER_AI_CLI_WORKDIR=/absolute/path/to/workspace \
TC_WORKER_SERVER_URL=http://127.0.0.1:8080 \
go run ./tc-worker/cmd/tc-worker
```

For Claude Code:

```sh
TC_WORKER_EXECUTOR=ai-cli \
TC_WORKER_AI_CLI_COMMAND=claude \
TC_WORKER_AI_CLI_ARGS=-p \
TC_WORKER_AI_CLI_WORKDIR=/absolute/path/to/workspace \
TC_WORKER_SERVER_URL=http://127.0.0.1:8080 \
go run ./tc-worker/cmd/tc-worker
```

LLM mode uses the OpenAI Responses-compatible HTTP contract:

```sh
TC_WORKER_EXECUTOR=llm \
TC_WORKER_CAPABILITIES=ai.generate \
TC_WORKER_LLM_API_KEY="$OPENAI_API_KEY" \
TC_WORKER_LLM_MODEL=gpt-5.4 \
TC_WORKER_SERVER_URL=http://127.0.0.1:8080 \
go run ./tc-worker/cmd/tc-worker
```

Optional LLM settings:

- `TC_WORKER_LLM_PROVIDER` defaults to `openai_responses`
- `TC_WORKER_LLM_BASE_URL` defaults to `https://api.openai.com/v1`
- `TC_WORKER_LLM_SYSTEM_PROMPT` sets the worker persona
- `TC_WORKER_LLM_TIMEOUT` defaults to `60s`
- `TC_WORKER_LLM_MAX_OUTPUT_TOKENS` limits the model output

Skill mode loads `SKILL.md` guidance and injects it into the backend executor context. Its default backend is local AI CLI, not a provider API:

```sh
tcctl skill register /absolute/path/to/SKILL.md

TC_WORKER_EXECUTOR=skill \
TC_WORKER_SKILL_BACKEND=ai-cli \
TC_WORKER_SKILL_REGISTRY="$HOME/.touch-connect/skills/registry.json" \
TC_WORKER_AI_CLI_COMMAND=codex \
TC_WORKER_AI_CLI_ARGS='exec,--skip-git-repo-check,--sandbox,read-only,-c,approval_policy="never",-' \
TC_WORKER_SERVER_URL=http://127.0.0.1:8080 \
go run ./tc-worker/cmd/tc-worker
```

Skill settings:

- `TC_WORKER_SKILL_REGISTRY` points at a local registry JSON written by `tcctl skill register`
- `TC_WORKER_SKILLS_DIR` loads every nested `SKILL.md` in an absolute directory
- `TC_WORKER_SKILL_BACKEND` is `ai-cli`, `llm`, `command`, or `echo`; default is `ai-cli`
- `TC_WORKER_CAPABILITIES` can narrow which registered skill capabilities this worker advertises

Detailed implementation docs are maintained as local living contracts and are intentionally not tracked in the public Git repository.

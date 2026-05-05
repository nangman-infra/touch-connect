COMPOSE_FILE ?= docker-compose.dev.yml
COMPOSE ?= docker compose -f $(COMPOSE_FILE)
TCCTL ?= go run ./tcctl/cmd/tcctl
CLAUDE_MODEL ?= opus[1m]

.PHONY: dev-up dev-down dev-logs dev-ps smoke host-codex-worker host-claude-worker host-gemini-worker

dev-up:
	$(COMPOSE) up -d --build nats tc-server tc-control tc-worker-echo

dev-down:
	$(COMPOSE) down

dev-logs:
	$(COMPOSE) logs -f nats tc-server tc-control tc-worker-echo

dev-ps:
	$(COMPOSE) ps

smoke:
	$(COMPOSE) run --rm tcctl endpoint list
	$(COMPOSE) run --rm tcctl message send --capability code.change --summary "compose smoke" --body "Verify compose echo worker can receive and complete a message." --quality-gate=skip

host-codex-worker:
	go run ./tc-worker/cmd/tc-worker join --backend codex --model gpt-5.4-mini --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_codex_worker

host-claude-worker:
	go run ./tc-worker/cmd/tc-worker join --backend claude --model "$(CLAUDE_MODEL)" --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_claude_worker

host-gemini-worker:
	go run ./tc-worker/cmd/tc-worker join --backend gemini --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_gemini_worker

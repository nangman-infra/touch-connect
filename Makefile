COMPOSE_FILE ?= docker-compose.dev.yml
COMPOSE ?= docker compose -f $(COMPOSE_FILE)
TC_DEV_DATA_DIR ?= $(CURDIR)/.touch-connect/dev
export TC_DEV_DATA_DIR
TCCTL ?= go run ./tcctl/cmd/tcctl
TCCTL_COMPOSE ?= $(COMPOSE) run --rm tcctl
CLAUDE_MODEL ?= opus[1m]
CAPABILITY ?= code.change
TASK_REF ?= tc://task/dev_demo
DEMO_SUMMARY ?= Manager requests worker result
DEMO_BODY_FILE ?= $(CURDIR)/examples/messages/dev-demo-body.md
WORKER_SERVER_URL ?= http://127.0.0.1:8080
WORKER_SUBCOMMAND := $(word 2,$(MAKECMDGOALS))

ifeq ($(firstword $(MAKECMDGOALS)),worker)
ifneq ($(WORKER_SUBCOMMAND),)
.PHONY: $(WORKER_SUBCOMMAND)
$(WORKER_SUBCOMMAND):
	@:
endif
endif

.PHONY: help dev dev-storage dev-up dev-down dev-logs dev-ps endpoint-list manager manager-watch monitor message-tail send-demo watch-demo history-demo smoke ensure-dev-server worker worker-join worker-setup host-codex-worker host-claude-worker host-gemini-worker

help:
	@echo "touch-connect development commands"
	@echo ""
	@echo "  make dev             foreground NATS + tc-server + tc-control"
	@echo "  make dev-storage     create project-local dev bind-mount directories"
	@echo "  make dev-up          detached NATS + tc-server + tc-control"
	@echo "  make dev-down        stop compose stack"
	@echo "  make dev-logs        follow server/control/NATS logs"
	@echo "  make dev-ps          show compose service status"
	@echo ""
	@echo "  make worker          start local stack if needed, then join a host local AI CLI worker"
	@echo "  make worker-setup    write or edit local worker config"
	@echo "  make manager         print one manager cockpit frame"
	@echo "  make manager-watch   watch manager cockpit for TASK_REF=$(TASK_REF)"
	@echo "  make endpoint-list   list registered endpoints"
	@echo "  make monitor         print one operator monitor frame"
	@echo "  make message-tail    stream messages for CAPABILITY=$(CAPABILITY)"
	@echo ""
	@echo "  make send-demo       send demo task TASK_REF=$(TASK_REF) DEMO_BODY_FILE=$(DEMO_BODY_FILE)"
	@echo "  make watch-demo      print current task flow for TASK_REF=$(TASK_REF)"
	@echo "  make history-demo    print task history for TASK_REF=$(TASK_REF)"
	@echo "  make smoke           run compose echo worker smoke path"

dev-storage:
	mkdir -p "$(TC_DEV_DATA_DIR)/server" "$(TC_DEV_DATA_DIR)/nats" "$(TC_DEV_DATA_DIR)/artifacts"
	chmod 0777 "$(TC_DEV_DATA_DIR)/server" "$(TC_DEV_DATA_DIR)/nats" "$(TC_DEV_DATA_DIR)/artifacts"

dev: dev-storage
	$(COMPOSE) up --build nats tc-server tc-control

dev-up: dev-storage
	$(COMPOSE) up -d --build nats tc-server tc-control

dev-down:
	$(COMPOSE) down

dev-logs:
	$(COMPOSE) logs -f nats tc-server tc-control

dev-ps:
	$(COMPOSE) ps

endpoint-list:
	$(TCCTL) endpoint list

manager:
	$(TCCTL) manager --once

manager-watch:
	$(TCCTL) manager --task "$(TASK_REF)" --watch

monitor:
	$(TCCTL) monitor --once

message-tail:
	$(TCCTL) message tail --capability $(CAPABILITY)

send-demo:
	$(TCCTL) manager --send --capability $(CAPABILITY) --summary "$(DEMO_SUMMARY)" --body-file "$(DEMO_BODY_FILE)" --task "$(TASK_REF)" --readback-required --quality-gate=warn --once

watch-demo:
	$(TCCTL) task watch "$(TASK_REF)" --once

history-demo:
	$(TCCTL) task history "$(TASK_REF)"

smoke: dev-storage
	$(COMPOSE) --profile smoke up -d --build nats tc-server tc-control tc-worker-echo
	$(COMPOSE) run --rm tcctl endpoint list
	$(COMPOSE) run --rm tcctl message send --capability code.change --summary "compose smoke" --body "Verify compose echo worker can receive and complete a message." --quality-gate=skip

ensure-dev-server:
	@if [ -n "$$TC_WORKER_SERVER_URL" ]; then \
		echo "using explicit TC_WORKER_SERVER_URL=$$TC_WORKER_SERVER_URL"; \
		exit 0; \
	fi; \
	if curl -fsS "$(WORKER_SERVER_URL)/healthz" >/dev/null 2>&1; then \
		echo "tc-server healthy at $(WORKER_SERVER_URL)"; \
		exit 0; \
	fi; \
	echo "tc-server is not healthy at $(WORKER_SERVER_URL); starting local dev stack..."; \
	$(MAKE) dev-storage; \
	$(COMPOSE) up -d --build nats tc-server tc-control; \
	i=0; \
	while [ $$i -lt 60 ]; do \
		if curl -fsS "$(WORKER_SERVER_URL)/healthz" >/dev/null 2>&1; then \
			echo "tc-server healthy at $(WORKER_SERVER_URL)"; \
			exit 0; \
		fi; \
		i=$$((i + 1)); \
		sleep 1; \
	done; \
	echo "tc-server did not become healthy at $(WORKER_SERVER_URL)"; \
	$(COMPOSE) ps; \
	exit 1

worker:
	@case "$(WORKER_SUBCOMMAND)" in \
		""|join) \
			$(MAKE) ensure-dev-server && go run ./tc-worker/cmd/tc-worker join --skills-dir $(CURDIR)/examples/skills ;; \
		setup) \
			$(MAKE) worker-setup ;; \
		*) \
			echo "unknown worker subcommand: $(WORKER_SUBCOMMAND)"; \
			echo "use: make worker, make worker join, or make worker setup"; \
			exit 2 ;; \
	esac

worker-join: worker

worker-setup:
	go run ./tc-worker/cmd/tc-worker setup --skills-dir $(CURDIR)/examples/skills

host-codex-worker: ensure-dev-server
	go run ./tc-worker/cmd/tc-worker join --backend codex --model gpt-5.4-mini --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_codex_worker --permission auto-approve

host-claude-worker: ensure-dev-server
	go run ./tc-worker/cmd/tc-worker join --backend claude --model "$(CLAUDE_MODEL)" --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_claude_worker --permission auto-approve

host-gemini-worker: ensure-dev-server
	go run ./tc-worker/cmd/tc-worker join --backend gemini --skills-dir $(CURDIR)/examples/skills --endpoint-ref tc://endpoint/host_gemini_worker --permission auto-approve

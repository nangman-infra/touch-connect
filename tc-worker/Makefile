ROOT_DIR := $(abspath ..)
WORKER_SUBCOMMAND := $(word 2,$(MAKECMDGOALS))

.PHONY: help worker join setup worker-setup host-codex-worker host-claude-worker host-gemini-worker

help:
	$(MAKE) -C $(ROOT_DIR) help

worker:
	@case "$(WORKER_SUBCOMMAND)" in \
		""|join) $(MAKE) -C $(ROOT_DIR) worker ;; \
		setup) $(MAKE) -C $(ROOT_DIR) worker-setup ;; \
		*) \
			echo "unknown worker subcommand: $(WORKER_SUBCOMMAND)"; \
			echo "use: make worker, make worker join, or make worker setup"; \
			exit 2 ;; \
	esac

join:
ifeq ($(firstword $(MAKECMDGOALS)),worker)
	@:
else
	$(MAKE) -C $(ROOT_DIR) worker
endif

setup:
ifeq ($(firstword $(MAKECMDGOALS)),worker)
	@:
else
	$(MAKE) -C $(ROOT_DIR) worker-setup
endif

worker-setup:
	$(MAKE) -C $(ROOT_DIR) worker-setup

host-codex-worker:
	$(MAKE) -C $(ROOT_DIR) host-codex-worker

host-claude-worker:
	$(MAKE) -C $(ROOT_DIR) host-claude-worker

host-gemini-worker:
	$(MAKE) -C $(ROOT_DIR) host-gemini-worker

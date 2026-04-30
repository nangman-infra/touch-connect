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

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)

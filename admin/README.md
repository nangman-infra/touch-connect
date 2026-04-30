# admin

`admin` is the touch-connect web admin frontend.

Responsibilities:

- show operational state for endpoints, tasks, messages, artifacts, approvals, and DLQ records
- provide human workflows for approval decisions, retry, cancel, artifact finalization, and DLQ replay
- call `tc-control` as its only backend API
- render server-accepted state without creating hidden local truth

It does not talk directly to `tc-server`, `tc-worker`, NATS/JetStream, or storage.

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)

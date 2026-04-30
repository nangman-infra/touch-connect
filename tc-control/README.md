# tc-control

`tc-control` is the touch-connect control plane backend.

Responsibilities:

- expose the Control API used by `tcctl` and `admin`
- provide read models for endpoints, tasks, messages, artifacts, approvals, and DLQ records
- accept operator commands for task creation, message send, approval decisions, retry, cancel, artifact finalization, and DLQ replay
- validate control-plane authorization and audit every mutation
- forward accepted data-plane commands to `tc-server` through an explicit command path

It does not own message hot-path routing, worker execution, NATS/JetStream transport internals, or the web UI.

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)

# tc-server

`tc-server` is the touch-connect message routing and delivery data plane.

Responsibilities:

- endpoint and capability runtime registry
- message ingress validation
- capability-first and direct routing
- delivery records, attempts, checkpoints, and DLQ event creation
- side effect execution ledger writes on the runtime path
- NATS/Core and JetStream integration
- worker-facing registration, heartbeat, claim, lease, checkpoint, completion, and failure APIs

It does not own:

- operator/admin Control API
- admin web UI
- `tcctl` command handling
- approval decision authority
- retry, replay, cancel, or inspection UX
- local shell execution
- endpoint-internal skill selection

Detailed implementation docs:

- [docs/implementation-contract.md](docs/implementation-contract.md)
- [docs/definition-of-done.md](docs/definition-of-done.md)
- [docs/implementation-task-list.md](docs/implementation-task-list.md)

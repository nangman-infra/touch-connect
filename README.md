# touch-connect

`touch-connect` is a message-quality and handoff-governance layer for heterogeneous AI agents, compatible with A2A and AGNTCY.

A2A and AGNTCY move agent messages across systems. `touch-connect` makes those handoffs sufficient, correctly understood, approval-aware, and auditable.

It is not a queue, transport, workflow engine, or new wire protocol. Production transport, durability, and replay belong behind adapters such as NATS JetStream, Temporal, A2A, and AGNTCY-compatible bindings. The built-in memory and SQLite paths exist for local development and tests.

The project is still contract-first, but living contract docs are maintained locally and are intentionally not tracked in the public Git repository.

Root app units:

- `tc-server`
  - message governance records and adapter-backed delivery data plane
- `tc-control`
  - control plane backend API for `tcctl` and `admin`
- `tc-worker`
  - execution endpoint runtime
- `tcctl`
  - operator/admin CLI
- `admin`
  - web admin frontend

Runtime relationship:

```text
tc-worker -> tc-server
tcctl    -> tc-control
admin    -> tc-control
tc-control -> tc-server control command path
tc-server  -> NATS/JetStream + Temporal + A2A/AGNTCY adapters
```

`tc-server` must stay focused on accepted message records, quality policy enforcement, governance state, and adapter-backed dispatch.
Operator APIs, admin workflows, approvals, retries, DLQ replay, and inspection belong to `tc-control`.

Docker Compose can later bind these as separate services:

- `tc-server`
- `tc-control`
- `tc-worker`
- `admin`
- `nats`
- selected storage adapter

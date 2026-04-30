# touch-connect

`touch-connect` is a TCP/IP-like AI communication layer for fast, reliable message handoff between AI endpoints.

The project is currently docs-first. Start here:

- [docs/README.md](docs/README.md)
  - active product, contract, engineering, and governance documents

Root app units:

- `tc-server`
  - message routing and delivery data plane
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
tc-server  -> NATS/JetStream + storage adapters
```

`tc-server` must stay focused on low-latency routing and delivery.
Operator APIs, admin workflows, approvals, retries, DLQ replay, and inspection belong to `tc-control`.

Docker Compose can later bind these as separate services:

- `tc-server`
- `tc-control`
- `tc-worker`
- `admin`
- `nats`
- selected storage adapter

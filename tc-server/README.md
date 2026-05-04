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

Detailed implementation docs are maintained as local living contracts and are intentionally not tracked in the public Git repository.

## Local Run

```text
tc-server -h
tc-server -bind 127.0.0.1:8080 -storage memory
tc-server -storage sqlite -sqlite-path /absolute/path/to/touch-connect.sqlite
```

The same settings are available through `TC_SERVER_BIND_ADDR`, `TC_SERVER_STORAGE`, and `TC_SERVER_SQLITE_PATH`.

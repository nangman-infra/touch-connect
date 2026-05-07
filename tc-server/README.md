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

Compose is the default standalone local path:

```text
docker compose -f docker-compose.dev.yml up -d --build
docker compose -f docker-compose.dev.yml run --rm tcctl endpoint list
```

Manual process mode is still useful while developing one component:

```text
tc-server -h
tc-server -bind 127.0.0.1:8080 -storage memory
tc-server -storage sqlite -sqlite-path /absolute/path/to/touch-connect.sqlite
```

The same settings are available through `TC_SERVER_BIND_ADDR`, `TC_SERVER_STORAGE`, and `TC_SERVER_SQLITE_PATH`.

## Local Discovery

`tc-server` advertises itself on the local network with mDNS/Bonjour by default:

```text
service  _touch-connect._tcp.local.
txt      component=tc-server, version=<server-version>, url=<optional reachable URL>
```

Worker join uses this advertisement before falling back to localhost and LAN `/healthz` probes.

Useful flags and environment variables:

```text
tc-server -discovery=false
tc-server -discovery-name touch-connect-office
tc-server -advertise-url http://192.168.10.34:8080

TC_SERVER_DISCOVERY=false
TC_SERVER_DISCOVERY_NAME=touch-connect-office
TC_SERVER_ADVERTISE_URL=http://192.168.10.34:8080
```

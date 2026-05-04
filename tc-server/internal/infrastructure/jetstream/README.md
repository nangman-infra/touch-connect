# JetStream Adapter Contract

This directory contains the production NATS JetStream adapter path.

The adapter must implement transport-facing behavior without changing public
`tc://...` refs or application ledger semantics.

## Port Mapping

| touch-connect port | JetStream responsibility |
| --- | --- |
| `MessageLedger` | Persist logical message state outside broker-native sequence ids. |
| `ProcessingLedger` | Own domain claim, lease, attempt, checkpoint, and expired-claim reconciliation. |
| `ReadbackLedger` | Persist readback evidence as handoff quality data, not broker ack data. |
| `ArtifactLedger` | Persist artifact version/finalization and lineage metadata outside stream retention. |
| `GovernanceLedger` | Persist approval decisions and protected side effect execution records. |
| `QualityLedger` | Persist append-only PhraseologyPolicy decisions outside broker delivery state. |
| `ProjectionReader` | Build operator-facing read models from ledgers and adapter metadata. |
| `RefAllocator` | Create public `tc://...` refs; never use stream sequence as public identity. |
| `DeliveryAdapter` | Publish/fetch/ack/nak JetStream messages and preserve adapter metadata. |

## JetStream Mapping

| JetStream feature | Adapter rule |
| --- | --- |
| `Nats-Msg-Id` | Use for publish dedupe. It does not prove side effect exactly-once. |
| `WorkQueuePolicy` | Use for work distribution. It does not replace task history. |
| pull consumer fetch | Map to `DeliveryAdapter.FetchNextDelivery`, then call `ProcessingLedger` for domain claim. |
| `AckExplicit` | Ack only after terminal domain checkpoint and message state update succeed. |
| `AckWait` | Feed lease/reconcile decisions; do not expose it as a domain state. |
| `MaxDeliver` | Preserve as adapter redelivery metadata and map into `max_redelivery` policy. |

## Subject Convention

```text
tc.messages.<capability>
tc.events.attempt.<state>
tc.events.governance.<kind>
tc.events.artifact.<kind>
```

## Metadata Convention

```text
tc_message_ref
tc_delivery_ref
tc_attempt_ref
tc_correlation_ref
tc_capability
adapter_stream
adapter_consumer
adapter_stream_seq
adapter_consumer_seq
```

Adapter metadata is for troubleshooting, replay input, and trace correlation.
It must not replace `message_ref`, `attempt_ref`, `correlation_ref`, approval
refs, or artifact version refs in API responses or CLI output.

## Current Implementation

Implemented:

- `Config`
- `Adapter`
- `NewAdapter`
- stream creation with `WorkQueuePolicy`
- `PublishAcceptedMessage`
- durable pull consumer creation with `AckExplicit`, `AckWait`, and `MaxDeliver`
- `FetchNextDelivery`
- `AckDelivery`
- `NakDelivery`
- publish dedupe through `Nats-Msg-Id`
- adapter metadata receipt for stream, consumer, sequence, subject, duplicate status, and redelivery count
- optional service bridge through `NewServerWithPortsAndDeliveryAdapter`
- `IngressMessage` publish to `DeliveryAdapter`
- `ClaimNextMessage` delivery fetch followed by `ProcessingLedger.ClaimMessage`
- terminal checkpoint `AckDelivery` after domain checkpoint and message state update

Not connected yet:

- concrete `tc-server` config profile that instantiates this JetStream adapter
- non-terminal blocked/retrying checkpoint broker policy

Current W2 bootstrap command:

```bash
docker compose -f docker-compose.dev.yml up -d nats
NATS_URL=nats://127.0.0.1:4222 NATS_MONITOR_URL=http://127.0.0.1:8222 go test -tags=integration,jetstream ./tests
```

The bootstrap integration test only proves that local dev NATS has JetStream
enabled. It intentionally does not claim that the production adapter is
implemented.

The adapter lifecycle integration test is:

```bash
NATS_URL=nats://127.0.0.1:4222 go test -tags=integration,jetstream ./tc-server/internal/infrastructure/jetstream
```

It verifies stream creation, message publish, duplicate publish detection,
stored metadata headers, pull consumer fetch, ack removal, and nak redelivery.

The fetched JetStream message is tracked in process by `delivery_ref` until
`AckDelivery` or `NakDelivery` is called. That pending map only binds broker
ack state to the public delivery ref; it is not task history, attempt state, or
lineage storage.

The application service bridge is optional. Local memory and SQLite servers keep
their existing dev/test path when no `DeliveryAdapter` is provided.

## Sources

- NATS JetStream: https://docs.nats.io/nats-concepts/jetstream
- NATS JetStream streams: https://docs.nats.io/nats-concepts/jetstream/streams
- NATS JetStream consumers: https://docs.nats.io/nats-concepts/jetstream/consumers
- NATS JetStream headers: https://docs.nats.io/nats-concepts/jetstream/headers
- NATS Go client: https://github.com/nats-io/nats.go

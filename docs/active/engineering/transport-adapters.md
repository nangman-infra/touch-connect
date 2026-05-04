> Document Status: active
> Document Type: engineering-baseline
> Scope: JetStream, Temporal, A2A, AGNTCY adapter 작성 규약과 Store port split 적용 기준
> Canonical Path: `docs/active/engineering/transport-adapters.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-05
> Supersedes: `none`
> Superseded By: `none`

# Transport Adapters

## 목적

이 문서는 `touch-connect`가 production 경로에서 자체 queue 구현에 시간을 쓰지 않도록 adapter 경계를 고정한다.

한 줄 결정:

`touch-connect`는 message quality와 handoff governance를 제품 표면으로 삼고, transport, durability, replay는 NATS JetStream, Temporal, A2A, AGNTCY-compatible adapter 뒤로 보낸다.

## Adapter 원칙

### 1. Domain contract를 오염시키지 않는다

adapter는 infrastructure primitive를 domain method signature에 흘리면 안 된다.

금지 예:

- JetStream stream sequence를 domain message revision으로 사용
- NATS subject를 routing domain key로 노출
- Temporal workflow run id를 task identity로 대체
- SQLite transaction boundary를 application store contract에 노출
- A2A wire field를 내부 approval state name으로 직접 사용

### 2. Business reliability는 adapter 기능이 아니라 ledger로 검증한다

JetStream이나 Temporal이 강한 durability를 제공해도 아래는 `touch-connect` ledger가 판단한다.

- readback 완료 여부
- approval decision 여부
- protected side effect exactly-once
- artifact lineage 연결 여부
- final task completion 여부

### 3. Dev/test store와 production adapter를 분리한다

memory와 SQLite 기반 store는 local dev/test 및 deterministic integration test 용도다.

규칙:

- production profile의 default는 memory/SQLite가 아니다.
- memory/SQLite store에는 새 production capability를 추가하지 않는다.
- memory/SQLite store에는 버그 fix와 contract conformance test만 허용한다.
- production adapter가 없을 때 local 개발은 explicit dev/test profile로만 실행한다.

## Adapter별 책임

| Adapter | 위치 | 책임 |
| --- | --- | --- |
| JetStream | `tc-server/internal/infrastructure/jetstream/` | production work distribution, retention, dedupe, ack mapping |
| Temporal | `tc-server/internal/infrastructure/temporal/` | long-running task, durable execution, retry policy mapping |
| A2A | `tc-server/internal/infrastructure/a2a/` | A2A 1.0 Task/Message/Artifact inbound/outbound mapping |
| AGNTCY-compatible | `tc-server/internal/infrastructure/agntcy/` | maintained AGNTCY-compatible protocol binding |
| memory | `tc-server/internal/infrastructure/memory/` | local dev/test deterministic store |
| SQLite | `tc-server/internal/infrastructure/sqlite/` | local dev/test durable-ish store |

`agntcy/acp-spec` GitHub repository는 2026-04-11 archived 되었으므로, AGNTCY-compatible adapter는 현재 maintained specification을 확인한 뒤 exact target을 확정한다.

## JetStream Mapping

| touch-connect concept | JetStream concept |
| --- | --- |
| message publish dedupe | `Nats-Msg-Id` |
| processing work queue | work-queue retention stream |
| worker claim-next | pull consumer fetch |
| ack | explicit consumer ack |
| redelivery | ack wait and max deliver |
| replay | stream replay or projection rebuild |
| duplicate protection | idempotency key plus execution ledger |

규칙:

- `Nats-Msg-Id`는 publish dedupe를 돕지만 protected side effect exactly-once를 대체하지 않는다.
- consumer ack는 `attempt completed`가 아니라 adapter-level processing acknowledgement다.
- work-queue retention을 쓰더라도 task history와 artifact lineage는 별도 ledger에 남긴다.

### JetStream adapter v0 design

`tc-server/internal/infrastructure/jetstream/` adapter는 transport/durability/replay를 맡고, message quality와 governance 판단은 application ledgers에 남긴다.

| JetStream setting or signal | v0 adapter mapping | 금지되는 해석 |
| --- | --- | --- |
| `Nats-Msg-Id` | `message_ref` 또는 explicit idempotency key를 publish dedupe header로 보낸다. | `Nats-Msg-Id`를 protected side effect exactly-once 증거로 쓰지 않는다. |
| `WorkQueuePolicy` | dispatch stream은 work queue semantics로 두고 worker가 한 번씩 fetch한다. | stream retention을 task history source of truth로 쓰지 않는다. |
| pull consumer fetch | `DeliveryAdapter.FetchNextDelivery` 후보가 된다. | fetch 성공을 domain claim 성공으로 간주하지 않는다. |
| `AckExplicit` | attempt가 terminal checkpoint로 닫힌 뒤 delivery ack를 보낸다. | ack를 readback/approval/completion으로 해석하지 않는다. |
| `AckWait` | lease expiry/reconcile의 adapter input으로 사용한다. | `AckWait` 이름을 domain state로 노출하지 않는다. |
| `MaxDeliver` | adapter redelivery metadata로 보존하고 `max_redelivery` 정책과 매핑한다. | broker max deliver만으로 business DLQ를 확정하지 않는다. |

v0 subject convention:

```text
tc.messages.<capability>
tc.events.attempt.<state>
tc.events.governance.<kind>
tc.events.artifact.<kind>
```

v0 metadata convention:

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

adapter metadata는 troubleshooting과 replay input으로만 사용한다.
public contract와 CLI output은 계속 `tc://...` refs를 기준으로 한다.

## Temporal Mapping

| touch-connect concept | Temporal concept |
| --- | --- |
| long-running task | workflow |
| attempt execution | activity or child workflow |
| retry policy | workflow/activity retry policy |
| checkpoint | workflow event or activity heartbeat projection |
| takeover | workflow recovery plus touch-connect claim epoch |
| observability | workflow history plus OpenTelemetry span |

규칙:

- Temporal workflow id는 `task_ref`를 대체하지 않는다.
- Temporal retry success는 approval success가 아니다.
- side effect uniqueness는 `idempotency_key + protected_scope` execution ledger로 검증한다.

## A2A Mapping

| touch-connect concept | A2A concept |
| --- | --- |
| task_ref | Task id or metadata mapped ref |
| message_ref | Message metadata mapped ref |
| artifact version | Artifact plus metadata version |
| checkpoint | Task status update |
| readback | Message or task status requiring explicit confirmation |
| correlation graph | Task, message, artifact metadata edges |

규칙:

- inbound A2A client message는 PhraseologyPolicy를 거친 뒤 internal message로 accept한다.
- outbound dispatch는 A2A Task/Message/Artifact surface에 `tc://...` refs를 metadata로 보존한다.
- A2A history는 recovery input일 수 있지만 ApprovalChain과 ArtifactLineage source of truth를 대체하지 않는다.

## Store Port Split Reference

현재 `tc-server/internal/application/store.go`의 `Store`는 dev/test composite marker다.
production adapter 전환 기준은 `Store`가 아니라 `NewServerWithPorts`의 개별 ports다.
구체적인 target ports, current method mapping, migration order, acceptance criteria는 [store-port-split-contract.md](docs/active/contracts/store-port-split-contract.md)를 따른다.

현재 좋은 점:

- `Service`가 `Store` field 없이 `EndpointRegistry`, `MessageLedger`, `ProcessingLedger`, `ReadbackLedger`, `ArtifactLedger`, `GovernanceLedger`, `RefAllocator`, `ProjectionReader`만 사용한다.
- `NewServerWithPorts`가 production adapter 주입 기준점이다.
- `tests/server_adapter_contract_test.go`가 memory/SQLite를 같은 behavior contract로 검증한다.

정제해야 할 지점:

- `ClaimMessage`와 `ClaimNextMessage`는 transport ack와 domain claim을 분리하는 `DeliveryAdapter` plus `ProcessingLedger` 경계로 쪼개야 한다.
- memory/SQLite는 한 struct가 모든 port를 제공하므로 production adapter 구현 전에 port별 contract tests를 유지해야 한다.
- JetStream ack와 attempt terminal checkpoint의 연결 순서를 명시해야 한다.

권장 분리:

```text
EndpointRegistry
MessageLedger
ProcessingLedger
ReadbackLedger
ArtifactLedger
GovernanceLedger
ProjectionReader
RefAllocator
DeliveryAdapter
```

M1 W1의 완료 기준은 위 분리를 코드에 한 번에 전부 적용하는 것이 아니라, 현재 통합테스트가 adapter interface 위에서 같은 의미로 통과할 수 있도록 SQLite/memory 전용 가정이 어디에 있는지 제거하는 것이다.

## Test Matrix

M1부터 test matrix는 adapter별로 분리한다.

```bash
go test ./...
go test -tags devstore ./...
NATS_URL=nats://127.0.0.1:4222 NATS_MONITOR_URL=http://127.0.0.1:8222 go test -tags=integration,jetstream ./tests
```

규칙:

- `go test ./...`는 external service 없이 빠르게 돌아야 한다.
- `devstore` tag는 memory/SQLite adapter conformance를 명시한다.
- `integration,jetstream` tag는 docker-compose dev NATS를 요구한다.
- Temporal과 A2A reference client 검증은 별도 integration job으로 둔다.

## Docker Compose Dev Baseline

M1에서 `docker-compose.dev.yml`은 최소 아래 service를 제공해야 한다.

- `nats`
- `jaeger`
- `tc-server`
- `tc-control`
- `tc-worker`

`nats`는 JetStream enabled 상태로 띄운다.
`jaeger`는 send, claim, execute, ack 전체 trace를 확인하기 위한 local observability target이다.

현재 W2 시작점:

```bash
docker compose -f docker-compose.dev.yml up -d nats
NATS_URL=nats://127.0.0.1:4222 NATS_MONITOR_URL=http://127.0.0.1:8222 go test -tags=integration,jetstream ./tests
```

`docker-compose.dev.yml`의 `nats` service는 official `nats` image를 `-js`로 실행하고 `-sd /data`로 JetStream data directory를 명시한다.
`tests/jetstream_integration_test.go`는 `/jsz` monitor endpoint를 확인해 JetStream이 켜져 있는지만 검증한다.
concrete JetStream adapter behavior는 다음 PR에서 `DeliveryAdapter` 구현과 함께 추가한다.

## Stop Doing Enforcement

아래 작업은 새 이슈나 PR에서 중단 대상이다.

- memory/SQLite store에 새 production behavior 추가
- 독자 wire protocol 공개
- tc-worker를 여러 공식 runtime으로 키우기
- MCP server를 직접 hosting하는 방향
- transport ownership을 암시하는 wide infrastructure positioning

## Related Docs

- [touch-connect-product-definition.md](docs/active/product/touch-connect-product-definition.md)
- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [message-quality-policy.md](docs/active/contracts/message-quality-policy.md)
- [store-port-split-contract.md](docs/active/contracts/store-port-split-contract.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)

## Sources

- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream
- NATS JetStream Streams
  - https://docs.nats.io/nats-concepts/jetstream/streams
- NATS JetStream Consumers
  - https://docs.nats.io/nats-concepts/jetstream/consumers
- NATS JetStream Headers
  - https://docs.nats.io/nats-concepts/jetstream/headers
- A2A latest specification
  - https://a2a-protocol.org/latest/specification/
- Linux Foundation A2A 2026 production adoption update
  - https://www.linuxfoundation.org/press/a2a-protocol-surpasses-150-organizations-lands-in-major-cloud-platforms-and-sees-enterprise-production-use-in-first-year
- Temporal Series D / Durable Execution
  - https://temporal.io/news/temporal-raises-300M-to-make-agentic-ai-real-for-companies
- AGNTCY ACP archived repository
  - https://github.com/agntcy/acp-spec

> Document Status: active
> Document Type: contract-model
> Scope: tc-server application store를 transport-agnostic ports로 분리하는 계약
> Canonical Path: `docs/active/contracts/store-port-split-contract.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-05
> Supersedes: `none`
> Superseded By: `none`

# Store Port Split Contract

## 목적

이 문서는 `tc-server/internal/application/store.go`의 넓은 `Store` interface를 production adapter 전환에 맞게 분리하는 계약이다.

핵심 목표:

- application layer가 NATS JetStream, Temporal, A2A, SQLite 같은 infrastructure primitive를 모르도록 한다.
- memory/SQLite는 dev/test conformance adapter로 낮추고 production capability surface가 되지 않게 한다.
- JetStream adapter가 들어와도 domain identity, claim, attempt, approval, artifact lineage의 의미가 흔들리지 않게 한다.
- 다음 구현자가 어떤 port부터 쪼개야 하는지 알 수 있게 한다.

## 현재 문제

현재 `Store` interface는 production write contract가 아니라 dev/test composite marker다.

`Service`와 `NewServerWithPorts`는 아래 개별 port를 직접 받는다.

```text
EndpointRegistry
MessageLedger
ProcessingLedger
ReadbackLedger
ArtifactLedger
GovernanceLedger
RefAllocator
ProjectionReader
```

남은 문제는 concrete memory/SQLite store가 아직 모든 port를 한 struct에서 구현한다는 점이다.
이는 local dev/test conformance adapter로는 허용하지만 production adapter의 기본 형태가 되면 안 된다.

분리 전 넓은 `Store` interface는 빠른 e2e 검증에는 유용했지만 production adapter 계약으로는 너무 넓었다.

현재 포함된 책임:

```text
endpoint registry
message ledger
claim and lease
attempt ledger
checkpoint ledger
readback ledger
artifact ledger
approval ledger
side effect ledger
expired claim reconciliation
ref allocation
snapshot projection
```

문제는 세 가지다.

1. concrete memory/SQLite store가 여전히 모든 port를 구현하므로 production adapter 단위 테스트가 없으면 다시 넓어질 수 있다.
2. `ClaimMessage`와 `ClaimNextMessage`는 아직 domain claim과 adapter fetch 분리 전 단계다.
3. JetStream consumer ack와 domain attempt completion의 연결 지점이 아직 DeliveryAdapter로 코드화되지 않았다.

이 상태로 JetStream adapter를 붙이면 JetStream consumer ack, stream sequence, durable consumer state가 domain store로 새어 들어갈 가능성이 크다.

## 분리 원칙

### 1. Port는 business meaning을 소유한다

port 이름은 broker 기능이 아니라 `touch-connect` domain 의미를 따른다.

허용:

- `MessageLedger`
- `ProcessingLedger`
- `GovernanceLedger`

금지:

- `JetStreamStore`
- `SQLiteClaimStore`
- `TemporalTaskStore`

infrastructure 이름은 adapter package 안에만 둔다.

### 2. Ref는 adapter id가 아니다

`tc://message/msg_...`, `tc://attempt/att_...`, `tc://artifact/art_...`는 touch-connect identity다.

금지:

- JetStream stream sequence를 `message_ref`로 사용
- Temporal workflow id를 `task_ref`로 대체
- A2A task id를 내부 task id로 직접 덮어쓰기
- SQLite row id를 public ref로 노출

adapter-native id는 metadata mapping에만 저장한다.

### 3. Delivery ack와 processing completion은 분리한다

JetStream consumer ack, Temporal activity completion, A2A accepted status는 모두 adapter-level signal이다.

이 signal은 아래와 같지 않다.

- readback 완료
- attempt completed
- approval approved
- protected side effect succeeded
- task completed

business state는 ledger record로 판단한다.

### 4. Projection은 write ledger가 아니다

operator view와 `tcctl inspect`를 위한 aggregate read는 `ProjectionReader`가 맡는다.

write ledger는 append/update invariant를 지킨다.
projection은 query shape를 만든다.

## Target Ports

### EndpointRegistry

endpoint 등록과 capability advertisement를 소유한다.

```text
SaveEndpoint(endpoint)
GetEndpoint(endpoint_ref)
UpdateCapabilities(endpoint_ref, capabilities)
UpdateEndpoint(endpoint)
CapabilityEndpoints(capability_claim)
```

규칙:

- capability matching은 `CapabilityClaim` 기준이다.
- 현재 `target_capability` string은 v0 projection으로만 받는다.
- endpoint 내부 skill, prompt, credential은 registry에 저장하지 않는다.

### MessageLedger

message identity와 immutable message event를 소유한다.

```text
SaveMessage(message)
GetMessage(message_ref)
UpdateMessageProjection(message)
AppendQualityDecision(decision)
```

규칙:

- original message body는 수정하지 않는다.
- state projection update는 허용하되 message payload mutation은 금지한다.
- quality decision은 append-only record다.

### ProcessingLedger

claim, lease, attempt, checkpoint를 소유한다.

```text
ClaimMessage(claim_request)
ClaimNextMessage(claim_next_request)
SaveAttempt(attempt)
GetAttempt(attempt_ref)
UpdateAttempt(attempt)
SaveCheckpoint(checkpoint)
ReconcileExpiredClaims(now)
```

규칙:

- claim unit은 message다.
- attempt는 endpoint가 message를 한 번 맡은 실행 단위다.
- `ReconcileExpiredClaims`는 production에서는 adapter timer나 processing service가 호출한다.
- adapter ack timeout은 expired claim reconciliation의 input일 수 있지만 domain state name이 되면 안 된다.

### ReadbackLedger

receiver understanding evidence를 소유한다.

```text
SaveReadback(readback)
```

규칙:

- readback은 processing checkpoint가 아니라 handoff quality evidence다.
- readback 저장은 ack, claim, completion을 뜻하지 않는다.
- PhraseologyPolicy와 연결될 수 있지만 transport adapter ack로 대체하면 안 된다.

### ArtifactLedger

artifact version과 finalization을 소유한다.

```text
SaveArtifactVersion(version)
GetArtifactVersion(artifact_version_ref)
SaveArtifactFinalization(finalization)
GetArtifactFinalization(artifact_version_ref)
AppendArtifactLineage(edge)
```

규칙:

- message는 logical artifact가 아니라 exact artifact version을 참조한다.
- artifact lineage edge는 `parent_version`, `derived_from`, `supersedes`를 표현한다.
- storage backend path나 object key는 `storage_ref`로만 보관한다.

### GovernanceLedger

approval, identity binding, approval chain, protected side effect execution을 소유한다.

```text
SaveApprovalDecision(decision)
GetApprovalDecision(approval_ref)
SaveSideEffectExecution(execution)
GetSideEffectExecution(execution_ref)
UpdateSideEffectExecution(execution)
AppendApprovalChainEvent(event)
GetApprovalChain(ref)
```

규칙:

- approval은 실행 허가이지 side effect 결과가 아니다.
- scope widening은 재승인 대상이다.
- scope narrowing은 approval chain re-binding으로 처리할 수 있다.
- `idempotency_key + protected_scope`가 uniqueness boundary다.
- execution record 없이 외부 side effect를 시작하면 contract violation이다.
- unknown external result는 success로 dedupe하지 않는다.

### ProjectionReader

control-plane read model을 소유한다.

```text
Snapshot()
InspectMessage(message_ref)
ListMessages(filter)
TaskHistory(task_ref)
ArtifactLineage(artifact_ref)
ApprovalChain(approval_ref)
```

규칙:

- projection은 source of truth가 아니다.
- projection은 ledger state를 읽기 쉽게 조합한다.
- projection method는 production write adapter의 필수 method가 아니다.

### RefAllocator

touch-connect public ref 생성을 소유한다.

```text
NextRef(kind)
ReserveRef(kind, idempotency_key)
```

규칙:

- public ref는 adapter-native id와 분리한다.
- kind별 sequence나 UUID 전략은 RefAllocator 구현 세부사항이다.
- empty polling은 ref를 소비하면 안 된다.

### DeliveryAdapter

external transport와 domain ledger 사이의 mapping을 소유한다.

```text
PublishAcceptedMessage(message)
FetchNextDelivery(endpoint_ref, capability_claim)
AckDelivery(delivery_ref)
NakDelivery(delivery_ref, reason)
MapAdapterRedelivery(delivery_ref, adapter_metadata)
```

규칙:

- DeliveryAdapter는 broker ack를 수행하지만 attempt completion을 선언하지 않는다.
- DeliveryAdapter는 adapter metadata를 보존하되 domain identity를 대체하지 않는다.
- adapter redelivery는 ProcessingLedger의 retry/takeover rule로 매핑한다.

## Current Method Mapping

현재 `tc-server/internal/application/store.go` method는 아래 target port로 이동한다.

| Current method | Target port | Migration note |
| --- | --- | --- |
| `SaveEndpoint` | `EndpointRegistry` | 그대로 이동 |
| `GetEndpoint` | `EndpointRegistry` | 그대로 이동 |
| `UpdateCapabilities` | `EndpointRegistry` | `CapabilityClaim` matching 준비 |
| `UpdateEndpoint` | `EndpointRegistry` | 그대로 이동 |
| `CapabilityEndpoints` | `EndpointRegistry` | string에서 claim matching으로 확장 |
| `SaveMessage` | `MessageLedger` | quality decision과 분리 |
| `GetMessage` | `MessageLedger` | 그대로 이동 |
| `UpdateMessage` | `MessageLedger` | `UpdateMessageProjection`으로 이름 축소 |
| `ClaimMessage` | `ProcessingLedger` | DeliveryAdapter fetch와 분리 |
| `ClaimNextMessage` | `ProcessingLedger` + `DeliveryAdapter` | adapter fetch 후 domain claim |
| `SaveAttempt` | `ProcessingLedger` | 그대로 이동 |
| `GetAttempt` | `ProcessingLedger` | 그대로 이동 |
| `UpdateAttempt` | `ProcessingLedger` | 그대로 이동 |
| `SaveCheckpoint` | `ProcessingLedger` | 그대로 이동 |
| `SaveReadback` | `ReadbackLedger` | handoff quality evidence로 분리 |
| `SaveArtifactVersion` | `ArtifactLedger` | lineage edge 추가 필요 |
| `GetArtifactVersion` | `ArtifactLedger` | 그대로 이동 |
| `SaveArtifactFinalization` | `ArtifactLedger` | 그대로 이동 |
| `GetArtifactFinalization` | `ArtifactLedger` | 그대로 이동 |
| `SaveApprovalDecision` | `GovernanceLedger` | ApprovalChain event 추가 필요 |
| `GetApprovalDecision` | `GovernanceLedger` | 그대로 이동 |
| `SaveSideEffectExecution` | `GovernanceLedger` | approval-enforced execution ledger |
| `GetSideEffectExecution` | `GovernanceLedger` | 그대로 이동 |
| `UpdateSideEffectExecution` | `GovernanceLedger` | 그대로 이동 |
| `ReconcileExpiredClaims` | `ProcessingLedger` service | production timer에서 호출 |
| `NextRef` | `RefAllocator` | Service는 RefAllocator만 사용, Store는 dev/test composite marker |
| `Snapshot` | `ProjectionReader` | Service는 ProjectionReader만 사용, Store는 dev/test composite marker |

## Migration Order

구현은 아래 순서로 진행한다.

### Step 1. RefAllocator 추출

가장 작고 위험이 낮다.

변경 기준:

- `Service`가 `store.NextRef` 대신 `refs.NextRef`를 호출한다.
- memory/SQLite ref 생성 구현은 기존 동작을 유지한다.
- empty `claim-next`가 ref를 소비하지 않는 현재 회귀 테스트를 유지한다.

### Step 2. ProjectionReader 추출

control-plane 조회와 write ledger를 분리한다.

변경 기준:

- `Snapshot()`은 `ProjectionReader`로 이동한다.
- `tc-control`과 `tcctl inspect/list/history` 경로는 projection을 읽는다.
- write path는 projection shape를 몰라야 한다.

### Step 3. Ledger ports 분리

domain aggregate별 port를 만든다.

변경 기준:

- endpoint/message/processing/readback/artifact/governance method가 각 port로 이동한다.
- `Service` constructor는 작은 ports를 받되, dev/test adapter는 composite struct로 한 번에 제공할 수 있다.
- 기존 memory/SQLite integration tests는 같은 behavior로 통과해야 한다.

현재 구현 상태:

- `Service`는 `Store` field를 갖지 않는다.
- `NewServerWithPorts`는 `Store`를 받지 않고 개별 port를 받는다.
- `Store`는 memory/SQLite dev/test adapter를 위한 composite marker다.
- `tests/server_adapter_contract_test.go`가 memory/SQLite를 같은 conformance flow로 검증한다.

### Step 4. DeliveryAdapter 도입

claim-next를 external transport fetch와 domain claim으로 분리한다.

변경 기준:

- JetStream pull consumer fetch는 DeliveryAdapter가 소유한다.
- domain claim, lease, attempt 생성은 ProcessingLedger가 소유한다.
- adapter ack는 attempt completion 이후 별도 step으로 실행한다.

### Step 5. Adapter conformance tests

adapter별로 같은 domain behavior를 검증한다.

필수 test group:

- endpoint advertisement
- message accept and quality decision
- claim-next empty without ref consumption
- claim-next creates attempt only when message exists
- readback required handoff
- artifact lineage append
- approval chain scope narrowing/widening
- side effect idempotency
- snapshot/projection readback

## Non-goals

이 계약은 아래를 요구하지 않는다.

- 이번 단계에서 JetStream adapter 구현 완료
- memory/SQLite 삭제
- 모든 service method 즉시 리팩터링
- A2A wire mapping 구현 완료
- Temporal workflow implementation

단, 새 production behavior를 memory/SQLite store에 먼저 추가하는 것은 금지한다.

## Acceptance Criteria

Store port split PR은 아래를 만족해야 한다.

1. `go test ./...`가 external service 없이 통과한다.
2. memory adapter와 SQLite adapter가 같은 conformance test를 통과한다.
3. `Service`는 `Store` field 없이 개별 ports만 사용한다.
4. `NewServerWithPorts`는 `Store`가 아니라 개별 ports를 받는다.
5. `Store`는 memory/SQLite dev/test composite marker로만 남는다.
6. public refs는 adapter-native id와 분리된다.
7. docs validator가 통과한다.

W1 종료 시점의 남은 explicit handoff:

- `ClaimNextMessage`는 다음 단계에서 `DeliveryAdapter` fetch와 `ProcessingLedger` domain claim으로 분리한다.
- JetStream stream sequence, consumer sequence, ack metadata는 public `tc://...` refs를 대체하지 않는다.

## Sources

- NATS JetStream consumers
  - https://docs.nats.io/nats-concepts/jetstream/consumers
- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream
- NATS JetStream streams
  - https://docs.nats.io/nats-concepts/jetstream/streams
- NATS queue groups
  - https://docs.nats.io/nats-concepts/core-nats/queue
- Temporal documentation
  - https://docs.temporal.io/
- A2A latest specification
  - https://a2a-protocol.org/latest/specification/

## Related Docs

- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [message-quality-policy.md](docs/active/contracts/message-quality-policy.md)
- [transport-adapters.md](docs/active/engineering/transport-adapters.md)

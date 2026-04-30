> Document Status: active
> Document Type: contract-model
> Scope: checkpoint, raw history, artifact ledger, takeover, claim/lease, redelivery의 current contract
> Canonical Path: `docs/active/contracts/checkpoint-and-takeover-model.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30
> Supersedes: `none`
> Superseded By: `none`

# Checkpoint And Takeover Model

## 목적

이 문서는 `touch-connect`의 takeover, redelivery, 처리 보장 계약을 고정한다.

현재 기준으로 이 문서가 닫는 질문은 아래다.

- raw history를 어디까지 믿을 수 있는가
- takeover를 위해 어떤 상태를 별도로 저장해야 하는가
- endpoint가 죽었을 때 message를 어떻게 다시 claim하게 할 것인가
- DLQ와 재전송이 처리 보장과 어떻게 연결되는가

이 문서는 위 질문에 대한 현재 계약이다.

## 문제 재정의

`touch-connect`는 단순 message broker가 아니다.  
사용자가 원하는 것은 `메시지가 갔는가`보다 `누군가 실제로 이어받아 처리했는가`에 가깝다.

따라서 아래를 분리해야 한다.

- `delivery`
  - 메시지가 전달되었는가
- `processing`
  - 누가 claim했고, 실제 처리 중인가
- `takeover`
  - 처리 중 endpoint가 사라졌을 때 다른 endpoint가 이어받을 수 있는가

## 조사에서 확인된 외부 기준

### 1. A2A는 history만으로 critical information을 보장하지 않는다

A2A 공식 specification은 `Task.history`를 둘 수 있지만,

- 모든 message가 history에 남는다고 보장하지 않는다
- 스트리밍 연결이 끊기면 일부 status update를 놓칠 수 있다
- critical information의 신뢰 가능한 전달 수단으로 message만 믿으면 안 된다

즉, raw history는 필요하지만 그것만으로 takeover와 처리 보장을 설계하면 안 된다.

### 2. Durable execution 계열은 상태를 별도로 남긴다

Temporal durable execution 자료는 process나 container가 죽어도 이어서 실행하려면 각 step과 state를 지속화해야 한다는 점을 보여준다.

`touch-connect`가 workflow engine 전체가 되려는 것은 아니지만,  
takeover를 지원하려면 적어도 `현재 처리 위치`, `claim owner`, `retry state`, `partial result`는 따로 남겨야 한다.

### 3. JetStream과 RabbitMQ는 delivery reliability와 processing guarantee를 분리한다

NATS JetStream 공식 문서는 base QoS가 `at least once`라서 드문 failure case에서 duplicate처럼 보이는 상황이 생길 수 있다고 설명한다.  
RabbitMQ도 ack와 confirm은 data safety의 일부일 뿐이며, application이 processing contract를 따로 가져야 한다는 방향이다.

즉, `touch-connect`는 broker feature만으로 exactly-once processing을 주장하면 안 된다.

## 현재 계약

v1 이후를 포함해도 `touch-connect`는 아래 3층을 함께 가져야 한다.

### 1. Raw History

보존 대상:

- 원문 message
- 원문 reply
- message correction / supersede chain
- message-level correlation

역할:

- audit
- postmortem
- human inspection
- ambiguous takeover 시 보조 참조

제한:

- 이것만으로 current processing state를 복원하면 안 된다

### 2. Processing Checkpoint

takeover와 resume를 위해 raw history와 별도로 관리하는 구조화 상태다.

최소 필드:

```text
checkpoint_ref
message_ref
attempt_ref
claim_owner_endpoint_ref
claim_epoch
lease_expires_at
current_processing_state
checkpoint_revision
progress_summary
last_progress_at
required_fields_status
missing_fields
missing_reasons
checkpoint_artifact_refs
failure_reason_code
retry_mode
retry_attempt
```

의미:

- `claim_owner_endpoint_ref`
  - 현재 처리 책임을 가진 endpoint
- `claim_epoch`
  - claim 재획득 시 증가하는 monotonic counter
- `lease_expires_at`
  - owner가 progress-aware heartbeat를 갱신하지 못할 때 claim이 만료되는 시점
- `required_fields_status`
  - handoff contract 충족 여부
- `missing_fields`
  - 재요청 시 다음 AI에게 명시해야 하는 누락 항목
- `missing_reasons`
  - 누락 필드가 왜 필요한지에 대한 짧은 설명
- `checkpoint_artifact_refs`
  - takeover가 raw history 전체를 읽기 전에 확인할 핵심 산출물
- `failure_reason_code`
  - 실패 시 재시도 가능 여부와 후속 처리 분기를 위해 필요한 분류 코드
- `retry_mode`
  - `same_endpoint` 또는 `reassigned_endpoint`
- `retry_attempt`
  - checkpoint payload 안에서 현재 `attempt_no`를 복사해 남기는 값이며, execution identity는 `attempt_ref`다

### 3. Artifact Ledger

takeover에서 가장 중요한 것은 실제 결과물과 중간 산출물이다.

따라서 artifact version은 이미 active 계약에서 정의한 것처럼 별도 ledger로 유지하고,  
checkpoint는 그 요약 ref만 가진다.

## Claim / Lease / Takeover

### Claim

- message가 processing queue에 들어오면 eligible endpoint 중 하나가 claim한다
- claim 성공 시 checkpoint의 `claim_owner_endpoint_ref`와 `claim_epoch`를 갱신한다
- claim은 영구 lock이 아니라 lease 기반이다
- claim의 기본 단위는 `task`가 아니라 `message`다

### Lease

- owner endpoint는 주기적으로 checkpoint 또는 work heartbeat를 보낸다
- lease가 만료되면 server는 claim을 orphaned 상태로 본다
- orphaned message/task는 다른 endpoint가 takeover claim할 수 있다

lease는 고정 timeout만으로 판정하지 않는다.

- checkpoint가 전진하고 있는가
- progress summary가 갱신되는가
- artifact progress가 있는가

를 먼저 보고, 시간은 보조값으로만 쓴다.

### Takeover

다른 endpoint가 takeover할 때 읽는 순서는 아래를 기본으로 둔다.

1. current checkpoint
2. checkpoint에 연결된 artifact refs
3. 마지막 처리 message
4. 필요 시 raw history

즉, takeover는 `history replay only`가 아니라 `checkpoint-first recovery`여야 한다.

## DLQ와 재전송

### DLQ에 가는 조건

- max redelivery 초과
- claim은 되었지만 lease 만료가 반복됨
- required fields 부족으로 재요청이 반복되지만 해결되지 않음
- policy block으로 계속 진행 불가

### DLQ에 남겨야 하는 최소 정보

```text
dead_letter_id
message_id
task_id
last_claim_owner_endpoint_id
dead_letter_reason
redelivery_count
last_checkpoint_id
replay_eligibility
created_at
```

### 재전송 원칙

- broker redelivery와 business reprocessing을 분리한다
- duplicate delivery는 가능하다고 가정한다
- processing은 `message_id + claim_epoch + idempotency_key` 조합으로 보호한다
- takeover 시 새 endpoint는 같은 message 위의 새 attempt로 붙고, 이전 attempt history는 append-only로 남긴다

## v1에 필요한 최소 판단

이 문서는 `touch-connect`가 판단자가 아니라는 전제를 유지한다.

하지만 아래는 판단이 아니라 계약 검증으로 본다.

- 필수 필드가 비었는가
- claim owner가 살아 있는가
- lease가 만료되었는가
- 재전송 가능한가
- takeover 가능한 checkpoint가 있는가

즉, `touch-connect`는 의미 해석을 하지 않지만 `processing contract validator` 역할은 한다.

## Related Docs

이 계약은 아래 active 문서와 함께 적용한다.

- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
  - task state와 checkpoint state의 관계
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
  - ack/redelivery/DLQ와 claim lease의 관계
- [artifact-model.md](docs/active/contracts/artifact-model.md)
  - checkpoint_artifact_refs와 artifact ledger 연결
- [mvp-canonical-scenario.md](docs/active/product/mvp-canonical-scenario.md)
  - planner/developer/reviewer takeover path 반영 여부

## 현재까지 닫힌 결정

- checkpoint는 처리 중인 worker/AI가 직접 보낸다
- server는 checkpoint의 source of truth를 기록하고 계약만 검증한다
- claim의 기본 단위는 `task`가 아니라 `message`다
- 상태 판단의 1차 기준은 시간 경과가 아니라 `checkpoint`와 progress signal이다
- `retrying`은 같은 endpoint 재시도와 다른 endpoint reassignment를 모두 포함한다
- takeover 시 새 endpoint는 전체 raw history만 다시 읽는 것이 아니라 `직전 attempt 요약 + ref`를 우선 사용한다

## v1 이후로 남기는 결정

아래 항목은 current contract 바깥의 확장 지점으로 둔다.

- worker local session state를 얼마나 checkpoint에 반영할 것인가
- DLQ에서 human/operator intervention 없이 자동 replay 가능한 범위
- endpoint 재연결 시 기존 claim 복구 조건

## Sources

- A2A Protocol specification
  - https://a2a-protocol.org/dev/specification/
- A2A Core Concepts
  - https://a2a-protocol.org/latest/topics/key-concepts/
- A2A Streaming and Asynchronous Operations
  - https://a2a-protocol.org/latest/topics/streaming-and-async/
- Temporal Durable Execution Guide
  - https://assets.temporal.io/durable-execution.pdf
- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream
- NATS JetStream Streams
  - https://docs.nats.io/nats-concepts/jetstream/streams
- RabbitMQ Consumer Acknowledgements and Publisher Confirms
  - https://www.rabbitmq.com/docs/4.1/confirms
- RabbitMQ Dead Letter Exchanges
  - https://www.rabbitmq.com/docs/dlx

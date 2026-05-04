> Document Status: active
> Document Type: contract-model
> Scope: AI 간 message quality와 handoff governance를 위한 통합 구현 계약
> Canonical Path: `docs/active/contracts/ai-communication-layer-contract.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-03
> Supersedes: `none`
> Superseded By: `none`

# AI Communication Layer Contract

## 목적

이 문서는 `touch-connect` 구현자가 가장 먼저 참조해야 하는 AI 간 통신 계층의 통합 계약이다.

기존 active 계약들은 message, delivery, checkpoint, approval, artifact를 각각 다룬다.
이 문서는 그 계약들을 하나의 message-quality and handoff-governance 모델로 묶고, transport와 storage가 domain contract를 오염시키지 못하게 하는 adapter 경계를 명시한다.

## 한 줄 정의

`touch-connect`는 AI endpoint 사이의 message handoff가 충분한지, 정확히 이해되었는지, 승인 범위와 산출물 lineage가 보존되었는지를 검증하는 message-quality and handoff-governance layer다.

transport, durability, replay는 production 경로에서 NATS JetStream, Temporal, A2A, AGNTCY-compatible adapter가 맡는다.
`touch-connect`가 다루는 단위는 packet이나 broker message가 아니라 AI 작업 message, artifact ref, checkpoint, approval intent, quality decision이다.

## 통신 모델 대전제

`touch-connect`는 AI 간 소통 자동화를 위한 message governance layer다.

목표는 단순히 message를 보내는 것이 아니다.
아래 질문에 답할 수 있어야 한다.

- 어떤 AI endpoint가 어떤 capability를 제공하는가
- message를 어떤 endpoint로 라우팅해야 하는가
- receiver가 message를 받았고 이해했는가
- 누가 message를 claim했고 처리 중인가
- 처리 중 endpoint가 사라졌을 때 누가 이어받을 수 있는가
- 외부 side effect가 중복 실행되지 않았는가
- 어떤 PhraseologyPolicy가 적용되었고 어떤 quality decision이 내려졌는가
- 어떤 ApprovalChain과 ArtifactLineage가 handoff에 연결되었는가

## 책임 경계

| Layer | 책임 주체 | touch-connect 위치 |
| --- | --- | --- |
| Transport, durability, replay | NATS JetStream, Temporal, AGNTCY-compatible transport | adapter consumer |
| Wire protocol, agent discovery | A2A 1.0 호환 surface | compatible/superset |
| Message quality | touch-connect | product surface |
| Handoff governance | touch-connect | product surface |
| Tool integration | MCP 등 외부 edge layer | caller/client |

memory와 SQLite 구현은 local dev/test adapter로만 둔다. production profile은 자체 queue 기능 추가 없이 external transport adapter를 통해 동작해야 한다.

## Capability-first routing

라우팅의 핵심은 agent 이름이 아니라 CapabilityClaim이다.

- endpoint는 동적으로 등록되고 사라질 수 있다.
- routing의 1차 기준은 agent 이름이나 role이 아니라 `CapabilityClaim`이다.
- endpoint registry는 `endpoint_ref`, `connection_state`, `capabilities`, `execution_hints`를 공개한다.
- endpoint 내부의 skill, prompt, local path, credential은 routing layer에 노출하지 않는다.
- direct routing, broadcast, reply correlation을 지원할 수 있다.
- 현재 구현 호환을 위해 단일 `target_capability` string은 `CapabilityClaim.capabilities[0].name`으로 해석한다.

### Queue-like endpoint polling

v1 worker는 특정 `message_ref`를 미리 알지 않아도 서버에 붙어 다음 작업을 요청할 수 있어야 한다.

필수 규칙:

- worker는 등록된 `endpoint_ref`와 capability registry를 기준으로 `claim-next`를 요청한다.
- server는 endpoint capability와 message `CapabilityClaim`이 맞는 message만 반환한다.
- 선택 우선순위는 `takeover_candidate`가 `available`보다 높다.
- claim 가능한 message가 없으면 실패가 아니라 empty queue 응답을 반환한다.
- empty queue는 delivery failure가 아니며 worker는 poll interval 후 다시 요청한다.
- server는 endpoint 내부 skill 선택이나 업무 판단을 하지 않는다.
- claim response는 worker execution adapter가 판단할 수 있도록 message payload와 constraints 원문을 포함한다.

### Delivery control

- `ack`는 envelope를 받았고 처리 대상으로 수락했다는 뜻이다.
- `ack`는 작업 완료, 승인 완료, side effect 완료를 뜻하지 않는다.
- critical handoff는 PhraseologyPolicy에 따라 receiver가 goal, constraints, next_action을 다시 확인해야 한다.
- 현재 구현 호환을 위해 `readback_required=true`는 `PhraseologyPolicy.readback.mode=required`로 해석한다.
- delivery timeout, message expiry, max redelivery, dead-letter policy는 broker default에 숨기지 않는다.
- duplicate delivery는 가능하다고 가정하고, dedupe와 idempotency로 보호한다.

### Processing continuity

transport만으로는 AI 작업의 처리 연속성을 보장할 수 없다.

따라서 `touch-connect`는 별도의 processing continuity layer를 둔다.

- message가 processing queue에 들어오면 eligible endpoint 중 하나가 claim한다.
- claim은 영구 lock이 아니라 lease 기반이다.
- claim의 기본 단위는 `task`가 아니라 `message`다.
- worker는 attempt 안에서 checkpoint를 직접 보낸다.
- server는 checkpoint를 추론해서 만들지 않고 기록하고 계약만 검증한다.
- takeover는 raw history replay가 아니라 checkpoint-first recovery를 기본으로 한다.
- long-running worker는 register 이후 heartbeat, claim-next polling, lease refresh, checkpoint 제출을 반복한다.
- worker가 정상 종료되면 endpoint는 `offline` heartbeat를 보낸다.

### Worker execution adapter

worker는 claim된 message를 직접 hard-coded success로 닫으면 안 된다.

필수 규칙:

- worker runtime은 message payload, constraints, resume context를 execution adapter에 넘긴다.
- execution adapter는 endpoint 내부 책임이며 server는 adapter 내부 판단을 하지 않는다.
- adapter 결과는 `completed`, `missing_fields`, `failed` 중 하나로 정규화된다.
- `completed`는 `in_progress` checkpoint 이후 attempt completion으로 기록한다.
- `missing_fields`는 readback과 `blocked_missing_fields` checkpoint로 기록한다.
- `failed`는 `failed` checkpoint와 `failure_reason_code`로 기록한다.
- artifact refs를 checkpoint에 연결하려면 먼저 artifact ledger에 등록된 exact artifact version이어야 한다.

### Local command execution adapter

로컬 명령 실행은 worker 내부 execution adapter의 한 종류다.

필수 규칙:

- local command executor는 명시적으로 켠 경우에만 사용한다.
- 실행 가능한 command는 allowlist에 있어야 한다.
- command는 shell string으로 실행하지 않고 command와 args를 분리해 실행한다.
- workdir는 absolute path여야 한다.
- timeout은 worker 설정으로 명시해야 하며, timeout 초과는 `failed`와 `command_timeout`으로 기록한다.
- allowlist 밖 command는 실행하지 않고 `failed`와 `command_not_allowed`로 기록한다.
- non-zero exit은 `failed`와 `command_exit_nonzero`로 기록한다.
- stdout, stderr, exit code는 worker execution result에 구조화해 남기며, artifact ledger 등록은 별도 단계에서 수행한다.

### Execution log artifact

worker가 command execution result를 artifact로 남기도록 설정된 경우 아래를 지킨다.

- artifact directory는 absolute path여야 한다.
- execution log는 message ref와 attempt ref를 기반으로 deterministic file name을 가져야 한다.
- execution log artifact는 `log_bundle` kind와 `application/json` media type을 사용한다.
- artifact version은 서버 artifact ledger에 등록된 뒤 checkpoint artifact refs에 연결한다.
- 완료된 command는 completed checkpoint가 execution log artifact version ref를 포함해야 한다.
- 실패한 command는 failed checkpoint가 execution log artifact version ref를 포함해야 한다.
- artifact `storage_ref`는 worker storage adapter가 만든 opaque reference이며, 서버는 내용을 직접 해석하지 않는다.

## Store와 adapter 경계

application layer의 store interface는 transport-agnostic해야 한다.

필수 규칙:

- NATS subject, JetStream sequence, Temporal workflow/run id, SQLite transaction 같은 infrastructure primitive를 domain method signature에 노출하지 않는다.
- claim, lease, redelivery는 domain event와 adapter mapping으로 표현한다.
- `NextRef` 같은 ref 생성은 store 고유 기능이 아니라 `RefAllocator` port로 분리한다.
- `Snapshot` 같은 전체 조회는 production write store가 아니라 projection/query port로 분리한다.
- external adapter는 `ack`, `nak`, `dedupe`, `lease timeout`, `replay`를 domain state transition에 매핑하되 domain state name을 broker state name으로 바꾸면 안 된다.

구체적인 adapter 작성 규칙과 현재 `tc-server/internal/application/store.go` 평가 기준은 [transport-adapters.md](docs/active/engineering/transport-adapters.md)를 따른다.

## 만들지 않는 것

`touch-connect`는 아래를 하지 않는다.

- 실제 transport 대체 프로토콜 구현
- 새로운 agent-to-agent 표준 정의
- 자체 queue, broker, transport, replay engine의 production 구현
- 전체 workflow planning
- task decomposition
- skill selection
- 업무 의미 해석
- plan 품질 판단

즉 `touch-connect`는 판단 시스템이나 transport 시스템이 아니라 message quality와 handoff governance 시스템이다.

## 구현 필수 계약

### 1. Message identity

message는 immutable communication event다.

필수 규칙:

- message 본문은 수정하지 않는다.
- 정정은 새 message와 `supersedes_message_id`로 표현한다.
- 하나의 message는 하나의 primary task만 참조한다.
- 같은 `message_id`는 같은 logical message다.

Canonical envelope는 [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)를 따른다.

### 2. Routing

routing은 CapabilityClaim-first다.

필수 규칙:

- domain routing key는 `CapabilityClaim`이다.
- `target_capability`는 v0 호환 projection이다.
- `from_role`과 `to_role`은 UI/projection label일 수 있지만 routing key가 아니다.
- 실제 skill 선택은 endpoint 내부 책임이다.
- correlation은 trace 용도이며 ordering이나 lifecycle ownership을 뜻하지 않는다.

### 2.5 Message quality

message quality는 delivery 전에 적용되는 policy gate다.

필수 규칙:

- handoff message는 PhraseologyPolicy를 가져야 한다.
- required field 누락, readback target 누락, ambiguous constraint는 quality decision으로 기록한다.
- policy action이 `reject`면 message를 dispatch하면 안 된다.
- policy action이 `warn`이면 warning decision을 durable history에 남긴다.
- quality decision은 message body를 수정하지 않고 append-only record로 남긴다.

세부 계약은 [message-quality-policy.md](docs/active/contracts/message-quality-policy.md)를 따른다.

### 3. Delivery

delivery는 transport QoS와 business reliability를 분리한다.

필수 규칙:

- global ordering은 없다.
- conversational ordering은 `thread_sequence` 범위에서만 보장한다.
- task projection ordering은 `task_revision` 범위에서만 보장한다.
- `handoff`와 `approval_request`는 ack required다.
- `state_update`는 durable하고 idempotent해야 한다.
- expired message는 새 작업이나 side effect를 시작하면 안 된다.

세부 계약은 [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)를 따른다.

### 4. Processing

processing은 delivery와 별도 계약이다.

필수 규칙:

- delivery 성공은 processing 성공이 아니다.
- 한 endpoint가 한 message를 맡은 실행 단위는 `attempt_ref`로 식별한다.
- `attempt_no`는 task-local retry projection이다.
- retry나 reassignment가 발생하면 새 attempt를 만든다.
- 한 attempt가 실패해도 retry policy나 takeover가 남아 있으면 task가 반드시 failed가 되는 것은 아니다.

세부 계약은 [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)를 따른다.

### 5. Checkpoint and takeover

checkpoint는 처리 연속성의 기준점이다.

필수 규칙:

- stalled 판단은 시간보다 checkpoint, progress summary, artifact progress를 먼저 본다.
- takeover endpoint는 current checkpoint, checkpoint artifact refs, 마지막 처리 message, raw history 순서로 읽는다.
- raw history만으로 current processing state를 복원하면 안 된다.
- checkpoint history와 attempt history는 append-only로 유지한다.

### 6. Artifact

artifact는 message와 동등한 1급 참조 대상이다.

필수 규칙:

- message는 logical artifact가 아니라 exact artifact version을 참조한다.
- artifact version은 immutable snapshot이다.
- lineage와 provenance는 optional convenience가 아니라 core contract다.
- latest pointer는 UI convenience일 수 있지만 domain contract가 아니다.

세부 계약은 [artifact-model.md](docs/active/contracts/artifact-model.md)를 따른다.

### 7. Approval and side effect

protected side effect는 approval과 execution ledger를 모두 요구한다.

필수 규칙:

- approval은 실행 허가이고 실행 결과 자체가 아니다.
- side effect 실행 전 approval decision record가 있어야 한다.
- 실행 시점의 `approval_hash`는 승인된 hash와 일치해야 한다.
- `idempotency_key + protected_scope` 조합은 side effect uniqueness boundary다.
- protected side effect 실행 여부는 delivery contract의 execution ledger로 판단한다.

세부 계약은 [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)와 [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)를 함께 따른다.

## 구현 불변식

구현은 아래 불변식을 깨면 안 된다.

- `server-side accepted records = truth`
- `tc-server = data-plane runtime truth`
- `tc-control = control-plane API over accepted records and projections`
- `worker = execution`
- `message`는 immutable이다.
- `ack != completion`
- `delivery != processing`
- `routing key = CapabilityClaim`
- `skill`은 endpoint 내부 책임이다.
- `claim unit = message`
- `checkpoint first, timeout second`
- `side effect exactly-once`는 transport가 아니라 execution ledger로 검증한다.
- broker, storage, MCP adapter는 domain contract를 오염시키면 안 된다.
- built-in memory/SQLite store는 dev/test profile이며 production capability가 아니다.

구현 디렉터리 관점에서는 `tc-server`가 message routing/delivery data plane이고, `tc-control`이 `tcctl`과 `admin`을 위한 control plane이다.
`tcctl`과 `admin`은 source of truth가 아니며, `tc-control`이 반환한 server-accepted state나 그 projection만 표시한다.

## 구현 읽기 순서

통신 계층을 구현할 때는 아래 순서로 읽는다.

1. 이 문서
2. [message-quality-policy.md](docs/active/contracts/message-quality-policy.md)
3. [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
4. [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
5. [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
6. [artifact-model.md](docs/active/contracts/artifact-model.md)
7. [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
8. [transport-adapters.md](docs/active/engineering/transport-adapters.md)
9. [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)

## Related Docs

- [touch-connect-product-definition.md](docs/active/product/touch-connect-product-definition.md)
- [message-quality-policy.md](docs/active/contracts/message-quality-policy.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [transport-adapters.md](docs/active/engineering/transport-adapters.md)
- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)

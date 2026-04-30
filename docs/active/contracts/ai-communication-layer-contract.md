> Document Status: active
> Document Type: contract-model
> Scope: AI 간 통신을 위한 TCP/IP-like message layer의 통합 구현 계약
> Canonical Path: `docs/active/contracts/ai-communication-layer-contract.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30
> Supersedes: `none`
> Superseded By: `none`

# AI Communication Layer Contract

## 목적

이 문서는 `touch-connect` 구현자가 가장 먼저 참조해야 하는 AI 간 통신 계층의 통합 계약이다.

기존 active 계약들은 message, delivery, checkpoint, approval, artifact를 각각 다룬다.
이 문서는 그 계약들을 하나의 통신 모델로 묶고, TCP/IP 관점에서 무엇을 차용하는지 명시한다.

## 한 줄 정의

`touch-connect`는 AI endpoint 사이에서 message를 빠르고 정확하게 라우팅하고, 수신 확인과 재전송과 처리 연속성을 보장하는 TCP/IP-like AI communication layer다.

단, 실제 TCP/IP 프로토콜을 새로 구현한다는 뜻은 아니다.
`touch-connect`가 다루는 단위는 packet이 아니라 AI 작업 message, artifact ref, checkpoint, approval intent다.

## 통신 모델 대전제

`touch-connect`는 AI 간 소통 자동화를 위한 message network다.

목표는 단순히 message를 보내는 것이 아니다.
아래 질문에 답할 수 있어야 한다.

- 어떤 AI endpoint가 어떤 capability를 제공하는가
- message를 어떤 endpoint로 라우팅해야 하는가
- receiver가 message를 받았고 이해했는가
- 누가 message를 claim했고 처리 중인가
- 처리 중 endpoint가 사라졌을 때 누가 이어받을 수 있는가
- 외부 side effect가 중복 실행되지 않았는가

## TCP/IP 관점에서 차용하는 것

### IP-like routing

IP에서 차용하는 관점은 addressing과 routing이다.

- endpoint는 동적으로 등록되고 사라질 수 있다.
- routing의 1차 기준은 agent 이름이나 role이 아니라 `target_capability`다.
- endpoint registry는 `endpoint_ref`, `connection_state`, `capabilities`, `execution_hints`를 공개한다.
- endpoint 내부의 skill, prompt, local path, credential은 routing layer에 노출하지 않는다.
- direct routing, broadcast, reply correlation을 지원할 수 있다.

### TCP-like delivery control

TCP에서 차용하는 관점은 수신 확인과 재전송과 중복 방지다.

- `ack`는 envelope를 받았고 처리 대상으로 수락했다는 뜻이다.
- `ack`는 작업 완료, 승인 완료, side effect 완료를 뜻하지 않는다.
- critical handoff는 `readback_required=true`로 receiver가 goal, constraints, next_action을 다시 확인해야 한다.
- delivery timeout, message expiry, max redelivery, dead-letter policy는 broker default에 숨기지 않는다.
- duplicate delivery는 가능하다고 가정하고, dedupe와 idempotency로 보호한다.

### Processing continuity

TCP/IP만으로는 AI 작업의 처리 연속성을 보장할 수 없다.

따라서 `touch-connect`는 별도의 processing continuity layer를 둔다.

- message가 processing queue에 들어오면 eligible endpoint 중 하나가 claim한다.
- claim은 영구 lock이 아니라 lease 기반이다.
- claim의 기본 단위는 `task`가 아니라 `message`다.
- worker는 attempt 안에서 checkpoint를 직접 보낸다.
- server는 checkpoint를 추론해서 만들지 않고 기록하고 계약만 검증한다.
- takeover는 raw history replay가 아니라 checkpoint-first recovery를 기본으로 한다.

## 차용하지 않는 것

`touch-connect`는 아래를 하지 않는다.

- 실제 TCP/IP 대체 프로토콜 구현
- 새로운 agent-to-agent 표준 정의
- 전체 workflow planning
- task decomposition
- skill selection
- 업무 의미 해석
- plan 품질 판단

즉 `touch-connect`는 판단 시스템이 아니라 통신과 처리 연속성 시스템이다.

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

routing은 capability-first다.

필수 규칙:

- domain routing key는 `target_capability`다.
- `from_role`과 `to_role`은 UI/projection label일 수 있지만 routing key가 아니다.
- 실제 skill 선택은 endpoint 내부 책임이다.
- correlation은 trace 용도이며 ordering이나 lifecycle ownership을 뜻하지 않는다.

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
- `routing key = target_capability`
- `skill`은 endpoint 내부 책임이다.
- `claim unit = message`
- `checkpoint first, timeout second`
- `side effect exactly-once`는 transport가 아니라 execution ledger로 검증한다.
- broker, storage, MCP adapter는 domain contract를 오염시키면 안 된다.

구현 디렉터리 관점에서는 `tc-server`가 message routing/delivery data plane이고, `tc-control`이 `tcctl`과 `admin`을 위한 control plane이다.
`tcctl`과 `admin`은 source of truth가 아니며, `tc-control`이 반환한 server-accepted state나 그 projection만 표시한다.

## 구현 읽기 순서

통신 계층을 구현할 때는 아래 순서로 읽는다.

1. 이 문서
2. [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
3. [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
4. [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
5. [artifact-model.md](docs/active/contracts/artifact-model.md)
6. [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
7. [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)

## Related Docs

- [touch-connect-product-definition.md](docs/active/product/touch-connect-product-definition.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)

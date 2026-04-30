> Document Status: active
> Document Type: foundation
> Scope: touch-connect의 최종 제품 정의, 핵심 객체, 책임 경계, v1 제품 철학
> Canonical Path: `docs/active/product/touch-connect-product-definition.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30
> Supersedes: `none`
> Superseded By: `none`

# Touch Connect Product Definition

## 목적

이 문서는 `touch-connect`의 최종 제품 정의를 고정한다.

기존 문서들이 문제 재정의, 시장 리서치, 계약 모델, 대표 시나리오를 각각 다뤘다면,  
이 문서는 그 결론을 하나의 제품 정의로 수렴하는 현재 기준 문서다.

## 한 줄 정의

`touch-connect`는 여러 이기종 AI와 에이전트가 메시지로 연결되고, 빠르게 라우팅되며, 처리 상태와 연속성을 잃지 않고 협업할 수 있게 만드는 message-layer platform이다.

## 제품 철학

`touch-connect`의 본질은 단순 message queue가 아니다.

이 제품은 아래를 동시에 만족해야 한다.

- message queue처럼 빠르게 전달된다
- IP 네트워크처럼 유기적으로 라우팅된다
- 연결이 살아 있는 endpoint network처럼 동작한다
- 처리 상태를 저장한다
- 끊김 이후에도 연속성을 보장한다

즉, `touch-connect`는 `AI message network with processing continuity`에 가깝다.

## AI communication layer 기준

구현 기준 통신 모델은 [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)를 따른다.

핵심 비유는 아래다.

- IP-like 관점: endpoint discovery와 capability-based routing
- TCP-like 관점: ack, readback, redelivery, dedupe, expiry, ordering boundary
- processing continuity 관점: claim, lease, attempt, checkpoint, takeover, DLQ

이 비유는 구현 방향을 설명하기 위한 것이며, 실제 TCP/IP 대체 프로토콜을 만든다는 뜻은 아니다.

## 우리가 만드는 것

`touch-connect`는 아래를 만드는 제품이다.

- 동적으로 등록되고 사라지는 AI endpoint network
- capability 기준의 message routing layer
- 처리 단위와 진행 상태를 추적하는 checkpoint layer
- artifact와 lineage를 잇는 reference system
- retry, redelivery, DLQ를 포함한 processing guarantee layer

## 우리가 만들지 않는 것

`touch-connect`는 아래를 하지 않는다.

- 전체 workflow planner
- task decomposition engine
- AI skill selector
- 업무 의미를 해석하는 orchestrator
- 새로운 agent-to-agent protocol 표준
- 범용 채팅앱

한 줄로 말하면, `touch-connect`는 `판단 시스템`이 아니라 `연결 시스템`이다.

## 핵심 원칙

### 1. connect는 판단하지 않는다

`touch-connect`는 아래를 하지 않는다.

- 어떤 plan이 더 좋은지 결정
- 큰 작업을 어떻게 쪼갤지 결정
- 어떤 skill로 처리할지 결정
- correlation이나 그룹 의미를 해석

대신 아래를 한다.

- 적절한 endpoint로 전달
- 필수 계약 충족 여부 검증
- 처리 상태 기록
- retry / redelivery / DLQ 관리

### 2. message가 중심이다

핵심 단위는 agent가 아니라 `message`다.

agent와 모델과 skill은 바뀔 수 있지만,  
협업의 연속성은 `어떤 메시지가 어떤 제약과 참조와 상태로 전달되었는가`에 달려 있다.

### 3. capability는 공개되고 skill은 내부에 남는다

- registry에 공개되는 것은 `capability`
- 실제 처리 방식은 endpoint 내부의 `skill`

즉 `touch-connect`는 `누가 무엇을 할 수 있는가`까지만 안다.  
`어떻게 하는가`는 endpoint 내부 책임이다.

### 4. 시간보다 checkpoint가 기준이다

AI 작업은 오래 걸릴 수 있으므로 단순 timeout만으로 실패를 판정하면 안 된다.

`touch-connect`는 아래 순서로 본다.

1. checkpoint가 전진하고 있는가
2. 진행 중인 summary가 갱신되는가
3. artifact progress가 있는가
4. 그다음에야 시간은 stalled 판단의 보조값으로 쓴다

즉 기준점은 `시간`이 아니라 `checkpoint`다.

### 5. delivery guarantee보다 processing guarantee가 더 중요하다

메시지가 도착했는지만으로는 부족하다.  
실제로 누군가 claim했고, 처리했고, 실패하면 복구 가능한지가 더 중요하다.

따라서 `touch-connect`는 아래를 분리한다.

- delivery
- processing
- retry
- DLQ

## 제품 표면

v1의 구현 표면은 data plane, control plane, execution endpoint, control client로 나눈다.

### tc-server

message routing과 delivery를 담당하는 data plane이다.
연결/기록/처리 보장에 필요한 runtime accepted record를 만든다.

### tc-control

`tcctl`과 `admin`이 사용하는 control plane backend다.
조회, 승인, retry, cancel, DLQ replay 같은 operator/admin API를 담당한다.

### tc-worker

사용자 로컬이나 실행 환경에서 실제 capability를 수행하는 endpoint runtime이다.

### tcctl

운영자와 개발자가 사용하는 CLI control client다.

### admin

운영자가 브라우저로 상태를 조회하고 보호된 작업을 승인/재시도하는 web control client다.

## server와 worker의 역할 분리

### server-side accepted records가 소유하는 것

- endpoint registry
- message ledger
- attempt ledger
- checkpoint ledger
- artifact metadata와 version ledger
- delivery ledger
- retry state
- DLQ state

`tc-server`는 data-plane accepted record를 만들고, `tc-control`은 그 accepted record나 projection 위에서 control-plane API를 제공한다.
`tcctl`과 `admin`은 source of truth가 아니다.

### worker가 소유하는 것

- 실제 capability 수행
- 내부 skill 선택
- 로컬 파일 접근
- shell/process 실행
- 중간 산출물 생성
- checkpoint 송신

즉:

- `server-side accepted records = truth`
- `tc-server = data-plane`
- `tc-control = control-plane`
- `worker = execution`

## 1급 객체

v1의 핵심 1급 객체는 아래 다섯 개다.

- `endpoint`
- `message`
- `attempt`
- `checkpoint`
- `artifact`

각 객체는 안정적인 ref를 가진다.

- `tc://endpoint/ep_...`
- `tc://message/msg_...`
- `tc://attempt/att_...`
- `tc://checkpoint/ckp_...`
- `tc://artifact/art_...`

ref는 사람이 읽기 쉬운 URI 형태로 보이되, 내부적으로는 안정적인 ID 기반이다.

## 객체 간 관계

### endpoint

동적으로 등록/해제되는 연결 주체다.

- 휘발적이다
- capability를 등록한다
- connection state를 가진다
- skill은 내부에 숨긴다

### message

실제 처리의 기본 입력 단위다.

- message 하나는 `target_capability` 하나를 가진다
- 복합 작업은 여러 message로 쪼개져야 한다
- 그 분해 책임은 보내는 AI가 가진다

### attempt

같은 message를 한 endpoint가 한 번 맡아 처리한 실행 단위다.

- retry나 reassignment가 발생하면 새 attempt가 생긴다
- message는 유지되고 attempt만 바뀐다
- `attempt_ref`는 실행 단위의 stable identity다
- `attempt_no`는 task 안에서 retry 순서를 보여주는 projection 값이다

### checkpoint

attempt 내부의 진행 기록이다.

- worker가 직접 보낸다
- server가 추론해서 만들지 않는다
- processing continuity의 기준점이다

### artifact

산출물과 그 lineage를 담는 객체다.

- ref는 고정
- version만 증가
- 예: `tc://artifact/art_01...?version=3`

## registry 공개 원칙

registry에는 아래만 공개한다.

- `endpoint_ref`
- `display_name`
- `connection_state`
- `capabilities`
- `execution_hints`

registry에 공개하지 않는 것:

- skill 목록
- skill 선택 로직
- 내부 프롬프트
- 로컬 경로
- credential
- 내부 실행 전략

## routing 원칙

### 기본 원칙

- 1차 라우팅 기준은 `capability`
- 실제 `skill` 선택은 endpoint가 한다

### 형태

- broadcast 가능
- direct routing 가능
- reply correlation 가능

즉 단순 pub/sub broker가 아니라,  
`discovery + routing + queue + correlation`을 함께 가진다.

### correlation

- `correlation_ref`는 선택값이다
- 보내는 AI가 넣는다
- `touch-connect`는 형식만 검증하고 의미는 해석하지 않는다

## message 최소 계약

v1 message 최소 계약은 아래다.

```text
message_ref
room_ref
thread_ref
sender_endpoint_ref
target_capability
delivery_class
readback_required
payload.summary
payload.body
payload.references[]
constraints[]
artifact_version_refs[]
correlation_ref (optional)
idempotency_key (protected side effect intent only)
supersedes_message_ref (optional)
```

### payload 원칙

`payload`는 자유 텍스트 하나가 아니라 반구조화된 객체다.

- `summary`
- `body`
- `references`

`references`는 항상 배열이고, 비어 있어도 허용한다.

`artifact_version_refs`는 artifact의 logical id가 아니라 exact version ref를 가리킨다.

### constraints 원칙

`constraints`도 항상 배열이고, 비어 있어도 허용한다.

각 항목은 최소 아래 구조를 가진다.

```text
code
summary
```

선택 필드:

- `source_ref`
- `details`

### routing과 역할 label

domain routing은 `target_capability` 기준이다.

- `from_role`과 `to_role`은 UI나 projection label로 쓸 수 있다.
- endpoint registry에 공개되는 routing key는 role name이 아니라 capability다.
- 외부 표면에서는 `tc://...` 형태의 ref를 쓰고, 내부 domain model은 같은 identity를 id로 저장할 수 있다.

## checkpoint 최소 계약

checkpoint는 `고정 상태코드 + 짧은 설명`으로 간다.

### 상태코드

- `claimed`
- `validating`
- `blocked_missing_fields`
- `in_progress`
- `retrying`
- `completed`
- `failed`

### 보조 원칙

- `artifact_updated`는 상태가 아니라 이벤트다
- `completed`는 처리 중인 AI가 선언한다
- server는 의미 판단이 아니라 계약 검증과 기록만 한다
- `failed`에는 `failure_reason_code`가 필수다
- `blocked_missing_fields`는 실패가 아니라 막힘 상태다

### blocked_missing_fields

이 상태에는 아래가 포함되어야 한다.

- `missing_fields`
- `missing_reasons`

즉 누락된 필드 목록만이 아니라, 왜 필요한지도 같이 전달한다.

### retrying

`retrying`은 넓은 상태로 두고, 보조 필드로 구체화한다.

- `retry_mode`
  - `same_endpoint`
  - `reassigned_endpoint`
- `retry_reason_code`
- `retry_attempt`

`retry_attempt`는 checkpoint payload 안에서 현재 `attempt_no`를 복사해 남기는 값이다.
실행 단위의 identity는 항상 `attempt_ref`다.

## continuity 원칙

### raw history와 checkpoint를 함께 유지한다

checkpoint만 있으면 맥락이 부족하고, raw history만 있으면 복구가 느리고 애매하다.

따라서 `touch-connect`는 둘 다 유지한다.

- raw history
- structured checkpoint
- artifact ledger

### reassignment 시 attempt를 새로 만든다

이전 AI가 실패하거나 끊겨도 message 자체는 유지한다.  
새 AI는 같은 줄기 위의 새 attempt로 붙는다.

즉:

- `message`는 유지
- `attempt`는 새로 생성
- `checkpoint history`는 append-only로 유지

### 새 AI는 요약과 ref를 함께 받는다

새 attempt는 이전 전체를 다시 해석하는 대신 아래를 받는다.

- 직전 attempt 요약
- checkpoint refs
- artifact refs
- 필요 시 raw history ref

여기서 `ref`는 외부 URL이 아니라 내부 참조값이다.

## failure / retry / DLQ 원칙

### 실패와 막힘은 다르다

- `blocked_missing_fields`
  - 재요청 대상
- `failed`
  - attempt 실패

### DLQ는 message 수준 최종 결과다

- `failed`는 attempt 수준
- `DLQ`는 message 수준 최종 격리 결과

즉 한 attempt가 실패해도 바로 DLQ가 아니다.  
재시도 정책이 끝난 뒤에도 복구가 안 될 때만 DLQ다.

## v1 canonical workload

v1의 대표 workload는 아래다.

- 사람 사용자가 Go 코드 변경 작업을 요청한다
- 이후 planner/developer/reviewer AI가 message와 artifact로 협업한다
- local CLI와 Go 실행 흐름을 포함한다
- SonarQube gate와 human approval을 거친다

즉 `touch-connect`는 시작부터 일반론적 플랫폼이 아니라,  
`Go 코드 변경 협업`을 통해 message-layer 책임을 먼저 검증한다.

## 제품을 이렇게 정의하는 이유

이 정의는 두 극단을 피하기 위한 것이다.

### 피해야 하는 극단 1

그냥 빠른 queue만 있는 제품

이 경우:

- 재시도
- 처리 추적
- takeover
- artifact lineage

가 약해진다.

### 피해야 하는 극단 2

workflow orchestrator까지 다 하는 제품

이 경우:

- connect가 판단자가 되고
- skill selector가 되고
- planner가 되고
- 내부 책임이 과도해진다

`touch-connect`는 이 사이에서  
`message routing + processing continuity`에 집중하는 제품이어야 한다.

## 최종 포지셔닝

### 내부 정의

`A message-layer platform for heterogeneous AI endpoint networks`

### 외부 설명

`Touch Connect helps heterogeneous AI endpoints stay connected, route work quickly, preserve constraints, and keep processing continuity across retries, failures, and handoffs.`

## Related Docs

- [touch-connect-overview.md](docs/active/foundation/touch-connect-overview.md)
- [message-centered-platform-principles.md](docs/active/foundation/message-centered-platform-principles.md)
- [market-and-research.md](docs/active/foundation/market-and-research.md)
- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)
- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
- [mvp-canonical-scenario.md](docs/active/product/mvp-canonical-scenario.md)

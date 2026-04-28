> Document Status: active
> Document Type: contract-model
> Scope: ordering, ack, readback, redelivery, dedupe, expiry, supersede 계약
> Canonical Path: `/Volumes/WD/Developments/touch-connect/docs/active/contracts/delivery-semantics.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-26

# Delivery Semantics

## 목적

이 문서는 브로커 선택과 무관하게 `touch-connect`가 지켜야 하는 전달 semantics를 고정한다.

현재 기준으로 중요한 원칙은 아래다.

- global ordering은 없다.
- ack는 completion이 아니다.
- expired message는 history에 남을 수 있지만 새 side effect를 만들면 안 된다.
- transport QoS와 business reliability는 분리한다.

## 정렬 경계

### Thread ordering

- `thread_sequence`는 같은 thread 안에서만 total order를 가진다.
- 서로 다른 thread 사이에는 ordering을 가정하지 않는다.

### Task ordering

- `task_revision`은 같은 task 안에서만 monotonic order를 가진다.
- 더 낮은 revision의 late arrival update는 current task state를 rollback하지 못한다.

## Delivery class

현재 delivery class는 아래 네 개를 기본으로 둔다.

### `informational`

- 단순 알림
- ack optional
- readback 없음
- expiry 후 조용히 무시될 수 있다

### `handoff`

- 역할 간 작업 넘김
- ack required
- readback optional, 단 `readback_required=true`면 필수
- durable history에 남아야 한다

### `approval_request`

- 보호된 action에 대한 승인 요청
- ack required
- durable해야 한다
- approval decision 전에는 side effect를 실행하면 안 된다

### `state_update`

- task/artifact/approval projection용 업데이트
- durable해야 한다
- idempotent하게 처리해야 한다
- human-facing readback은 기본적으로 요구하지 않는다

## Ack와 readback 의미

### Ack

ack는 아래 의미만 가진다.

- envelope를 받았다
- 파싱 가능하다
- 처리 대상으로 수락했다

ack는 아래를 뜻하지 않는다.

- 작업 완료
- 승인 완료
- 외부 side effect 완료

### Readback

readback은 receiver가 아래를 다시 확인하는 행위다.

- goal
- constraints
- next_action

critical handoff에서는 ack만으로 충분하지 않고 readback이 필요할 수 있다.

## Timeout과 redelivery

구체적인 숫자는 deployment config가 정하지만, 아래 항목은 반드시 존재해야 한다.

```text
ack_timeout
message_expiry
max_redelivery
dead_letter_policy
```

규칙:

- 값은 broker default에 숨기지 말고 운영 설정으로 명시한다.
- `max_redelivery`를 넘기면 `delivery_failed` record를 남긴다.
- 이후 처리는 dead-letter store 또는 운영자 개입 경로로 넘긴다.

## Dedupe와 idempotency

- 같은 `message_id`는 같은 logical message다.
- 같은 `idempotency_key`와 같은 protected scope 조합은 같은 side effect intent로 본다.
- duplicate arrival은 history에 남길 수 있지만 side effect는 한 번만 실행해야 한다.

## Late arrival와 supersede

- late arrival message는 보존할 수 있다.
- 하지만 더 최신 `task_revision`이나 `thread_sequence`를 뒤집어서는 안 된다.
- `supersedes_message_id`가 지정되면 이전 message는 obsolete로 표시한다.
- obsolete message는 history에서 제거하지 않는다.

## Expiry 규칙

- expired message는 새 작업을 시작하면 안 된다.
- expired approval request는 자동으로 `expired` 상태가 된다.
- expired informational message는 기록은 남아도 operational side effect를 만들지 않는다.

## Request/response와 correlation

- request-response 성격의 message는 `correlation_id`로 연결한다.
- correlation은 trace 용도이며 ordering이나 ownership을 뜻하지 않는다.
- response는 원 request의 intent를 덮어쓰지 않는다. 덮어쓰려면 새 message와 `supersedes_message_id`를 사용한다.

## 플랫폼 비종속 원칙

이 문서는 아래 구현체 모두에 적용된다.

- MQTT
- NATS / JetStream
- A2A streaming or polling
- 내부 custom bus

브로커가 바뀌어도 아래 계약은 바뀌면 안 된다.

- ordering boundary
- ack 의미
- readback 의미
- redelivery rule
- dedupe와 idempotency rule
- expiry와 supersede rule

## 현재 구현 기본값

- conversational order는 `thread_sequence`만 믿는다.
- task projection order는 `task_revision`만 믿는다.
- protected side effect는 `idempotency_key` 없이 실행하지 않는다.
- `approval_request`와 `state_update`는 항상 durable path로 보낸다.

## Related Docs

- [message-task-state-model.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/message-task-state-model.md)
- [approval-identity-policy.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/approval-identity-policy.md)
- [artifact-model.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/artifact-model.md)

## Sources

- MQTT 5.0
  - https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.pdf
- A2A Protocol specification
  - https://a2a-protocol.org/dev/specification/
- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream

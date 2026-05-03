> Document Status: active
> Document Type: contract-model
> Scope: ordering, ack, readback, redelivery, dedupe, expiry, supersede, protected side effect 실행 계약
> Canonical Path: `docs/active/contracts/delivery-semantics.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-03

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

## Worker polling과 empty queue

worker daemon은 서버에 등록된 뒤 직접 message id를 알지 못해도 다음 처리 대상을 요청할 수 있다.

`claim-next` 응답은 아래 두 종류다.

- `empty=true`
  - 현재 endpoint capability로 claim 가능한 message가 없다.
  - 오류가 아니며 DLQ나 delivery failure를 만들지 않는다.
  - worker는 poll interval 후 다시 요청한다.
- `claim`
  - 서버가 message 단위 claim을 성공시켰다.
  - worker는 즉시 `claimed` checkpoint를 append해야 한다.
  - worker execution adapter가 판단할 수 있도록 payload와 constraints 원문을 포함한다.
  - 이후 readback, lease refresh, progress checkpoint, completion checkpoint는 attempt 안에 남긴다.

규칙:

- worker는 queue empty를 실패로 해석하면 안 된다.
- worker는 processing 중 lease를 갱신해야 한다.
- worker가 정상 종료되면 endpoint 상태를 `offline`으로 갱신해야 한다.
- offline은 failure가 아니라 현재 연결이 없는 상태다.

## Worker execution 결과 매핑

worker runtime은 endpoint 내부 execution adapter 결과를 delivery/processing ledger로 정규화한다.

결과 매핑:

- `completed`
  - `in_progress` checkpoint를 남긴다.
  - attempt를 `completed`로 닫는다.
- `missing_fields`
  - 누락 필드와 이유가 포함된 readback을 남긴다.
  - attempt checkpoint는 `blocked_missing_fields`가 된다.
  - message state는 `input_required`가 된다.
- `failed`
  - `failure_reason_code`가 포함된 `failed` checkpoint를 남긴다.
  - message state는 `failed`가 된다.

규칙:

- server는 execution adapter 내부 결정을 대신하지 않는다.
- server는 결과 형식, attempt owner, lease, checkpoint artifact refs만 검증한다.
- executor error는 protected side effect 성공으로 dedupe하면 안 되며 `failed` checkpoint로 남겨야 한다.

## Local command result reason codes

local command executor가 사용하는 기본 실패 reason code는 아래다.

- `command_not_allowed`
  - command가 worker allowlist에 없어서 실행하지 않았다.
- `command_workdir_not_absolute`
  - workdir가 absolute path가 아니어서 실행하지 않았다.
- `command_timeout`
  - worker command timeout 안에 종료되지 않아 중단했다.
- `command_exit_nonzero`
  - 프로세스는 실행됐지만 exit code가 0이 아니었다.
- `command_start_failed`
  - 프로세스 시작 자체가 실패했다.
- `command_request_invalid_json`
  - payload body가 command request JSON으로 파싱되지 않았다.

payload body가 비어 있거나 `command` 필드가 없으면 실행 실패가 아니라 `missing_fields`로 처리한다.

## Execution log artifact 연결

worker가 execution log artifact store를 사용하면 command result는 checkpoint 전에 artifact ledger에 등록된다.

규칙:

- artifact 등록 실패 시 completed 또는 failed checkpoint를 먼저 남기면 안 된다.
- completed checkpoint는 성공 command의 execution log artifact version ref를 포함해야 한다.
- failed checkpoint는 실패 command의 execution log artifact version ref를 포함해야 한다.
- `blocked_missing_fields`는 command가 실행되지 않은 상태이므로 execution log artifact가 필수는 아니다.
- execution log artifact는 stdout, stderr, exit code, duration, reason code를 구조화해 담는다.

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
- protected side effect의 exactly-once 보장은 transport가 아니라 side effect execution ledger로 검증한다.
- side effect 결과를 알 수 없는 실패는 성공으로 dedupe하면 안 되며 운영자 확인이나 explicit retry 경로로 넘긴다.
- material change로 `approval_hash`가 바뀌면 protected side effect intent도 바뀐 것이므로 새 `idempotency_key`를 발급해야 한다.

## Protected side effect execution ledger

protected side effect는 실행 전에 durable execution record를 가져야 한다.

최소 필드:

```text
side_effect_execution_id
idempotency_key
protected_scope
approval_id
approval_hash
message_id
task_id
attempt_ref
operation_kind
external_target
requested_by_actor_id
executed_by_actor_id
status
started_at
completed_at
result_ref
failure_reason_code
```

status는 아래를 기본으로 둔다.

- `pending`
- `executing`
- `succeeded`
- `failed`
- `canceled`
- `deduped`

규칙:

- 승인 decision record 없이 protected side effect를 실행하면 안 된다.
- `approval_hash`가 현재 intent와 일치하지 않으면 external call을 시작하지 않는다.
- `idempotency_key + protected_scope` 조합은 uniqueness boundary다.
- 같은 uniqueness boundary의 duplicate request는 새 external call을 만들지 않고 기존 execution record를 반환하거나 `deduped` record로 연결한다.
- execution record 없이 외부 시스템에 side effect가 발생하면 contract violation이다.

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
- protected side effect execution ledger
- expiry와 supersede rule

## 현재 구현 기본값

- conversational order는 `thread_sequence`만 믿는다.
- task projection order는 `task_revision`만 믿는다.
- protected side effect는 `idempotency_key`와 side effect execution record 없이 실행하지 않는다.
- protected side effect 실행 여부는 execution ledger로만 판단한다.
- `approval_request`와 `state_update`는 항상 durable path로 보낸다.

## Related Docs

- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)

## Sources

- MQTT 5.0
  - https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.pdf
- A2A Protocol specification
  - https://a2a-protocol.org/dev/specification/
- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream

> Document Status: active
> Document Type: contract-model
> Scope: message, room, thread, task, correlation의 관계와 task state machine
> Canonical Path: `docs/active/contracts/message-task-state-model.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-04

# Message Task State Model

## 목적

이 문서는 구현 전에 `message`, `room`, `thread`, `task`, `correlation`의 관계와 task lifecycle을 고정한다.

현재 기준으로 가장 중요한 원칙은 아래다.

- `room`은 협업 공간 경계다.
- `thread`는 메시지 정렬 경계다.
- `task`는 작업 상태 경계다.
- `correlation`은 causal trace 경계다.
- `message`는 immutable event다.

## 핵심 객체와 ownership

### Tenant

배포와 정책의 최상위 경계다.  
single-tenant 배포에서는 논리 필드로만 존재할 수 있다.

### Workspace

제품/프로젝트 단위의 작업 경계다.  
설정, role assignment, 기본 policy의 적용 범위다.

### Room

하나의 mission 또는 협업 공간이다.

- thread와 task를 포함한다.
- membership과 visibility scope를 소유한다.

### Thread

room 안의 ordered conversation lane이다.

- 메시지의 정렬 경계다.
- `thread_sequence`는 thread 내부에서만 단조 증가한다.
- global ordering은 제공하지 않는다.

### Task

하나의 명확한 work item이다.

- 현재 상태의 lifecycle owner다.
- `task_revision`은 task 상태 변경마다 단조 증가한다.
- 같은 room에 속한다.

### Message

append-only communication event다.

- 정확히 하나의 room과 하나의 thread에 속한다.
- zero 또는 one task를 참조할 수 있다.
- 본문을 수정하지 않는다.
- 정정이나 변경은 새 message를 만들고 `supersedes_message_id`로 연결한다.

하나의 message가 여러 task를 동시에 변경하면 경계가 흐려지므로 허용하지 않는다.  
다수 task에 영향을 주는 경우 task별 message를 분리한다.

### Correlation

메시지, artifact, 외부 호출을 묶는 causal trace 식별자다.

- lifecycle owner가 아니다.
- ordering owner도 아니다.
- room, thread, task를 대체하지 않는다.

## 관계 규칙

- 하나의 room은 여러 thread를 가질 수 있다.
- 하나의 room은 여러 task를 가질 수 있다.
- 하나의 thread는 여러 message를 가진다.
- 하나의 task는 여러 message와 artifact를 가질 수 있다.
- 하나의 message는 하나의 primary task만 참조한다.
- `correlation_id`는 message, artifact, external operation across contexts를 묶을 수 있다.

## 필수 식별자

구현 전 최소 식별자는 아래를 고정한다.

```text
tenant_id (optional in single-tenant deployments)
workspace_id
room_id
thread_id
thread_sequence
task_id
task_revision
message_id
correlation_id
attempt_ref
attempt_no
```

규칙:

- `thread_sequence`는 thread 내부 정렬용이다.
- `task_revision`은 task 상태 projection용이다.
- `attempt_ref`는 특정 endpoint가 특정 message를 맡은 실행 단위의 stable identity다.
- `attempt_no`는 task-local retry projection이며 retry 시 증가한다.
- identity 참조와 lineage는 `attempt_ref`를 쓰고, 사람이 읽는 retry 순서나 task projection은 `attempt_no`를 쓴다.
- `correlation_id`는 요청-응답-외부 호출을 묶지만 parent container는 아니다.
- 내부 domain model은 `_id`를 기준으로 저장하고, 외부 message/API surface는 같은 identity를 `tc://...` 형태의 `_ref`로 노출할 수 있다.

## Canonical message envelope

현재 기준으로 message envelope는 아래 필드를 최소 계약으로 둔다.

```text
message_id
room_id
thread_id
thread_sequence
sender_endpoint_ref
capability_claim
task_id (optional)
correlation_id (optional)
delivery_class
phraseology_policy_ref or phraseology_policy
payload.summary
payload.body
payload.references[]
constraints[]
artifact_version_refs[]
idempotency_key (required for protected side effect intent)
supersedes_message_id (optional)
created_at
```

규칙:

- `capability_claim`은 routing의 1차 기준이며 endpoint 내부 skill을 노출하지 않는다.
- `target_capability`는 현재 구현 호환을 위한 단일 capability projection이다.
- `payload.references[]`는 message가 참조하는 입력 ref이고, `artifact_version_refs[]`는 artifact ledger의 exact version ref다.
- `delivery_class`와 `phraseology_policy`는 delivery contract와 message quality policy를 따른다.
- `readback_required`는 현재 구현 호환을 위한 boolean projection이다.
- `idempotency_key`는 보호된 외부 side effect intent가 있을 때만 필수다.
- `from_role`이나 `to_role`은 UI/projection label일 수 있지만 domain routing key가 아니다.

## Task 상태 집합

현재 기준으로 task 상태는 아래 6개를 기본으로 둔다.

- `submitted`
  - task가 생성되고 시스템이 수락했다.
- `working`
  - 한 actor가 task를 처리 중이다.
- `input_required`
  - approval, clarification, missing artifact 등 추가 입력이 필요하다.
- `completed`
  - terminal state. 성공적으로 끝났다.
- `failed`
  - 현재 task가 실패 projection 상태이며 explicit retry 또는 cancel 결정이 필요하다.
- `canceled`
  - terminal state. 더 진행하지 않는다.

`completed`와 `canceled`는 terminal state다.  
`failed`는 terminal state가 아니며 explicit retry가 있을 때만 다시 `working`으로 복귀할 수 있다.

## Task failure와 attempt failure

attempt failure와 task failure는 분리한다.

- attempt failure는 특정 endpoint가 맡은 한 실행의 실패 record다.
- task `failed`는 현재 task projection이 더 진행할 수 없음을 나타내는 상태다.
- 한 attempt가 실패해도 retry policy나 takeover가 남아 있으면 task가 반드시 `failed`로 전이되는 것은 아니다.
- retry가 확정되면 같은 `task_id`를 유지하고 새 attempt를 만든 뒤 `attempt_no`를 증가시킨다.
- intent 자체가 바뀌면 retry가 아니라 새 `task_id`를 발급한다.

## Input Required reason

`input_required`는 아래 reason 중 하나를 가져야 한다.

- `approval`
- `clarification`
- `missing_artifact`
- `external_dependency`
- `policy_block`

reason은 free text가 아니라 enum으로 관리하는 것을 기본값으로 둔다.

## 허용 상태 전이

- `submitted -> working`
- `submitted -> input_required`
- `submitted -> failed`
- `submitted -> canceled`
- `working -> input_required`
- `working -> completed`
- `working -> failed`
- `working -> canceled`
- `input_required -> working`
- `input_required -> failed`
- `input_required -> canceled`
- `failed -> working`
- `failed -> canceled`

허용하지 않는 것:

- `completed -> *`
- `canceled -> *`
- `completed -> failed`
- `completed -> working`

## Retry 규칙

- retry는 explicit user or policy action으로만 시작한다.
- retry 시 `task_id`는 유지하고 새 attempt를 만들며 `attempt_no`를 증가시킨다.
- retry로 다시 시작할 때 state는 `working`으로 전이한다.
- 기존 failure record는 history에 남긴다.
- intent 자체가 바뀌면 retry가 아니라 새 `task_id`를 발급한다.

## Message 규칙

- message는 immutable이다.
- message correction은 본문 수정이 아니라 새로운 message 생성이다.
- supersede된 message는 history에서 삭제하지 않는다.
- late arrival message는 기록할 수 있지만 최신 `task_revision`을 rollback하지 못한다.

## 현재 구현 기본값

현재 구현은 아래를 기본으로 둔다.

- 하나의 task는 하나의 room에만 속한다.
- task 상태 projection은 `task_revision` 기준으로만 갱신한다.
- delivery ordering은 `thread_sequence` 범위에서만 보장한다.
- room/thread/task/correlation은 서로 다른 aggregate 경계로 다룬다.

## Related Docs

- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
- [mvp-canonical-scenario.md](docs/active/product/mvp-canonical-scenario.md)

## Sources

- A2A Protocol specification
  - https://a2a-protocol.org/latest/specification/

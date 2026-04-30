> Document Status: active
> Document Type: product-scenario
> Scope: v1에서 반드시 성공해야 하는 대표 흐름과 완료 조건
> Canonical Path: `docs/active/product/mvp-canonical-scenario.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30

# MVP Canonical Scenario

## 목적

이 문서는 `touch-connect` v1이 반드시 성공해야 하는 대표 시나리오를 고정한다.

이 시나리오는 아래 조건을 동시에 검증해야 한다.

- message handoff
- artifact versioning
- CLI 기반 Go 작업
- SonarQube 품질 게이트
- human approval
- failure and retry

## 시나리오 한 줄 정의

사람이 Go 코드베이스 변경을 요청하면, planner/developer/reviewer agent가 message와 artifact로 handoff하고, SonarQube gate와 human approval을 거쳐 외부 side effect를 수행한다.

## 참여 주체

- `human_requester`
  - 목표와 제약을 제시한다.
- `planner_agent`
  - brief를 만든다.
- `developer_agent`
  - local CLI에서 Go 변경안을 만든다.
- `reviewer_agent`
  - 테스트와 validation artifact를 만든다.
- `human_approver`
  - protected side effect를 승인하거나 거절한다.

## 핵심 artifact

- `task_brief`
- `code_patch`
- `validation_report`
- `approval_record`

## 핵심 durable record

- `side_effect_execution_record`

## 기본 흐름

### 1. 작업 생성

- human_requester가 room에서 새 task를 만든다.
- task state는 `submitted`다.
- 첫 message는 `delivery_class=handoff`이고 `readback_required=true`다.

### 2. Planner handoff

- planner_agent가 ack 후 readback을 보낸다.
- task를 `working`으로 올린다.
- `task_brief` artifact version `candidate`를 만든다.

### 3. Developer handoff

- developer_agent가 planner의 brief와 constraints를 읽고 readback을 보낸다.
- local CLI 환경에서 Go 변경을 수행한다.
- `code_patch` artifact version `candidate`를 만든다.

### 4. Validation

- reviewer_agent가 테스트 결과와 품질 검증 결과를 수집한다.
- `validation_report` artifact를 만든다.
- SonarQube quality gate 결과는 `validation_report` artifact에 포함되어야 한다.
- gate가 `pass`가 아니면 protected side effect 단계로 진행하면 안 된다.

### 5. Approval

- 외부 side effect가 필요하면 approval request를 만든다.
- task state는 `input_required(reason=approval)`로 전이한다.
- human_approver가 승인 또는 거절한다.

### 6. Side effect 수행

- approval이 `approved`이고 `approval_hash`가 현재 intent와 일치하면 task를 다시 `working`으로 전이한다.
- protected side effect execution record를 `pending`으로 만든다.
- `idempotency_key + protected_scope` 중복 검사를 통과한 뒤 protected side effect를 수행한다.
- 실행 결과를 side effect execution record에 기록한다.
- 예:
  - PR 생성
  - 외부 tracker 갱신
  - MCP 기반 system update

### 7. 종료

- 최종 artifact version을 `final`로 지정한다.
- task state를 `completed`로 전이한다.

## 실패 경로

### Clarification required

- developer_agent가 brief의 제약이 불충분하다고 판단하면 task를 `input_required(reason=clarification)`로 전이한다.
- 새 clarification message가 도착하면 다시 `working`으로 복귀한다.

### SonarQube gate failure

- validation 결과 gate failure가 나오면 task를 `failed`로 전이한다.
- retry 승인 시 새 `attempt_ref`를 만들고 `attempt_no`를 올린 뒤 `working`으로 복귀한다.

### Approval rejection

- approval이 거절되면 기존 approval request는 `rejected`로 남긴다.
- task는 `input_required(reason=approval)`로 유지하거나 수정 요청에 따라 다시 `working`으로 복귀한다.

## 완료 조건

v1의 canonical scenario는 아래를 모두 만족해야 완료다.

- room 1개, primary thread 1개, task 1개가 생성된다.
- 최소 1회의 `readback_required` handoff가 성공한다.
- 최소 2개의 artifact version이 생성된다.
- `validation_report` artifact 안에 SonarQube quality gate `pass` 결과가 남는다.
- 최소 1회의 approval decision이 기록되고, canonical success run에서는 그 decision이 `approved`다.
- approval 이후 protected side effect execution record가 1개 생성되고 `succeeded` 상태가 된다.
- 같은 `idempotency_key + protected_scope` 조합으로 외부 side effect가 정확히 1회 수행된다.
- 실패 또는 clarification 경로 중 최소 1개를 재현 가능하게 지원한다.
- canonical success run의 최종 task state는 반드시 `completed`다.
- `failed/canceled`는 별도 negative path에서만 허용된다.

## 왜 이 시나리오를 먼저 푸는가

- `CLI + Go + DDD + SonarQube` 원칙을 바로 검증할 수 있다.
- message, artifact, approval, delivery semantics를 모두 한 흐름에서 검증할 수 있다.
- 특정 SaaS에 강하게 종속되지 않으면서도 enterprise relevance가 있다.

## Related Docs

- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)

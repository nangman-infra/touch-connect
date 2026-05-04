> Document Status: active
> Document Type: contract-model
> Scope: PhraseologyPolicy, CapabilityClaim, readback, missing-constraint, quality decision 계약
> Canonical Path: `docs/active/contracts/message-quality-policy.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-05
> Supersedes: `none`
> Superseded By: `none`

# Message Quality Policy

## 목적

이 문서는 `touch-connect`가 queue나 transport가 아니라 message quality와 handoff governance layer가 되기 위한 핵심 계약을 정의한다.

핵심 질문:

- 이 message는 다음 AI가 처리하기에 충분한가
- receiver가 goal, constraints, next action을 정확히 readback할 수 있는가
- capability claim이 version, scope, fallback을 충분히 표현하는가
- approval과 artifact lineage가 message handoff에 연결되어 있는가
- 부족하거나 위험한 message를 dispatch 전에 막거나 경고할 수 있는가

## 제품 결정

`touch-connect`는 production 경로에서 자체 queue 기능을 키우지 않는다.

대신 아래 다섯 surface를 제품 중심으로 올린다.

- `PhraseologyPolicy`
- `CapabilityClaim`
- `CorrelationGraph`
- `ApprovalChain`
- `ArtifactLineage`

이 문서는 그중 message quality gate에 직접 필요한 `PhraseologyPolicy`와 `CapabilityClaim`을 먼저 고정한다.

## PhraseologyPolicy

`PhraseologyPolicy`는 기존 `readback_required` boolean의 상위 모델이다.

최소 필드:

```text
policy_ref
policy_version
scope_kind
scope_ref
applies_to_capabilities[]
required_fields[]
readback
constraint_rules[]
ambiguity_rules[]
fallback_action
severity
audit_mode
```

### 필드 의미

- `policy_ref`
  - stable policy identity다.
- `policy_version`
  - policy 변경 시 증가한다.
- `scope_kind`
  - `global`, `capability`, `task`, `approval_scope` 중 하나다.
- `scope_ref`
  - scope가 가리키는 ref다. `global`이면 비울 수 있다.
- `applies_to_capabilities`
  - 이 policy가 적용되는 CapabilityClaim 목록이다.
- `required_fields`
  - message가 dispatch 전에 가져야 하는 필드 목록이다.
- `readback`
  - readback mode와 receiver가 다시 말해야 하는 field set이다.
- `constraint_rules`
  - missing constraint, ambiguous constraint, forbidden expansion을 검증한다.
- `ambiguity_rules`
  - 애매한 표현을 reject할지 warn할지 정한다.
- `fallback_action`
  - policy 위반 시 `reject`, `warn`, `request_clarification`, `route_to_review` 중 하나다.
- `severity`
  - `info`, `warning`, `blocking` 중 하나다.
- `audit_mode`
  - quality decision을 history에 남기는 방식을 정한다.

## CapabilityClaim

`CapabilityClaim`은 기존 `target_capability` string의 상위 모델이다.

최소 필드:

```text
claim_ref
capabilities[]
version_constraints[]
scope
fallback_chain[]
required_evidence[]
```

`capabilities[]` 항목:

```text
name
version
scope
priority
```

규칙:

- 단일 `target_capability`는 `capabilities[0].name`의 v0 호환 projection이다.
- 복수 capability가 필요하면 순서와 fallback을 명시해야 한다.
- scope가 넓어지는 capability 변경은 approval 재검토 대상이다.
- endpoint registry는 capability를 공개하지만 endpoint 내부 skill은 공개하지 않는다.

## QualityDecision

PhraseologyPolicy 검증 결과는 message body를 수정하지 않고 append-only record로 남긴다.

최소 필드:

```text
quality_decision_ref
message_ref
policy_ref
policy_version
decision
violations[]
fallback_action
created_at
created_by
```

`decision` 값:

- `passed`
- `warned`
- `rejected`
- `clarification_required`
- `review_required`
- `skipped`

`violations[]` 항목:

```text
code
field
detail
severity
suggested_fix
```

## Validator Rules v0

v0 policy surface는 아래 다섯 rule을 정의한다.
현재 deterministic 구현은 `missing_required_field`, `missing_readback_target`, `missing_lineage_reference` 세 rule부터 시작한다.
`ambiguous_constraint`와 `scope_expansion_without_approval`은 v0.1에서 rule catalog와 GovernanceLedger dependency를 붙여 구현한다.

### 1. `missing_required_field`

Policy가 요구한 field가 message envelope, payload, constraints, artifact refs 중 어디에도 없으면 위반이다.
현재 구현은 `message_ref`, `sender_endpoint_ref`, `target_capability`, `correlation_ref`, `readback_required`, `payload.summary`, `payload.body`, `payload.references`, `constraints`와 constraint/reference key lookup만 지원한다.
임의 nested dotted path는 v0.1 범위다.

예:

- `payload.summary` 없음
- `payload.body` 없음
- protected side effect인데 `idempotency_key` 없음
- approval이 필요한 scope인데 `approval_ref` 없음

### 2. `missing_readback_target`

readback required policy인데 receiver가 다시 확인해야 할 field set이 없으면 위반이다.

최소 readback target:

- `goal`
- `constraints`
- `next_action`

현재 구현 호환:

- `readback_required=true`인데 PhraseologyPolicy가 없으면 default policy를 적용한다.
- default policy는 `goal`, `constraints`, `next_action` readback을 요구한다.
- non-nil `PhraseologyPolicy`가 있으면 `readback_required` boolean projection보다 policy의 `readback` 설정이 우선한다.

### 3. `ambiguous_constraint`

constraint가 모호하거나 실행자가 서로 다르게 해석할 수 있으면 위반이다.

대표 패턴:

- `적당히`, `가능하면`, `나중에`, `필요하면`처럼 기준이 없는 표현
- 금지사항과 허용사항이 같은 문장에 섞여 scope가 불명확한 표현
- side effect 대상이 특정되지 않은 표현

v0는 deterministic rule 기반으로 시작한다. LLM-assisted detector는 optional extension이며, 그 결과도 QualityDecision으로만 남긴다.

### 4. `scope_expansion_without_approval`

message가 이전 승인 범위보다 넓은 tool, file, external system, side effect scope를 요구하면 위반이다.

규칙:

- scope가 좁아지는 변경은 approval chain에 자동 re-binding할 수 있다.
- scope가 넓어지는 변경은 재승인을 요구한다.
- approval이 없는 protected side effect는 dispatch 전에 reject한다.

### 5. `missing_lineage_reference`

message가 artifact를 수정, 대체, 파생한다고 말하지만 exact artifact version ref나 lineage edge가 없으면 위반이다.

필수 edge:

- `parent_version`
- `derived_from`
- `supersedes`

최소 하나의 edge 없이 기존 artifact를 바꾸는 message는 review 또는 clarification 대상이다.
현재 구현은 payload body의 deterministic keyword pattern과 `payload.references[]`의 artifact/artifact_version ref만 본다.
한글 `대체로`, `기반이 약하다`나 영문 `derive a benefit`처럼 artifact lineage 의도가 아닌 표현은 위반으로 보지 않는다.
LLM-assisted lineage intent detection은 v0.1 범위다.

## CLI 적용 규칙

`tcctl message send`는 기본적으로 PhraseologyPolicy 검증을 수행한다.

규칙:

- default는 quality gate enabled다.
- `--quality-gate=enforce|warn|skip`을 지원한다.
- `enforce`는 blocking decision을 reject하고, `warn`은 violation을 기록하되 dispatch하며, `skip`은 validator를 실행하지 않고 skipped decision만 남긴다.
- operator는 explicit flag로 local dev/test에서만 gate를 끌 수 있다.
- gate를 끈 message도 `quality_decision=skipped` record를 남긴다.
- `reject` decision이면 server dispatch를 시작하지 않는다.
- `warn` decision이면 warning을 표시하고 dispatch한다.

## Server 적용 규칙

server는 quality decision을 source of truth로 기록한다.

필수 규칙:

- message body를 mutate하지 않는다.
- quality decision은 append-only다.
- policy version을 decision record에 저장한다.
- policy 실패를 delivery failure로 기록하지 않는다.
- policy 실패는 `input_required`, `clarification_required`, `review_required` 같은 message state로 투영한다.

## Related Docs

- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [transport-adapters.md](docs/active/engineering/transport-adapters.md)

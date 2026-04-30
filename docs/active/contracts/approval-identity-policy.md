> Document Status: active
> Document Type: contract-model
> Scope: identity subject, capability policy, approval request와 re-approval 계약
> Canonical Path: `docs/active/contracts/approval-identity-policy.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30

# Approval Identity Policy

## 목적

이 문서는 `누가`, `무슨 권한으로`, `어떤 scope에서`, `무엇을 승인할 수 있는지`를 정의한다.

현재 기준으로 identity, policy, approval은 분리해서 본다.

- identity는 `누가 행동했는가`
- policy는 `무엇을 할 수 있는가`
- approval은 `추가 승인이 필요한가`

## Identity subject

현재 기준으로 actor type은 아래 세 가지를 기본으로 둔다.

- `human_user`
- `agent_instance`
- `service_principal`

규칙:

- model name이나 provider name은 identity 그 자체가 아니다.
- `agent_instance`는 role과 분리된 subject다.
- 같은 role을 여러 actor가 공유할 수 있지만 actor identity는 개별적이다.

## Identity 필수 필드

```text
actor_id
actor_type
tenant_id
workspace_id
role_ids
display_name
credential_ref
```

`credential_ref`는 구현체가 실제 credential material을 직접 노출하지 않도록 하기 위한 reference다.

## Policy scope

현재 기준으로 policy는 아래 scope를 지원해야 한다.

- tenant
- workspace
- room
- task
- artifact
- tool
- egress

scope는 넓은 것에서 좁은 것으로 갈수록 추가 제한을 걸 수 있다.

## Capability 기본 집합

초기 capability는 아래를 기본으로 둔다.

- `message.send`
- `message.read`
- `task.claim`
- `task.update`
- `artifact.read`
- `artifact.write`
- `artifact.finalize`
- `tool.invoke`
- `side_effect.execute`
- `approval.request`
- `approval.grant`

capability naming은 broker나 vendor product 이름을 직접 포함하지 않는다.

## Approval object

approval은 first-class object로 다룬다.

최소 필드:

```text
approval_id
target_type
target_id
requested_by_actor_id
approver_subjects_or_roles
approval_scope
approval_hash
status
reason
requested_at
expires_at
decided_at
decided_by_actor_id
decision_note
```

status는 아래 다섯 개를 기본으로 둔다.

- `pending`
- `approved`
- `rejected`
- `expired`
- `canceled`

추가 규칙:

- `pending` 상태에서는 `decided_by_actor_id`와 `decided_at`이 비어 있을 수 있다.
- `approved`, `rejected`, `canceled` 상태에서는 `decided_by_actor_id`와 `decided_at`이 반드시 있어야 한다.
- audit와 self-approval 검증은 `decided_by_actor_id` 기준으로 수행한다.

## 승인 대상

초기 기준으로 approval이 필요한 대표 대상은 아래다.

- 외부 시스템 side effect
- cross-boundary data egress
- protected artifact finalization
- privileged tool invocation
- policy escalation

## 승인 흐름 규칙

- `requires_approval`은 proposal flag이지 최종 권한 부여 자체가 아니다.
- approval pending 상태의 task는 `input_required(reason=approval)`로 표현한다.
- approval decision은 원래 message를 수정하지 않고 별도 record로 남긴다.
- requester와 approver가 동일한 actor인 self-approval은 기본적으로 금지한다.
- break-glass 예외는 정책으로만 허용한다.

## Re-approval 규칙

material change가 발생하면 기존 approval은 무효다.

현재 기준으로 material change는 아래를 포함한다.

- protected action 목록 변경
- external target 변경
- artifact version 변경
- approval scope 변경
- 핵심 constraints 변경

이를 판정하기 위해 `approval_hash`를 둔다.

규칙:

- `approval_hash`가 바뀌면 기존 approval은 재사용하지 않는다.
- 승인 후 내용이 바뀌면 새 approval request를 만든다.

## Protected side effect 승인 경계

approval은 실행 허가이고, protected side effect 실행 자체는 delivery contract의 execution ledger로 남긴다.

approval 관점의 규칙:

- protected side effect는 approved approval decision 없이 실행하면 안 된다.
- 실행 시점의 `approval_hash`는 승인된 `approval_hash`와 일치해야 한다.
- `expires_at`이 지난 approval은 side effect 실행에 사용할 수 없다.
- 같은 `protected_scope`와 `idempotency_key` 조합은 같은 side effect intent이며, dedupe와 실행 상태는 delivery contract가 소유한다.
- side effect를 호출하기 전에 delivery contract의 execution record를 만들거나 기존 record를 claim해야 한다.
- 실행 결과가 불명확하면 approval을 성공 근거로 삼지 않고 delivery contract의 explicit retry 또는 운영자 확인 경로로 넘긴다.

## 역할과 subject의 분리

- role은 capability bundle이다.
- actor는 실제 행위 주체다.
- policy audit는 role이 아니라 actor 기준으로 남긴다.

즉, `developer-agent` role로 일했더라도 audit에는 어떤 `agent_instance`가 행동했는지 남아야 한다.

## 플랫폼 비종속 원칙

현재 계약은 특정 auth protocol에 종속되지 않는다.

- OIDC
- mTLS
- API key
- local credential store

이들은 adapter choice이고, identity/approval domain contract는 `actor identity`, `role`, `capability`, `approval`만 소유한다.

## 현재 구현 기본값

- actor type은 `human_user`, `agent_instance`, `service_principal` 세 가지만 먼저 구현한다.
- approval은 boolean flag가 아니라 별도 aggregate로 구현한다.
- 모든 protected action은 approval decision record와 delivery side effect execution record 없이 실행되면 안 된다.

## Related Docs

- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)

## Sources

- OpenAI MCP Risks and Safety
  - https://platform.openai.com/docs/guides/tools-remote-mcp?lang=python
- Building MCP servers for ChatGPT and API integrations
  - https://platform.openai.com/docs/mcp/

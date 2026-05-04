> Document Status: active
> Document Type: contract-model
> Scope: artifact identity, versioning, retention, access, lineage 계약
> Canonical Path: `docs/active/contracts/artifact-model.md`
> Source Of Truth: yes
> Last Reviewed: 2026-05-04

# Artifact Model

## 목적

이 문서는 `touch-connect`의 artifact를 message와 동등한 1급 객체로 정의한다.

현재 기준으로 artifact는 아래를 만족해야 한다.

- stable identity를 가진다.
- versioned snapshot으로 관리된다.
- lineage와 provenance를 남긴다.
- storage backend와 분리된 참조 방식을 가진다.

## 핵심 개념

### ArtifactLineage

`ArtifactLineage`는 기존 artifact version list의 상위 제품 surface다.

최소 edge:

- `parent_version`
- `derived_from`
- `supersedes`

규칙:

- 기존 artifact를 수정, 파생, 대체하는 message는 exact artifact version ref와 lineage edge를 남겨야 한다.
- lineage edge가 없는 artifact 변경 handoff는 message quality violation이다.
- latest pointer는 UI convenience일 뿐 lineage source of truth가 아니다.

### Artifact

논리적 산출물의 stable identity다.

예:

- task brief
- code patch
- validation report
- design mock
- launch copy

### Artifact Version

artifact의 immutable content snapshot이다.

규칙:

- 내용이 바뀌면 새 version을 만든다.
- 기존 version은 수정하지 않는다.
- lineage는 version 간의 관계로 남긴다.

### Artifact Ref

message나 task가 참조하는 것은 기본적으로 `artifact_version_id`다.

규칙:

- `artifact_id`만 참조해서는 안 된다.
- 항상 `artifact_version_id` 또는 version-resolved ref를 사용한다.
- latest pointer는 UI convenience일 수 있지만 domain contract는 아니다.

## 필수 식별자와 메타데이터

현재 기준으로 artifact version은 최소 아래 필드를 가져야 한다.

```text
artifact_id
artifact_version_id
room_id
task_id
task_revision
kind
media_type
size_bytes
checksum
storage_ref
created_by_actor_id
created_at
retention_class
access_scope
based_on_message_ids
based_on_artifact_version_ids
```

추가 규칙:

- 현재 계약의 artifact version은 모두 task-owned object로 본다.
- 따라서 `task_id`는 필수이며, 이 version을 만든 owning task를 의미한다.
- `task_revision`은 이 version이 생성된 task state 시점을 의미한다.
- `checksum`은 content integrity 확인에 사용한다.
- `storage_ref`는 opaque reference다.

## Artifact kind

초기 기본 kind는 아래로 둔다.

- `document`
- `code_patch`
- `design_asset`
- `test_report`
- `log_bundle`
- `structured_data`

새 kind를 추가할 수 있지만, 기존 kind를 overloaded free text로 쓰지 않는다.

### Execution log artifact

worker execution log는 `log_bundle` artifact version이다.

필수 내용:

- `message_ref`
- `attempt_ref`
- `target_capability`
- `outcome`
- `summary`
- `failure_reason_code`
- `stdout`
- `stderr`
- `exit_code`
- `duration_ms`

규칙:

- execution log file name은 message ref와 attempt ref에서 deterministic하게 만든다.
- execution log는 task-owned artifact로 등록한다.
- `storage_ref`는 local file, object store, content-addressed key 등 worker storage adapter가 정한다.
- checkpoint는 logical artifact가 아니라 exact execution log artifact version ref를 참조한다.

## Version lifecycle

artifact version lifecycle은 아래를 기본으로 둔다.

- `candidate`
  - 작업 중인 버전
- `approved`
  - 승인 절차를 통과한 버전
- `final`
  - task의 canonical output으로 지정된 버전
- `superseded`
  - 새 버전으로 대체된 버전
- `archived`
  - 장기 보관 상태

규칙:

- `final`은 단순히 최신 버전이 아니라 explicit designation 결과다.
- 한 task 안에서 동일 logical artifact의 `final` version은 하나만 둔다.
- `superseded` version도 lineage와 audit 때문에 history에 남긴다.

## Inline vs reference

artifact payload는 inline 또는 reference로 보관할 수 있다.

### Inline 허용 조건

- 크기가 작다.
- text 또는 structured payload다.
- 민감도 정책상 inline 저장이 허용된다.

### Reference 사용 조건

- binary payload다.
- 크기가 크다.
- 민감도 또는 보존 규칙상 외부 storage가 필요하다.

현재 계약에서 `storage_ref`는 vendor-neutral reference다.  
예: file path, object store URI, git blob ref, content-addressed storage key.

## Retention class

보존 기간의 숫자는 배포 정책에 맡기되, 분류는 아래로 고정한다.

- `transient`
- `operational`
- `audit`

규칙:

- `audit` class는 hard delete보다 tombstone 또는 restricted access가 우선이다.
- 실제 보존 기간은 deployment policy가 정한다.

## Access scope

artifact access는 최소 아래 범위를 표현할 수 있어야 한다.

- room
- task
- actor
- role
- tenant

artifact는 message보다 더 넓은 scope를 가질 수 없고, policy가 더 좁게 제한하는 것은 허용된다.

## Lineage와 provenance

모든 artifact version은 아래를 남긴다.

- 누가 만들었는지
- 어떤 message로부터 만들어졌는지
- 어떤 artifact version을 기반으로 했는지
- 어떤 task revision 시점에서 생성되었는지

artifact lineage는 optional convenience가 아니라 core contract다.

## 삭제와 보존 규칙

- artifact version은 immutable이다.
- 삭제는 version rewrite가 아니라 tombstone 또는 access withdrawal 방식으로 처리한다.
- audit class artifact는 domain API에서 즉시 hard delete하지 않는다.

## 현재 구현 기본값

- message는 logical artifact가 아니라 exact artifact version을 참조한다.
- final designation 전까지 latest version은 canonical output이 아니다.
- artifact storage backend는 domain model 바깥 adapter에서 결정한다.

## Related Docs

- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)

## Sources

- A2A Protocol specification
  - https://a2a-protocol.org/latest/specification/

> Document Status: active
> Document Type: root-index
> Scope: touch-connect 문서 체계 전체와 최상위 인덱스
> Canonical Path: `/Volumes/WD/Developments/touch-connect/docs/README.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-27

# Touch Connect Docs

`docs/README.md`는 이 저장소의 문서 체계에 대한 최상위 인덱스이자 source of truth다.

이 문서의 역할은 두 가지다.

- 현재 어떤 문서가 `active`, `planned`, `archive` 상태인지 보여준다.
- 문서를 어디에 두고 어떻게 승격하거나 보관할지의 입구 역할을 한다.

## 문서 계약 핵심

- 문서의 상태는 `경로만`으로 결정하지 않는다.
- 문서의 상태는 `문서 상단 메타데이터 + 인덱스 등록 상태 + 경로 배치`를 함께 검증한다.
- 세 요소가 충돌하면 그 문서는 `invalid state`이며, current contract로 쓰지 않는다.
- 기본적으로 active 문서는 `docs/active/` 아래에 둔다.
- 다만 `docs/README.md`, `docs/planned/README.md`, `docs/archive/README.md` 같은 상태 디렉터리 인덱스 문서는 구조 문서이므로 예외적으로 현재 문서일 수 있다.
- 템플릿 같은 지원 자산은 상태 관리 문서와 분리해서 `docs/templates/` 아래에 둔다.

## 상태 모델

### `active`

현재 제품 정의, 구현, 의사결정에서 직접 참조되는 문서다.

- 현재 계약이나 방향을 설명한다.
- `docs/active/README.md`에 반드시 등록된다.
- 다른 active 문서와 구현 문서에서 참조 가능하다.

### `planned`

계획 중이거나 초안 상태이며 아직 active source of truth로 승격되지 않은 문서다.  
실무적으로는 `미참조 상태`의 작업 문서를 의미한다.

- 아이디어 정리, 초기 설계, 검토 전 메모를 둔다.
- active 인덱스에서는 현재 계약 문서로 참조하지 않는다.
- 승격 전까지는 현재 계약으로 간주하지 않는다.

### `archive`

더 이상 현재 기능 구현이나 운영 판단의 기준으로 직접 참조되지 않는 문서다.

- 완료된 단계 문서
- 대체된 설계안
- 오래된 리서치나 폐기된 초안

archive 문서는 `왜 archived 되었는지`, `지금 무엇이 이를 대체하는지`를 설명해야 한다.

## 디렉터리 구조

- [active/README.md](/Volumes/WD/Developments/touch-connect/docs/active/README.md)
  - 현재 참조되는 문서들의 두 번째 인덱스
- [planned/README.md](/Volumes/WD/Developments/touch-connect/docs/planned/README.md)
  - 아직 승격되지 않은 문서들의 관리 규칙
- [archive/README.md](/Volumes/WD/Developments/touch-connect/docs/archive/README.md)
  - 보관 문서 규칙과 archive note 규칙
- [active/governance/document-lifecycle.md](/Volumes/WD/Developments/touch-connect/docs/active/governance/document-lifecycle.md)
  - 상태 전이, 인덱스 규칙, 파일명/이동 규칙
- [templates/document-template.md](/Volumes/WD/Developments/touch-connect/docs/templates/document-template.md)
  - 상태 관리 문서가 아닌 문서 작성용 템플릿

## 현재 active 문서

현재 active 문서 목록은 [active/README.md](/Volumes/WD/Developments/touch-connect/docs/active/README.md)에서 관리한다.

핵심 시작점:

- [touch-connect-overview.md](/Volumes/WD/Developments/touch-connect/docs/active/foundation/touch-connect-overview.md)
- [touch-connect-product-definition.md](/Volumes/WD/Developments/touch-connect/docs/active/product/touch-connect-product-definition.md)
- [message-centered-platform-principles.md](/Volumes/WD/Developments/touch-connect/docs/active/foundation/message-centered-platform-principles.md)
- [market-and-research.md](/Volumes/WD/Developments/touch-connect/docs/active/foundation/market-and-research.md)
- [go-ddd-sonarqube-baseline.md](/Volumes/WD/Developments/touch-connect/docs/active/engineering/go-ddd-sonarqube-baseline.md)
- [message-task-state-model.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/message-task-state-model.md)
- [artifact-model.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/approval-identity-policy.md)
- [delivery-semantics.md](/Volumes/WD/Developments/touch-connect/docs/active/contracts/delivery-semantics.md)
- [mvp-canonical-scenario.md](/Volumes/WD/Developments/touch-connect/docs/active/product/mvp-canonical-scenario.md)

## 운영 규칙

- top-level 인덱스는 항상 이 파일이다.
- active 문서는 `docs/active/README.md`에 등록되지 않으면 active로 취급하지 않는다.
- planned 문서는 자유롭게 추가할 수 있지만 active 문서에서 현재 계약처럼 참조하지 않는다.
- 문서가 단독으로 열려도 상태를 알 수 있도록 상단 메타데이터를 유지한다.
- 같은 주제를 두 개 이상의 active 문서가 다루면 더 구체적인 범위 문서가 우선한다.
- archive로 이동한 문서는 현재 기준 문서가 무엇인지 명시해야 한다.
- 문서를 이동하면 상위 인덱스와 교차참조를 같이 갱신한다.

## 검증

문서 상태가 실제로 유효한지는 아래 명령으로 확인한다.

```bash
python3 /Volumes/WD/Developments/touch-connect/scripts/validate_docs.py
```

이 스크립트는 아래를 검사한다.

- 문서 상단 메타데이터 필수 필드 존재 여부
- `Canonical Path`와 실제 파일 경로 일치 여부
- `Document Status`, active registry, 디렉터리 배치의 일치 여부
- `Last Reviewed`의 `YYYY-MM-DD` 형식과 실제 날짜 유효성
- `Supersedes`, `Superseded By`의 docs 경로 형식과 대상 존재 여부
- `docs/README.md`, `docs/planned/README.md`, `docs/archive/README.md`의 구조 인덱스 예외 처리
- `docs/templates/` 아래 support 자산의 `Source Of Truth: no` 강제
- archive 문서의 archive note 존재 여부

## 현재 기준

- 작성일: 2026-04-26
- 관점: `여러 이기종 AI/에이전트를 하나의 메시지 체계 안에서 협업시키는 플랫폼`
- 전제:
  - 새로운 프로토콜을 정의하지 않는다.
  - `CLI + Skills`를 중심으로 시작한다.
  - `MCP`는 외부 시스템 연결을 위한 edge layer로 둔다.

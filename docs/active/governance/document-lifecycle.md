> Document Status: active
> Document Type: governance-policy
> Scope: 문서 상태, 우선순위, 전이, 인덱스 관리 규칙
> Canonical Path: `docs/active/governance/document-lifecycle.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-26

# Document Lifecycle

## 목적

이 문서는 `touch-connect` 문서가 일회성 메모가 아니라 계속 수정되고 유지되는 체계가 되도록 상태와 이동 규칙을 정의한다.

## peer review로 확인된 기존 문제

- 상태를 경로로만 해석하게 만드는 문장이 있었다.
- `planned`가 단순 초안인지, 아직 current reference가 아닌 미참조 상태인지 경계가 약했다.
- 문서가 단독으로 열렸을 때 상태와 범위를 알 수 있는 메타데이터가 없었다.
- `rename`, `split`, `merge`, `active -> active 대체` 같은 실무 전이가 빠져 있었다.
- `docs/planned/README.md`, `docs/archive/README.md` 같은 구조 문서의 예외 규칙이 없었다.

이 문서는 위 문제를 해결하는 방향으로 유지한다.

## 문서 계약의 3요소

문서 상태는 아래 세 요소를 함께 봐서 판단한다.

1. 문서 상단 메타데이터
2. 인덱스 등록 상태
3. 디렉터리 배치

이 세 요소는 `우선순위`가 아니라 `검증 집합`이다.

- 메타데이터는 문서가 주장하는 상태다.
- 인덱스는 현재 시스템이 인정하는 상태다.
- 경로는 문서가 놓인 물리적 위치다.

세 요소가 서로 일치해야만 유효한 상태로 인정한다.  
하나라도 충돌하면 그 문서는 `invalid state`이며, current source of truth로 사용할 수 없다.

예:

- `docs/archive/...` 아래 있는데 `Document Status: active`면 invalid state
- active 인덱스에 없는데 `Document Status: active`면 invalid state
- planned 문서인데 active current contract처럼 참조되면 invalid state

## 문서 메타데이터 계약

현재 유지되는 문서는 상단에 아래 필드를 둔다.

```md
> Document Status: active | planned | archived | support
> Document Type: root-index | active-index | governance-policy | engineering-baseline | contract-model | product-scenario | foundation | research | working-draft | archive-note | state-directory-index | template-asset
> Scope: 이 문서가 책임지는 범위
> Canonical Path: repo-relative 경로
> Source Of Truth: yes | no
> Last Reviewed: YYYY-MM-DD
```

선택 필드:

```md
> Supersedes: repo-relative docs 경로 또는 `none`
> Superseded By: repo-relative docs 경로 또는 `none`
```

## 상태 정의

### 1. Planned

아직 계획 중이거나 초안이며 current reference로 승격되지 않은 상태다.

이 상태의 문서는 다음 성격을 가진다.

- 아이디어 스케치
- 구조 초안
- 검토 전 가설
- 아직 active 인덱스에 올리면 안 되는 문서
- 현재 기준으로 직접 참조되지 않는 미참조 작업 문서

규칙:

- `docs/planned/` 아래에 둔다.
- `docs/active/README.md`에는 올리지 않는다.
- 구현이나 운영 판단의 source of truth로 사용하지 않는다.
- active 문서에서 링크할 수는 있지만, 그 경우 반드시 `planned` 또는 `초안`이라고 명시한다.

### 2. Active

현재 직접 참조되는 상태다.

이 상태의 문서는 다음 성격을 가진다.

- 현재 제품 정의
- 현재 설계 원칙
- 현재 리서치 기준
- 현재 계약 문서

규칙:

- 기본적으로 `docs/active/` 아래에 둔다.
- `docs/active/README.md`에 반드시 등록한다.
- 현재 기준 문서로 취급한다.
- 문서 메타데이터의 `Scope`가 현재 ownership을 정의한다.

### 3. Archive

더 이상 현재 기준으로 직접 참조하지 않는 상태다.

이 상태의 문서는 다음 성격을 가진다.

- 완료된 단계 문서
- 대체된 설계
- 폐기된 초안
- 더 이상 직접 기준으로 쓰지 않는 조사 문서

규칙:

- `docs/archive/` 아래에 둔다.
- 문서 상단에 archive note를 남긴다.
- 무엇이 현재 기준 문서인지 명시한다.
- archive 문서는 active 규칙을 override하지 못한다.

### 4. Support

상태 관리 문서는 아니지만 문서 작업을 돕기 위해 유지하는 지원 자산이다.

예:

- 템플릿
- 작성용 scaffold
- 참고용 보조 파일

규칙:

- `docs/templates/` 아래에 둔다.
- `Source Of Truth: no`여야 한다.
- active/planned/archive 인덱스에 current contract 문서처럼 등록하지 않는다.

## 상태 전이

### Planned -> Active

다음 조건을 만족하면 승격한다.

- 문서의 목적과 범위가 명확하다.
- 현재 기준으로 참조할 가치가 있다.
- active 문서와 충돌하지 않는다.
- `docs/active/README.md`에 등록했다.
- 문서 상단 메타데이터를 active 상태로 갱신했다.

### Active -> Archive

다음 상황이면 archive로 이동한다.

- 더 최신 active 문서가 이를 대체한다.
- 특정 단계 완료 후 더 이상 현재 기준이 아니다.
- 폐기되었지만 기록 보존은 필요하다.

이동 시 반드시 해야 할 일:

1. archive 경로로 이동
2. 문서 상단에 archive note 추가
3. active 인덱스에서 제거
4. 상위 인덱스와 교차참조 갱신

### Planned -> Archive

검토 없이 폐기되었지만 기록은 남겨야 할 때 사용한다.

### Active -> Active

현재 active 문서를 다른 active 문서로 대체하거나 재구성할 때 사용한다.

대표 사례:

- rename
- split
- merge
- scope 재정의

규칙:

- 새 문서를 먼저 current 상태로 준비한다.
- 같은 턴에서 active 인덱스를 갱신한다.
- 대체된 문서는 곧바로 archive로 내리거나, 이름만 바뀐 경우 canonical path와 링크를 전부 갱신한다.
- 두 active 문서가 같은 ownership을 장기간 공유하면 안 된다.

## 인덱스 규칙

### 최상위 인덱스

- `docs/README.md`가 최상위 source of truth다.
- 이 문서는 문서 체계 규칙의 입구다.

### active 인덱스

- `docs/active/README.md`가 active 문서의 두 번째 인덱스다.
- active 문서는 여기에 없으면 active로 취급하지 않는다.
- 단, `docs/active/README.md` 자신은 self-registering index로 본다.

### planned 인덱스

- `docs/planned/README.md`는 planned 상태의 보관 규칙과 목록 관리용이다.
- planned 문서는 active 인덱스에 등장하지 않는다.

### archive 인덱스

- `docs/archive/README.md`는 archive 규칙과 보관 경로의 입구다.

## 구조 문서 예외

다음 문서는 위치상 `docs/active/` 밖에 있어도 현재 유효한 구조 문서일 수 있다.

- `docs/README.md`
- `docs/planned/README.md`
- `docs/archive/README.md`

이 문서들은 `Document Type: state-directory-index` 또는 이에 준하는 index 타입을 가져야 하며, 경로 자체가 상태를 뜻하지 않는다.

## 지원 자산 예외

`docs/templates/` 아래의 문서는 current contract가 아니다.

- active 인덱스에 current 문서처럼 등록하지 않는다.
- frontmatter에 `Document Status: support`와 `Source Of Truth: no`를 둔다.
- 상태 관리 문서와 혼동되면 안 된다.

## 충돌 해결 규칙

문서가 서로 충돌할 때는 아래 순서를 따른다.

1. archived 문서는 active나 planned를 이기지 못한다.
2. planned 문서는 active를 이기지 못한다.
3. active 문서끼리 충돌하면 더 구체적인 `Scope` 문서가 우선한다.
4. 같은 구체성의 active 문서가 충돌하면 둘 중 하나는 잘못된 상태이므로 즉시 정리한다.

## 파일명 규칙

- 숫자 순번보다 의미가 드러나는 이름을 우선한다.
- 예:
  - `touch-connect-overview.md`
  - `message-centered-platform-principles.md`
  - `market-and-research.md`

이유:

- 문서가 살아 움직일 때 순번은 쉽게 낡는다.
- 의미 기반 파일명이 링크와 참조에 더 안정적이다.

## Archive Note 규칙

archive 문서 상단에는 최소한 아래가 있어야 한다.

```md
> Archived: YYYY-MM-DD
> Reason: 왜 archived 되었는지
> Superseded by: 현재 기준 문서 경로 또는 `none`
```

## 실무 규칙

- active 문서를 새로 만들면 같은 턴에서 active 인덱스도 같이 갱신한다.
- 문서를 archive로 옮기면 active 링크를 남겨두지 않는다.
- planned 문서는 자유롭게 쌓을 수 있지만 current policy처럼 서술하지 않는다.
- 하나의 주제에 active 문서가 여러 개면 역할을 분리해서 중복 기준을 만들지 않는다.
- 새 문서를 만들 때는 먼저 `Scope`를 정의한 뒤 작성한다.
- 문서를 rename할 때는 canonical path와 링크를 같이 갱신한다.

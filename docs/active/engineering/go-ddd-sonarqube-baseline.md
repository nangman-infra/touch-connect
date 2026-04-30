> Document Status: active
> Document Type: engineering-baseline
> Scope: Go 구현, DDD 경계, SonarQube 품질 게이트 기준
> Canonical Path: `docs/active/engineering/go-ddd-sonarqube-baseline.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30

# Go DDD SonarQube Baseline

## 목적

이 문서는 `touch-connect` 구현의 고정 개발 원칙을 정의한다.

현재 기준으로 구현 원칙은 아래 세 가지다.

- 구현 언어는 `Go`다.
- 코드 구조와 경계는 `DDD`를 기준으로 잡는다.
- 품질 검증과 merge readiness는 `SonarQube quality gate`에 위임한다.

## 고정 개발 원칙

### Go

- backend/runtime/service 코드는 `Go`로 작성한다.
- transport, storage, broker, MCP adapter 선택은 구현 세부사항이며 domain contract를 오염시키면 안 된다.
- 표준 라이브러리 우선 원칙을 따른다.

### DDD

- 기술 계층보다 `도메인 경계`가 먼저다.
- aggregate root가 도메인 불변식을 소유한다.
- application layer는 use case를 orchestration하지만 도메인 규칙을 소유하지 않는다.
- infrastructure는 domain/application이 선언한 port를 구현한다.
- 특정 브로커, DB, SaaS SDK 타입이 domain model 안으로 들어오면 안 된다.

### SonarQube

- `SonarQube`의 configured quality gate가 merge 가능 여부의 최종 기준이다.
- 숫자 임계치는 문서에 복제하지 않고 SonarQube 설정을 source of truth로 둔다.
- 로컬 lint/test는 보조 수단이며 SonarQube 결과를 override하지 못한다.

## 초기 bounded context

구현 시작 시점의 기본 bounded context는 아래로 둔다.

- `messaging`
  - room, thread, message envelope, readback, supersede 처리
- `endpoints`
  - endpoint registry, connection state, capability registration
- `processing`
  - attempt lifecycle, checkpoint, claim/lease, takeover 처리
- `tasks`
  - task lifecycle, state projection, retry, task revision
- `artifacts`
  - artifact identity, version, retention, lineage
- `approvals`
  - approval request, decision, re-approval, expiry
- `identity`
  - human, agent, service principal, role assignment
- `policy`
  - capability decision, scope evaluation, approval requirement 판정
- `delivery`
  - ack, timeout, redelivery, dedupe, dead-letter 처리

`shared` 또는 `common` 같은 큰 공용 패키지는 기본값으로 만들지 않는다.  
공유가 필요하면 먼저 어떤 bounded context가 owning context인지 정한다.

## Go package baseline

기본 package 구조는 아래를 따른다.

```text
/cmd/<app>/main.go
/internal/<bounded-context>/domain
/internal/<bounded-context>/application
/internal/<bounded-context>/infrastructure
/internal/platform/<adapter-kind>
```

레이어 의미는 아래와 같다.

- `domain`
  - entity, value object, aggregate, domain service, domain event
- `application`
  - command, query, use case, transaction boundary, orchestration
- `infrastructure`
  - repository 구현, broker adapter, persistence, external client
- `platform`
  - bootstrap, transport server, config loading, tracing, metrics, cross-cutting adapter

규칙:

- 기본값으로 `/pkg`는 만들지 않는다.
- domain은 같은 bounded context의 domain primitive와 표준 라이브러리만 의존한다.
- application은 여러 context를 엮을 수 있지만 직접 infrastructure 구현체를 참조하지 않는다.
- infrastructure는 interface를 구현하지만 domain rule을 추가하지 않는다.

## DDD 모델링 규칙

- aggregate root 외부에서는 aggregate 내부 상태를 직접 변경하지 않는다.
- repository는 aggregate root 기준으로 정의한다.
- 같은 business invariant를 두 aggregate에 나눠 담지 않는다.
- cross-context 동기 호출은 application port를 통해서만 한다.
- cross-context 비동기 연동은 domain event 또는 integration event로 표현한다.
- message/task/artifact/approval은 서로 다른 lifecycle을 가지므로 하나의 giant aggregate로 묶지 않는다.

## 초기 aggregate 후보

구현 초기에 aggregate root 후보는 아래를 기본값으로 둔다.

- `Room`
- `Task`
- `Artifact`
- `ApprovalRequest`
- `Actor` 또는 `RoleAssignment`
- `Endpoint`
- `Attempt`

`Message`는 append-only record 성격이 강하므로 `messaging` context 안에서 별도 aggregate 또는 event log record로 취급한다.

## SonarQube 품질 계약

- SonarQube quality gate 통과 전에는 merge-ready 상태로 간주하지 않는다.
- Go 분석 대상에서 `go.mod`가 빠지면 안 된다.
- coverage는 `go test` 기반 산출물을 SonarQube에 전달한다.
- new code 기준 품질 판단은 SonarQube gate를 우선한다.
- 예외 승인이나 suppression은 SonarQube의 review workflow 안에서만 남긴다.

## 최소 검증 루프

```bash
go test ./... -coverprofile=coverage.out
sonar-scanner
```

추가 규칙:

- `sonar.sources`나 `sonar.go.exclusions`를 조정할 때도 `go.mod`는 스캔에 포함한다.
- coverage 결과와 scanner 설정은 CI에서 재현 가능해야 한다.

## 비목표

이 문서는 아래를 지금 결정하지 않는다.

- 특정 브로커 제품 채택
- 특정 DB 제품 채택
- 특정 MCP server 목록
- 특정 웹 프레임워크 채택

이 문서는 기술 선택보다 `코드 경계와 품질 판정 방식`을 먼저 고정하는 문서다.

## Sources

- SonarQube Go
  - https://docs.sonarsource.com/sonarqube/latest/analyzing-source-code/languages/go/
- SonarQube Quality Gates
  - https://docs.sonarsource.com/sonarqube/latest/user-guide/quality-gates

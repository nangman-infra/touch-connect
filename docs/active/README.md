> Document Status: active
> Document Type: active-index
> Scope: 현재 기준으로 직접 참조되는 문서 registry
> Canonical Path: `docs/active/README.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-30

# Active Docs

`docs/active/README.md`는 현재 직접 참조되는 문서의 두 번째 인덱스다.

이 파일에 등록된 문서만 현재 기준 문서로 간주한다.

## Active 원칙

- 현재 의사결정과 기능 정의에서 직접 참조된다.
- 다른 active 문서나 구현 문서에서 링크할 수 있다.
- supersede되거나 완료되면 `archive/`로 이동한다.
- 아직 검토 중인 초안은 `planned/`에 둔다.
- active 문서는 각자 `소유 범위(scope)`가 분명해야 한다.
- 같은 주제를 다룰 때는 더 구체적인 active 문서가 더 넓은 문서보다 우선한다.

## 문서 영역

### Structural Indexes

- [docs/README.md](docs/README.md)
  - 문서 체계 전체의 최상위 인덱스
- [planned/README.md](docs/planned/README.md)
  - planned 디렉터리의 의미와 사용 규칙
- [archive/README.md](docs/archive/README.md)
  - archive 디렉터리의 의미와 사용 규칙

### Foundation

- [touch-connect-overview.md](docs/active/foundation/touch-connect-overview.md)
  - 제품 한 줄 정의, 문제 재정의, 제품 경계, 핵심 가설
- [message-centered-platform-principles.md](docs/active/foundation/message-centered-platform-principles.md)
  - 메시지 중심 협업 플랫폼 원칙, handoff 기준, 엔터프라이즈 요구사항
- [market-and-research.md](docs/active/foundation/market-and-research.md)
  - 시장/표준/논문/벤치마크 리서치

### Engineering

- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)
  - Go 구현 원칙, DDD 경계, SonarQube 품질 게이트 기준

### Contracts

- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
  - room/thread/task/message/correlation 관계와 task state machine
- [artifact-model.md](docs/active/contracts/artifact-model.md)
  - artifact identity, versioning, retention, lineage
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
  - actor identity, capability policy, approval와 re-approval 규칙
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
  - ordering, ack, readback, redelivery, dedupe, expiry, side effect execution 규칙
- [checkpoint-and-takeover-model.md](docs/active/contracts/checkpoint-and-takeover-model.md)
  - checkpoint, claim/lease, takeover, processing DLQ 규칙

### Product

- [touch-connect-product-definition.md](docs/active/product/touch-connect-product-definition.md)
  - 제품의 최종 정의, 1급 객체, 책임 경계, v1 message-layer 철학
- [mvp-canonical-scenario.md](docs/active/product/mvp-canonical-scenario.md)
  - v1에서 반드시 성공해야 하는 대표 시나리오와 완료 조건

### Governance

- [document-lifecycle.md](docs/active/governance/document-lifecycle.md)
  - 문서 상태 모델, 승격/보관 규칙, 인덱스 관리 규칙

## 읽는 순서

1. Structural Indexes의 `docs/README.md`
2. Foundation의 `overview`
3. Product의 `touch-connect-product-definition`
4. Foundation의 `principles`
5. Foundation의 `market-and-research`
6. Engineering의 `go-ddd-sonarqube-baseline`
7. Contracts의 `message-task-state-model`
8. Contracts의 `artifact-model`
9. Contracts의 `approval-identity-policy`
10. Contracts의 `delivery-semantics`
11. Contracts의 `checkpoint-and-takeover-model`
12. Product의 `mvp-canonical-scenario`
13. Governance의 `document-lifecycle`

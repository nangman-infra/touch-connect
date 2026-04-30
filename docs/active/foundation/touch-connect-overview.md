> Document Status: active
> Document Type: foundation
> Scope: touch-connect의 제품 정의와 문제 재정의
> Canonical Path: `docs/active/foundation/touch-connect-overview.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-27

# Touch Connect Overview

## 한 줄 정의

`touch-connect`는 여러 이기종 AI/에이전트가 하나의 메시지 체계 안에서 오해 없이 협업하도록 만드는 메시지 중심 협업 플랫폼이다.

## 문제 재정의

많은 멀티에이전트 제품은 `오케스트레이션`에 집중한다.  
하지만 실제 현장에서 더 자주 깨지는 것은 `에이전트 간 handoff`다.

문제는 보통 아래에서 발생한다.

- 이전 에이전트가 무엇을 목표로 했는지 다음 에이전트가 모른다.
- 산출물은 전달되지만 제약사항과 금지사항이 같이 전달되지 않는다.
- 누락된 맥락을 다음 에이전트가 추정으로 메우며 편향이 커진다.
- 외부 도구 접근 권한과 승인 흐름이 메시지와 분리되어 있다.
- 사람은 나중에 결과만 보고 왜 그렇게 되었는지 추적하기 어렵다.

따라서 `touch-connect`가 푸는 핵심 문제는 단순 전송이 아니다.

`여러 AI가 같은 일을 이어서 하더라도, 메시지 전달 과정에서 의미와 제약과 책임이 깨지지 않게 만드는 것`

## 우리가 만들려는 것

`touch-connect`는 다음을 목표로 한다.

- 여러 역할의 AI를 하나의 협업 공간에서 연결한다.
- 메시지 단위를 중심으로 작업을 넘긴다.
- 메시지와 산출물, 제약, 승인, 추적 정보를 함께 전달한다.
- CLI 기반 작업 흐름과 skill 기반 워크플로를 우선한다.
- 외부 시스템 연결은 마지막에 MCP로 붙인다.

## 만들지 않으려는 것

다음은 현재 방향에서 의도적으로 피해야 한다.

- 또 하나의 일반 채팅앱
- 또 하나의 범용 agent orchestrator
- 새로운 agent-to-agent 프로토콜
- 모든 맥락을 무조건 많이 전달하는 시스템
- MCP를 내부 메시징 버스로까지 확장한 설계

## 제품 경계

### 제품의 중심

- 내부 AI 팀 커뮤니케이션
- 역할 간 handoff
- 메시지 품질 보장
- 산출물 중심 협업
- 승인과 추적

### 제품의 바깥

- 대규모 범용 모델 학습 플랫폼
- 독자 표준 제정
- 모든 벤더 생태계를 대체하는 전체 agent runtime

## 핵심 가설

1. 멀티에이전트 실패의 큰 비율은 `모델 성능`보다 `메시지 handoff 품질` 문제에서 온다.
2. 엔터프라이즈는 단순한 빠른 전달보다 `누락 방지`, `추적 가능성`, `권한 통제`를 더 크게 산다.
3. `CLI + Skills + MCP edge` 구조는 실제 현업의 로컬 작업 문맥과 외부 연동을 가장 현실적으로 분리한다.
4. 멀티벤더 환경에서는 벤더 중립적 메시지 레이어가 필요하다.

## 제품 원칙

- 메시지는 자유 텍스트가 아니라 `협업 단위`다.
- 빠름보다 `의미 보존`이 우선이다.
- 누락 방지와 과잉 문맥 방지를 동시에 고려한다.
- 산출물 참조가 메시지의 중심이어야 한다.
- 사람 승인과 AI 자동화를 같은 흐름 안에 둔다.
- 내부 협업 레이어와 외부 연동 레이어를 분리한다.
- 실시간 전달 경로와 durable task state를 분리한다.
- 메시지 문법은 짧되 readback 가능한 구조를 가져야 한다.

## 제안하는 포지셔닝 문장

### 내부 제품 정의

`A message-centered collaboration layer for cross-role AI teams`

### 외부 설명 문장

`Touch Connect helps heterogeneous AI teams communicate, hand off work, and preserve context without losing intent, constraints, or accountability.`

## 왜 지금 가능한가

- 업계가 `AGENTS.md`, `Skills`, `MCP`, `A2A`로 각 계층을 빠르게 표준화하고 있다.
- 대형 벤더들도 agent collaboration과 governance를 전면에 내세우고 있다.
- 아직도 실제 현업에서는 `메시지 중심 handoff 품질`을 제품으로 풀어낸 사례가 약하다.

## 구현 전 기준 문서

현재 제품의 최종 운영 정의는 아래 문서에서 더 구체적으로 닫는다.

- [touch-connect-product-definition.md](docs/active/product/touch-connect-product-definition.md)

현재 구현 전 핵심 계약은 아래 문서에서 닫는다.

- [go-ddd-sonarqube-baseline.md](docs/active/engineering/go-ddd-sonarqube-baseline.md)
- [ai-communication-layer-contract.md](docs/active/contracts/ai-communication-layer-contract.md)
- [message-task-state-model.md](docs/active/contracts/message-task-state-model.md)
- [artifact-model.md](docs/active/contracts/artifact-model.md)
- [approval-identity-policy.md](docs/active/contracts/approval-identity-policy.md)
- [delivery-semantics.md](docs/active/contracts/delivery-semantics.md)
- [mvp-canonical-scenario.md](docs/active/product/mvp-canonical-scenario.md)

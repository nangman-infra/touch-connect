> Document Status: active
> Document Type: foundation
> Scope: 메시지 중심 협업 플랫폼 설계 원칙과 handoff 기준
> Canonical Path: `/Volumes/WD/Developments/touch-connect/docs/active/foundation/message-centered-platform-principles.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-26

# Message-Centered Platform Principles

## 왜 메시지가 중심인가

`touch-connect`의 핵심 단위는 agent 자체가 아니라 `메시지`다.

이유는 단순하다.

- agent는 교체될 수 있다.
- 모델은 바뀔 수 있다.
- skill도 교체될 수 있다.
- 하지만 실제 협업의 연속성은 `무엇이 어떻게 전달되었는가`에 달려 있다.

따라서 제품 설계의 중심은 `누가 말을 잘하느냐`보다 `무엇이 어떤 형식과 통제로 전달되느냐`에 있어야 한다.

## 메시지의 정의

여기서 메시지는 단순 텍스트가 아니다.

메시지는 아래를 함께 담는 협업 객체다.

- 목표
- 현재 상태
- 산출물 참조
- 제약사항
- 검증되지 않은 가정
- 위험도 또는 확신도
- 다음 액션
- 승인 필요 여부

즉, 메시지는 `다음 역할이 바로 일할 수 있도록 만드는 handoff payload`다.

## 설계 목표

### 1. 누락 방지

다음 에이전트가 중요한 제약이나 의도를 놓치지 않게 해야 한다.

### 2. 과잉 방지

모든 히스토리를 넘겨 컨텍스트를 부풀리면 오히려 잡음과 비용이 커진다.

### 3. 편향 완화

이전 에이전트의 강한 추정이나 잘못된 프레이밍이 다음 단계로 그대로 전파되지 않게 해야 한다.

### 4. 실행 가능성

메시지는 읽기 좋은 설명문이 아니라 다음 역할이 즉시 행동할 수 있는 구조여야 한다.

### 5. 추적 가능성

어떤 메시지로 인해 어떤 행동과 산출물이 나왔는지 되짚을 수 있어야 한다.

## 메시지 설계 원칙

### 필요충분한 구조화

`최소한의 구조화`만 강조하면 중요한 제약이 사라질 수 있다.  
`완전한 구조화`만 강조하면 시스템이 경직되고 유지보수가 어려워질 수 있다.

그래서 원칙은 `필요충분한 구조화`다.

필드 수를 늘리는 것이 목표가 아니라, handoff 실패를 줄이는 것이 목표다.

### 산출물 우선

메시지는 말보다 산출물과 연결되어야 한다.

예시:

- 코드 diff
- 문서 초안
- 디자인 이미지
- 로그
- 체크리스트
- 외부 링크

다음 에이전트는 대화 전체보다 `무엇을 기반으로 이어받아야 하는지`를 먼저 알아야 한다.

### 가정의 명시

실제 실패는 확정 사실과 임시 가정이 섞일 때 생긴다.

따라서 메시지에는 아래가 분리되어야 한다.

- 확인된 사실
- 아직 검증되지 않은 가정
- 사람이 최종 판단해야 하는 부분

### 금지사항의 명시

많은 멀티에이전트 시스템이 `무엇을 할지`는 잘 넘기지만 `무엇을 하면 안 되는지`는 잘 안 넘긴다.

따라서 handoff에는 최소한 아래가 들어가야 한다.

- 바꾸면 안 되는 것
- 외부 공유하면 안 되는 것
- 사람 승인 없이는 실행하면 안 되는 것

### 신뢰 경계의 분리

내부 메시지와 외부 시스템 호출은 동일 경계로 보면 안 된다.

- 내부 메시지: 빠르고 자주 오가는 협업 경로
- 외부 호출: 권한, 보안, 승인, 감사가 필요한 경로

이 기준에서 MCP는 `edge integration layer`로 두는 편이 맞다.

### 실시간 전달과 durable state의 분리

실시간 스트림과 현재 기준 상태를 같은 것으로 보면 안 된다.

- 실시간 메시지는 빠른 협업 경로다.
- task 상태, 최종 산출물, 승인 결과, 중요한 제약은 durable state에 남아야 한다.
- critical information은 일시적인 streaming event에만 의존하면 안 된다.
- 재접속, 재구독, 세션 복구 이후에도 다시 읽을 수 있는 history가 필요하다.

따라서 `touch-connect`는 `live message path`와 `durable task/artifact history`를 분리해야 한다.

### 재전송과 중복에 대한 현실적 계약

메시징 시스템의 QoS와 업무 의미의 exactly-once는 같은 것이 아니다.

- transport는 at-most-once, at-least-once, exactly-once-like semantics를 제공할 수 있다.
- 하지만 업무 단계에서의 side effect는 별도의 idempotency 계약이 없으면 중복 실행될 수 있다.
- 승인, 외부 API 호출, 파일 수정, 상태 전이는 재전송에 안전해야 한다.

그래서 `touch-connect`는 아래를 기본 계약으로 둔다.

- 모든 중요한 handoff에는 `message_id`가 있어야 한다.
- task 단위의 연속성에는 `task_id`와 `correlation_id`가 있어야 한다.
- 외부 side effect에는 `idempotency_key` 또는 이에 준하는 중복 방지 키가 있어야 한다.
- 메시지 재전송은 허용하되, 같은 의미의 업무가 두 번 실행되지 않도록 설계해야 한다.

### 무전식 phraseology와 readback

짧은 메시지가 좋은 메시지는 아니다.  
좋은 메시지는 `짧고`, `오해가 적고`, `받는 쪽이 다시 확인할 수 있는 메시지`다.

실무적으로는 아래 원칙이 필요하다.

- 불필요한 수식어와 장황한 배경 설명을 줄인다.
- 하지만 제약, 승인 필요 여부, 다음 액션처럼 빠지면 안 되는 필드는 줄이지 않는다.
- critical handoff는 단순 수신이 아니라 `ack` 또는 `readback`을 요구할 수 있어야 한다.
- jargon보다 팀 내에서 합의된 표준 어휘를 우선한다.

즉, `touch-connect`의 메시지 문법은 `brevity first`가 아니라 `unambiguous brevity`여야 한다.

## 권장 handoff 필드

실제 구현 전 단계의 개념 스키마로는 아래 정도가 적절하다.

```text
message_id
task_id
correlation_id
thread_id
room_id
from_role
to_role
goal
summary
state
artifact_refs
constraints
assumptions
open_questions
risk_level
confidence
next_action
delivery_class
readback_required
requires_approval
approval_scope
idempotency_key
supersedes_message_id
provenance
timestamp
```

### 필드 해석 원칙

- `summary`는 전체 배경 설명이 아니라 다음 역할이 즉시 이해해야 하는 요약이다.
- `state`는 자유 텍스트 감상이 아니라 task의 현재 상태를 설명하는 운영 필드다.
- `artifact_refs`는 대화 복사본보다 우선한다.
- `delivery_class`는 단순 알림인지, 작업 요청인지, 승인 요청인지 구분한다.
- `readback_required`는 오해 비용이 큰 handoff에만 사용한다.
- `idempotency_key`는 외부 side effect가 걸린 작업에 붙인다.
- `supersedes_message_id`는 이전 지시를 무효화하거나 덮어쓸 때 사용한다.

## 현재 확정하는 메시지 계약

현재 기준으로 `touch-connect`는 아래 계약을 고정한다.

1. live message stream은 source of truth가 아니다.
2. critical information은 반드시 task history 또는 artifact history에 남긴다.
3. 메시지 재전송은 허용하지만, 외부 side effect는 idempotent해야 한다.
4. critical handoff는 `ack` 또는 `readback` 가능한 문법을 가져야 한다.
5. 내부 메시지 레이어는 `replay`, `retention`, `correlation`, `backpressure`, `dedupe`를 지원할 수 있어야 한다.
6. MQTT는 참고할 전송 철학이고, A2A는 상호운용과 task/state 경계이며, MCP는 외부 도구 연동의 edge layer다.

## 역할 기반 협업 원칙

각 역할은 같은 메시지를 받더라도 같은 식으로 읽지 않는다.

- 개발 AI는 실행 가능성과 변경 범위를 본다.
- 디자인 AI는 표현과 일관성을 본다.
- 기획 AI는 요구사항 누락과 우선순위를 본다.
- 마케팅 AI는 외부 메시지와 톤을 본다.

그래서 메시지는 사람용 요약문 하나로 끝나면 안 되고, 역할별 해석이 가능하도록 구조화되어야 한다.

## 사람과 AI의 역할 분리

### 인간이 강한 영역

- 목표 정의
- 우선순위 결정
- 위험 허용치 판단
- 승인 기준 설정
- 가치 판단

### AI가 강한 영역

- 문맥 압축
- 누락 탐지
- 역할별 재구성
- 메시지 라우팅
- 충돌 검사
- provenance 기록

이 구분이 중요한 이유는 `touch-connect`가 사람을 대체하는 플랫폼이 아니라, Human-AI 팀의 handoff 품질을 높이는 플랫폼이어야 하기 때문이다.

## 엔터프라이즈 요구사항

메시지 중심 플랫폼이라면 최소한 다음을 만족해야 한다.

- 누가 메시지를 보냈는지 식별 가능해야 한다.
- 어떤 산출물과 근거를 바탕으로 보냈는지 추적 가능해야 한다.
- 어떤 메시지가 외부 시스템 호출로 이어졌는지 감사 가능해야 한다.
- 역할별로 허용된 도구와 데이터 범위가 달라야 한다.
- 민감 작업은 승인 흐름을 강제할 수 있어야 한다.
- 나중에 replay와 postmortem이 가능해야 한다.
- 끊긴 스트림 이후에도 critical state를 복구할 수 있어야 한다.
- 메시지 중복과 순서 뒤바뀜을 운영적으로 감당할 수 있어야 한다.

## 제품적으로 가장 중요한 경고

메시지 중심 제품은 쉽게 `AI용 Slack`처럼 보일 수 있다.  
하지만 실제 차별화는 UI가 아니라 아래에서 나온다.

- handoff 품질
- 메시지 검증
- 권한/승인
- 산출물 lineage
- audit/replay

이 다섯 가지가 빠지면 제품은 그냥 agent chat wrapper에 머문다.

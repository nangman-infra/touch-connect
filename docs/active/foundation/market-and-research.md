> Document Status: active
> Document Type: research
> Scope: touch-connect 관련 시장, 표준, 논문, 벤치마크 리서치
> Canonical Path: `/Volumes/WD/Developments/touch-connect/docs/active/foundation/market-and-research.md`
> Source Of Truth: yes
> Last Reviewed: 2026-04-26

# Market And Research

## 문서 목적

이 문서는 `touch-connect`를 `메시지 중심 AI 협업 플랫폼`으로 정의했을 때, 현재 시장/표준/논문/벤치마크가 무엇을 말하는지 정리한 딥 리서치 문서다.

기준 날짜는 `2026-04-26`이다.

## 1. 시장의 큰 흐름

### 1.1 연결 표준은 이미 빠르게 정리되고 있다

현재 시장은 다음 계층으로 정리되고 있다.

- `AGENTS.md`: 저장소/프로젝트 수준의 지속적 지침
- `Skills`: 반복 가능한 워크플로 패키징
- `MCP`: 외부 도구/데이터 연결
- `A2A`: agent-to-agent 상호운용

이건 중요한 의미를 가진다.

`touch-connect`가 차별화해야 할 지점은 표준 자체가 아니라, 이 표준들 사이에서 실제 협업 품질을 보장하는 제품 계층이라는 뜻이다.

### 1.2 대형 벤더의 초점도 협업 + 거버넌스로 이동했다

2025년 이후 주요 벤더는 모두 비슷한 방향을 밀고 있다.

- Microsoft: multi-agent orchestration, Agent ID, Purview, Teams 기반 A2A/MCP
- AWS: Bedrock multi-agent collaboration
- ServiceNow: AI Control Tower, AI Agent Fabric
- Salesforce: Agentforce 3, Command Center, MCP interoperability
- Google Cloud: Gemini Enterprise, A2A, Agent Platform

즉, 업계는 이미 `에이전트 여러 개 쓰기` 자체보다 `어떻게 연결하고 보이고 통제하느냐`로 넘어갔다.

## 2. OpenAI와 Anthropic 공식 문서가 시사하는 것

### 2.1 OpenAI Codex의 계층 구조

OpenAI Codex 공식 customization 문서는 아래 계층을 함께 보라고 정리한다.

- `AGENTS.md`
- `Memories`
- `Skills`
- `MCP`
- `Subagents`

또한 build order를 사실상 아래처럼 제시한다.

1. AGENTS.md
2. plugin 또는 skill
3. MCP
4. subagents

이건 `touch-connect` 방향과 잘 맞는다.  
즉, 외부 연동보다 먼저 로컬 문맥과 재사용 가능한 workflow를 안정화하는 게 우선이라는 뜻이다.

### 2.2 OpenAI가 말하는 MCP의 현실

OpenAI 공식 문서는 MCP와 connector가 강력하지만 위험하다고 명시한다.

핵심 포인트:

- remote MCP server는 third-party service다.
- 민감한 액션은 `require_approval`로 막아야 한다.
- `allowed_tools`로 tool surface를 좁혀야 한다.
- remote MCP server는 tool call에 실린 데이터는 볼 수 있다.
- 데이터 공유 로그를 남기고 검토해야 한다.

즉, MCP는 내부 agent 메시징의 기본 경로보다 `외부 egress layer`에 더 적합하다.

### 2.3 Anthropic이 보여주는 CLI 기반 실제 운영 모델

Anthropic의 흐름도 비슷하다.

- Skills는 점진적으로 로드된다.
- Subagent는 별도 context window와 별도 tool scope를 가질 수 있다.
- Channels는 `실행 중인 session`으로 외부 이벤트를 push한다.

이건 중요한 시사점을 준다.

`멀티에이전트 협업의 핵심은 새 세션 생성보다, 이미 살아 있는 세션과 작업 문맥을 보존하면서 메시지를 주입하는 것`이라는 점이다.

## 3. 논문이 말하는 것

## 3.1 멀티에이전트의 핵심은 communication이다

초기 대표 연구:

- `AutoGen`: 대화 자체를 multi-agent 인프라로 봄
- `ChatDev`: 언어 기반 역할 협업을 강조
- `MetaGPT`: SOP와 역할 분업을 강조

최근 survey는 이 흐름을 더 분명하게 만든다.

- `Beyond Self-Talk`는 LLM 기반 multi-agent system을 communication-centric하게 봐야 한다고 정리했다.
- `The Five Ws of Multi-Agent Communication`은 설계 문제를 `누가`, `언제`, `무엇을`, `왜` 전달하는가로 나눴다.

이건 `touch-connect`의 핵심 질문과 정확히 맞물린다.

## 3.2 실패는 종종 모델보다 handoff에서 생긴다

연구상으로도 메시지 전달 설계는 단순 구현 문제가 아니다.

- `Evaluating AGENTS.md`는 과한 context file이 성공률을 떨어뜨리고 비용을 20% 이상 늘릴 수 있다고 봤다.
- `On the Impact of AGENTS.md Files`는 반대로 적절한 repo instruction이 runtime과 token usage를 줄일 수 있다고 봤다.

둘을 함께 보면 결론은 명확하다.

`문맥은 많을수록 좋은 것이 아니라, 정확할수록 좋다.`

그래서 `touch-connect`는 `많이 전달하는 플랫폼`이 아니라 `필요충분한 문맥을 전달하는 플랫폼`이어야 한다.

## 3.3 메시지 계층 자체가 공격면이다

`Red-Teaming LLM Multi-Agent Systems via Communication Attacks`는 inter-agent message를 조작하는 것만으로 전체 시스템을 무너뜨릴 수 있다고 보였다.

이 연구가 주는 메시지는 직접적이다.

- agent-to-agent message는 신뢰 경계다.
- 내부 메시지도 공격면으로 봐야 한다.
- 메시지는 단순 payload가 아니라 검증 대상이다.

또 `Towards Secure Agent Skills`는 skill 생태계의 구조적 위험을 지적했다.

- data-instruction boundary 부재
- persistent trust 문제
- marketplace security review 부재

즉, `skills + messages + external tools`가 결합될수록 security model이 제품 중심 기능이 된다.

## 4. 전송 기술과 메시지 문법이 주는 시사점

### 4.1 MQTT는 좋은 비유이지만 전체 답은 아니다

MQTT 5.0은 `touch-connect`가 참고할 가치가 큰 특성을 보여준다.

- `Session Expiry`: 끊긴 연결 이후 세션 상태를 얼마나 유지할지 분리한다.
- `Message Expiry`: 늦게 도착하면 의미가 없는 메시지를 버릴 수 있다.
- `Shared Subscriptions`: 여러 소비자에게 부하를 나눌 수 있다.
- `Request/Response`, `Response Topic`, `Correlation Data`: 요청과 응답을 연결할 수 있다.
- `Topic Alias`, `Flow Control`: 전송 오버헤드와 in-flight 부하를 줄일 수 있다.

이건 `불안정한 연결`, `짧은 메시지`, `재전송`, `토큰/바이트 효율`을 중시하는 agent 협업 환경과 잘 맞는다.

하지만 MQTT만으로는 충분하지 않다.

- topic과 QoS는 `의미`를 보장하지 않는다.
- `이 메시지가 승인 요청인지`, `최종 산출물인지`, `이전 지시를 대체하는지`는 payload 계약이 따로 필요하다.
- transport의 exactly-once와 업무 의미의 exactly-once는 다르다.

즉, MQTT는 `전송 철학`으로는 유효하지만, `touch-connect` 전체 모델을 대신하진 못한다.

### 4.2 NATS/JetStream은 core bus 후보로 더 가깝다

NATS Core와 JetStream은 `빠른 실시간 경로`와 `저장/재생 경로`를 비교적 자연스럽게 분리한다.

- Core NATS는 빠른 request path에 적합하다.
- JetStream은 message storage, replay, retention, deduplication, consumer ack를 제공한다.
- `Nats-Msg-Id` 기반 dedupe, replay policy, work-queue/interest retention은 agent room과 task 흐름에 잘 맞는다.

다만 여기에도 중요한 현실 제약이 있다.

- JetStream의 acknowledgement는 곧바로 `영구적으로 절대 안 잃는다`는 뜻이 아니다.
- 공식 문서도 기본 file sync 설정에서는 OS failure 시 최근 acknowledged message 유실 가능성을 설명한다.
- 따라서 product contract는 broker feature만 믿지 말고, artifact/state layer에서 다시 보강해야 한다.

내 해석으로는 `touch-connect`의 내부 core bus는 MQTT보다 JetStream 류에 더 가깝고, MQTT는 edge bridge나 간헐 연결 환경에서 더 잘 맞는다.

### 4.3 DDS는 강하지만 현재 우선순위는 아니다

DDS는 real-time publish-subscribe와 세밀한 QoS, deadline, bandwidth, resource limit 제어에 강하다.

하지만 현재 `touch-connect`가 풀려는 문제는 초저지연 제어 시스템보다 `heterogeneous AI handoff`, `artifact`, `approval`, `audit`에 가깝다.

그래서 DDS는 참고할 QoS 개념은 많지만, 초기 제품의 중심 선택지로는 우선순위가 낮다.

### 4.4 A2A는 transport보다 task/state 모델에 가깝다

A2A 공식 스펙은 중요한 현실을 명시한다.

- streaming client는 재연결 시 모든 status update를 못 받을 수 있다.
- critical information에 대해 message만을 신뢰하면 안 된다.
- 중요한 내용은 task history와 artifact에 남겨야 한다.

이건 `touch-connect`에 직접적인 계약으로 번역된다.

- live stream은 빠른 길이다.
- current state는 task/history/artifact가 맡아야 한다.
- critical handoff는 history 조회와 replay가 가능해야 한다.

즉, A2A는 `전송 프로토콜`보다 `상태 모델과 상호운용 경계`로 보는 편이 맞다.

### 4.5 무전식 phraseology는 실제 메시지 문법 설계에 유효하다

FAA는 공식 문서에서 brevity를 강조하지만, 동시에 필요한 말은 반드시 해야 한다고 본다.

핵심은 아래다.

- 짧게 말하되 이해가 먼저다.
- 호출 전 듣고, 말하기 전에 생각한다.
- jargon과 잡담은 배제한다.
- 표준 phraseology가 안전을 높인다.

이걸 agent 협업에 번역하면 아래와 같다.

- message는 짧아야 하지만 제약과 다음 액션은 빠지면 안 된다.
- critical handoff는 `ack/readback` 가능한 문법을 가져야 한다.
- 자유 채팅보다 표준 어휘와 구조 필드가 우선해야 한다.

따라서 `touch-connect`는 `아날로그 무전처럼 보이는 UI`가 아니라 `무전식 의미 압축과 readback 규칙`을 제품화해야 한다.

## 5. 벤치마크가 보여주는 현실

`SWE-bench Verified`는 코딩 에이전트 성능을 보는 대표 지표다.  
`OSWorld`는 실제 컴퓨터 환경에서의 open-ended task 수행 능력을 평가한다.

특히 OSWorld는 현재 한계를 매우 분명하게 보여준다.

- 인간은 72.36% 이상 수행
- 최고 모델은 12.24% 수준

이 수치는 현재 agent가 여전히 완전 자동화보다 `부분 자율 + 감독 + handoff` 구조에 더 적합하다는 점을 보여준다.

즉, `touch-connect`는 agent autonomy를 과신하는 제품보다 `협업과 감독`을 강화하는 제품이 되어야 한다.

## 6. 시장의 빈자리

현재 시장에는 다음이 많다.

- orchestration platform
- agent builder
- tool integration layer
- enterprise governance/control tower

하지만 아직 약한 영역은 아래다.

- 역할 간 handoff를 메시지 수준에서 관리하는 제품
- 로컬 CLI 세션 문맥을 보존하는 협업 레이어
- 산출물 lineage와 메시지 provenance를 같은 축에서 보는 제품
- 누락, 과잉, 편향을 줄이는 message quality layer

여기가 `touch-connect`가 들어갈 자리다.

## 7. 제품 전략으로 번역하면

### 7.1 맞는 방향

- 메시지를 1급 객체로 본다.
- 역할 간 handoff 품질을 제품화한다.
- 내부 협업과 외부 MCP 연동을 분리한다.
- CLI/skill 기반 로컬 실행 문맥을 보존한다.
- approval, audit, provenance를 초기부터 넣는다.
- live transport와 durable state를 분리한다.
- retransmission-safe와 idempotent handoff를 기본값으로 둔다.
- phraseology와 readback 규칙을 메시지 문법에 넣는다.

### 7.2 피해야 할 방향

- agent chat wrapper
- protocol-first product
- MCP-only 내부 통신
- full-history broadcast 방식
- 오케스트레이터 범용 플랫폼으로의 과도한 확장
- transport QoS를 그대로 업무 신뢰성으로 오해하는 설계

## 8. 요약

현재 시장은 이미 `agent를 여러 개 돌리는 것` 자체를 경쟁력으로 보지 않는다.  
핵심은 `그 agent들이 무엇을 어떻게 주고받고, 그 전달이 얼마나 신뢰 가능하며, 사람이 얼마나 통제할 수 있는가`다.

따라서 `touch-connect`의 핵심 가치는 아래로 정리된다.

`Touch Connect is the message layer that helps heterogeneous AI teams communicate without losing intent, constraints, or accountability.`

## Sources

- OpenAI Codex Customization
  - https://developers.openai.com/codex/concepts/customization
- OpenAI MCP and Connectors
  - https://developers.openai.com/api/docs/guides/tools-connectors-mcp
- Anthropic Agent Skills engineering post
  - https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills
- Anthropic Agent Skills announcement
  - https://www.anthropic.com/news/skills
- Anthropic Claude Code Subagents
  - https://docs.anthropic.com/en/docs/claude-code/sub-agents
- Anthropic Claude Code Channels
  - https://code.claude.com/docs/en/channels
- AGENTS.md
  - https://agents.md/
- Google A2A announcement
  - https://developers.googleblog.com/pt-br/a2a-a-new-era-of-agent-interoperability
- Linux Foundation A2A launch
  - https://www.linuxfoundation.org/press/linux-foundation-launches-the-agent2agent-protocol-project-to-enable-secure-intelligent-communication-between-ai-agents
- A2A Protocol v1.0
  - https://a2a-protocol.org/latest/announcing-1.0/
- A2A Protocol specification
  - https://a2a-protocol.org/dev/specification/
- MQTT 5.0 OASIS standard
  - https://docs.oasis-open.org/mqtt/mqtt/v5.0/mqtt-v5.0.pdf
- NATS JetStream
  - https://docs.nats.io/nats-concepts/jetstream
- NATS JetStream Streams
  - https://docs.nats.io/nats-concepts/jetstream/streams
- OMG DDS
  - https://www.omg.org/omg-dds-portal/index.htm
- FAA Radio Communications Phraseology
  - https://www.faa.gov/air_traffic/publications/atpubs/aim_html/chap4_section_2.html
- Microsoft Build 2025 Copilot announcements
  - https://www.microsoft.com/en-us/microsoft-365/blog/2025/05/19/introducing-microsoft-365-copilot-tuning-multi-agent-orchestration-and-more-from-microsoft-build-2025/
- AWS Bedrock multi-agent collaboration
  - https://aws.amazon.com/blogs/machine-learning/amazon-bedrock-announces-general-availability-of-multi-agent-collaboration/
- ServiceNow AI Control Tower / AI Agent Fabric
  - https://newsroom.servicenow.com/press-releases/details/2025/ServiceNow-Launches-AI-Control-Tower-a-Centralized-Command-Center-to-Govern-Manage-Secure-and-Realize-Value-From-Any-AI-Agent-Model-and-Workflow/
- Salesforce Agentforce 3
  - https://www.salesforce.com/news/press-releases/2025/06/23/agentforce-3-announcement/
- AutoGen
  - https://arxiv.org/abs/2308.08155
- ChatDev
  - https://arxiv.org/abs/2307.07924
- MetaGPT
  - https://arxiv.org/abs/2308.00352
- Beyond Self-Talk
  - https://arxiv.org/abs/2502.14321
- The Five Ws of Multi-Agent Communication
  - https://arxiv.org/abs/2602.11583
- Evaluating AGENTS.md
  - https://arxiv.org/abs/2602.11988
- On the Impact of AGENTS.md Files
  - https://arxiv.org/abs/2601.20404
- Red-Teaming LLM Multi-Agent Systems via Communication Attacks
  - https://arxiv.org/abs/2502.14847
- Towards Secure Agent Skills
  - https://arxiv.org/abs/2604.02837
- SWE-bench
  - https://www.swebench.com/
- OSWorld
  - https://os-world.github.io/

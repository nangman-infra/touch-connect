package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestLLMExecutorCallsOpenAIResponsesAPI(t *testing.T) {
	var seenAuthorization string
	var seenRequest map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("expected /responses path, got %s", r.URL.Path)
		}
		seenAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenRequest); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		writeOpenAIResponsesText(w, "AI worker understood and completed the handoff.")
	}))
	defer provider.Close()

	executor, err := tcworker.NewLLMExecutor(tcworker.LLMExecutorOptions{
		BaseURL:      provider.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		SystemPrompt: "You are reviewer A.",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("create LLM executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("execute LLM handoff: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if !strings.Contains(result.Summary, "completed the handoff") || result.Stdout != result.Summary {
		t.Fatalf("expected provider text to become summary/stdout, got %+v", result)
	}
	if seenAuthorization != "Bearer test-key" {
		t.Fatalf("expected bearer authorization, got %q", seenAuthorization)
	}
	if seenRequest["model"] != "test-model" || seenRequest["instructions"] != "You are reviewer A." {
		t.Fatalf("unexpected provider request: %+v", seenRequest)
	}
	input, ok := seenRequest["input"].(string)
	if !ok || !strings.Contains(input, "message_ref: tc://message/msg_ai") || !strings.Contains(input, "payload.body:") {
		t.Fatalf("expected prompt to include touch-connect context, got %+v", seenRequest)
	}
}

func TestLLMExecutorTurnsProviderFailureIntoFailedResult(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"message":"model overloaded"}}`, http.StatusServiceUnavailable)
	}))
	defer provider.Close()

	executor, err := tcworker.NewLLMExecutor(tcworker.LLMExecutorOptions{
		BaseURL: provider.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create LLM executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("executor should return a failed result instead of error: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeFailed || result.FailureReasonCode != "llm_provider_status" {
		t.Fatalf("expected provider status failure result, got %+v", result)
	}
}

func TestLLMExecutorIncludesResumeArtifactContext(t *testing.T) {
	var seenRequest map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenRequest); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		writeOpenAIResponsesText(w, "continued from partial output")
	}))
	defer provider.Close()

	executor, err := tcworker.NewLLMExecutor(tcworker.LLMExecutorOptions{
		BaseURL: provider.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create LLM executor: %v", err)
	}
	input := llmExecutionInput()
	input.Takeover = true
	input.LastCheckpointRef = "tc://checkpoint/previous"
	input.ResumeSummary = "previous attempt timed out after partial work"
	input.ResumeArtifactRefs = []string{"tc://artifact-version/partial"}
	input.HandoffContext.Artifacts = append(input.HandoffContext.Artifacts, tcworker.HandoffArtifact{
		ArtifactVersionRef: "tc://artifact-version/partial",
		Summary:            "partial work",
		Stdout:             "already changed files A and B",
		Stderr:             "timeout",
	})
	if _, err := executor.Execute(context.Background(), input); err != nil {
		t.Fatalf("execute LLM: %v", err)
	}
	prompt, ok := seenRequest["input"].(string)
	if !ok {
		t.Fatalf("expected prompt input, got %+v", seenRequest)
	}
	for _, want := range []string{"takeover: true", "resume_summary: previous attempt timed out", "tc://artifact-version/partial", "already changed files A and B"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to include %q, got %s", want, prompt)
		}
	}
}

func TestWorkerEnvSelectsLLMExecutorAndCapabilities(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeOpenAIResponsesText(w, "env selected LLM executor")
	}))
	defer provider.Close()

	t.Setenv("TC_WORKER_EXECUTOR", "llm")
	t.Setenv("TC_WORKER_LLM_BASE_URL", provider.URL)
	t.Setenv("TC_WORKER_LLM_API_KEY", "test-key")
	t.Setenv("TC_WORKER_LLM_MODEL", "test-model")
	t.Setenv("TC_WORKER_CAPABILITIES", "ai.generate,ai.review")

	executor, err := tcworker.ExecutorFromEnv()
	if err != nil {
		t.Fatalf("create env executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("execute env LLM: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted {
		t.Fatalf("expected completed env LLM result, got %+v", result)
	}
	config := tcworker.ConfigFromEnv()
	if len(config.Capabilities) != 2 || config.Capabilities[0].Name != "ai.generate" || config.Capabilities[1].Name != "ai.review" {
		t.Fatalf("expected env capabilities to replace default capability, got %+v", config.Capabilities)
	}
}

func TestAICLIExecutorPassesTouchConnectPromptOnStdin(t *testing.T) {
	command := writeFakeAIScript(t, `#!/bin/sh
set -eu
input=$(cat)
case "$input" in
  *"message_ref: tc://message/msg_ai"*payload.body:*) echo "AI CLI completed handoff";;
  *) echo "missing touch-connect context" >&2; exit 7;;
esac
`)
	executor, err := tcworker.NewAICLIExecutor(tcworker.AICLIExecutorOptions{
		Command: command,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create AI CLI executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("execute AI CLI: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted || strings.TrimSpace(result.Stdout) != "AI CLI completed handoff" {
		t.Fatalf("expected AI CLI stdout to complete handoff, got %+v", result)
	}
}

func TestAICLIExecutorIncludesHandoffContextOnStdin(t *testing.T) {
	command := writeFakeAIScript(t, `#!/bin/sh
set -eu
input=$(cat)
printf '%s' "$input" | grep -q "handoff_context:" || { echo "missing handoff context" >&2; exit 11; }
printf '%s' "$input" | grep -q "tc://message/msg_prior" || { echo "missing prior message" >&2; exit 12; }
printf '%s' "$input" | grep -q "IMPLEMENTER_OUTPUT_OK" || { echo "missing prior artifact stdout" >&2; exit 13; }
echo "reviewer saw prior implementer output"
`)
	executor, err := tcworker.NewAICLIExecutor(tcworker.AICLIExecutorOptions{
		Command: command,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create AI CLI executor: %v", err)
	}
	input := llmExecutionInput()
	input.TargetCapability = "ai.review"
	input.HandoffContext = tcworker.HandoffContext{
		TaskRef: "tc://task/ai_handoff",
		Messages: []tcworker.HandoffMessage{
			{
				MessageRef:       "tc://message/msg_prior",
				TargetCapability: "ai.generate",
				State:            "completed",
				AttemptRef:       "tc://attempt/att_prior",
				Summary:          "prior implementation summary",
				Body:             "Create an implementation handoff.",
			},
		},
		Artifacts: []tcworker.HandoffArtifact{
			{
				ArtifactVersionRef: "tc://artifact-version/prior_execution",
				MessageRef:         "tc://message/msg_prior",
				AttemptRef:         "tc://attempt/att_prior",
				Summary:            "skill=tc://skill/implementer IMPLEMENTER_OUTPUT_OK",
				Stdout:             "IMPLEMENTER_OUTPUT_OK",
			},
		},
	}
	result, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute AI CLI: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted || !strings.Contains(result.Stdout, "reviewer saw prior implementer output") {
		t.Fatalf("expected AI CLI to receive handoff context, got %+v", result)
	}
}

func TestAICLIExecutorCanPassPromptAsArgument(t *testing.T) {
	command := writeFakeAIScript(t, `#!/bin/sh
set -eu
input="$*"
case "$input" in
  *"message_ref: tc://message/msg_ai"*payload.body:*) echo "AI CLI received prompt argument";;
  *) echo "missing prompt argument context" >&2; exit 17;;
esac
`)
	executor, err := tcworker.NewAICLIExecutor(tcworker.AICLIExecutorOptions{
		Command: command,
		Args:    []string{"{{prompt}}"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create AI CLI executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("execute AI CLI: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted || !strings.Contains(result.Stdout, "prompt argument") {
		t.Fatalf("expected prompt argument execution, got %+v", result)
	}
}

func TestWorkerEnvSelectsAICLIExecutor(t *testing.T) {
	command := writeFakeAIScript(t, `#!/bin/sh
cat >/dev/null
echo "env selected AI CLI"
`)
	t.Setenv("TC_WORKER_EXECUTOR", "ai-cli")
	t.Setenv("TC_WORKER_AI_CLI_COMMAND", command)
	t.Setenv("TC_WORKER_AI_CLI_TIMEOUT", "1s")

	executor, err := tcworker.ExecutorFromEnv()
	if err != nil {
		t.Fatalf("create env AI CLI executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), llmExecutionInput())
	if err != nil {
		t.Fatalf("execute env AI CLI: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted || !strings.Contains(result.Summary, "env selected AI CLI") {
		t.Fatalf("expected completed env AI CLI result, got %+v", result)
	}
}

func TestSkillExecutorAddsGuidanceBeforeCallingBackend(t *testing.T) {
	skill := contracts.SkillDefinition{
		SkillRef:     "tc://skill/design-review",
		Name:         "Design Review",
		Kind:         contracts.SkillKindGuidance,
		Capabilities: []string{"frontend.design-review"},
		Body:         "Check layout geometry before copy. Verify desktop and mobile.",
	}
	var seen tcworker.ExecutionInput
	backend := executorFunc(func(_ context.Context, input tcworker.ExecutionInput) (tcworker.ExecutionResult, error) {
		seen = input
		return tcworker.ExecutionResult{
			Outcome: tcworker.ExecutionOutcomeCompleted,
			Summary: "design guidance applied",
			Stdout:  "review complete",
		}, nil
	})
	executor, err := tcworker.NewSkillExecutor(tcworker.SkillExecutorOptions{
		Skills:  []tcworker.SkillDefinition{skill},
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("create skill executor: %v", err)
	}
	input := llmExecutionInput()
	input.TargetCapability = "frontend.design-review"
	input.Payload.Body = "Review this UI patch."
	result, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute skill: %v", err)
	}
	if len(result.UsedSkillRefs) != 1 || result.UsedSkillRefs[0] != skill.SkillRef {
		t.Fatalf("expected used skill ref, got %+v", result)
	}
	if !strings.Contains(result.Summary, "skill=tc://skill/design-review") {
		t.Fatalf("expected skill marker in summary, got %q", result.Summary)
	}
	if !strings.Contains(seen.Payload.Body, "SKILL.md instructions") || !strings.Contains(seen.Payload.Body, "Check layout geometry") || !strings.Contains(seen.Payload.Body, "Original message body") {
		t.Fatalf("expected skill guidance to be injected into backend input, got %q", seen.Payload.Body)
	}
}

func TestSkillExecutorReloadsSkillsBeforeExecution(t *testing.T) {
	codeSkill := contracts.SkillDefinition{
		SkillRef:     "tc://skill/code-worker",
		Name:         "Code Worker",
		Kind:         contracts.SkillKindGuidance,
		Capabilities: []string{"code.change"},
		Body:         "Change code safely.",
	}
	reviewSkill := contracts.SkillDefinition{
		SkillRef:     "tc://skill/review-worker",
		Name:         "Review Worker",
		Kind:         contracts.SkillKindGuidance,
		Capabilities: []string{"ai.review"},
		Body:         "Review work carefully.",
	}
	loaded := []contracts.SkillDefinition{codeSkill}
	backend := executorFunc(func(_ context.Context, input tcworker.ExecutionInput) (tcworker.ExecutionResult, error) {
		return tcworker.ExecutionResult{
			Outcome: tcworker.ExecutionOutcomeCompleted,
			Summary: "backend completed",
			Stdout:  input.Payload.Body,
		}, nil
	})
	executor, err := tcworker.NewSkillExecutor(tcworker.SkillExecutorOptions{
		Reload: func() ([]contracts.SkillDefinition, error) {
			return append([]contracts.SkillDefinition(nil), loaded...), nil
		},
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("create reloadable skill executor: %v", err)
	}

	input := llmExecutionInput()
	input.TargetCapability = "code.change"
	result, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute initial skill: %v", err)
	}
	if len(result.UsedSkillRefs) != 1 || result.UsedSkillRefs[0] != codeSkill.SkillRef {
		t.Fatalf("expected initial code skill, got %+v", result)
	}

	loaded = []contracts.SkillDefinition{reviewSkill}
	input.TargetCapability = "ai.review"
	result, err = executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute reloaded skill: %v", err)
	}
	if len(result.UsedSkillRefs) != 1 || result.UsedSkillRefs[0] != reviewSkill.SkillRef {
		t.Fatalf("expected reloaded review skill, got %+v", result)
	}
	capabilities, err := executor.RefreshCapabilities(context.Background())
	if err != nil {
		t.Fatalf("refresh capabilities: %v", err)
	}
	if len(capabilities) != 1 || capabilities[0].Name != "ai.review" {
		t.Fatalf("expected reloaded advertised capability, got %+v", capabilities)
	}
}

func writeFakeAIScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-ai-cli.sh")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("write fake AI CLI: %v", err)
	}
	return path
}

func llmExecutionInput() tcworker.ExecutionInput {
	return tcworker.ExecutionInput{
		MessageRef:       "tc://message/msg_ai",
		AttemptRef:       "tc://attempt/att_ai",
		TargetCapability: "ai.generate",
		CorrelationRef:   "tc://task/ai_handoff",
		Payload: contracts.Payload{
			Summary: "AI handoff smoke",
			Body:    "Draft a short implementation note and hand it to the reviewer.",
		},
		Constraints: []contracts.Constraint{
			{Code: "preserve_intent", Summary: "Do not drop the user's requested outcome."},
		},
	}
}

func writeOpenAIResponsesText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":     "resp_test",
		"status": "completed",
		"output": []map[string]any{
			{
				"type":   "message",
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": text},
				},
			},
		},
	})
}

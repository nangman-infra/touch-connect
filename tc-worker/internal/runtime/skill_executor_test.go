package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestSkillExecutorReloadsSkillsAndAdvertisesCapabilities(t *testing.T) {
	codeSkill := testSkill("tc://skill/code", "Code Worker", "code.change", "Change code safely.")
	reviewSkill := testSkill("tc://skill/review", "Review Worker", "ai.review", "Review work carefully.")
	loaded := []contracts.SkillDefinition{codeSkill}
	backend := staticExecutor{result: ExecutionResult{
		Outcome: ExecutionOutcomeCompleted,
		Summary: "backend completed",
		Stdout:  "backend output",
	}}
	executor, err := NewSkillExecutor(SkillExecutorOptions{
		Reload: func() ([]contracts.SkillDefinition, error) {
			return append([]contracts.SkillDefinition(nil), loaded...), nil
		},
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("create skill executor: %v", err)
	}

	input := ExecutionInput{
		MessageRef:       "tc://message/msg_skill",
		AttemptRef:       "tc://attempt/att_skill",
		TargetCapability: "code.change",
		Payload: contracts.Payload{
			Summary: "skill execution",
			Body:    "Apply the selected skill.",
		},
	}
	result, err := executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute initial skill: %v", err)
	}
	if len(result.UsedSkillRefs) != 1 || result.UsedSkillRefs[0] != codeSkill.SkillRef {
		t.Fatalf("expected code skill ref, got %+v", result)
	}
	if !strings.Contains(result.Stdout, "used_skill_ref="+codeSkill.SkillRef) {
		t.Fatalf("expected stdout to carry used skill marker, got %q", result.Stdout)
	}

	loaded = []contracts.SkillDefinition{reviewSkill}
	input.TargetCapability = "ai.review"
	result, err = executor.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute reloaded skill: %v", err)
	}
	if len(result.UsedSkillRefs) != 1 || result.UsedSkillRefs[0] != reviewSkill.SkillRef {
		t.Fatalf("expected reloaded review skill ref, got %+v", result)
	}
	capabilities, err := executor.RefreshCapabilities(context.Background())
	if err != nil {
		t.Fatalf("refresh capabilities: %v", err)
	}
	if len(capabilities) != 1 || capabilities[0].Name != "ai.review" {
		t.Fatalf("expected reloaded advertised capability, got %+v", capabilities)
	}
}

func TestSkillExecutorReturnsStructuredFailures(t *testing.T) {
	executor, err := NewSkillExecutor(SkillExecutorOptions{
		Skills: []contracts.SkillDefinition{testSkill("tc://skill/code", "Code Worker", "code.change", "Change code safely.")},
	})
	if err != nil {
		t.Fatalf("create skill executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), ExecutionInput{TargetCapability: "ai.review"})
	if err != nil {
		t.Fatalf("execute missing skill: %v", err)
	}
	if result.Outcome != ExecutionOutcomeFailed || result.FailureReasonCode != "skill_not_found" {
		t.Fatalf("expected skill_not_found failure, got %+v", result)
	}

	reloadErr := errors.New("registry unreadable")
	executor, err = NewSkillExecutor(SkillExecutorOptions{
		Skills: []contracts.SkillDefinition{testSkill("tc://skill/code", "Code Worker", "code.change", "Change code safely.")},
		Reload: func() ([]contracts.SkillDefinition, error) {
			return nil, reloadErr
		},
	})
	if err != nil {
		t.Fatalf("create reload-failing skill executor: %v", err)
	}
	result, err = executor.Execute(context.Background(), ExecutionInput{TargetCapability: "code.change"})
	if err != nil {
		t.Fatalf("execute reload-failing skill: %v", err)
	}
	if result.Outcome != ExecutionOutcomeFailed || result.FailureReasonCode != "skill_reload_failed" || !strings.Contains(result.Stderr, reloadErr.Error()) {
		t.Fatalf("expected skill_reload_failed result, got %+v", result)
	}
}

func TestSkillAugmentedBodyIncludesGuidanceFields(t *testing.T) {
	skill := testSkill("tc://skill/design", "Design Review", "frontend.design-review", "Check layout geometry.")
	skill.Description = "review UI structure"
	skill.AppliesTo = []string{"terminal-ui"}
	skill.ApprovalRequired = true
	skill.ExecutorHint = "ai-cli"
	body := skillAugmentedBody(skill, ExecutionInput{
		Payload: contracts.Payload{Body: "Original task body."},
	})
	for _, want := range []string{"skill_ref: tc://skill/design", "skill_description: review UI structure", "skill_applies_to: terminal-ui", "skill_approval_required: true", "skill_executor_hint: ai-cli", "Original task body."} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected augmented body to include %q, got %s", want, body)
		}
	}
}

func testSkill(ref string, name string, capability string, body string) contracts.SkillDefinition {
	return contracts.SkillDefinition{
		SkillRef:     ref,
		Name:         name,
		Kind:         contracts.SkillKindGuidance,
		Capabilities: []string{capability},
		Body:         body,
	}
}

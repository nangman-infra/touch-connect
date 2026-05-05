package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/internal/communication/skills"
)

type SkillExecutorOptions struct {
	Skills  []contracts.SkillDefinition
	Backend WorkerExecutor
}

type SkillExecutor struct {
	skills  []contracts.SkillDefinition
	backend WorkerExecutor
}

func NewSkillExecutor(options SkillExecutorOptions) (*SkillExecutor, error) {
	if len(options.Skills) == 0 {
		return nil, errors.New("at least one skill is required")
	}
	loaded := make([]contracts.SkillDefinition, 0, len(options.Skills))
	for _, skill := range options.Skills {
		accepted, err := skills.Validate(skill)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, accepted)
	}
	backend := options.Backend
	if backend == nil {
		backend = EchoExecutor{}
	}
	return &SkillExecutor{skills: loaded, backend: backend}, nil
}

func (e *SkillExecutor) Execute(ctx context.Context, input ExecutionInput) (ExecutionResult, error) {
	skill, ok := skills.MatchCapability(e.skills, input.TargetCapability)
	if !ok {
		return ExecutionResult{
			Outcome:           ExecutionOutcomeFailed,
			Summary:           "no registered skill matches target capability",
			FailureReasonCode: "skill_not_found",
		}, nil
	}
	augmented := input
	augmented.Payload.Body = skillAugmentedBody(skill, input)
	result, err := e.backend.Execute(ctx, augmented)
	if err != nil {
		return result, err
	}
	result.UsedSkillRefs = appendUnique(result.UsedSkillRefs, skill.SkillRef)
	result.Summary = skillSummary(skill, result.Summary)
	if result.Stdout != "" {
		result.Stdout = "used_skill_ref=" + skill.SkillRef + "\n" + result.Stdout
	}
	return result, nil
}

func skillAugmentedBody(skill contracts.SkillDefinition, input ExecutionInput) string {
	var builder strings.Builder
	builder.WriteString("touch-connect skill guidance\n")
	builder.WriteString("skill_ref: ")
	builder.WriteString(skill.SkillRef)
	builder.WriteByte('\n')
	builder.WriteString("skill_name: ")
	builder.WriteString(skill.Name)
	builder.WriteByte('\n')
	builder.WriteString("skill_kind: ")
	builder.WriteString(skill.Kind)
	builder.WriteByte('\n')
	if skill.Description != "" {
		builder.WriteString("skill_description: ")
		builder.WriteString(skill.Description)
		builder.WriteByte('\n')
	}
	if len(skill.Capabilities) > 0 {
		builder.WriteString("skill_capabilities: ")
		builder.WriteString(strings.Join(skill.Capabilities, ", "))
		builder.WriteByte('\n')
	}
	if len(skill.AppliesTo) > 0 {
		builder.WriteString("skill_applies_to: ")
		builder.WriteString(strings.Join(skill.AppliesTo, ", "))
		builder.WriteByte('\n')
	}
	if skill.ApprovalRequired {
		builder.WriteString("skill_approval_required: true\n")
	}
	if skill.ExecutorHint != "" {
		builder.WriteString("skill_executor_hint: ")
		builder.WriteString(skill.ExecutorHint)
		builder.WriteByte('\n')
	}
	builder.WriteString("\nSKILL.md instructions:\n")
	builder.WriteString(skill.Body)
	builder.WriteString("\n\nOriginal message body:\n")
	builder.WriteString(input.Payload.Body)
	return builder.String()
}

func skillSummary(skill contracts.SkillDefinition, summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "skill-guided execution completed"
	}
	return "skill=" + skill.SkillRef + " " + summary
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

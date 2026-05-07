package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/internal/communication/skills"
)

type SkillExecutorOptions struct {
	Skills  []contracts.SkillDefinition
	Backend WorkerExecutor
	Reload  func() ([]contracts.SkillDefinition, error)
}

type SkillExecutor struct {
	skills  []contracts.SkillDefinition
	backend WorkerExecutor
	reload  func() ([]contracts.SkillDefinition, error)
	mu      sync.RWMutex
}

func NewSkillExecutor(options SkillExecutorOptions) (*SkillExecutor, error) {
	loaded := options.Skills
	if len(loaded) == 0 && options.Reload != nil {
		var err error
		loaded, err = options.Reload()
		if err != nil {
			return nil, err
		}
	}
	accepted, err := validateSkillDefinitions(loaded)
	if err != nil {
		return nil, err
	}
	backend := options.Backend
	if backend == nil {
		backend = EchoExecutor{}
	}
	return &SkillExecutor{skills: accepted, backend: backend, reload: options.Reload}, nil
}

func (e *SkillExecutor) Execute(ctx context.Context, input ExecutionInput) (ExecutionResult, error) {
	loaded, err := e.currentSkills()
	if err != nil {
		return ExecutionResult{
			Outcome:           ExecutionOutcomeFailed,
			Summary:           "skill reload failed",
			FailureReasonCode: "skill_reload_failed",
			Stderr:            err.Error(),
		}, nil
	}
	skill, ok := skills.MatchCapability(loaded, input.TargetCapability)
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

func (e *SkillExecutor) RefreshCapabilities(_ context.Context) ([]contracts.Capability, error) {
	loaded, err := e.currentSkills()
	if err != nil {
		return nil, err
	}
	return capabilitiesFromSkillDefinitions(loaded), nil
}

func (e *SkillExecutor) currentSkills() ([]contracts.SkillDefinition, error) {
	if e.reload == nil {
		return e.snapshotSkills(), nil
	}
	return e.reloadSkills()
}

func (e *SkillExecutor) snapshotSkills() []contracts.SkillDefinition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]contracts.SkillDefinition(nil), e.skills...)
}

func (e *SkillExecutor) reloadSkills() ([]contracts.SkillDefinition, error) {
	loaded, err := e.reload()
	if err != nil {
		return nil, err
	}
	accepted, err := validateSkillDefinitions(loaded)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.skills = accepted
	e.mu.Unlock()
	return append([]contracts.SkillDefinition(nil), accepted...), nil
}

func validateSkillDefinitions(items []contracts.SkillDefinition) ([]contracts.SkillDefinition, error) {
	if len(items) == 0 {
		return nil, errors.New("at least one skill is required")
	}
	loaded := make([]contracts.SkillDefinition, 0, len(items))
	for _, skill := range items {
		accepted, err := skills.Validate(skill)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, accepted)
	}
	return loaded, nil
}

func capabilitiesFromSkillDefinitions(items []contracts.SkillDefinition) []contracts.Capability {
	capabilities := make([]contracts.Capability, 0)
	seen := map[string]struct{}{}
	for _, skill := range items {
		for _, name := range skill.Capabilities {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			capabilities = append(capabilities, contracts.Capability{
				Name:           name,
				ExecutionHints: []string{"checkpoint_progress", "skill_guided", "ai_execution"},
			})
		}
	}
	return capabilities
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

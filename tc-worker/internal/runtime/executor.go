package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	ExecutionOutcomeCompleted     = "completed"
	ExecutionOutcomeMissingFields = "missing_fields"
	ExecutionOutcomeFailed        = "failed"
	ExecutionOutcomeDropped       = "dropped"
)

type WorkerExecutor interface {
	Execute(context.Context, ExecutionInput) (ExecutionResult, error)
}

type ExecutionInput struct {
	MessageRef         string
	AttemptRef         string
	TargetCapability   string
	CorrelationRef     string
	Payload            contracts.Payload
	Constraints        []contracts.Constraint
	HandoffContext     HandoffContext
	Takeover           bool
	RedeliveryCount    int
	LastCheckpointRef  string
	ResumeSummary      string
	ResumeArtifactRefs []string
}

type HandoffContext struct {
	TaskRef   string
	Messages  []HandoffMessage
	Artifacts []HandoffArtifact
}

type HandoffMessage struct {
	MessageRef       string
	TargetCapability string
	State            string
	AttemptRef       string
	RedeliveryCount  int
	Summary          string
	Body             string
}

type HandoffArtifact struct {
	ArtifactVersionRef string
	ArtifactRef        string
	MessageRef         string
	AttemptRef         string
	Kind               string
	MediaType          string
	StorageRef         string
	Summary            string
	Stdout             string
	Stderr             string
	UsedSkillRefs      []string
	Content            string
}

func (c HandoffContext) Empty() bool {
	return c.TaskRef == "" && len(c.Messages) == 0 && len(c.Artifacts) == 0
}

type ExecutionResult struct {
	Outcome           string
	Summary           string
	ArtifactRefs      []string
	UsedSkillRefs     []string
	MissingFields     []MissingField
	FailureReasonCode string
	Stdout            string
	Stderr            string
	ExitCode          int
	DurationMS        int64
}

type EchoExecutor struct{}

func (EchoExecutor) Execute(_ context.Context, input ExecutionInput) (ExecutionResult, error) {
	for _, constraint := range input.Constraints {
		switch constraint.Code {
		case "worker.missing_field":
			name := strings.TrimSpace(constraint.SourceRef)
			if name == "" {
				name = "required_input"
			}
			reason := strings.TrimSpace(constraint.Details)
			if reason == "" {
				reason = constraint.Summary
			}
			return ExecutionResult{
				Outcome: ExecutionOutcomeMissingFields,
				Summary: "processing blocked because required information is missing",
				MissingFields: []MissingField{
					{Name: name, Reason: reason},
				},
			}, nil
		case "worker.fail":
			reason := strings.TrimSpace(constraint.Details)
			if reason == "" {
				reason = "executor_failed"
			}
			return ExecutionResult{
				Outcome:           ExecutionOutcomeFailed,
				Summary:           constraint.Summary,
				FailureReasonCode: reason,
			}, nil
		}
	}
	return ExecutionResult{
		Outcome: ExecutionOutcomeCompleted,
		Summary: "message completed",
	}, nil
}

func (r ExecutionResult) validated() (ExecutionResult, error) {
	if r.Summary == "" {
		r.Summary = "worker execution result"
	}
	switch r.Outcome {
	case "":
		r.Outcome = ExecutionOutcomeCompleted
	case ExecutionOutcomeCompleted:
		return r, nil
	case ExecutionOutcomeMissingFields:
		if len(r.MissingFields) == 0 {
			return ExecutionResult{}, errors.New("missing field result requires at least one missing field")
		}
		return r, nil
	case ExecutionOutcomeFailed:
		if r.FailureReasonCode == "" {
			return ExecutionResult{}, errors.New("failed result requires failure reason code")
		}
		return r, nil
	default:
		return ExecutionResult{}, errors.New("unknown worker execution outcome")
	}
	return r, nil
}

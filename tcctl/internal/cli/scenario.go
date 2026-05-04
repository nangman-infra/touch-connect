package cli

import (
	"context"
	"fmt"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

type scenarioCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type scenarioReport struct {
	Name   string          `json:"name"`
	Task   string          `json:"task"`
	Passed bool            `json:"passed"`
	Checks []scenarioCheck `json:"checks"`
}

func (r Runtime) scenario(ctx context.Context, args []string) error {
	if err := requireArgs(args, 2, "tcctl scenario <run|verify> canonical"); err != nil {
		return err
	}
	if args[1] != "canonical" {
		return usageError(fmt.Errorf("only canonical scenario is supported"))
	}
	switch args[0] {
	case "run":
		return r.runCanonicalScenario(ctx, args[2:])
	case "verify":
		return r.verifyCanonicalScenario(ctx, args[2:])
	default:
		return usageError(fmt.Errorf("unknown scenario command %q", args[0]))
	}
}

func (r Runtime) runCanonicalScenario(ctx context.Context, args []string) error {
	flags := commandFlagSet("scenario run canonical", r.stderr)
	taskRef := flags.String("task", "tc://task/canonical", "task/correlation ref")
	capability := flags.String("capability", "code.change", "target capability")
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	summary := flags.String("summary", "Canonical handoff", "message summary")
	body := flags.String("body", "Run validation, produce artifacts, request approval, then execute the protected side effect.", "message body")
	if err := flags.Parse(args); err != nil {
		return usageError(err)
	}
	value, err := r.client.SendMessage(ctx, contracts.MessageIngressRequest{
		SenderEndpointRef: *sender,
		TargetCapability:  *capability,
		CorrelationRef:    *taskRef,
		ReadbackRequired:  true,
		Constraints:       []contracts.Constraint{{Code: "canonical_scenario", Summary: "prove readback, artifact, approval, and side-effect lineage"}},
		Payload: contracts.Payload{
			Summary:    *summary,
			Body:       *body,
			References: []contracts.Reference{},
		},
	})
	if err != nil {
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.MessageRef, value.State, value.DeliveryRef)
	return nil
}

func (r Runtime) verifyCanonicalScenario(ctx context.Context, args []string) error {
	flags := commandFlagSet("scenario verify canonical", r.stderr)
	taskRef := flags.String("task", "tc://task/canonical", "task/correlation ref")
	if err := flags.Parse(args); err != nil {
		return usageError(err)
	}
	messages, err := r.client.Messages(ctx, *taskRef)
	if err != nil {
		return unavailableError(err)
	}
	artifacts, err := r.client.Artifacts(ctx, *taskRef)
	if err != nil {
		return unavailableError(err)
	}
	approvals, err := r.client.Approvals(ctx)
	if err != nil {
		return unavailableError(err)
	}
	sideEffects, err := r.client.SideEffects(ctx, *taskRef)
	if err != nil {
		return unavailableError(err)
	}
	report := scenarioReport{Name: "canonical", Task: *taskRef}
	report.Checks = append(report.Checks,
		check("readback_required_handoff", hasReadbackRequiredMessage(messages), "at least one task message requires readback"),
		check("at_least_two_artifacts", len(artifacts) >= 2, fmt.Sprintf("artifact_versions=%d", len(artifacts))),
		check("approval_approved", hasApprovedApproval(approvals), "at least one approval is approved"),
		check("side_effect_succeeded", hasSucceededSideEffect(sideEffects), "at least one task side effect succeeded"),
		check("final_task_completed", allMessagesCompleted(messages), "all task messages are completed"),
	)
	report.Passed = allChecksPassed(report.Checks)
	return output.WriteJSON(r.stdout, report)
}

func check(name string, passed bool, detail string) scenarioCheck {
	return scenarioCheck{Name: name, Passed: passed, Detail: detail}
}

func hasReadbackRequiredMessage(items []contracts.MessageRecord) bool {
	for _, item := range items {
		if item.ReadbackRequired {
			return true
		}
	}
	return false
}

func hasApprovedApproval(items []contracts.ApprovalRecord) bool {
	for _, item := range items {
		if item.Status == "approved" {
			return true
		}
	}
	return false
}

func hasSucceededSideEffect(items []contracts.SideEffectRecord) bool {
	for _, item := range items {
		if item.Status == "succeeded" {
			return true
		}
	}
	return false
}

func allMessagesCompleted(items []contracts.MessageRecord) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.State != "completed" {
			return false
		}
	}
	return true
}

func allChecksPassed(items []scenarioCheck) bool {
	for _, item := range items {
		if !item.Passed {
			return false
		}
	}
	return true
}

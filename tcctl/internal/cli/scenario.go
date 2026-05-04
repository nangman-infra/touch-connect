package cli

import (
	"context"
	"fmt"
	"io"
	"time"

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

type scenarioRunReport struct {
	Name    string                           `json:"name"`
	Task    string                           `json:"task"`
	Message contracts.MessageIngressResponse `json:"message"`
	Passed  bool                             `json:"passed"`
	Checks  []scenarioCheck                  `json:"checks"`
}

func (r Runtime) scenario(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeScenarioHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.scenarioCommandHelp(args[1:])
	}
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
	flags := commandFlagSet("scenario run canonical [flags]", r.stderr)
	taskRef := flags.String("task", "tc://task/canonical", "task/correlation ref")
	capability := flags.String("capability", "code.change", "target capability")
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	summary := flags.String("summary", "Canonical handoff", "message summary")
	body := flags.String("body", "Run validation, produce artifacts, request approval, then execute the protected side effect.", "message body")
	wait := flags.Bool("wait", true, "wait until canonical verification passes")
	waitTimeout := flags.Duration("wait-timeout", 10*time.Second, "maximum time to wait for canonical verification")
	pollInterval := flags.Duration("poll-interval", 200*time.Millisecond, "verification poll interval while waiting")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
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
	if *wait {
		report, err := r.waitForCanonicalScenario(ctx, *taskRef, *waitTimeout, *pollInterval)
		if err != nil {
			return err
		}
		runReport := scenarioRunReport{
			Name:    "canonical",
			Task:    *taskRef,
			Message: value,
			Passed:  report.Passed,
			Checks:  report.Checks,
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, runReport)
		}
		fmt.Fprintf(r.stdout, "%s\t%s\t%s\tpassed=%t\n", value.MessageRef, value.State, value.DeliveryRef, report.Passed)
		return nil
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.MessageRef, value.State, value.DeliveryRef)
	return nil
}

func (r Runtime) verifyCanonicalScenario(ctx context.Context, args []string) error {
	flags := commandFlagSet("scenario verify canonical [flags]", r.stderr)
	taskRef := flags.String("task", "tc://task/canonical", "task/correlation ref")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	report, err := r.canonicalScenarioReport(ctx, *taskRef)
	if err != nil {
		return err
	}
	return output.WriteJSON(r.stdout, report)
}

func (r Runtime) waitForCanonicalScenario(ctx context.Context, taskRef string, timeout time.Duration, pollInterval time.Duration) (scenarioReport, error) {
	if timeout <= 0 {
		return scenarioReport{}, usageError(fmt.Errorf("--wait-timeout must be positive"))
	}
	if pollInterval <= 0 {
		return scenarioReport{}, usageError(fmt.Errorf("--poll-interval must be positive"))
	}
	deadline := time.Now().Add(timeout)
	var latest scenarioReport
	for {
		report, err := r.canonicalScenarioReport(ctx, taskRef)
		if err != nil {
			return scenarioReport{}, err
		}
		latest = report
		if report.Passed {
			return report, nil
		}
		if !time.Now().Before(deadline) {
			return latest, commandError(fmt.Errorf("canonical scenario did not pass before timeout: %s", firstFailedCheck(latest.Checks)))
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return latest, commandError(ctx.Err())
		case <-timer.C:
		}
	}
}

func (r Runtime) canonicalScenarioReport(ctx context.Context, taskRef string) (scenarioReport, error) {
	snapshot, err := r.client.Snapshot(ctx)
	if err != nil {
		return scenarioReport{}, unavailableError(err)
	}
	messages := messagesForTask(snapshot.Messages, taskRef)
	attemptRefs := attemptRefsForMessages(messages)
	artifacts := artifactsForTask(snapshot.Artifacts, taskRef)
	approvals := approvalsForAttempts(snapshot.Approvals, attemptRefs)
	sideEffects := sideEffectsForTask(snapshot.SideEffects, taskRef)
	readbacks := readbacksForAttempts(snapshot.Readbacks, attemptRefs)
	report := scenarioReport{Name: "canonical", Task: taskRef}
	report.Checks = append(report.Checks,
		check("readback_required_handoff", hasReadbackRequiredMessage(messages) && len(readbacks) > 0, fmt.Sprintf("readback_required_messages=%d readbacks=%d", countReadbackRequiredMessages(messages), len(readbacks))),
		check("at_least_two_artifacts", len(artifacts) >= 2, fmt.Sprintf("artifact_versions=%d", len(artifacts))),
		check("approval_approved", hasApprovedApproval(approvals), fmt.Sprintf("approvals=%d", len(approvals))),
		check("side_effect_succeeded", hasSucceededSideEffect(sideEffects), fmt.Sprintf("side_effects=%d", len(sideEffects))),
		check("final_task_completed", allMessagesCompleted(messages), "all task messages are completed"),
	)
	report.Passed = allChecksPassed(report.Checks)
	return report, nil
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

func countReadbackRequiredMessages(items []contracts.MessageRecord) int {
	count := 0
	for _, item := range items {
		if item.ReadbackRequired {
			count++
		}
	}
	return count
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

func messagesForTask(items []contracts.MessageRecord, taskRef string) []contracts.MessageRecord {
	matches := make([]contracts.MessageRecord, 0)
	for _, item := range items {
		if item.CorrelationRef == taskRef {
			matches = append(matches, item)
		}
	}
	return matches
}

func attemptRefsForMessages(items []contracts.MessageRecord) map[string]bool {
	refs := map[string]bool{}
	for _, item := range items {
		if item.AttemptRef != "" {
			refs[item.AttemptRef] = true
		}
	}
	return refs
}

func artifactsForTask(items []contracts.ArtifactRecord, taskRef string) []contracts.ArtifactRecord {
	matches := make([]contracts.ArtifactRecord, 0)
	for _, item := range items {
		if item.TaskRef == taskRef {
			matches = append(matches, item)
		}
	}
	return matches
}

func approvalsForAttempts(items []contracts.ApprovalRecord, attempts map[string]bool) []contracts.ApprovalRecord {
	matches := make([]contracts.ApprovalRecord, 0)
	for _, item := range items {
		if attempts[item.AttemptRef] {
			matches = append(matches, item)
		}
	}
	return matches
}

func sideEffectsForTask(items []contracts.SideEffectRecord, taskRef string) []contracts.SideEffectRecord {
	matches := make([]contracts.SideEffectRecord, 0)
	for _, item := range items {
		if item.TaskRef == taskRef {
			matches = append(matches, item)
		}
	}
	return matches
}

func readbacksForAttempts(items []contracts.ReadbackRecord, attempts map[string]bool) []contracts.ReadbackRecord {
	matches := make([]contracts.ReadbackRecord, 0)
	for _, item := range items {
		if attempts[item.AttemptRef] {
			matches = append(matches, item)
		}
	}
	return matches
}

func firstFailedCheck(items []scenarioCheck) string {
	for _, item := range items {
		if !item.Passed {
			return item.Name + " (" + item.Detail + ")"
		}
	}
	return "unknown"
}

func writeScenarioHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl scenario <run|verify> canonical")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  run canonical      create the canonical handoff and wait for worker evidence")
	fmt.Fprintln(w, "  verify canonical   verify canonical server records")
}

func (r Runtime) scenarioCommandHelp(args []string) error {
	if len(args) == 0 {
		writeScenarioHelp(r.stdout)
		return nil
	}
	if len(args) >= 2 && args[1] != "canonical" {
		return usageError(fmt.Errorf("only canonical scenario is supported"))
	}
	switch args[0] {
	case "run":
		return r.runCanonicalScenario(context.Background(), []string{"-h"})
	case "verify":
		return r.verifyCanonicalScenario(context.Background(), []string{"-h"})
	default:
		return usageError(fmt.Errorf("unknown scenario command %q", args[0]))
	}
}

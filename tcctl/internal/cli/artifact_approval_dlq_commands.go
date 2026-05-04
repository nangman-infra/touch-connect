package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

func (r Runtime) artifact(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeArtifactHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.artifactCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl artifact <list|inspect|lineage|finalize>"); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		flags := commandFlagSet("artifact list [flags]", r.stderr)
		taskRef := flags.String("task", "", "task ref filter")
		if err := parseCommandFlags(flags, args[1:]); err != nil {
			return err
		}
		value, err := r.client.Artifacts(ctx, *taskRef)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteArtifacts(r.stdout, value)
	case "inspect":
		if err := requireArgs(args[1:], 1, "tcctl artifact inspect <artifact_version_ref>"); err != nil {
			return err
		}
		value, err := r.client.Artifact(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteArtifact(r.stdout, value)
	case "lineage":
		if err := requireArgs(args[1:], 1, "tcctl artifact lineage <artifact_ref|artifact_version_ref|message_ref|task_ref>"); err != nil {
			return err
		}
		value, err := r.client.ArtifactLineage(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteArtifactLineage(r.stdout, value)
	case "finalize":
		return r.finalizeArtifact(ctx, args[1:])
	default:
		return usageError(fmt.Errorf("unknown artifact command %q", args[0]))
	}
	return nil
}

func (r Runtime) finalizeArtifact(ctx context.Context, args []string) error {
	if helpOnly(args) {
		args = []string{"-h"}
	}
	flags := commandFlagSet("artifact finalize <artifact_version_ref> --actor ACTOR [flags]", r.stderr)
	actor := flags.String("actor", "", "actor id finalizing the artifact")
	reason := flags.String("reason", "", "finalization reason")
	artifactVersionRef, flagArgs := splitFirstPositionalArg(args)
	if err := parseCommandFlags(flags, flagArgs); err != nil {
		return err
	}
	if artifactVersionRef == "" || *actor == "" {
		flags.Usage()
		return usageError(fmt.Errorf("usage: tcctl artifact finalize <artifact_version_ref> --actor ACTOR"))
	}
	value, err := r.client.FinalizeArtifact(ctx, contracts.ArtifactFinalizeRequest{
		ArtifactVersionRef: artifactVersionRef,
		ActorID:            *actor,
		Reason:             *reason,
	})
	if err != nil {
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.ArtifactVersionRef, value.State, value.FinalizationRef)
	return nil
}

func (r Runtime) approval(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeApprovalHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.approvalCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl approval <list|inspect|chain|approve|reject>"); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		value, err := r.client.Approvals(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteApprovals(r.stdout, value)
	case "inspect":
		if err := requireArgs(args[1:], 1, "tcctl approval inspect <approval_ref>"); err != nil {
			return err
		}
		value, err := r.client.Approval(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteApproval(r.stdout, value)
	case "chain":
		if err := requireArgs(args[1:], 1, "tcctl approval chain <approval_ref|message_ref|attempt_ref|target_ref>"); err != nil {
			return err
		}
		value, err := r.client.ApprovalChain(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteApprovalChain(r.stdout, value)
	case "approve":
		return r.recordApproval(ctx, args[1:], "approved")
	case "reject":
		return r.recordApproval(ctx, args[1:], "rejected")
	default:
		return usageError(fmt.Errorf("unknown approval command %q", args[0]))
	}
	return nil
}

func (r Runtime) recordApproval(ctx context.Context, args []string, status string) error {
	if helpOnly(args) {
		args = []string{"-h"}
	}
	action := "approve"
	if status == "rejected" {
		action = "reject"
	}
	flags := commandFlagSet("approval "+action+" <approval_ref> --attempt-ref REF --target-ref REF --requested-by ACTOR --approvers ROLE --scope SCOPE --hash HASH --decided-by ACTOR [flags]", r.stderr)
	attemptRef := flags.String("attempt-ref", "", "attempt ref")
	targetType := flags.String("target-type", "side_effect", "approval target type")
	targetRef := flags.String("target-ref", "", "approval target ref")
	requestedBy := flags.String("requested-by", "", "actor that requested approval")
	approvers := flags.String("approvers", "", "comma-separated approver subjects or roles")
	scope := flags.String("scope", "", "approval scope")
	hash := flags.String("hash", "", "approval hash")
	decidedBy := flags.String("decided-by", "", "actor that made the decision")
	note := flags.String("note", "", "decision note")
	reason := flags.String("reason", "", "rejection reason")
	expiresAt := flags.String("expires-at", "", "RFC3339Nano expiration")
	approvalRef, flagArgs := splitFirstPositionalArg(args)
	if err := parseCommandFlags(flags, flagArgs); err != nil {
		return err
	}
	if approvalRef == "" {
		flags.Usage()
		return usageError(fmt.Errorf("usage: tcctl approval %s <approval_ref> --attempt-ref REF --target-ref REF --requested-by ACTOR --approvers ROLE --scope SCOPE --hash HASH --decided-by ACTOR", action))
	}
	if *attemptRef == "" || *targetRef == "" || *requestedBy == "" || *approvers == "" || *scope == "" || *hash == "" || *decidedBy == "" {
		flags.Usage()
		return usageError(fmt.Errorf("--attempt-ref, --target-ref, --requested-by, --approvers, --scope, --hash, and --decided-by are required"))
	}
	req := contracts.ApprovalCommandRequest{
		AttemptRef:              *attemptRef,
		ApprovalRef:             approvalRef,
		TargetType:              *targetType,
		TargetRef:               *targetRef,
		RequestedByActorID:      *requestedBy,
		ApproverSubjectsOrRoles: splitCSV(*approvers),
		ApprovalScope:           *scope,
		ApprovalHash:            *hash,
		Status:                  status,
		Reason:                  *reason,
		ExpiresAt:               *expiresAt,
		DecidedByActorID:        *decidedBy,
		DecisionNote:            *note,
	}
	value, err := r.client.RecordApproval(ctx, req)
	if err != nil {
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.ApprovalRef, value.Status, value.DecidedByActorID)
	return nil
}

func (r Runtime) dlq(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeDLQHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.dlqCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl dlq <list|inspect|replay>"); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		value, err := r.client.DeadLetters(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteDeadLetters(r.stdout, value)
	case "inspect":
		if err := requireArgs(args[1:], 1, "tcctl dlq inspect <dead_letter_ref>"); err != nil {
			return err
		}
		value, err := r.client.DeadLetter(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteDeadLetter(r.stdout, value)
	case "replay":
		if err := requireArgs(args[1:], 1, "tcctl dlq replay <dead_letter_ref>"); err != nil {
			return err
		}
		value, err := r.client.ReplayDeadLetter(ctx, contracts.DLQReplayRequest{DeadLetterRef: args[1]})
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.DeadLetterRef, value.MessageRef, value.State)
	default:
		return usageError(fmt.Errorf("unknown dlq command %q", args[0]))
	}
	return nil
}

func writeArtifactHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl artifact <list|inspect|lineage|finalize>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  list [--task <task_ref>]                  list artifact versions")
	fmt.Fprintln(w, "  inspect <artifact_version_ref>            inspect one artifact version")
	fmt.Fprintln(w, "  lineage <ref>                             show artifact lineage graph")
	fmt.Fprintln(w, "  finalize <artifact_version_ref> --actor   finalize an artifact version")
}

func (r Runtime) artifactCommandHelp(args []string) error {
	if len(args) == 0 {
		writeArtifactHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return r.artifact(context.Background(), []string{"list", "-h"})
	case "inspect":
		fmt.Fprintln(r.stdout, "usage: tcctl artifact inspect <artifact_version_ref>")
	case "lineage":
		fmt.Fprintln(r.stdout, "usage: tcctl artifact lineage <artifact_ref|artifact_version_ref|message_ref|task_ref>")
	case "finalize":
		return r.finalizeArtifact(context.Background(), []string{"-h"})
	default:
		return usageError(fmt.Errorf("unknown artifact command %q", args[0]))
	}
	return nil
}

func writeApprovalHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl approval <list|inspect|chain|approve|reject>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  list       list approval records")
	fmt.Fprintln(w, "  inspect    inspect one approval record")
	fmt.Fprintln(w, "  chain      show approval chain")
	fmt.Fprintln(w, "  approve    record an approved decision")
	fmt.Fprintln(w, "  reject     record a rejected decision")
}

func (r Runtime) approvalCommandHelp(args []string) error {
	if len(args) == 0 {
		writeApprovalHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "list":
		fmt.Fprintln(r.stdout, "usage: tcctl approval list")
	case "inspect":
		fmt.Fprintln(r.stdout, "usage: tcctl approval inspect <approval_ref>")
	case "chain":
		fmt.Fprintln(r.stdout, "usage: tcctl approval chain <approval_ref|message_ref|attempt_ref|target_ref>")
	case "approve":
		return r.recordApproval(context.Background(), []string{"-h"}, "approved")
	case "reject":
		return r.recordApproval(context.Background(), []string{"-h"}, "rejected")
	default:
		return usageError(fmt.Errorf("unknown approval command %q", args[0]))
	}
	return nil
}

func writeDLQHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl dlq <list|inspect|replay>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  list                         list dead-letter records")
	fmt.Fprintln(w, "  inspect <dead_letter_ref>    inspect one dead-letter record")
	fmt.Fprintln(w, "  replay <dead_letter_ref>     replay one dead-letter record")
}

func (r Runtime) dlqCommandHelp(args []string) error {
	if len(args) == 0 {
		writeDLQHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "list":
		fmt.Fprintln(r.stdout, "usage: tcctl dlq list")
	case "inspect":
		fmt.Fprintln(r.stdout, "usage: tcctl dlq inspect <dead_letter_ref>")
	case "replay":
		fmt.Fprintln(r.stdout, "usage: tcctl dlq replay <dead_letter_ref>")
	default:
		return usageError(fmt.Errorf("unknown dlq command %q", args[0]))
	}
	return nil
}

func splitFirstPositionalArg(args []string) (string, []string) {
	positional := ""
	flagArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if positional == "" && !strings.HasPrefix(arg, "-") {
			positional = arg
			continue
		}
		flagArgs = append(flagArgs, arg)
	}
	return positional, flagArgs
}

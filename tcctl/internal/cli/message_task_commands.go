package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

func (r Runtime) message(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeMessageHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.messageCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl message <send|list|inspect|history>"); err != nil {
		return err
	}
	switch args[0] {
	case "send":
		return r.sendMessage(ctx, args[1:])
	case "list":
		return r.listMessages(ctx, args[1:])
	case "inspect":
		if err := requireArgs(args[1:], 1, "tcctl message inspect <message_ref>"); err != nil {
			return err
		}
		value, err := r.client.Message(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteMessage(r.stdout, value)
	case "history":
		return r.listMessages(ctx, args[1:])
	default:
		return usageError(fmt.Errorf("unknown message command %q", args[0]))
	}
	return nil
}

func (r Runtime) sendMessage(ctx context.Context, args []string) error {
	flags := commandFlagSet("message send --capability CAP --summary TEXT --body TEXT [flags]", r.stderr)
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	capability := flags.String("capability", "", "target capability")
	summary := flags.String("summary", "", "payload summary")
	body := flags.String("body", "", "payload body")
	taskRef := flags.String("task", "", "task/correlation ref")
	messageRef := flags.String("message-ref", "", "optional message ref")
	readbackRequired := flags.Bool("readback-required", false, "require worker readback")
	qualityGate := flags.String("quality-gate", contracts.QualityGateEnforce.String(), "quality gate mode: enforce, warn, or skip")
	constraint := flags.String("constraint", "", "optional code:summary constraint")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	gate, err := contracts.ParseQualityGateMode(*qualityGate)
	if err != nil {
		return usageError(err)
	}
	if *capability == "" || *summary == "" || *body == "" {
		flags.Usage()
		return usageError(fmt.Errorf("--capability, --summary, and --body are required"))
	}
	req := contracts.MessageIngressRequest{
		MessageRef:        *messageRef,
		SenderEndpointRef: *sender,
		TargetCapability:  *capability,
		CorrelationRef:    *taskRef,
		ReadbackRequired:  *readbackRequired,
		QualityGate:       gate,
		Constraints:       []contracts.Constraint{},
		Payload: contracts.Payload{
			Summary:    *summary,
			Body:       *body,
			References: []contracts.Reference{},
		},
	}
	if *constraint != "" {
		req.Constraints = []contracts.Constraint{parseConstraint(*constraint)}
	}
	value, err := r.client.SendMessage(ctx, req)
	if err != nil {
		return messageSendError(err, r.stderr)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.MessageRef, value.State, value.DeliveryRef)
	return nil
}

func messageSendError(err error, stderr io.Writer) error {
	var apiErr contracts.APIError
	if errors.As(err, &apiErr) && apiErr.Response.Code == contracts.ErrorCodeQualityRejected && apiErr.Response.QualityDecision != nil {
		decision := apiErr.Response.QualityDecision
		fmt.Fprintf(stderr, "quality gate rejected %s decision=%s\n", decision.QualityDecisionRef, decision.Decision)
		for _, violation := range decision.Violations {
			fmt.Fprintf(stderr, "- %s field=%s severity=%s: %s\n", violation.Code, violation.Field, violation.Severity, violation.Detail)
		}
		return commandError(err)
	}
	return unavailableError(err)
}

func (r Runtime) listMessages(ctx context.Context, args []string) error {
	flags := commandFlagSet("message list [flags]", r.stderr)
	taskRef := flags.String("task", "", "task/correlation ref filter")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	value, err := r.client.Messages(ctx, *taskRef)
	if err != nil {
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	output.WriteMessages(r.stdout, value)
	return nil
}

func (r Runtime) task(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeTaskHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.taskCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl task <create|status|history|cancel|retry>"); err != nil {
		return err
	}
	switch args[0] {
	case "create":
		return r.createTask(ctx, args[1:])
	case "status":
		if err := requireArgs(args[1:], 1, "tcctl task status <task_ref>"); err != nil {
			return err
		}
		value, err := r.client.TaskStatus(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		return output.WriteJSON(r.stdout, value)
	case "history":
		if err := requireArgs(args[1:], 1, "tcctl task history <task_ref>"); err != nil {
			return err
		}
		value, err := r.client.TaskHistory(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		return output.WriteJSON(r.stdout, value)
	case "cancel":
		return r.taskCommand(ctx, args[1:], "cancel")
	case "retry":
		return r.taskCommand(ctx, args[1:], "retry")
	default:
		return usageError(fmt.Errorf("unknown task command %q", args[0]))
	}
}

func (r Runtime) createTask(ctx context.Context, args []string) error {
	if helpOnly(args) {
		flags := commandFlagSet("task create <task_ref> --capability CAP --summary TEXT --body TEXT [flags]", r.stderr)
		flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
		flags.String("capability", "", "target capability")
		flags.String("summary", "", "payload summary")
		flags.String("body", "", "payload body")
		flags.Bool("readback-required", true, "require worker readback")
		flags.Usage()
		return errHelpRequested
	}
	if err := requireArgs(args, 1, "tcctl task create <task_ref> --capability CAP --summary TEXT --body TEXT"); err != nil {
		return err
	}
	flags := commandFlagSet("task create <task_ref> --capability CAP --summary TEXT --body TEXT [flags]", r.stderr)
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	capability := flags.String("capability", "", "target capability")
	summary := flags.String("summary", "", "payload summary")
	body := flags.String("body", "", "payload body")
	readbackRequired := flags.Bool("readback-required", true, "require worker readback")
	if err := parseCommandFlags(flags, args[1:]); err != nil {
		return err
	}
	if *capability == "" || *summary == "" || *body == "" {
		flags.Usage()
		return usageError(fmt.Errorf("--capability, --summary, and --body are required"))
	}
	value, err := r.client.SendMessage(ctx, contracts.MessageIngressRequest{
		SenderEndpointRef: *sender,
		TargetCapability:  *capability,
		CorrelationRef:    args[0],
		ReadbackRequired:  *readbackRequired,
		Constraints:       []contracts.Constraint{},
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
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", args[0], value.MessageRef, value.State, value.DeliveryRef)
	return nil
}

func (r Runtime) taskCommand(ctx context.Context, args []string, action string) error {
	if err := requireArgs(args, 1, "tcctl task "+action+" <task_ref>"); err != nil {
		return err
	}
	req := contracts.TaskCommandRequest{TaskRef: args[0]}
	var value contracts.TaskCommandResponse
	var err error
	switch action {
	case "cancel":
		value, err = r.client.CancelTask(ctx, req)
	case "retry":
		value, err = r.client.RetryTask(ctx, req)
	default:
		return usageError(fmt.Errorf("unknown task action %q", action))
	}
	if err != nil {
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\tmessages=%d\tattempts=%d\n", value.TaskRef, value.State, value.AffectedMessages, value.AffectedAttempts)
	return nil
}

func writeMessageHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl message <send|list|inspect|history>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  send      create a message handoff")
	fmt.Fprintln(w, "  list      list messages, optionally filtered by task")
	fmt.Fprintln(w, "  inspect   inspect one message")
	fmt.Fprintln(w, "  history   alias for list")
}

func (r Runtime) messageCommandHelp(args []string) error {
	if len(args) == 0 {
		writeMessageHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "send":
		return r.sendMessage(context.Background(), []string{"-h"})
	case "list", "history":
		return r.listMessages(context.Background(), []string{"-h"})
	case "inspect":
		fmt.Fprintln(r.stdout, "usage: tcctl message inspect <message_ref>")
		return nil
	default:
		return usageError(fmt.Errorf("unknown message command %q", args[0]))
	}
}

func writeTaskHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl task <create|status|history|cancel|retry>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  create <task_ref>   create the initial task handoff")
	fmt.Fprintln(w, "  status <task_ref>   show task projection")
	fmt.Fprintln(w, "  history <task_ref>  show task messages, attempts, and artifacts")
	fmt.Fprintln(w, "  cancel <task_ref>   cancel task messages")
	fmt.Fprintln(w, "  retry <task_ref>    explicitly retry task messages")
}

func (r Runtime) taskCommandHelp(args []string) error {
	if len(args) == 0 {
		writeTaskHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "create":
		return r.createTask(context.Background(), []string{"-h"})
	case "status":
		fmt.Fprintln(r.stdout, "usage: tcctl task status <task_ref>")
	case "history":
		fmt.Fprintln(r.stdout, "usage: tcctl task history <task_ref>")
	case "cancel":
		fmt.Fprintln(r.stdout, "usage: tcctl task cancel <task_ref>")
	case "retry":
		fmt.Fprintln(r.stdout, "usage: tcctl task retry <task_ref>")
	default:
		return usageError(fmt.Errorf("unknown task command %q", args[0]))
	}
	return nil
}

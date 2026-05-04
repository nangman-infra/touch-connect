package cli

import (
	"context"
	"fmt"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

func (r Runtime) message(ctx context.Context, args []string) error {
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
	flags := commandFlagSet("message send", r.stderr)
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	capability := flags.String("capability", "", "target capability")
	summary := flags.String("summary", "", "payload summary")
	body := flags.String("body", "", "payload body")
	taskRef := flags.String("task", "", "task/correlation ref")
	messageRef := flags.String("message-ref", "", "optional message ref")
	readbackRequired := flags.Bool("readback-required", false, "require worker readback")
	constraint := flags.String("constraint", "", "optional code:summary constraint")
	if err := flags.Parse(args); err != nil {
		return usageError(err)
	}
	if *capability == "" || *summary == "" || *body == "" {
		return usageError(fmt.Errorf("--capability, --summary, and --body are required"))
	}
	req := contracts.MessageIngressRequest{
		MessageRef:        *messageRef,
		SenderEndpointRef: *sender,
		TargetCapability:  *capability,
		CorrelationRef:    *taskRef,
		ReadbackRequired:  *readbackRequired,
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
		return unavailableError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, value)
	}
	fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", value.MessageRef, value.State, value.DeliveryRef)
	return nil
}

func (r Runtime) listMessages(ctx context.Context, args []string) error {
	flags := commandFlagSet("message list", r.stderr)
	taskRef := flags.String("task", "", "task/correlation ref filter")
	if err := flags.Parse(args); err != nil {
		return usageError(err)
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
	if err := requireArgs(args, 1, "tcctl task create <task_ref> --capability CAP --summary TEXT --body TEXT"); err != nil {
		return err
	}
	flags := commandFlagSet("task create", r.stderr)
	sender := flags.String("sender", "tc://endpoint/tcctl", "sender endpoint ref")
	capability := flags.String("capability", "", "target capability")
	summary := flags.String("summary", "", "payload summary")
	body := flags.String("body", "", "payload body")
	readbackRequired := flags.Bool("readback-required", true, "require worker readback")
	if err := flags.Parse(args[1:]); err != nil {
		return usageError(err)
	}
	if *capability == "" || *summary == "" || *body == "" {
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

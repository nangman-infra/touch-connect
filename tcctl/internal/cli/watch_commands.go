package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type watchState struct {
	messages    map[string]contracts.MessageRecord
	attempts    map[string]contracts.AttemptRecord
	checkpoints map[string]contracts.CheckpointRecord
	readbacks   map[string]contracts.ReadbackRecord
	artifacts   map[string]contracts.ArtifactRecord
}

func newWatchState() watchState {
	return watchState{
		messages:    map[string]contracts.MessageRecord{},
		attempts:    map[string]contracts.AttemptRecord{},
		checkpoints: map[string]contracts.CheckpointRecord{},
		readbacks:   map[string]contracts.ReadbackRecord{},
		artifacts:   map[string]contracts.ArtifactRecord{},
	}
}

func (r Runtime) watchTask(ctx context.Context, args []string) error {
	flags := commandFlagSet("task watch <task_ref> [flags]", r.stderr)
	interval := flags.Duration("interval", time.Second, "poll interval")
	once := flags.Bool("once", false, "print current matching events once and exit")
	stream := flags.Bool("stream", true, "use tc-control server-sent event stream")
	if helpOnly(args) {
		flags.Usage()
		return errHelpRequested
	}
	if err := requireArgs(args, 1, "tcctl task watch <task_ref> [flags]"); err != nil {
		return err
	}
	if err := parseCommandFlags(flags, args[1:]); err != nil {
		return err
	}
	if *interval <= 0 {
		return usageError(fmt.Errorf("--interval must be positive"))
	}
	return r.watchSnapshots(ctx, watchOptions{
		TaskRef:  args[0],
		Interval: *interval,
		Once:     *once,
		Stream:   *stream,
	})
}

func (r Runtime) tailMessages(ctx context.Context, args []string) error {
	flags := commandFlagSet("message tail [flags]", r.stderr)
	taskRef := flags.String("task", "", "task/correlation ref filter")
	capability := flags.String("capability", "", "target capability filter")
	interval := flags.Duration("interval", time.Second, "poll interval")
	once := flags.Bool("once", false, "print current matching events once and exit")
	stream := flags.Bool("stream", true, "use tc-control server-sent event stream")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	if *interval <= 0 {
		return usageError(fmt.Errorf("--interval must be positive"))
	}
	return r.watchSnapshots(ctx, watchOptions{
		TaskRef:    *taskRef,
		Capability: *capability,
		Interval:   *interval,
		Once:       *once,
		Stream:     *stream,
	})
}

type watchOptions struct {
	TaskRef    string
	Capability string
	Interval   time.Duration
	Once       bool
	Stream     bool
}

func (r Runtime) watchSnapshots(ctx context.Context, options watchOptions) error {
	if options.Stream && !options.Once {
		return r.watchEventStream(ctx, options)
	}
	state := newWatchState()
	for {
		snapshot, err := r.client.Snapshot(ctx)
		if err != nil {
			return unavailableError(err)
		}
		events := diffWatchSnapshot(state, snapshot, options)
		for _, event := range events {
			fmt.Fprintln(r.stdout, event)
		}
		if options.Once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(options.Interval):
		}
	}
}

func (r Runtime) watchEventStream(ctx context.Context, options watchOptions) error {
	return r.client.StreamEvents(ctx, options.TaskRef, options.Capability, options.Interval, func(event contracts.EventRecord) error {
		fmt.Fprintln(r.stdout, formatStreamEvent(event))
		return nil
	})
}

func formatStreamEvent(event contracts.EventRecord) string {
	if event.Kind == "" {
		event.Kind = "event"
	}
	parts := []string{event.Kind}
	if event.Ref != "" {
		parts = append(parts, "ref="+event.Ref)
	}
	if event.MessageRef != "" && event.MessageRef != event.Ref {
		parts = append(parts, "msg="+event.MessageRef)
	}
	if event.AttemptRef != "" && event.AttemptRef != event.Ref {
		parts = append(parts, "attempt="+event.AttemptRef)
	}
	if event.EndpointRef != "" {
		parts = append(parts, "endpoint="+event.EndpointRef)
	}
	if event.TargetCapability != "" {
		parts = append(parts, "cap="+event.TargetCapability)
	}
	if event.State != "" {
		parts = append(parts, "state="+event.State)
	}
	if event.TaskRef != "" {
		parts = append(parts, "task="+event.TaskRef)
	}
	if event.Summary != "" {
		parts = append(parts, fmt.Sprintf("summary=%q", event.Summary))
	}
	return strings.Join(parts, " ")
}

func diffWatchSnapshot(state watchState, snapshot contracts.SnapshotResponse, options watchOptions) []string {
	var events []string
	for _, message := range sortedMessages(snapshot.Messages) {
		if !watchMessageMatches(message, options) {
			continue
		}
		previous, ok := state.messages[message.MessageRef]
		if !ok {
			events = append(events, formatMessageEvent("message", message))
		} else if previous.State != message.State || previous.AttemptRef != message.AttemptRef || previous.RedeliveryCount != message.RedeliveryCount {
			events = append(events, formatMessageEvent("message.update", message))
		}
		state.messages[message.MessageRef] = message
	}
	messageRefs := matchingMessageRefs(snapshot.Messages, options)
	attemptRefs := map[string]struct{}{}
	for _, attempt := range sortedAttempts(snapshot.Attempts) {
		if _, ok := messageRefs[attempt.MessageRef]; !ok {
			continue
		}
		attemptRefs[attempt.AttemptRef] = struct{}{}
		previous, ok := state.attempts[attempt.AttemptRef]
		if !ok {
			events = append(events, formatAttemptEvent("attempt", attempt))
		} else if previous.State != attempt.State || previous.Revision != attempt.Revision || previous.ClaimEpoch != attempt.ClaimEpoch {
			events = append(events, formatAttemptEvent("attempt.update", attempt))
		}
		state.attempts[attempt.AttemptRef] = attempt
	}
	for _, checkpoint := range sortedCheckpoints(snapshot.Checkpoints) {
		if _, ok := attemptRefs[checkpoint.AttemptRef]; !ok {
			continue
		}
		if _, ok := state.checkpoints[checkpoint.CheckpointRef]; ok {
			continue
		}
		state.checkpoints[checkpoint.CheckpointRef] = checkpoint
		events = append(events, formatCheckpointEvent(checkpoint))
	}
	for _, readback := range sortedReadbacks(snapshot.Readbacks) {
		if _, ok := attemptRefs[readback.AttemptRef]; !ok {
			continue
		}
		if _, ok := state.readbacks[readback.ReadbackRef]; ok {
			continue
		}
		state.readbacks[readback.ReadbackRef] = readback
		events = append(events, formatReadbackEvent(readback))
	}
	for _, artifact := range sortedArtifacts(snapshot.Artifacts) {
		if _, ok := messageRefs[artifact.MessageRef]; !ok {
			continue
		}
		if _, ok := state.artifacts[artifact.ArtifactVersionRef]; ok {
			continue
		}
		state.artifacts[artifact.ArtifactVersionRef] = artifact
		events = append(events, formatArtifactEvent(artifact))
	}
	return events
}

func watchMessageMatches(message contracts.MessageRecord, options watchOptions) bool {
	if options.TaskRef != "" && message.CorrelationRef != options.TaskRef {
		return false
	}
	if options.Capability != "" && message.TargetCapability != options.Capability {
		return false
	}
	return true
}

func matchingMessageRefs(messages []contracts.MessageRecord, options watchOptions) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, message := range messages {
		if watchMessageMatches(message, options) {
			refs[message.MessageRef] = struct{}{}
		}
	}
	return refs
}

func formatMessageEvent(kind string, message contracts.MessageRecord) string {
	return fmt.Sprintf("%s ref=%s cap=%s state=%s attempt=%s task=%s summary=%q",
		kind, message.MessageRef, message.TargetCapability, message.State, message.AttemptRef, message.CorrelationRef, message.Payload.Summary)
}

func formatAttemptEvent(kind string, attempt contracts.AttemptRecord) string {
	return fmt.Sprintf("%s ref=%s msg=%s endpoint=%s state=%s revision=%d epoch=%d",
		kind, attempt.AttemptRef, attempt.MessageRef, attempt.EndpointRef, attempt.State, attempt.Revision, attempt.ClaimEpoch)
}

func formatCheckpointEvent(checkpoint contracts.CheckpointRecord) string {
	return fmt.Sprintf("checkpoint ref=%s attempt=%s state=%s artifacts=%d summary=%q",
		checkpoint.CheckpointRef, checkpoint.AttemptRef, checkpoint.State, len(checkpoint.ArtifactRefs), checkpoint.Summary)
}

func formatReadbackEvent(readback contracts.ReadbackRecord) string {
	return fmt.Sprintf("readback ref=%s attempt=%s endpoint=%s understanding=%q missing=%s",
		readback.ReadbackRef, readback.AttemptRef, readback.EndpointRef, readback.Understanding, strings.Join(readback.MissingFields, ","))
}

func formatArtifactEvent(artifact contracts.ArtifactRecord) string {
	return fmt.Sprintf("artifact ref=%s task=%s msg=%s attempt=%s kind=%s storage=%s",
		artifact.ArtifactVersionRef, artifact.TaskRef, artifact.MessageRef, artifact.AttemptRef, artifact.Kind, artifact.StorageRef)
}

func sortedMessages(items []contracts.MessageRecord) []contracts.MessageRecord {
	out := append([]contracts.MessageRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MessageRef < out[j].MessageRef })
	return out
}

func sortedAttempts(items []contracts.AttemptRecord) []contracts.AttemptRecord {
	out := append([]contracts.AttemptRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].AttemptRef < out[j].AttemptRef })
	return out
}

func sortedCheckpoints(items []contracts.CheckpointRecord) []contracts.CheckpointRecord {
	out := append([]contracts.CheckpointRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CheckpointRef < out[j].CheckpointRef })
	return out
}

func sortedReadbacks(items []contracts.ReadbackRecord) []contracts.ReadbackRecord {
	out := append([]contracts.ReadbackRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ReadbackRef < out[j].ReadbackRef })
	return out
}

func sortedArtifacts(items []contracts.ArtifactRecord) []contracts.ArtifactRecord {
	out := append([]contracts.ArtifactRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ArtifactVersionRef < out[j].ArtifactVersionRef })
	return out
}

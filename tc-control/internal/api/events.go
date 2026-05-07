package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type eventStreamState struct {
	messages    map[string]contracts.MessageRecord
	attempts    map[string]contracts.AttemptRecord
	checkpoints map[string]contracts.CheckpointRecord
	readbacks   map[string]contracts.ReadbackRecord
	artifacts   map[string]contracts.ArtifactRecord
}

type eventStreamOptions struct {
	TaskRef    string
	Capability string
	Interval   time.Duration
	Once       bool
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "response writer does not support streaming")
		return
	}
	options, err := eventOptionsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	state := newEventStreamState()
	for {
		snapshot, err := h.service.Snapshot(r.Context())
		if err != nil {
			writeSSE(w, contracts.EventRecord{Kind: "error", Summary: err.Error()})
			flusher.Flush()
			return
		}
		for _, event := range diffEventSnapshot(state, snapshot, options) {
			writeSSE(w, event)
		}
		flusher.Flush()
		if options.Once {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(options.Interval):
		}
	}
}

func eventOptionsFromRequest(r *http.Request) (eventStreamOptions, error) {
	query := r.URL.Query()
	interval := time.Second
	if raw := strings.TrimSpace(query.Get("interval")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return eventStreamOptions{}, fmt.Errorf("interval must be a positive duration")
		}
		interval = parsed
	}
	return eventStreamOptions{
		TaskRef:    strings.TrimSpace(query.Get("task")),
		Capability: strings.TrimSpace(query.Get("capability")),
		Interval:   interval,
		Once:       query.Get("once") == "true" || query.Get("once") == "1",
	}, nil
}

func newEventStreamState() eventStreamState {
	return eventStreamState{
		messages:    map[string]contracts.MessageRecord{},
		attempts:    map[string]contracts.AttemptRecord{},
		checkpoints: map[string]contracts.CheckpointRecord{},
		readbacks:   map[string]contracts.ReadbackRecord{},
		artifacts:   map[string]contracts.ArtifactRecord{},
	}
}

func diffEventSnapshot(state eventStreamState, snapshot contracts.SnapshotResponse, options eventStreamOptions) []contracts.EventRecord {
	var events []contracts.EventRecord
	for _, message := range sortedEventMessages(snapshot.Messages) {
		if !eventMessageMatches(message, options) {
			continue
		}
		previous, ok := state.messages[message.MessageRef]
		if !ok {
			events = append(events, eventFromMessage("message", message))
		} else if previous.State != message.State || previous.AttemptRef != message.AttemptRef || previous.RedeliveryCount != message.RedeliveryCount {
			events = append(events, eventFromMessage("message.update", message))
		}
		state.messages[message.MessageRef] = message
	}
	messageRefs := eventMessageRefs(snapshot.Messages, options)
	attemptRefs := map[string]struct{}{}
	for _, attempt := range sortedEventAttempts(snapshot.Attempts) {
		if _, ok := messageRefs[attempt.MessageRef]; !ok {
			continue
		}
		attemptRefs[attempt.AttemptRef] = struct{}{}
		previous, ok := state.attempts[attempt.AttemptRef]
		if !ok {
			events = append(events, eventFromAttempt("attempt", attempt))
		} else if previous.State != attempt.State || previous.Revision != attempt.Revision || previous.ClaimEpoch != attempt.ClaimEpoch {
			events = append(events, eventFromAttempt("attempt.update", attempt))
		}
		state.attempts[attempt.AttemptRef] = attempt
	}
	for _, checkpoint := range sortedEventCheckpoints(snapshot.Checkpoints) {
		if _, ok := attemptRefs[checkpoint.AttemptRef]; !ok {
			continue
		}
		if _, ok := state.checkpoints[checkpoint.CheckpointRef]; ok {
			continue
		}
		state.checkpoints[checkpoint.CheckpointRef] = checkpoint
		events = append(events, contracts.EventRecord{
			Kind:       "checkpoint",
			Ref:        checkpoint.CheckpointRef,
			AttemptRef: checkpoint.AttemptRef,
			State:      checkpoint.State,
			Summary:    checkpoint.Summary,
		})
	}
	for _, readback := range sortedEventReadbacks(snapshot.Readbacks) {
		if _, ok := attemptRefs[readback.AttemptRef]; !ok {
			continue
		}
		if _, ok := state.readbacks[readback.ReadbackRef]; ok {
			continue
		}
		state.readbacks[readback.ReadbackRef] = readback
		events = append(events, contracts.EventRecord{
			Kind:        "readback",
			Ref:         readback.ReadbackRef,
			AttemptRef:  readback.AttemptRef,
			EndpointRef: readback.EndpointRef,
			Summary:     readback.Understanding,
		})
	}
	for _, artifact := range sortedEventArtifacts(snapshot.Artifacts) {
		if _, ok := messageRefs[artifact.MessageRef]; !ok {
			continue
		}
		if _, ok := state.artifacts[artifact.ArtifactVersionRef]; ok {
			continue
		}
		state.artifacts[artifact.ArtifactVersionRef] = artifact
		events = append(events, contracts.EventRecord{
			Kind:       "artifact",
			Ref:        artifact.ArtifactVersionRef,
			MessageRef: artifact.MessageRef,
			AttemptRef: artifact.AttemptRef,
			TaskRef:    artifact.TaskRef,
			Summary:    artifact.Kind,
		})
	}
	return events
}

func writeSSE(w http.ResponseWriter, event contracts.EventRecord) {
	payload, _ := json.Marshal(event)
	if event.Kind != "" {
		fmt.Fprintf(w, "event: %s\n", event.Kind)
	}
	fmt.Fprintf(w, "data: %s\n\n", payload)
}

func eventMessageMatches(message contracts.MessageRecord, options eventStreamOptions) bool {
	if options.TaskRef != "" && message.CorrelationRef != options.TaskRef {
		return false
	}
	if options.Capability != "" && message.TargetCapability != options.Capability {
		return false
	}
	return true
}

func eventMessageRefs(messages []contracts.MessageRecord, options eventStreamOptions) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, message := range messages {
		if eventMessageMatches(message, options) {
			refs[message.MessageRef] = struct{}{}
		}
	}
	return refs
}

func eventFromMessage(kind string, message contracts.MessageRecord) contracts.EventRecord {
	return contracts.EventRecord{
		Kind:             kind,
		Ref:              message.MessageRef,
		MessageRef:       message.MessageRef,
		AttemptRef:       message.AttemptRef,
		TargetCapability: message.TargetCapability,
		TaskRef:          message.CorrelationRef,
		State:            message.State,
		Summary:          message.Payload.Summary,
	}
}

func eventFromAttempt(kind string, attempt contracts.AttemptRecord) contracts.EventRecord {
	return contracts.EventRecord{
		Kind:        kind,
		Ref:         attempt.AttemptRef,
		AttemptRef:  attempt.AttemptRef,
		MessageRef:  attempt.MessageRef,
		EndpointRef: attempt.EndpointRef,
		State:       attempt.State,
	}
}

func sortedEventMessages(items []contracts.MessageRecord) []contracts.MessageRecord {
	out := append([]contracts.MessageRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MessageRef < out[j].MessageRef })
	return out
}

func sortedEventAttempts(items []contracts.AttemptRecord) []contracts.AttemptRecord {
	out := append([]contracts.AttemptRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].AttemptRef < out[j].AttemptRef })
	return out
}

func sortedEventCheckpoints(items []contracts.CheckpointRecord) []contracts.CheckpointRecord {
	out := append([]contracts.CheckpointRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CheckpointRef < out[j].CheckpointRef })
	return out
}

func sortedEventReadbacks(items []contracts.ReadbackRecord) []contracts.ReadbackRecord {
	out := append([]contracts.ReadbackRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ReadbackRef < out[j].ReadbackRef })
	return out
}

func sortedEventArtifacts(items []contracts.ArtifactRecord) []contracts.ArtifactRecord {
	out := append([]contracts.ArtifactRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ArtifactVersionRef < out[j].ArtifactVersionRef })
	return out
}

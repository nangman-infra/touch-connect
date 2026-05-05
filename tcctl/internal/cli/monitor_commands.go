package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func (r Runtime) monitor(ctx context.Context, args []string) error {
	flags := commandFlagSet("monitor [flags]", r.stderr)
	interval := flags.Duration("interval", time.Second, "poll interval")
	once := flags.Bool("once", false, "print one monitor frame and exit")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	if *interval <= 0 {
		return usageError(fmt.Errorf("--interval must be positive"))
	}
	for {
		snapshot, err := r.client.Snapshot(ctx)
		if err != nil {
			return unavailableError(err)
		}
		writeMonitorFrame(r.stdout, snapshot)
		if *once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*interval):
		}
	}
}

func writeMonitorFrame(w writer, snapshot contracts.SnapshotResponse) {
	fmt.Fprintf(w, "touch-connect monitor generated=%s source=%s\n", snapshot.Freshness.GeneratedAt, snapshot.Freshness.Source)
	writeMonitorEndpoints(w, snapshot.Endpoints)
	writeMonitorMessages(w, snapshot.Messages)
	writeMonitorTasks(w, snapshot.Messages)
	writeMonitorQuality(w, snapshot.QualityDecisions)
	writeMonitorArtifacts(w, snapshot.Artifacts)
}

type writer interface {
	Write([]byte) (int, error)
}

func writeMonitorEndpoints(w writer, endpoints []contracts.EndpointRecord) {
	online := 0
	for _, endpoint := range endpoints {
		if endpoint.ConnectionState == "online" {
			online++
		}
	}
	fmt.Fprintf(w, "workers online=%d total=%d\n", online, len(endpoints))
	for _, endpoint := range sortedEndpoints(endpoints) {
		fmt.Fprintf(w, "  worker ref=%s state=%s caps=%s hints=%s\n",
			endpoint.EndpointRef,
			endpoint.ConnectionState,
			strings.Join(endpointCapabilityNames(endpoint), ","),
			strings.Join(endpoint.ExecutionHints, ","),
		)
	}
}

func writeMonitorMessages(w writer, messages []contracts.MessageRecord) {
	counts := map[string]int{}
	for _, message := range messages {
		counts[message.State]++
	}
	fmt.Fprintf(w, "messages total=%d accepted=%d claimed=%d completed=%d blocked=%d failed=%d takeover_candidate=%d\n",
		len(messages),
		counts["accepted"],
		counts["claimed"],
		counts["completed"],
		counts["blocked"],
		counts["failed"],
		counts["takeover_candidate"],
	)
	for _, message := range lastMessages(messages, 5) {
		fmt.Fprintf(w, "  message ref=%s cap=%s state=%s task=%s summary=%q\n",
			message.MessageRef,
			message.TargetCapability,
			message.State,
			message.CorrelationRef,
			message.Payload.Summary,
		)
	}
}

func writeMonitorTasks(w writer, messages []contracts.MessageRecord) {
	type taskCounts struct {
		total     int
		completed int
		active    int
	}
	tasks := map[string]taskCounts{}
	for _, message := range messages {
		if message.CorrelationRef == "" {
			continue
		}
		counts := tasks[message.CorrelationRef]
		counts.total++
		if message.State == "completed" {
			counts.completed++
		} else {
			counts.active++
		}
		tasks[message.CorrelationRef] = counts
	}
	refs := make([]string, 0, len(tasks))
	for ref := range tasks {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	fmt.Fprintf(w, "tasks total=%d\n", len(refs))
	for _, ref := range refs {
		counts := tasks[ref]
		fmt.Fprintf(w, "  task ref=%s messages=%d active=%d completed=%d\n", ref, counts.total, counts.active, counts.completed)
	}
}

func writeMonitorQuality(w writer, decisions []contracts.QualityDecision) {
	counts := map[string]int{}
	for _, decision := range decisions {
		counts[decision.Decision]++
	}
	fmt.Fprintf(w, "quality total=%d passed=%d warned=%d rejected=%d review_required=%d clarification_required=%d\n",
		len(decisions),
		counts[contracts.QualityDecisionPassed],
		counts[contracts.QualityDecisionWarned],
		counts[contracts.QualityDecisionRejected],
		counts[contracts.QualityDecisionReviewRequired],
		counts[contracts.QualityDecisionClarificationRequired],
	)
}

func writeMonitorArtifacts(w writer, artifacts []contracts.ArtifactRecord) {
	fmt.Fprintf(w, "artifacts total=%d\n", len(artifacts))
	for _, artifact := range lastArtifacts(artifacts, 5) {
		fmt.Fprintf(w, "  artifact ref=%s task=%s msg=%s attempt=%s kind=%s\n",
			artifact.ArtifactVersionRef,
			artifact.TaskRef,
			artifact.MessageRef,
			artifact.AttemptRef,
			artifact.Kind,
		)
	}
}

func sortedEndpoints(items []contracts.EndpointRecord) []contracts.EndpointRecord {
	out := append([]contracts.EndpointRecord(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].EndpointRef < out[j].EndpointRef })
	return out
}

func endpointCapabilityNames(endpoint contracts.EndpointRecord) []string {
	names := make([]string, 0, len(endpoint.Capabilities))
	for name := range endpoint.Capabilities {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func lastMessages(items []contracts.MessageRecord, limit int) []contracts.MessageRecord {
	sorted := sortedMessages(items)
	if len(sorted) <= limit {
		return sorted
	}
	return sorted[len(sorted)-limit:]
}

func lastArtifacts(items []contracts.ArtifactRecord, limit int) []contracts.ArtifactRecord {
	sorted := sortedArtifacts(items)
	if len(sorted) <= limit {
		return sorted
	}
	return sorted[len(sorted)-limit:]
}

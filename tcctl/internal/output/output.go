package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func WriteJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func WriteHealth(w io.Writer, value contracts.HealthResponse) {
	fmt.Fprintf(w, "%s %s %s\n", value.Component, value.Status, value.Version)
}

func WriteVersion(w io.Writer, value contracts.VersionResponse) {
	fmt.Fprintf(w, "version=%s contract=%s minimum_worker=%s\n", value.Version, value.ContractVersion, value.MinimumWorker)
}

func WriteEndpoints(w io.Writer, items []contracts.EndpointRecord) {
	for _, item := range items {
		progress := item.ProgressSummary
		if item.CurrentAttemptRef != "" {
			progress = item.CurrentAttemptRef + " " + progress
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", item.EndpointRef, item.ConnectionState, item.DisplayName, item.WorkerVersion, strings.TrimSpace(progress))
	}
}

func WriteEndpoint(w io.Writer, item contracts.EndpointRecord) {
	fmt.Fprintf(w, "endpoint=%s\nstate=%s\nactor=%s\nworkspace=%s\ncapabilities=%s\ncurrent_attempt=%s\nlast_activity=%s\nprogress=%s\n",
		item.EndpointRef,
		item.ConnectionState,
		item.ActorID,
		item.WorkspaceID,
		strings.Join(sortedCapabilityNames(item.Capabilities), ","),
		item.CurrentAttemptRef,
		item.LastActivityAt,
		item.ProgressSummary,
	)
}

func WriteCapabilities(w io.Writer, items map[string][]string) {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "%s\t%s\n", name, strings.Join(items[name], ","))
	}
}

func WriteMessages(w io.Writer, items []contracts.MessageRecord) {
	for _, item := range items {
		target := item.TargetCapability
		if item.TargetEndpointRef != "" {
			target += "@" + item.TargetEndpointRef
		} else if item.PreferredEndpointRef != "" {
			target += "~" + item.PreferredEndpointRef
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.MessageRef, item.State, target, item.Payload.Summary)
	}
}

func WriteMessage(w io.Writer, item contracts.MessageRecord) {
	fmt.Fprintf(w, "message=%s\nstate=%s\ncapability=%s\ntarget_endpoint=%s\npreferred_endpoint=%s\ndepends_on=%s\nsummary=%s\ncorrelation=%s\n",
		item.MessageRef,
		item.State,
		item.TargetCapability,
		item.TargetEndpointRef,
		item.PreferredEndpointRef,
		strings.Join(item.DependsOnMessageRefs, ","),
		item.Payload.Summary,
		item.CorrelationRef,
	)
	if item.LatestQualityDecision == nil {
		fmt.Fprintln(w, "quality=none")
		return
	}
	fmt.Fprintf(w, "quality=%s\nquality_decision=%s\nquality_violations=%d\n",
		item.LatestQualityDecision.Decision,
		item.LatestQualityDecision.QualityDecisionRef,
		len(item.LatestQualityDecision.Violations),
	)
}

func WriteArtifacts(w io.Writer, items []contracts.ArtifactRecord) {
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", item.ArtifactVersionRef, item.TaskRef, item.Kind, item.SizeBytes)
	}
}

func WriteArtifact(w io.Writer, item contracts.ArtifactRecord) {
	fmt.Fprintf(w, "artifact_version=%s\nartifact=%s\ntask=%s\nkind=%s\nstorage=%s\n",
		item.ArtifactVersionRef,
		item.ArtifactRef,
		item.TaskRef,
		item.Kind,
		item.StorageRef,
	)
}

func WriteArtifactLineage(w io.Writer, item contracts.ArtifactLineage) {
	fmt.Fprintf(w, "lineage=%s\nquery=%s\nartifact=%s\ncurrent_version=%s\nversions=%d\nedges=%d\n",
		item.LineageRef,
		item.QueryRef,
		item.ArtifactRef,
		item.CurrentVersionRef,
		len(item.Versions),
		len(item.Edges),
	)
	for _, version := range item.Versions {
		fmt.Fprintf(w, "version\t%s\tartifact=%s\tmessage=%s\tattempt=%s\n",
			version.ArtifactVersionRef,
			version.ArtifactRef,
			version.MessageRef,
			version.AttemptRef,
		)
	}
	for _, edge := range item.Edges {
		fmt.Fprintf(w, "edge\t%s\t%s\t%s\n", edge.Relation, edge.FromRef, edge.ToRef)
	}
}

func WriteApprovals(w io.Writer, items []contracts.ApprovalRecord) {
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.ApprovalRef, item.Status, item.TargetType, item.TargetRef)
	}
}

func WriteApproval(w io.Writer, item contracts.ApprovalRecord) {
	fmt.Fprintf(w, "approval=%s\nstatus=%s\ntarget=%s:%s\nrequested_by=%s\ndecided_by=%s\n",
		item.ApprovalRef,
		item.Status,
		item.TargetType,
		item.TargetRef,
		item.RequestedByActorID,
		item.DecidedByActorID,
	)
}

func WriteApprovalChain(w io.Writer, item contracts.ApprovalChain) {
	currentStatus := ""
	currentRef := ""
	if item.Current != nil {
		currentStatus = item.Current.Status
		currentRef = item.Current.ApprovalRef
	}
	fmt.Fprintf(w, "chain=%s\nquery=%s\ncurrent=%s\nstatus=%s\ntarget=%s:%s\ndecisions=%d\n",
		item.ChainRef,
		item.QueryRef,
		currentRef,
		currentStatus,
		item.TargetType,
		item.TargetRef,
		len(item.Decisions),
	)
	for _, decision := range item.Decisions {
		fmt.Fprintf(w, "decision\t%s\t%s\tattempt=%s\tmessage=%s\tdecided_by=%s\n",
			decision.ApprovalRef,
			decision.Status,
			decision.AttemptRef,
			decision.MessageRef,
			decision.DecidedByActorID,
		)
	}
}

func WriteDeadLetters(w io.Writer, items []contracts.DeadLetterRecord) {
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", item.DeadLetterRef, item.MessageRef, item.Reason, item.RedeliveryCount)
	}
}

func WriteDeadLetter(w io.Writer, item contracts.DeadLetterRecord) {
	fmt.Fprintf(w, "dead_letter=%s\nmessage=%s\nlast_attempt=%s\nreason=%s\nredelivery_count=%d\n",
		item.DeadLetterRef,
		item.MessageRef,
		item.LastAttemptRef,
		item.Reason,
		item.RedeliveryCount,
	)
}

func sortedCapabilityNames(items map[string]contracts.Capability) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

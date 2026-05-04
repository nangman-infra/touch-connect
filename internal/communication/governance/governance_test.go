package governance

import (
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestBuildApprovalChainMatchesApprovalMessageAttemptOrTarget(t *testing.T) {
	approvals := []contracts.ApprovalRecord{
		{
			ApprovalRef:        "tc://approval/apr_1",
			AttemptRef:         "tc://attempt/att_1",
			MessageRef:         "tc://message/msg_1",
			TargetType:         "side_effect",
			TargetRef:          "tc://side-effect/se_1",
			Status:             "approved",
			RequestedAt:        "2026-05-05T00:00:00Z",
			DecidedAt:          "2026-05-05T00:00:01Z",
			DecidedByActorID:   "actor.approver",
			RequestedByActorID: "actor.requester",
		},
	}
	queries := []string{
		"tc://approval/apr_1",
		"tc://message/msg_1",
		"tc://attempt/att_1",
		"tc://side-effect/se_1",
	}
	for _, query := range queries {
		chain, ok := BuildApprovalChain(query, approvals)
		if !ok {
			t.Fatalf("expected approval chain for %s", query)
		}
		if chain.QueryRef != query || chain.Current == nil || chain.Current.ApprovalRef != "tc://approval/apr_1" || len(chain.Decisions) != 1 {
			t.Fatalf("unexpected chain for %s: %+v", query, chain)
		}
	}
}

func TestBuildArtifactLineageExpandsParentsAndChildren(t *testing.T) {
	artifacts := []contracts.ArtifactRecord{
		{
			ArtifactRef:        "tc://artifact/code_patch",
			ArtifactVersionRef: "tc://artifact-version/code_patch_v1",
			MessageRef:         "tc://message/msg_1",
			CreatedAt:          "2026-05-05T00:00:00Z",
			BasedOnMessageRefs: []string{"tc://message/msg_1"},
		},
		{
			ArtifactRef:                "tc://artifact/code_patch",
			ArtifactVersionRef:         "tc://artifact-version/code_patch_v2",
			MessageRef:                 "tc://message/msg_2",
			CreatedAt:                  "2026-05-05T00:00:01Z",
			BasedOnArtifactVersionRefs: []string{"tc://artifact-version/code_patch_v1"},
		},
	}
	lineage, ok := BuildArtifactLineage("tc://artifact-version/code_patch_v1", artifacts)
	if !ok {
		t.Fatalf("expected artifact lineage")
	}
	if lineage.CurrentVersionRef != "tc://artifact-version/code_patch_v2" || len(lineage.Versions) != 2 {
		t.Fatalf("expected parent and child versions, got %+v", lineage)
	}
	if !hasLineageEdge(lineage.Edges, "tc://message/msg_1", "tc://artifact-version/code_patch_v1", ArtifactRelationBasedOnMessage) {
		t.Fatalf("expected message lineage edge, got %+v", lineage.Edges)
	}
	if !hasLineageEdge(lineage.Edges, "tc://artifact-version/code_patch_v1", "tc://artifact-version/code_patch_v2", ArtifactRelationDerivedFrom) {
		t.Fatalf("expected artifact derived edge, got %+v", lineage.Edges)
	}
}

func hasLineageEdge(edges []contracts.ArtifactLineageEdge, from string, to string, relation string) bool {
	for _, edge := range edges {
		if edge.FromRef == from && edge.ToRef == to && edge.Relation == relation {
			return true
		}
	}
	return false
}

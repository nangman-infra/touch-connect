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

func TestBuildApprovalChainSortsPendingByRequestedAt(t *testing.T) {
	approvals := []contracts.ApprovalRecord{
		{
			ApprovalRef: "tc://approval/apr_old",
			MessageRef:  "tc://message/msg_1",
			TargetRef:   "tc://side-effect/se_1",
			Status:      "pending",
			RequestedAt: "2026-05-05T00:00:00Z",
		},
		{
			ApprovalRef: "tc://approval/apr_new",
			MessageRef:  "tc://message/msg_1",
			TargetRef:   "tc://side-effect/se_1",
			Status:      "pending",
			RequestedAt: "2026-05-05T00:01:00Z",
		},
	}
	chain, ok := BuildApprovalChain("tc://message/msg_1", approvals)
	if !ok || chain.Current == nil || chain.Current.ApprovalRef != "tc://approval/apr_new" {
		t.Fatalf("expected latest requested approval to be current, got %+v ok=%v", chain, ok)
	}
}

func TestBuildApprovalChainHandlesEmptyAndTieBreakers(t *testing.T) {
	if _, ok := BuildApprovalChain(" ", nil); ok {
		t.Fatal("empty approval query should not match")
	}
	approvals := []contracts.ApprovalRecord{
		{
			ApprovalRef: "tc://approval/apr_b",
			TargetRef:   "tc://side-effect/se_1",
			Status:      "approved",
			RequestedAt: "2026-05-05T00:00:00Z",
			DecidedAt:   "2026-05-05T00:01:00Z",
		},
		{
			ApprovalRef: "tc://approval/apr_a",
			TargetRef:   "tc://side-effect/se_1",
			Status:      "approved",
			RequestedAt: "2026-05-05T00:00:00Z",
			DecidedAt:   "2026-05-05T00:01:00Z",
		},
	}
	chain, ok := BuildApprovalChain("tc://side-effect/se_1", approvals)
	if !ok || chain.Decisions[0].ApprovalRef != "tc://approval/apr_a" {
		t.Fatalf("expected approval_ref tie breaker, got %+v ok=%v", chain, ok)
	}
	if _, ok := BuildApprovalChain("tc://approval/missing", approvals); ok {
		t.Fatal("missing approval query should not match")
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

func TestBuildArtifactLineageHandlesQueriesAndMissingParents(t *testing.T) {
	if _, ok := BuildArtifactLineage(" ", nil); ok {
		t.Fatal("empty artifact query should not match")
	}
	artifacts := []contracts.ArtifactRecord{
		{
			ArtifactRef:                "tc://artifact/a",
			ArtifactVersionRef:         "tc://artifact-version/a1",
			MessageRef:                 "tc://message/msg_1",
			AttemptRef:                 "tc://attempt/att_1",
			TaskRef:                    "tc://task/t",
			BasedOnArtifactVersionRefs: []string{"tc://artifact-version/missing"},
		},
		{
			ArtifactRef:        "tc://artifact/a",
			ArtifactVersionRef: "tc://artifact-version/a2",
			TaskRef:            "tc://task/t",
			CreatedAt:          "2026-05-05T00:01:00Z",
		},
	}
	for _, query := range []string{"tc://artifact/a", "tc://message/msg_1", "tc://attempt/att_1", "tc://task/t"} {
		lineage, ok := BuildArtifactLineage(query, artifacts)
		if !ok || lineage.QueryRef != query {
			t.Fatalf("expected lineage for %s, got %+v ok=%v", query, lineage, ok)
		}
	}
	lineage, ok := BuildArtifactLineage("tc://artifact-version/a1", artifacts)
	if !ok {
		t.Fatal("expected lineage for a1")
	}
	if hasLineageEdge(lineage.Edges, "tc://artifact-version/missing", "tc://artifact-version/a1", ArtifactRelationDerivedFrom) {
		t.Fatalf("missing parent should not create edge: %+v", lineage.Edges)
	}
	if _, ok := BuildArtifactLineage("tc://artifact/missing", artifacts); ok {
		t.Fatal("missing artifact query should not match")
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

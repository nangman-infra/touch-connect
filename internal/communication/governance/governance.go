package governance

import (
	"sort"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	ArtifactRelationBasedOnMessage = "based_on_message"
	ArtifactRelationDerivedFrom    = "derived_from"
)

func BuildApprovalChain(queryRef string, approvals []contracts.ApprovalRecord) (contracts.ApprovalChain, bool) {
	queryRef = strings.TrimSpace(queryRef)
	if queryRef == "" {
		return contracts.ApprovalChain{}, false
	}
	matches := make([]contracts.ApprovalRecord, 0)
	for _, approval := range approvals {
		if approvalMatches(queryRef, approval) {
			matches = append(matches, approval)
		}
	}
	if len(matches) == 0 {
		return contracts.ApprovalChain{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left := approvalSortKey(matches[i])
		right := approvalSortKey(matches[j])
		if left == right {
			return matches[i].ApprovalRef < matches[j].ApprovalRef
		}
		return left < right
	})
	current := matches[len(matches)-1]
	return contracts.ApprovalChain{
		ChainRef:   "tc://approval-chain/" + compactRef(queryRef),
		QueryRef:   queryRef,
		MessageRef: firstApprovalMessageRef(matches),
		AttemptRef: firstApprovalAttemptRef(matches),
		TargetType: firstApprovalTargetType(matches),
		TargetRef:  firstApprovalTargetRef(matches),
		Current:    &current,
		Decisions:  matches,
	}, true
}

func BuildArtifactLineage(queryRef string, artifacts []contracts.ArtifactRecord) (contracts.ArtifactLineage, bool) {
	queryRef = strings.TrimSpace(queryRef)
	if queryRef == "" {
		return contracts.ArtifactLineage{}, false
	}
	byVersion := map[string]contracts.ArtifactRecord{}
	included := map[string]bool{}
	for _, artifact := range artifacts {
		if artifact.ArtifactVersionRef != "" {
			byVersion[artifact.ArtifactVersionRef] = artifact
		}
		if artifactMatches(queryRef, artifact) {
			included[artifact.ArtifactVersionRef] = true
		}
	}
	if len(included) == 0 {
		return contracts.ArtifactLineage{}, false
	}
	expandArtifactClosure(included, artifacts, byVersion)
	versions := make([]contracts.ArtifactRecord, 0, len(included))
	for _, artifact := range artifacts {
		if included[artifact.ArtifactVersionRef] {
			versions = append(versions, artifact)
		}
	}
	sort.SliceStable(versions, func(i, j int) bool {
		left := artifactSortKey(versions[i])
		right := artifactSortKey(versions[j])
		if left == right {
			return versions[i].ArtifactVersionRef < versions[j].ArtifactVersionRef
		}
		return left < right
	})
	current := versions[len(versions)-1]
	edges := artifactLineageEdges(versions, included)
	return contracts.ArtifactLineage{
		LineageRef:        "tc://artifact-lineage/" + compactRef(queryRef),
		QueryRef:          queryRef,
		ArtifactRef:       firstArtifactRef(versions),
		CurrentVersionRef: current.ArtifactVersionRef,
		Versions:          versions,
		Edges:             edges,
	}, true
}

func approvalMatches(queryRef string, approval contracts.ApprovalRecord) bool {
	return approval.ApprovalRef == queryRef ||
		approval.MessageRef == queryRef ||
		approval.AttemptRef == queryRef ||
		approval.TargetRef == queryRef
}

func artifactMatches(queryRef string, artifact contracts.ArtifactRecord) bool {
	return artifact.ArtifactVersionRef == queryRef ||
		artifact.ArtifactRef == queryRef ||
		artifact.MessageRef == queryRef ||
		artifact.AttemptRef == queryRef ||
		artifact.TaskRef == queryRef
}

func expandArtifactClosure(included map[string]bool, artifacts []contracts.ArtifactRecord, byVersion map[string]contracts.ArtifactRecord) {
	changed := true
	for changed {
		changed = false
		for versionRef := range included {
			artifact := byVersion[versionRef]
			for _, parentRef := range artifact.BasedOnArtifactVersionRefs {
				if _, ok := byVersion[parentRef]; ok && !included[parentRef] {
					included[parentRef] = true
					changed = true
				}
			}
		}
		for _, artifact := range artifacts {
			if included[artifact.ArtifactVersionRef] {
				continue
			}
			for _, parentRef := range artifact.BasedOnArtifactVersionRefs {
				if included[parentRef] {
					included[artifact.ArtifactVersionRef] = true
					changed = true
					break
				}
			}
		}
	}
}

func artifactLineageEdges(versions []contracts.ArtifactRecord, included map[string]bool) []contracts.ArtifactLineageEdge {
	edges := make([]contracts.ArtifactLineageEdge, 0)
	for _, artifact := range versions {
		for _, messageRef := range artifact.BasedOnMessageRefs {
			edges = append(edges, contracts.ArtifactLineageEdge{
				FromRef:  messageRef,
				ToRef:    artifact.ArtifactVersionRef,
				Relation: ArtifactRelationBasedOnMessage,
			})
		}
		for _, parentRef := range artifact.BasedOnArtifactVersionRefs {
			if !included[parentRef] {
				continue
			}
			edges = append(edges, contracts.ArtifactLineageEdge{
				FromRef:  parentRef,
				ToRef:    artifact.ArtifactVersionRef,
				Relation: ArtifactRelationDerivedFrom,
			})
		}
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].FromRef != edges[j].FromRef {
			return edges[i].FromRef < edges[j].FromRef
		}
		if edges[i].ToRef != edges[j].ToRef {
			return edges[i].ToRef < edges[j].ToRef
		}
		return edges[i].Relation < edges[j].Relation
	})
	return edges
}

func approvalSortKey(approval contracts.ApprovalRecord) string {
	if approval.DecidedAt != "" {
		return approval.DecidedAt
	}
	return approval.RequestedAt
}

func artifactSortKey(artifact contracts.ArtifactRecord) string {
	if artifact.CreatedAt != "" {
		return artifact.CreatedAt
	}
	return artifact.ArtifactVersionRef
}

func firstApprovalMessageRef(items []contracts.ApprovalRecord) string {
	for _, item := range items {
		if item.MessageRef != "" {
			return item.MessageRef
		}
	}
	return ""
}

func firstApprovalAttemptRef(items []contracts.ApprovalRecord) string {
	for _, item := range items {
		if item.AttemptRef != "" {
			return item.AttemptRef
		}
	}
	return ""
}

func firstApprovalTargetType(items []contracts.ApprovalRecord) string {
	for _, item := range items {
		if item.TargetType != "" {
			return item.TargetType
		}
	}
	return ""
}

func firstApprovalTargetRef(items []contracts.ApprovalRecord) string {
	for _, item := range items {
		if item.TargetRef != "" {
			return item.TargetRef
		}
	}
	return ""
}

func firstArtifactRef(items []contracts.ArtifactRecord) string {
	for _, item := range items {
		if item.ArtifactRef != "" {
			return item.ArtifactRef
		}
	}
	return ""
}

func compactRef(ref string) string {
	ref = strings.TrimPrefix(ref, "tc://")
	replacer := strings.NewReplacer("/", "_", ":", "_")
	return replacer.Replace(ref)
}

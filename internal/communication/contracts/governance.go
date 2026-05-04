package contracts

type ApprovalChain struct {
	ChainRef   string           `json:"chain_ref"`
	QueryRef   string           `json:"query_ref"`
	MessageRef string           `json:"message_ref,omitempty"`
	AttemptRef string           `json:"attempt_ref,omitempty"`
	TargetType string           `json:"target_type,omitempty"`
	TargetRef  string           `json:"target_ref,omitempty"`
	Current    *ApprovalRecord  `json:"current,omitempty"`
	Decisions  []ApprovalRecord `json:"decisions"`
}

type ArtifactLineage struct {
	LineageRef        string                `json:"lineage_ref"`
	QueryRef          string                `json:"query_ref"`
	ArtifactRef       string                `json:"artifact_ref,omitempty"`
	CurrentVersionRef string                `json:"current_version_ref,omitempty"`
	Versions          []ArtifactRecord      `json:"versions"`
	Edges             []ArtifactLineageEdge `json:"edges,omitempty"`
}

type ArtifactLineageEdge struct {
	FromRef  string `json:"from_ref"`
	ToRef    string `json:"to_ref"`
	Relation string `json:"relation"`
}

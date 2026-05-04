package contracts

type ApprovalCommandRequest struct {
	AttemptRef              string   `json:"attempt_ref"`
	ApprovalRef             string   `json:"approval_ref"`
	TargetType              string   `json:"target_type"`
	TargetRef               string   `json:"target_ref"`
	RequestedByActorID      string   `json:"requested_by_actor_id"`
	ApproverSubjectsOrRoles []string `json:"approver_subjects_or_roles"`
	ApprovalScope           string   `json:"approval_scope"`
	ApprovalHash            string   `json:"approval_hash"`
	Status                  string   `json:"status"`
	Reason                  string   `json:"reason,omitempty"`
	ExpiresAt               string   `json:"expires_at,omitempty"`
	DecidedByActorID        string   `json:"decided_by_actor_id,omitempty"`
	DecisionNote            string   `json:"decision_note,omitempty"`
}

type TaskCommandRequest struct {
	TaskRef string `json:"task_ref"`
	Reason  string `json:"reason,omitempty"`
}

type TaskCommandResponse struct {
	TaskRef          string   `json:"task_ref"`
	State            string   `json:"state"`
	MessageRefs      []string `json:"message_refs"`
	AttemptRefs      []string `json:"attempt_refs,omitempty"`
	AffectedMessages int      `json:"affected_messages"`
	AffectedAttempts int      `json:"affected_attempts"`
}

type DLQReplayRequest struct {
	DeadLetterRef string `json:"dead_letter_ref"`
	Reason        string `json:"reason,omitempty"`
}

type DLQReplayResponse struct {
	DeadLetterRef string `json:"dead_letter_ref"`
	OriginalRef   string `json:"original_message_ref"`
	MessageRef    string `json:"message_ref"`
	DeliveryRef   string `json:"delivery_ref"`
	State         string `json:"state"`
}

type ArtifactFinalizeRequest struct {
	ArtifactVersionRef string `json:"artifact_version_ref"`
	ActorID            string `json:"actor_id"`
	Reason             string `json:"reason,omitempty"`
}

type ArtifactFinalizeResponse struct {
	ArtifactVersionRef string `json:"artifact_version_ref"`
	FinalizationRef    string `json:"finalization_ref"`
	State              string `json:"state"`
	FinalizedByActorID string `json:"finalized_by_actor_id"`
	FinalizedAt        string `json:"finalized_at"`
	Deduped            bool   `json:"deduped"`
}

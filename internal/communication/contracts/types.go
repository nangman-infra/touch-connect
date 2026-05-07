package contracts

type Capability struct {
	Name           string   `json:"name"`
	ExecutionHints []string `json:"execution_hints,omitempty"`
}

type EndpointRegistrationRequest struct {
	EndpointRef     string       `json:"endpoint_ref"`
	DisplayName     string       `json:"display_name"`
	ActorID         string       `json:"actor_id"`
	WorkspaceID     string       `json:"workspace_id"`
	ConnectionState string       `json:"connection_state"`
	Capabilities    []Capability `json:"capabilities"`
	ExecutionHints  []string     `json:"execution_hints,omitempty"`
	WorkerVersion   string       `json:"worker_version"`
	StartedAt       string       `json:"started_at"`
}

type EndpointRegistrationResponse struct {
	EndpointRef string `json:"endpoint_ref"`
	AcceptedRef string `json:"accepted_ref"`
}

type EndpointHeartbeatRequest struct {
	EndpointRef       string `json:"endpoint_ref"`
	ConnectionState   string `json:"connection_state"`
	ObservedAt        string `json:"observed_at,omitempty"`
	CurrentAttemptRef string `json:"current_attempt_ref,omitempty"`
	LastActivityAt    string `json:"last_activity_at,omitempty"`
	ProgressSummary   string `json:"progress_summary,omitempty"`
}

type EndpointHeartbeatResponse struct {
	EndpointRef      string `json:"endpoint_ref"`
	ConnectionState  string `json:"connection_state"`
	LastHeartbeatAt  string `json:"last_heartbeat_at"`
	HeartbeatExpires string `json:"heartbeat_expires"`
}

type CapabilityAdvertisementRequest struct {
	Capabilities []Capability `json:"capabilities"`
}

type CapabilityAdvertisementResponse struct {
	EndpointRef string   `json:"endpoint_ref"`
	Names       []string `json:"names"`
}

type Payload struct {
	Summary    string      `json:"summary"`
	Body       string      `json:"body"`
	References []Reference `json:"references"`
}

type Reference struct {
	Ref        string     `json:"ref"`
	Type       string     `json:"type"`
	Title      string     `json:"title,omitempty"`
	Version    int        `json:"version,omitempty"`
	SourceRisk SourceRisk `json:"source_risk,omitempty"`
}

type Constraint struct {
	Code      string `json:"code"`
	Summary   string `json:"summary"`
	SourceRef string `json:"source_ref,omitempty"`
	Details   string `json:"details,omitempty"`
}

type MessageIngressRequest struct {
	MessageRef           string       `json:"message_ref,omitempty"`
	SenderEndpointRef    string       `json:"sender_endpoint_ref"`
	TargetCapability     string       `json:"target_capability"`
	TargetEndpointRef    string       `json:"target_endpoint_ref,omitempty"`
	DependsOnMessageRefs []string     `json:"depends_on_message_refs,omitempty"`
	Payload              Payload      `json:"payload"`
	Constraints          []Constraint `json:"constraints"`
	CorrelationRef       string       `json:"correlation_ref,omitempty"`
	// ReadbackRequired is the v0 boolean projection. When PhraseologyPolicy is set,
	// the policy readback settings take precedence.
	ReadbackRequired  bool               `json:"readback_required,omitempty"`
	PhraseologyPolicy *PhraseologyPolicy `json:"phraseology_policy,omitempty"`
	// QualityGate controls whether quality violations block ingress, warn only, or are skipped.
	// Empty means enforce for backward-compatible callers.
	QualityGate QualityGateMode `json:"quality_gate,omitempty"`
}

type MessageIngressResponse struct {
	MessageRef         string `json:"message_ref"`
	DeliveryRef        string `json:"delivery_ref"`
	State              string `json:"state"`
	QualityDecisionRef string `json:"quality_decision_ref,omitempty"`
}

type ClaimMessageRequest struct {
	EndpointRef string `json:"endpoint_ref"`
}

type ClaimNextMessageRequest struct {
	EndpointRef string `json:"endpoint_ref"`
}

type ClaimNextMessageResponse struct {
	Empty bool                  `json:"empty"`
	Claim *ClaimMessageResponse `json:"claim,omitempty"`
}

type ClaimMessageResponse struct {
	MessageRef           string       `json:"message_ref"`
	AttemptRef           string       `json:"attempt_ref"`
	EndpointRef          string       `json:"endpoint_ref"`
	State                string       `json:"state"`
	LeaseExpiresAt       string       `json:"lease_expires_at"`
	Takeover             bool         `json:"takeover"`
	RedeliveryCount      int          `json:"redelivery_count"`
	LastCheckpointRef    string       `json:"last_checkpoint_ref,omitempty"`
	ResumeSummary        string       `json:"resume_summary,omitempty"`
	ResumeArtifactRefs   []string     `json:"resume_artifact_refs,omitempty"`
	ReadbackRequired     bool         `json:"readback_required"`
	TargetCapability     string       `json:"target_capability"`
	TargetEndpointRef    string       `json:"target_endpoint_ref,omitempty"`
	DependsOnMessageRefs []string     `json:"depends_on_message_refs,omitempty"`
	CorrelationRef       string       `json:"correlation_ref,omitempty"`
	Payload              Payload      `json:"payload"`
	Constraints          []Constraint `json:"constraints"`
	PayloadSummary       string       `json:"payload_summary"`
	ConstraintSummary    string       `json:"constraint_summary,omitempty"`
	// ConfidenceBand reflects PhraseologyPolicy compliance, not evidence-supported certainty.
	ConfidenceBand ConfidenceBand `json:"confidence_band,omitempty"`
}

type CheckpointRequest struct {
	EndpointRef       string   `json:"endpoint_ref"`
	State             string   `json:"state"`
	Summary           string   `json:"summary"`
	ArtifactRefs      []string `json:"artifact_refs,omitempty"`
	FailureReasonCode string   `json:"failure_reason_code,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	MissingReasons    []string `json:"missing_reasons,omitempty"`
}

type CheckpointResponse struct {
	CheckpointRef string `json:"checkpoint_ref"`
	AttemptRef    string `json:"attempt_ref"`
	State         string `json:"state"`
	Revision      int    `json:"revision"`
	// ConfidenceBand reflects PhraseologyPolicy compliance, not evidence-supported certainty.
	ConfidenceBand ConfidenceBand `json:"confidence_band,omitempty"`
}

type ReadbackRequest struct {
	EndpointRef    string   `json:"endpoint_ref"`
	Summary        string   `json:"summary"`
	Understanding  string   `json:"understanding"`
	Questions      []string `json:"questions,omitempty"`
	MissingFields  []string `json:"missing_fields,omitempty"`
	MissingReasons []string `json:"missing_reasons,omitempty"`
	// ConfidenceBand reflects PhraseologyPolicy compliance, not evidence-supported certainty.
	ConfidenceBand ConfidenceBand `json:"confidence_band,omitempty"`
}

type ReadbackResponse struct {
	ReadbackRef string `json:"readback_ref"`
	AttemptRef  string `json:"attempt_ref"`
	Revision    int    `json:"revision"`
}

type RefreshLeaseRequest struct {
	EndpointRef string `json:"endpoint_ref"`
}

type RefreshLeaseResponse struct {
	AttemptRef     string `json:"attempt_ref"`
	State          string `json:"state"`
	LeaseExpiresAt string `json:"lease_expires_at"`
}

type ArtifactVersionRequest struct {
	EndpointRef                string   `json:"endpoint_ref"`
	ArtifactRef                string   `json:"artifact_ref"`
	ArtifactVersionRef         string   `json:"artifact_version_ref"`
	RoomRef                    string   `json:"room_ref"`
	TaskRef                    string   `json:"task_ref"`
	TaskRevision               int      `json:"task_revision"`
	Kind                       string   `json:"kind"`
	MediaType                  string   `json:"media_type"`
	SizeBytes                  int64    `json:"size_bytes"`
	Checksum                   string   `json:"checksum"`
	StorageRef                 string   `json:"storage_ref"`
	RetentionClass             string   `json:"retention_class"`
	AccessScope                string   `json:"access_scope"`
	BasedOnMessageRefs         []string `json:"based_on_message_refs,omitempty"`
	BasedOnArtifactVersionRefs []string `json:"based_on_artifact_version_refs,omitempty"`
}

type ArtifactVersionResponse struct {
	ArtifactRef          string `json:"artifact_ref"`
	ArtifactVersionRef   string `json:"artifact_version_ref"`
	AttemptRef           string `json:"attempt_ref"`
	MessageRef           string `json:"message_ref"`
	CreatedByEndpointRef string `json:"created_by_endpoint_ref"`
	CreatedAt            string `json:"created_at"`
}

type ApprovalDecisionRequest struct {
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

type ApprovalDecisionResponse struct {
	ApprovalRef      string `json:"approval_ref"`
	AttemptRef       string `json:"attempt_ref"`
	Status           string `json:"status"`
	ApprovalHash     string `json:"approval_hash"`
	DecidedByActorID string `json:"decided_by_actor_id,omitempty"`
	DecidedAt        string `json:"decided_at,omitempty"`
}

type SideEffectExecutionRequest struct {
	EndpointRef        string `json:"endpoint_ref"`
	IdempotencyKey     string `json:"idempotency_key"`
	ProtectedScope     string `json:"protected_scope"`
	ApprovalRef        string `json:"approval_ref"`
	ApprovalHash       string `json:"approval_hash"`
	TaskRef            string `json:"task_ref"`
	OperationKind      string `json:"operation_kind"`
	ExternalTarget     string `json:"external_target"`
	RequestedByActorID string `json:"requested_by_actor_id"`
}

type SideEffectExecutionResponse struct {
	SideEffectExecutionRef string `json:"side_effect_execution_ref"`
	AttemptRef             string `json:"attempt_ref"`
	Status                 string `json:"status"`
	Deduped                bool   `json:"deduped"`
	StartedAt              string `json:"started_at,omitempty"`
}

type CompleteSideEffectExecutionRequest struct {
	EndpointRef       string `json:"endpoint_ref"`
	Status            string `json:"status"`
	ResultRef         string `json:"result_ref,omitempty"`
	FailureReasonCode string `json:"failure_reason_code,omitempty"`
}

type CompleteSideEffectExecutionResponse struct {
	SideEffectExecutionRef string `json:"side_effect_execution_ref"`
	Status                 string `json:"status"`
	CompletedAt            string `json:"completed_at,omitempty"`
}

type CompleteAttemptRequest struct {
	EndpointRef      string                   `json:"endpoint_ref"`
	Summary          string                   `json:"summary"`
	ArtifactRefs     []string                 `json:"artifact_refs,omitempty"`
	FollowUpMessages []FollowUpMessageRequest `json:"follow_up_messages,omitempty"`
}

type CompleteAttemptResponse struct {
	AttemptRef          string   `json:"attempt_ref"`
	State               string   `json:"state"`
	FollowUpMessageRefs []string `json:"follow_up_message_refs,omitempty"`
}

type FollowUpMessageRequest struct {
	MessageRef           string          `json:"message_ref,omitempty"`
	TargetCapability     string          `json:"target_capability"`
	TargetEndpointRef    string          `json:"target_endpoint_ref,omitempty"`
	DependsOnMessageRefs []string        `json:"depends_on_message_refs,omitempty"`
	Summary              string          `json:"summary"`
	Body                 string          `json:"body"`
	Constraints          []Constraint    `json:"constraints,omitempty"`
	ReadbackRequired     bool            `json:"readback_required,omitempty"`
	QualityGate          QualityGateMode `json:"quality_gate,omitempty"`
}

type ErrorResponse struct {
	Code            string           `json:"code"`
	Message         string           `json:"message"`
	QualityDecision *QualityDecision `json:"quality_decision,omitempty"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	Component string `json:"component,omitempty"`
	Version   string `json:"version,omitempty"`
}

type VersionResponse struct {
	Version         string `json:"version"`
	MinimumWorker   string `json:"minimum_worker,omitempty"`
	ContractVersion string `json:"contract_version,omitempty"`
}

package contracts

type SnapshotResponse struct {
	Endpoints        []EndpointRecord             `json:"endpoints"`
	Messages         []MessageRecord              `json:"messages"`
	Attempts         []AttemptRecord              `json:"attempts"`
	Checkpoints      []CheckpointRecord           `json:"checkpoints"`
	Readbacks        []ReadbackRecord             `json:"readbacks"`
	Artifacts        []ArtifactRecord             `json:"artifacts"`
	Finalizations    []ArtifactFinalizationRecord `json:"finalizations"`
	DeadLetters      []DeadLetterRecord           `json:"dead_letters"`
	Approvals        []ApprovalRecord             `json:"approvals"`
	SideEffects      []SideEffectRecord           `json:"side_effects"`
	QualityDecisions []QualityDecision            `json:"quality_decisions"`
	Freshness        FreshnessRecord              `json:"freshness"`
}

type FreshnessRecord struct {
	GeneratedAt string `json:"generated_at"`
	Source      string `json:"source"`
}

type EndpointRecord struct {
	EndpointRef     string                `json:"endpoint_ref"`
	DisplayName     string                `json:"display_name"`
	ActorID         string                `json:"actor_id"`
	WorkspaceID     string                `json:"workspace_id"`
	ConnectionState string                `json:"connection_state"`
	Capabilities    map[string]Capability `json:"capabilities"`
	ExecutionHints  []string              `json:"execution_hints,omitempty"`
	WorkerVersion   string                `json:"worker_version"`
	StartedAt       string                `json:"started_at"`
	RegisteredAt    string                `json:"registered_at"`
	LastHeartbeatAt string                `json:"last_heartbeat_at"`
}

type MessageRecord struct {
	MessageRef        string       `json:"message_ref"`
	DeliveryRef       string       `json:"delivery_ref"`
	SenderEndpointRef string       `json:"sender_endpoint_ref"`
	TargetCapability  string       `json:"target_capability"`
	Payload           Payload      `json:"payload"`
	Constraints       []Constraint `json:"constraints"`
	CorrelationRef    string       `json:"correlation_ref,omitempty"`
	ReadbackRequired  bool         `json:"readback_required"`
	State             string       `json:"state"`
	AttemptRef        string       `json:"attempt_ref,omitempty"`
	RedeliveryCount   int          `json:"redelivery_count"`
}

type AttemptRecord struct {
	AttemptRef     string `json:"attempt_ref"`
	MessageRef     string `json:"message_ref"`
	EndpointRef    string `json:"endpoint_ref"`
	State          string `json:"state"`
	LeaseExpiresAt string `json:"lease_expires_at"`
	Revision       int    `json:"revision"`
	AttemptNo      int    `json:"attempt_no"`
	ClaimEpoch     int    `json:"claim_epoch"`
}

type CheckpointRecord struct {
	CheckpointRef     string   `json:"checkpoint_ref"`
	AttemptRef        string   `json:"attempt_ref"`
	EndpointRef       string   `json:"endpoint_ref"`
	State             string   `json:"state"`
	Summary           string   `json:"summary"`
	Revision          int      `json:"revision"`
	ArtifactRefs      []string `json:"artifact_refs,omitempty"`
	FailureReasonCode string   `json:"failure_reason_code,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	MissingReasons    []string `json:"missing_reasons,omitempty"`
}

type ReadbackRecord struct {
	ReadbackRef    string   `json:"readback_ref"`
	AttemptRef     string   `json:"attempt_ref"`
	EndpointRef    string   `json:"endpoint_ref"`
	Summary        string   `json:"summary"`
	Understanding  string   `json:"understanding"`
	Questions      []string `json:"questions,omitempty"`
	MissingFields  []string `json:"missing_fields,omitempty"`
	MissingReasons []string `json:"missing_reasons,omitempty"`
	Revision       int      `json:"revision"`
}

type ArtifactRecord struct {
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
	CreatedByActorID           string   `json:"created_by_actor_id"`
	CreatedByEndpointRef       string   `json:"created_by_endpoint_ref"`
	MessageRef                 string   `json:"message_ref"`
	AttemptRef                 string   `json:"attempt_ref"`
	CreatedAt                  string   `json:"created_at"`
}

type ArtifactFinalizationRecord struct {
	ArtifactVersionRef string `json:"artifact_version_ref"`
	FinalizationRef    string `json:"finalization_ref"`
	FinalizedByActorID string `json:"finalized_by_actor_id"`
	Reason             string `json:"reason,omitempty"`
	FinalizedAt        string `json:"finalized_at"`
}

type DeadLetterRecord struct {
	DeadLetterRef     string `json:"dead_letter_ref"`
	MessageRef        string `json:"message_ref"`
	LastAttemptRef    string `json:"last_attempt_ref"`
	LastCheckpointRef string `json:"last_checkpoint_ref,omitempty"`
	Reason            string `json:"reason"`
	RedeliveryCount   int    `json:"redelivery_count"`
	CreatedAt         string `json:"created_at"`
}

type ApprovalRecord struct {
	ApprovalRef             string   `json:"approval_ref"`
	AttemptRef              string   `json:"attempt_ref"`
	MessageRef              string   `json:"message_ref"`
	TargetType              string   `json:"target_type"`
	TargetRef               string   `json:"target_ref"`
	RequestedByActorID      string   `json:"requested_by_actor_id"`
	ApproverSubjectsOrRoles []string `json:"approver_subjects_or_roles"`
	ApprovalScope           string   `json:"approval_scope"`
	ApprovalHash            string   `json:"approval_hash"`
	Status                  string   `json:"status"`
	Reason                  string   `json:"reason,omitempty"`
	DecidedByActorID        string   `json:"decided_by_actor_id,omitempty"`
	DecisionNote            string   `json:"decision_note,omitempty"`
	RequestedAt             string   `json:"requested_at"`
	ExpiresAt               string   `json:"expires_at,omitempty"`
	DecidedAt               string   `json:"decided_at,omitempty"`
}

type SideEffectRecord struct {
	SideEffectExecutionRef string `json:"side_effect_execution_ref"`
	IdempotencyKey         string `json:"idempotency_key"`
	ProtectedScope         string `json:"protected_scope"`
	ApprovalRef            string `json:"approval_ref"`
	ApprovalHash           string `json:"approval_hash"`
	MessageRef             string `json:"message_ref"`
	TaskRef                string `json:"task_ref"`
	AttemptRef             string `json:"attempt_ref"`
	OperationKind          string `json:"operation_kind"`
	ExternalTarget         string `json:"external_target"`
	RequestedByActorID     string `json:"requested_by_actor_id"`
	ExecutedByActorID      string `json:"executed_by_actor_id"`
	ExecutedByEndpointRef  string `json:"executed_by_endpoint_ref"`
	Status                 string `json:"status"`
	StartedAt              string `json:"started_at,omitempty"`
	CompletedAt            string `json:"completed_at,omitempty"`
	ResultRef              string `json:"result_ref,omitempty"`
	FailureReasonCode      string `json:"failure_reason_code,omitempty"`
}

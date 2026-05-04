package domain

import (
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	AttemptStateClaimed              = "claimed"
	AttemptStateValidating           = "validating"
	AttemptStateBlockedMissingFields = "blocked_missing_fields"
	AttemptStateInProgress           = "in_progress"
	AttemptStateRetrying             = "retrying"
	AttemptStateOrphaned             = "orphaned"
	AttemptStateCompleted            = "completed"
	AttemptStateFailed               = "failed"
	AttemptStateCanceled             = "canceled"
	MessageStateAvailable            = "available"
	MessageStateClaimed              = "claimed"
	MessageStateProcessing           = "processing"
	MessageStateTakeoverCandidate    = "takeover_candidate"
	MessageStateInputRequired        = "input_required"
	MessageStateCompleted            = "completed"
	MessageStateFailed               = "failed"
	MessageStateDeadLettered         = "dead_lettered"
	MessageStateCanceled             = "canceled"
	EndpointStateOnline              = "online"
	EndpointStateStale               = "stale"
	EndpointStateOffline             = "offline"
	ApprovalStatusPending            = "pending"
	ApprovalStatusApproved           = "approved"
	ApprovalStatusRejected           = "rejected"
	ApprovalStatusExpired            = "expired"
	ApprovalStatusCanceled           = "canceled"
	SideEffectStatusExecuting        = "executing"
	SideEffectStatusSucceeded        = "succeeded"
	SideEffectStatusFailed           = "failed"
	SideEffectStatusCanceled         = "canceled"
	SideEffectStatusDeduped          = "deduped"
)

type Endpoint struct {
	EndpointRef     string
	DisplayName     string
	ActorID         string
	WorkspaceID     string
	ConnectionState string
	Capabilities    map[string]contracts.Capability
	ExecutionHints  []string
	WorkerVersion   string
	StartedAt       string
	RegisteredAt    time.Time
	LastHeartbeatAt time.Time
}

type Message struct {
	MessageRef        string
	DeliveryRef       string
	SenderEndpointRef string
	TargetCapability  string
	Payload           contracts.Payload
	Constraints       []contracts.Constraint
	CorrelationRef    string
	ReadbackRequired  bool
	State             string
	AttemptRef        string
	RedeliveryCount   int
}

type Attempt struct {
	AttemptRef     string
	MessageRef     string
	EndpointRef    string
	State          string
	LeaseExpiresAt time.Time
	Revision       int
	AttemptNo      int
	ClaimEpoch     int
}

type Checkpoint struct {
	CheckpointRef     string
	AttemptRef        string
	EndpointRef       string
	State             string
	Summary           string
	Revision          int
	ArtifactRefs      []string
	FailureReasonCode string
	MissingFields     []string
	MissingReasons    []string
}

type ArtifactVersion struct {
	ArtifactRef                string
	ArtifactVersionRef         string
	RoomRef                    string
	TaskRef                    string
	TaskRevision               int
	Kind                       string
	MediaType                  string
	SizeBytes                  int64
	Checksum                   string
	StorageRef                 string
	RetentionClass             string
	AccessScope                string
	BasedOnMessageRefs         []string
	BasedOnArtifactVersionRefs []string
	CreatedByActorID           string
	CreatedByEndpointRef       string
	MessageRef                 string
	AttemptRef                 string
	CreatedAt                  time.Time
}

type ArtifactFinalization struct {
	ArtifactVersionRef string
	FinalizationRef    string
	FinalizedByActorID string
	Reason             string
	FinalizedAt        time.Time
}

type Readback struct {
	ReadbackRef    string
	AttemptRef     string
	EndpointRef    string
	Summary        string
	Understanding  string
	Questions      []string
	MissingFields  []string
	MissingReasons []string
	Revision       int
}

type DeadLetter struct {
	DeadLetterRef     string
	MessageRef        string
	LastAttemptRef    string
	LastCheckpointRef string
	Reason            string
	RedeliveryCount   int
	CreatedAt         time.Time
}

type ApprovalDecision struct {
	ApprovalRef             string
	AttemptRef              string
	MessageRef              string
	TargetType              string
	TargetRef               string
	RequestedByActorID      string
	ApproverSubjectsOrRoles []string
	ApprovalScope           string
	ApprovalHash            string
	Status                  string
	Reason                  string
	DecidedByActorID        string
	DecisionNote            string
	RequestedAt             time.Time
	ExpiresAt               time.Time
	DecidedAt               time.Time
}

type SideEffectExecution struct {
	SideEffectExecutionRef string
	IdempotencyKey         string
	ProtectedScope         string
	ApprovalRef            string
	ApprovalHash           string
	MessageRef             string
	TaskRef                string
	AttemptRef             string
	OperationKind          string
	ExternalTarget         string
	RequestedByActorID     string
	ExecutedByActorID      string
	ExecutedByEndpointRef  string
	Status                 string
	StartedAt              time.Time
	CompletedAt            time.Time
	ResultRef              string
	FailureReasonCode      string
}

type ClaimResult struct {
	Message            Message
	Attempt            Attempt
	Takeover           bool
	LastCheckpointRef  string
	ResumeSummary      string
	ResumeArtifactRefs []string
}

type ClaimRequest struct {
	MessageRef     string
	Endpoint       Endpoint
	AttemptRef     string
	DeadLetterRef  string
	LeaseExpiresAt time.Time
	Now            time.Time
	MaxRedelivery  int
}

type ClaimNextRequest struct {
	Endpoint       Endpoint
	AttemptRef     string
	DeadLetterRef  string
	LeaseExpiresAt time.Time
	Now            time.Time
	MaxRedelivery  int
}

type Snapshot struct {
	Endpoints     []Endpoint
	Messages      []Message
	Attempts      []Attempt
	Checkpoints   []Checkpoint
	Readbacks     []Readback
	Artifacts     []ArtifactVersion
	Finalizations []ArtifactFinalization
	DeadLetters   []DeadLetter
	Approvals     []ApprovalDecision
	SideEffects   []SideEffectExecution
}

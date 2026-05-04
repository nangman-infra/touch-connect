package application

import (
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

type Store interface {
	SaveEndpoint(endpoint domain.Endpoint) error
	GetEndpoint(endpointRef string) (domain.Endpoint, bool)
	UpdateCapabilities(endpointRef string, capabilities map[string]contracts.Capability) (domain.Endpoint, error)
	UpdateEndpoint(endpoint domain.Endpoint) error
	CapabilityEndpoints(capability string) []domain.Endpoint
	SaveMessage(message domain.Message) error
	GetMessage(messageRef string) (domain.Message, bool)
	UpdateMessage(message domain.Message) error
	ClaimMessage(claim domain.ClaimRequest) (domain.ClaimResult, error)
	ClaimNextMessage(claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error)
	SaveAttempt(attempt domain.Attempt) error
	GetAttempt(attemptRef string) (domain.Attempt, bool)
	UpdateAttempt(attempt domain.Attempt) error
	SaveCheckpoint(checkpoint domain.Checkpoint) (domain.Checkpoint, error)
	SaveReadback(readback domain.Readback) (domain.Readback, error)
	SaveArtifactVersion(version domain.ArtifactVersion) error
	GetArtifactVersion(artifactVersionRef string) (domain.ArtifactVersion, bool)
	SaveArtifactFinalization(finalization domain.ArtifactFinalization) error
	GetArtifactFinalization(artifactVersionRef string) (domain.ArtifactFinalization, bool)
	SaveApprovalDecision(decision domain.ApprovalDecision) error
	GetApprovalDecision(approvalRef string) (domain.ApprovalDecision, bool)
	SaveSideEffectExecution(execution domain.SideEffectExecution) (domain.SideEffectExecution, bool, error)
	GetSideEffectExecution(executionRef string) (domain.SideEffectExecution, bool)
	UpdateSideEffectExecution(execution domain.SideEffectExecution) error
	ReconcileExpiredClaims(now time.Time) int
}

type RefAllocator interface {
	NextRef(kind string) string
}

type ProjectionReader interface {
	Snapshot() domain.Snapshot
}

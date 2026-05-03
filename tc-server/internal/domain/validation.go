package domain

import (
	"errors"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

var (
	ErrInvalidInput         = errors.New("invalid_input")
	ErrEndpointNotFound     = errors.New("endpoint_not_found")
	ErrCapabilityNotFound   = errors.New("capability_not_found")
	ErrMessageNotFound      = errors.New("message_not_found")
	ErrMessageUnavailable   = errors.New("message_unavailable")
	ErrAttemptNotFound      = errors.New("attempt_not_found")
	ErrStaleAttempt         = errors.New("stale_attempt")
	ErrEndpointStale        = errors.New("endpoint_stale")
	ErrLeaseExpired         = errors.New("lease_expired")
	ErrMessageDeadLettered  = errors.New("message_dead_lettered")
	ErrArtifactNotFound     = errors.New("artifact_not_found")
	ErrArtifactExists       = errors.New("artifact_version_exists")
	ErrApprovalNotFound     = errors.New("approval_not_found")
	ErrApprovalRequired     = errors.New("approval_required")
	ErrApprovalRejected     = errors.New("approval_not_approved")
	ErrApprovalExpired      = errors.New("approval_expired")
	ErrApprovalHashMismatch = errors.New("approval_hash_mismatch")
	ErrSelfApproval         = errors.New("self_approval_forbidden")
	ErrSideEffectNotFound   = errors.New("side_effect_not_found")
	ErrSideEffectConflict   = errors.New("side_effect_conflict")
)

func ValidateEndpointRegistration(req contracts.EndpointRegistrationRequest) error {
	if blank(req.EndpointRef) || blank(req.DisplayName) || blank(req.ActorID) || blank(req.WorkspaceID) {
		return ErrInvalidInput
	}
	if req.ConnectionState != "" && req.ConnectionState != EndpointStateOnline {
		return ErrInvalidInput
	}
	if len(req.Capabilities) == 0 {
		return ErrInvalidInput
	}
	return ValidateCapabilities(req.Capabilities)
}

func ValidateHeartbeat(req contracts.EndpointHeartbeatRequest) error {
	if blank(req.EndpointRef) {
		return ErrInvalidInput
	}
	switch req.ConnectionState {
	case EndpointStateOnline, EndpointStateStale, EndpointStateOffline:
		return nil
	default:
		return ErrInvalidInput
	}
}

func ValidateCapabilities(capabilities []contracts.Capability) error {
	if len(capabilities) == 0 {
		return ErrInvalidInput
	}
	seen := map[string]struct{}{}
	for _, capability := range capabilities {
		if blank(capability.Name) {
			return ErrInvalidInput
		}
		if _, ok := seen[capability.Name]; ok {
			return ErrInvalidInput
		}
		seen[capability.Name] = struct{}{}
	}
	return nil
}

func ValidateMessage(req contracts.MessageIngressRequest) error {
	if blank(req.SenderEndpointRef) || blank(req.TargetCapability) {
		return ErrInvalidInput
	}
	if blank(req.Payload.Summary) || blank(req.Payload.Body) {
		return ErrInvalidInput
	}
	if req.Payload.References == nil || req.Constraints == nil {
		return ErrInvalidInput
	}
	return nil
}

func ValidateCheckpoint(req contracts.CheckpointRequest) error {
	if blank(req.EndpointRef) || blank(req.State) || blank(req.Summary) {
		return ErrInvalidInput
	}
	switch req.State {
	case AttemptStateClaimed, AttemptStateValidating, AttemptStateBlockedMissingFields, AttemptStateInProgress, AttemptStateRetrying, AttemptStateCompleted, AttemptStateFailed:
	default:
		return ErrInvalidInput
	}
	if req.State == AttemptStateFailed && blank(req.FailureReasonCode) {
		return ErrInvalidInput
	}
	if req.State == AttemptStateBlockedMissingFields && !validMissingFields(req.MissingFields, req.MissingReasons) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateReadback(req contracts.ReadbackRequest) error {
	if blank(req.EndpointRef) || blank(req.Summary) || blank(req.Understanding) {
		return ErrInvalidInput
	}
	if len(req.MissingFields) > 0 && !validMissingFields(req.MissingFields, req.MissingReasons) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateCompletion(req contracts.CompleteAttemptRequest) error {
	if blank(req.EndpointRef) || blank(req.Summary) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateArtifactVersion(req contracts.ArtifactVersionRequest) error {
	if blank(req.EndpointRef) ||
		blank(req.ArtifactRef) ||
		blank(req.ArtifactVersionRef) ||
		blank(req.RoomRef) ||
		blank(req.TaskRef) ||
		blank(req.Kind) ||
		blank(req.MediaType) ||
		blank(req.Checksum) ||
		blank(req.StorageRef) ||
		blank(req.RetentionClass) ||
		blank(req.AccessScope) {
		return ErrInvalidInput
	}
	if req.TaskRevision <= 0 || req.SizeBytes < 0 {
		return ErrInvalidInput
	}
	if !validArtifactKind(req.Kind) || !validRetentionClass(req.RetentionClass) || !validAccessScope(req.AccessScope) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateApprovalDecision(req contracts.ApprovalDecisionRequest) error {
	if blank(req.ApprovalRef) ||
		blank(req.TargetType) ||
		blank(req.TargetRef) ||
		blank(req.RequestedByActorID) ||
		blank(req.ApprovalScope) ||
		blank(req.ApprovalHash) ||
		blank(req.Status) {
		return ErrInvalidInput
	}
	if len(req.ApproverSubjectsOrRoles) == 0 {
		return ErrInvalidInput
	}
	if !validApprovalTarget(req.TargetType) || !validApprovalStatus(req.Status) {
		return ErrInvalidInput
	}
	if req.Status != ApprovalStatusPending && blank(req.DecidedByActorID) {
		return ErrInvalidInput
	}
	if req.Status != ApprovalStatusPending && req.DecidedByActorID == req.RequestedByActorID {
		return ErrSelfApproval
	}
	return nil
}

func ValidateSideEffectExecution(req contracts.SideEffectExecutionRequest) error {
	if blank(req.EndpointRef) ||
		blank(req.IdempotencyKey) ||
		blank(req.ProtectedScope) ||
		blank(req.ApprovalRef) ||
		blank(req.ApprovalHash) ||
		blank(req.TaskRef) ||
		blank(req.OperationKind) ||
		blank(req.ExternalTarget) ||
		blank(req.RequestedByActorID) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateSideEffectCompletion(req contracts.CompleteSideEffectExecutionRequest) error {
	if blank(req.EndpointRef) || blank(req.Status) {
		return ErrInvalidInput
	}
	switch req.Status {
	case SideEffectStatusSucceeded:
		if blank(req.ResultRef) {
			return ErrInvalidInput
		}
	case SideEffectStatusFailed:
		if blank(req.FailureReasonCode) {
			return ErrInvalidInput
		}
	case SideEffectStatusCanceled:
	default:
		return ErrInvalidInput
	}
	return nil
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func validMissingFields(fields []string, reasons []string) bool {
	if len(fields) == 0 || len(reasons) == 0 || len(fields) != len(reasons) {
		return false
	}
	for i := range fields {
		if blank(fields[i]) || blank(reasons[i]) {
			return false
		}
	}
	return true
}

func validArtifactKind(kind string) bool {
	switch kind {
	case "document", "code_patch", "design_asset", "test_report", "log_bundle", "structured_data":
		return true
	default:
		return false
	}
}

func validRetentionClass(retentionClass string) bool {
	switch retentionClass {
	case "transient", "operational", "audit":
		return true
	default:
		return false
	}
}

func validAccessScope(accessScope string) bool {
	switch accessScope {
	case "room", "task", "actor", "role", "tenant":
		return true
	default:
		return false
	}
}

func validApprovalTarget(targetType string) bool {
	switch targetType {
	case "side_effect", "egress", "artifact_finalization", "tool_invocation", "policy_escalation":
		return true
	default:
		return false
	}
}

func validApprovalStatus(status string) bool {
	switch status {
	case ApprovalStatusPending, ApprovalStatusApproved, ApprovalStatusRejected, ApprovalStatusExpired, ApprovalStatusCanceled:
		return true
	default:
		return false
	}
}

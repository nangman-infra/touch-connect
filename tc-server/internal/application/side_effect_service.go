package application

import (
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Service) RecordApprovalDecision(attemptRef string, req contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error) {
	if err := domain.ValidateApprovalDecision(req); err != nil {
		return contracts.ApprovalDecisionResponse{}, err
	}
	attempt, ok := s.store.GetAttempt(attemptRef)
	if !ok {
		return contracts.ApprovalDecisionResponse{}, domain.ErrAttemptNotFound
	}
	expiresAt, err := parseOptionalTime(req.ExpiresAt)
	if err != nil {
		return contracts.ApprovalDecisionResponse{}, domain.ErrInvalidInput
	}
	decision := domain.ApprovalDecision{
		ApprovalRef:             req.ApprovalRef,
		AttemptRef:              attempt.AttemptRef,
		MessageRef:              attempt.MessageRef,
		TargetType:              req.TargetType,
		TargetRef:               req.TargetRef,
		RequestedByActorID:      req.RequestedByActorID,
		ApproverSubjectsOrRoles: req.ApproverSubjectsOrRoles,
		ApprovalScope:           req.ApprovalScope,
		ApprovalHash:            req.ApprovalHash,
		Status:                  req.Status,
		Reason:                  req.Reason,
		DecidedByActorID:        req.DecidedByActorID,
		DecisionNote:            req.DecisionNote,
		RequestedAt:             s.now(),
		ExpiresAt:               expiresAt,
	}
	if req.Status != domain.ApprovalStatusPending {
		decision.DecidedAt = s.now()
	}
	if err := s.store.SaveApprovalDecision(decision); err != nil {
		return contracts.ApprovalDecisionResponse{}, err
	}
	return contracts.ApprovalDecisionResponse{
		ApprovalRef:      decision.ApprovalRef,
		AttemptRef:       decision.AttemptRef,
		Status:           decision.Status,
		ApprovalHash:     decision.ApprovalHash,
		DecidedByActorID: decision.DecidedByActorID,
		DecidedAt:        formatOptionalTime(decision.DecidedAt),
	}, nil
}

func (s *Service) StartSideEffectExecution(attemptRef string, req contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error) {
	if err := domain.ValidateSideEffectExecution(req); err != nil {
		return contracts.SideEffectExecutionResponse{}, err
	}
	attempt, err := s.requireLiveAttempt(attemptRef, req.EndpointRef)
	if err != nil {
		return contracts.SideEffectExecutionResponse{}, err
	}
	endpoint, ok := s.store.GetEndpoint(req.EndpointRef)
	if !ok {
		return contracts.SideEffectExecutionResponse{}, domain.ErrEndpointNotFound
	}
	approval, err := s.requireApprovedDecision(attempt, req)
	if err != nil {
		return contracts.SideEffectExecutionResponse{}, err
	}
	execution := domain.SideEffectExecution{
		SideEffectExecutionRef: s.store.NextRef("side-effect"),
		IdempotencyKey:         req.IdempotencyKey,
		ProtectedScope:         req.ProtectedScope,
		ApprovalRef:            approval.ApprovalRef,
		ApprovalHash:           req.ApprovalHash,
		MessageRef:             attempt.MessageRef,
		TaskRef:                req.TaskRef,
		AttemptRef:             attempt.AttemptRef,
		OperationKind:          req.OperationKind,
		ExternalTarget:         req.ExternalTarget,
		RequestedByActorID:     req.RequestedByActorID,
		ExecutedByActorID:      endpoint.ActorID,
		ExecutedByEndpointRef:  endpoint.EndpointRef,
		Status:                 domain.SideEffectStatusExecuting,
		StartedAt:              s.now(),
	}
	accepted, deduped, err := s.store.SaveSideEffectExecution(execution)
	if err != nil {
		return contracts.SideEffectExecutionResponse{}, err
	}
	status := accepted.Status
	if deduped {
		status = domain.SideEffectStatusDeduped
	}
	return contracts.SideEffectExecutionResponse{
		SideEffectExecutionRef: accepted.SideEffectExecutionRef,
		AttemptRef:             accepted.AttemptRef,
		Status:                 status,
		Deduped:                deduped,
		StartedAt:              formatOptionalTime(accepted.StartedAt),
	}, nil
}

func (s *Service) CompleteSideEffectExecution(executionRef string, req contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error) {
	if err := domain.ValidateSideEffectCompletion(req); err != nil {
		return contracts.CompleteSideEffectExecutionResponse{}, err
	}
	execution, ok := s.store.GetSideEffectExecution(executionRef)
	if !ok {
		return contracts.CompleteSideEffectExecutionResponse{}, domain.ErrSideEffectNotFound
	}
	if execution.ExecutedByEndpointRef != req.EndpointRef || execution.Status != domain.SideEffectStatusExecuting {
		return contracts.CompleteSideEffectExecutionResponse{}, domain.ErrSideEffectConflict
	}
	if _, err := s.requireLiveAttempt(execution.AttemptRef, req.EndpointRef); err != nil {
		return contracts.CompleteSideEffectExecutionResponse{}, err
	}
	execution.Status = req.Status
	execution.ResultRef = req.ResultRef
	execution.FailureReasonCode = req.FailureReasonCode
	execution.CompletedAt = s.now()
	if err := s.store.UpdateSideEffectExecution(execution); err != nil {
		return contracts.CompleteSideEffectExecutionResponse{}, err
	}
	return contracts.CompleteSideEffectExecutionResponse{
		SideEffectExecutionRef: execution.SideEffectExecutionRef,
		Status:                 execution.Status,
		CompletedAt:            formatOptionalTime(execution.CompletedAt),
	}, nil
}

func (s *Service) requireApprovedDecision(attempt domain.Attempt, req contracts.SideEffectExecutionRequest) (domain.ApprovalDecision, error) {
	approval, ok := s.store.GetApprovalDecision(req.ApprovalRef)
	if !ok {
		return domain.ApprovalDecision{}, domain.ErrApprovalRequired
	}
	if approval.AttemptRef != attempt.AttemptRef {
		return domain.ApprovalDecision{}, domain.ErrApprovalNotFound
	}
	if approval.Status != domain.ApprovalStatusApproved {
		return domain.ApprovalDecision{}, domain.ErrApprovalRejected
	}
	if !approval.ExpiresAt.IsZero() && !s.now().Before(approval.ExpiresAt) {
		return domain.ApprovalDecision{}, domain.ErrApprovalExpired
	}
	if approval.ApprovalHash != req.ApprovalHash {
		return domain.ApprovalDecision{}, domain.ErrApprovalHashMismatch
	}
	return approval, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}

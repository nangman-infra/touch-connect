package application

import (
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

type Service struct {
	store    Store
	settings Settings
}

func NewService(store Store, settings Settings) (*Service, error) {
	accepted, err := settings.Validated()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, settings: accepted}, nil
}

func (s *Service) Health() contracts.HealthResponse {
	return contracts.HealthResponse{Status: "ok", Component: "tc-server", Version: s.settings.Version}
}

func (s *Service) Version() contracts.VersionResponse {
	return contracts.VersionResponse{
		Version:         s.settings.Version,
		MinimumWorker:   s.settings.MinimumWorkerVersion,
		ContractVersion: ContractVersion,
	}
}

func (s *Service) RegisterEndpoint(req contracts.EndpointRegistrationRequest) (contracts.EndpointRegistrationResponse, error) {
	if err := domain.ValidateEndpointRegistration(req); err != nil {
		return contracts.EndpointRegistrationResponse{}, err
	}
	endpoint := domain.Endpoint{
		EndpointRef:     req.EndpointRef,
		DisplayName:     req.DisplayName,
		ActorID:         req.ActorID,
		WorkspaceID:     req.WorkspaceID,
		ConnectionState: req.ConnectionState,
		Capabilities:    capabilityMap(req.Capabilities),
		ExecutionHints:  req.ExecutionHints,
		WorkerVersion:   req.WorkerVersion,
		StartedAt:       req.StartedAt,
		RegisteredAt:    s.now(),
		LastHeartbeatAt: s.now(),
	}
	if err := s.store.SaveEndpoint(endpoint); err != nil {
		return contracts.EndpointRegistrationResponse{}, err
	}
	return contracts.EndpointRegistrationResponse{
		EndpointRef: endpoint.EndpointRef,
		AcceptedRef: s.store.NextRef("accepted"),
	}, nil
}

func (s *Service) HeartbeatEndpoint(endpointRef string, req contracts.EndpointHeartbeatRequest) (contracts.EndpointHeartbeatResponse, error) {
	if req.EndpointRef == "" {
		req.EndpointRef = endpointRef
	}
	if endpointRef != req.EndpointRef {
		return contracts.EndpointHeartbeatResponse{}, domain.ErrInvalidInput
	}
	if err := domain.ValidateHeartbeat(req); err != nil {
		return contracts.EndpointHeartbeatResponse{}, err
	}
	endpoint, ok := s.store.GetEndpoint(endpointRef)
	if !ok {
		return contracts.EndpointHeartbeatResponse{}, domain.ErrEndpointNotFound
	}
	now := s.now()
	endpoint.ConnectionState = req.ConnectionState
	endpoint.LastHeartbeatAt = now
	if err := s.store.UpdateEndpoint(endpoint); err != nil {
		return contracts.EndpointHeartbeatResponse{}, err
	}
	return contracts.EndpointHeartbeatResponse{
		EndpointRef:      endpoint.EndpointRef,
		ConnectionState:  endpoint.ConnectionState,
		LastHeartbeatAt:  formatTime(now),
		HeartbeatExpires: formatTime(now.Add(s.settings.EndpointHeartbeatTimeout)),
	}, nil
}

func (s *Service) AdvertiseCapabilities(endpointRef string, req contracts.CapabilityAdvertisementRequest) (contracts.CapabilityAdvertisementResponse, error) {
	if err := domain.ValidateCapabilities(req.Capabilities); err != nil {
		return contracts.CapabilityAdvertisementResponse{}, err
	}
	endpoint, err := s.store.UpdateCapabilities(endpointRef, capabilityMap(req.Capabilities))
	if err != nil {
		return contracts.CapabilityAdvertisementResponse{}, err
	}
	return contracts.CapabilityAdvertisementResponse{
		EndpointRef: endpoint.EndpointRef,
		Names:       capabilityNames(endpoint.Capabilities),
	}, nil
}

func (s *Service) IngressMessage(req contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	if err := domain.ValidateMessage(req); err != nil {
		return contracts.MessageIngressResponse{}, err
	}
	if len(s.store.CapabilityEndpoints(req.TargetCapability)) == 0 {
		return contracts.MessageIngressResponse{}, domain.ErrCapabilityNotFound
	}
	messageRef := req.MessageRef
	if messageRef == "" {
		messageRef = s.store.NextRef("message")
	}
	message := domain.Message{
		MessageRef:        messageRef,
		DeliveryRef:       s.store.NextRef("delivery"),
		SenderEndpointRef: req.SenderEndpointRef,
		TargetCapability:  req.TargetCapability,
		Payload:           req.Payload,
		Constraints:       req.Constraints,
		CorrelationRef:    req.CorrelationRef,
		ReadbackRequired:  req.ReadbackRequired,
		State:             domain.MessageStateAvailable,
	}
	if err := s.store.SaveMessage(message); err != nil {
		return contracts.MessageIngressResponse{}, err
	}
	return contracts.MessageIngressResponse{
		MessageRef:  message.MessageRef,
		DeliveryRef: message.DeliveryRef,
		State:       message.State,
	}, nil
}

func (s *Service) ClaimMessage(messageRef string, req contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error) {
	endpoint, ok := s.store.GetEndpoint(req.EndpointRef)
	if !ok {
		return contracts.ClaimMessageResponse{}, domain.ErrEndpointNotFound
	}
	message, ok := s.store.GetMessage(messageRef)
	if !ok {
		return contracts.ClaimMessageResponse{}, domain.ErrMessageNotFound
	}
	if _, ok := endpoint.Capabilities[message.TargetCapability]; !ok {
		return contracts.ClaimMessageResponse{}, domain.ErrCapabilityNotFound
	}
	if s.endpointIsStale(endpoint) {
		return contracts.ClaimMessageResponse{}, domain.ErrEndpointStale
	}
	leaseExpiresAt := s.now().Add(s.settings.AttemptLeaseDuration)
	result, err := s.store.ClaimMessage(domain.ClaimRequest{
		MessageRef:     message.MessageRef,
		Endpoint:       endpoint,
		AttemptRef:     s.store.NextRef("attempt"),
		DeadLetterRef:  s.store.NextRef("dead-letter"),
		LeaseExpiresAt: leaseExpiresAt,
		Now:            s.now(),
		MaxRedelivery:  s.settings.MaxRedelivery,
	})
	if err != nil {
		return contracts.ClaimMessageResponse{}, err
	}
	return claimResponseFromResult(result, endpoint.EndpointRef), nil
}

func (s *Service) ClaimNextMessage(req contracts.ClaimNextMessageRequest) (contracts.ClaimNextMessageResponse, error) {
	endpoint, ok := s.store.GetEndpoint(req.EndpointRef)
	if !ok {
		return contracts.ClaimNextMessageResponse{}, domain.ErrEndpointNotFound
	}
	if len(endpoint.Capabilities) == 0 {
		return contracts.ClaimNextMessageResponse{}, domain.ErrCapabilityNotFound
	}
	if s.endpointIsStale(endpoint) {
		return contracts.ClaimNextMessageResponse{}, domain.ErrEndpointStale
	}
	now := s.now()
	s.store.ReconcileExpiredClaims(now)
	result, found, err := s.store.ClaimNextMessage(domain.ClaimNextRequest{
		Endpoint:       endpoint,
		AttemptRef:     s.store.NextRef("attempt"),
		DeadLetterRef:  s.store.NextRef("dead-letter"),
		LeaseExpiresAt: now.Add(s.settings.AttemptLeaseDuration),
		Now:            now,
		MaxRedelivery:  s.settings.MaxRedelivery,
	})
	if err != nil {
		return contracts.ClaimNextMessageResponse{}, err
	}
	if !found {
		return contracts.ClaimNextMessageResponse{Empty: true}, nil
	}
	claim := claimResponseFromResult(result, endpoint.EndpointRef)
	return contracts.ClaimNextMessageResponse{Claim: &claim}, nil
}

func (s *Service) SubmitCheckpoint(attemptRef string, req contracts.CheckpointRequest) (contracts.CheckpointResponse, error) {
	if err := domain.ValidateCheckpoint(req); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	attempt, ok := s.store.GetAttempt(attemptRef)
	if !ok {
		return contracts.CheckpointResponse{}, domain.ErrAttemptNotFound
	}
	if attempt.EndpointRef != req.EndpointRef || attemptClosed(attempt.State) {
		return contracts.CheckpointResponse{}, domain.ErrStaleAttempt
	}
	if s.leaseExpired(attempt) {
		return contracts.CheckpointResponse{}, domain.ErrLeaseExpired
	}
	if err := s.validateCheckpointArtifactRefs(attempt, req.ArtifactRefs); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	checkpoint := domain.Checkpoint{
		CheckpointRef:     s.store.NextRef("checkpoint"),
		AttemptRef:        attempt.AttemptRef,
		EndpointRef:       req.EndpointRef,
		State:             req.State,
		Summary:           req.Summary,
		ArtifactRefs:      req.ArtifactRefs,
		FailureReasonCode: req.FailureReasonCode,
		MissingFields:     req.MissingFields,
		MissingReasons:    req.MissingReasons,
	}
	accepted, err := s.store.SaveCheckpoint(checkpoint)
	if err != nil {
		return contracts.CheckpointResponse{}, err
	}
	attempt.State = req.State
	attempt.Revision = accepted.Revision
	if err := s.store.UpdateAttempt(attempt); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	if err := s.updateMessageStateForCheckpoint(attempt.MessageRef, req.State); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	return contracts.CheckpointResponse{
		CheckpointRef: accepted.CheckpointRef,
		AttemptRef:    accepted.AttemptRef,
		State:         accepted.State,
		Revision:      accepted.Revision,
	}, nil
}

func (s *Service) SubmitReadback(attemptRef string, req contracts.ReadbackRequest) (contracts.ReadbackResponse, error) {
	if err := domain.ValidateReadback(req); err != nil {
		return contracts.ReadbackResponse{}, err
	}
	attempt, ok := s.store.GetAttempt(attemptRef)
	if !ok {
		return contracts.ReadbackResponse{}, domain.ErrAttemptNotFound
	}
	if attempt.EndpointRef != req.EndpointRef || attemptClosed(attempt.State) {
		return contracts.ReadbackResponse{}, domain.ErrStaleAttempt
	}
	if s.leaseExpired(attempt) {
		return contracts.ReadbackResponse{}, domain.ErrLeaseExpired
	}
	readback := domain.Readback{
		ReadbackRef:    s.store.NextRef("readback"),
		AttemptRef:     attempt.AttemptRef,
		EndpointRef:    req.EndpointRef,
		Summary:        req.Summary,
		Understanding:  req.Understanding,
		Questions:      req.Questions,
		MissingFields:  req.MissingFields,
		MissingReasons: req.MissingReasons,
	}
	accepted, err := s.store.SaveReadback(readback)
	if err != nil {
		return contracts.ReadbackResponse{}, err
	}
	return contracts.ReadbackResponse{
		ReadbackRef: accepted.ReadbackRef,
		AttemptRef:  accepted.AttemptRef,
		Revision:    accepted.Revision,
	}, nil
}

func (s *Service) RefreshLease(attemptRef string, req contracts.RefreshLeaseRequest) (contracts.RefreshLeaseResponse, error) {
	attempt, ok := s.store.GetAttempt(attemptRef)
	if !ok {
		return contracts.RefreshLeaseResponse{}, domain.ErrAttemptNotFound
	}
	if req.EndpointRef == "" || attempt.EndpointRef != req.EndpointRef || attemptClosed(attempt.State) {
		return contracts.RefreshLeaseResponse{}, domain.ErrStaleAttempt
	}
	if s.leaseExpired(attempt) {
		return contracts.RefreshLeaseResponse{}, domain.ErrLeaseExpired
	}
	attempt.LeaseExpiresAt = s.now().Add(s.settings.AttemptLeaseDuration)
	if err := s.store.UpdateAttempt(attempt); err != nil {
		return contracts.RefreshLeaseResponse{}, err
	}
	return contracts.RefreshLeaseResponse{
		AttemptRef:     attempt.AttemptRef,
		State:          attempt.State,
		LeaseExpiresAt: formatTime(attempt.LeaseExpiresAt),
	}, nil
}

func (s *Service) CompleteAttempt(attemptRef string, req contracts.CompleteAttemptRequest) (contracts.CompleteAttemptResponse, error) {
	if err := domain.ValidateCompletion(req); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	attempt, ok := s.store.GetAttempt(attemptRef)
	if !ok {
		return contracts.CompleteAttemptResponse{}, domain.ErrAttemptNotFound
	}
	if attempt.EndpointRef != req.EndpointRef {
		return contracts.CompleteAttemptResponse{}, domain.ErrStaleAttempt
	}
	if attemptClosed(attempt.State) {
		return contracts.CompleteAttemptResponse{}, domain.ErrStaleAttempt
	}
	if _, err := s.SubmitCheckpoint(attemptRef, contracts.CheckpointRequest{
		EndpointRef:  req.EndpointRef,
		State:        domain.AttemptStateCompleted,
		Summary:      req.Summary,
		ArtifactRefs: req.ArtifactRefs,
	}); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	attempt, _ = s.store.GetAttempt(attemptRef)
	attempt.State = domain.AttemptStateCompleted
	message, _ := s.store.GetMessage(attempt.MessageRef)
	message.State = domain.MessageStateCompleted
	if err := s.store.UpdateAttempt(attempt); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	if err := s.store.UpdateMessage(message); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	return contracts.CompleteAttemptResponse{AttemptRef: attempt.AttemptRef, State: attempt.State}, nil
}

func (s *Service) Snapshot() domain.Snapshot {
	snapshot := s.store.Snapshot()
	for index := range snapshot.Endpoints {
		endpoint := snapshot.Endpoints[index]
		if endpoint.ConnectionState == domain.EndpointStateOnline && s.endpointIsStale(endpoint) {
			snapshot.Endpoints[index].ConnectionState = domain.EndpointStateStale
		}
	}
	return snapshot
}

func (s *Service) ReconcileExpiredClaims() int {
	return s.store.ReconcileExpiredClaims(s.now())
}

func (s *Service) now() time.Time {
	return s.settings.Now().UTC()
}

func (s *Service) endpointIsStale(endpoint domain.Endpoint) bool {
	if endpoint.ConnectionState != domain.EndpointStateOnline {
		return true
	}
	return s.now().After(endpoint.LastHeartbeatAt.Add(s.settings.EndpointHeartbeatTimeout))
}

func (s *Service) leaseExpired(attempt domain.Attempt) bool {
	return !attempt.LeaseExpiresAt.IsZero() && s.now().After(attempt.LeaseExpiresAt)
}

func (s *Service) updateMessageStateForCheckpoint(messageRef string, attemptState string) error {
	message, ok := s.store.GetMessage(messageRef)
	if !ok {
		return domain.ErrMessageNotFound
	}
	switch attemptState {
	case domain.AttemptStateInProgress, domain.AttemptStateValidating:
		message.State = domain.MessageStateProcessing
	case domain.AttemptStateBlockedMissingFields:
		message.State = domain.MessageStateInputRequired
	case domain.AttemptStateFailed:
		message.State = domain.MessageStateFailed
	case domain.AttemptStateCompleted:
		message.State = domain.MessageStateCompleted
	}
	return s.store.UpdateMessage(message)
}

func capabilityMap(items []contracts.Capability) map[string]contracts.Capability {
	capabilities := make(map[string]contracts.Capability, len(items))
	for _, item := range items {
		capabilities[item.Name] = item
	}
	return capabilities
}

func capabilityNames(items map[string]contracts.Capability) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	return names
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func constraintSummary(items []contracts.Constraint) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Code != "" {
			parts = append(parts, item.Code)
		}
	}
	return strings.Join(parts, ",")
}

func claimResponseFromResult(result domain.ClaimResult, endpointRef string) contracts.ClaimMessageResponse {
	return contracts.ClaimMessageResponse{
		MessageRef:         result.Message.MessageRef,
		AttemptRef:         result.Attempt.AttemptRef,
		EndpointRef:        endpointRef,
		State:              result.Attempt.State,
		LeaseExpiresAt:     formatTime(result.Attempt.LeaseExpiresAt),
		Takeover:           result.Takeover,
		RedeliveryCount:    result.Message.RedeliveryCount,
		LastCheckpointRef:  result.LastCheckpointRef,
		ResumeSummary:      result.ResumeSummary,
		ResumeArtifactRefs: result.ResumeArtifactRefs,
		ReadbackRequired:   result.Message.ReadbackRequired,
		TargetCapability:   result.Message.TargetCapability,
		CorrelationRef:     result.Message.CorrelationRef,
		Payload:            result.Message.Payload,
		Constraints:        result.Message.Constraints,
		PayloadSummary:     result.Message.Payload.Summary,
		ConstraintSummary:  constraintSummary(result.Message.Constraints),
	}
}

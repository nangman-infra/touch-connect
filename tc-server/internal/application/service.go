package application

import (
	"errors"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/internal/communication/quality"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

type Service struct {
	endpoints   EndpointRegistry
	messages    MessageLedger
	processing  ProcessingLedger
	readbacks   ReadbackLedger
	artifacts   ArtifactLedger
	governance  GovernanceLedger
	quality     QualityLedger
	delivery    DeliveryAdapter
	refs        RefAllocator
	projections ProjectionReader
	settings    Settings
}

type ServicePorts struct {
	Endpoints   EndpointRegistry
	Messages    MessageLedger
	Processing  ProcessingLedger
	Readbacks   ReadbackLedger
	Artifacts   ArtifactLedger
	Governance  GovernanceLedger
	Quality     QualityLedger
	Refs        RefAllocator
	Projections ProjectionReader
}

func PortsFromStore(store Store) ServicePorts {
	return ServicePorts{
		Endpoints:   store,
		Messages:    store,
		Processing:  store,
		Readbacks:   store,
		Artifacts:   store,
		Governance:  store,
		Quality:     store,
		Refs:        store,
		Projections: store,
	}
}

func NewService(ports ServicePorts, settings Settings) (*Service, error) {
	return NewServiceWithDeliveryAdapter(ports, nil, settings)
}

func NewServiceWithDeliveryAdapter(ports ServicePorts, delivery DeliveryAdapter, settings Settings) (*Service, error) {
	if ports.Endpoints == nil {
		return nil, errors.New("endpoint registry is required")
	}
	if ports.Messages == nil {
		return nil, errors.New("message ledger is required")
	}
	if ports.Processing == nil {
		return nil, errors.New("processing ledger is required")
	}
	if ports.Readbacks == nil {
		return nil, errors.New("readback ledger is required")
	}
	if ports.Artifacts == nil {
		return nil, errors.New("artifact ledger is required")
	}
	if ports.Governance == nil {
		return nil, errors.New("governance ledger is required")
	}
	if ports.Quality == nil {
		return nil, errors.New("quality ledger is required")
	}
	if ports.Refs == nil {
		return nil, errors.New("ref allocator is required")
	}
	if ports.Projections == nil {
		return nil, errors.New("projection reader is required")
	}
	accepted, err := settings.Validated()
	if err != nil {
		return nil, err
	}
	return &Service{
		endpoints:   ports.Endpoints,
		messages:    ports.Messages,
		processing:  ports.Processing,
		readbacks:   ports.Readbacks,
		artifacts:   ports.Artifacts,
		governance:  ports.Governance,
		quality:     ports.Quality,
		delivery:    delivery,
		refs:        ports.Refs,
		projections: ports.Projections,
		settings:    accepted,
	}, nil
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
	if existing, ok := s.endpoints.GetEndpoint(req.EndpointRef); ok &&
		existing.ConnectionState == domain.EndpointStateOnline &&
		!s.endpointIsStale(existing) {
		return contracts.EndpointRegistrationResponse{}, domain.ErrEndpointAlreadyOnline
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
	if err := s.endpoints.SaveEndpoint(endpoint); err != nil {
		return contracts.EndpointRegistrationResponse{}, err
	}
	return contracts.EndpointRegistrationResponse{
		EndpointRef: endpoint.EndpointRef,
		AcceptedRef: s.refs.NextRef("accepted"),
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
	endpoint, ok := s.endpoints.GetEndpoint(endpointRef)
	if !ok {
		return contracts.EndpointHeartbeatResponse{}, domain.ErrEndpointNotFound
	}
	now := s.now()
	endpoint.ConnectionState = req.ConnectionState
	endpoint.LastHeartbeatAt = now
	endpoint.CurrentAttemptRef = strings.TrimSpace(req.CurrentAttemptRef)
	endpoint.ProgressSummary = strings.TrimSpace(req.ProgressSummary)
	if req.LastActivityAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, req.LastActivityAt); err == nil {
			endpoint.LastActivityAt = parsed.UTC()
		}
	} else if endpoint.CurrentAttemptRef != "" {
		endpoint.LastActivityAt = now
	}
	if err := s.endpoints.UpdateEndpoint(endpoint); err != nil {
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
	endpoint, err := s.endpoints.UpdateCapabilities(endpointRef, capabilityMap(req.Capabilities))
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
	if !s.ingressRouteAvailable(req) {
		return contracts.MessageIngressResponse{}, domain.ErrCapabilityNotFound
	}
	messageRef := req.MessageRef
	if messageRef == "" {
		messageRef = s.refs.NextRef("message")
	}
	message := domain.Message{
		MessageRef:           messageRef,
		DeliveryRef:          s.refs.NextRef("delivery"),
		SenderEndpointRef:    req.SenderEndpointRef,
		TargetCapability:     req.TargetCapability,
		TargetEndpointRef:    req.TargetEndpointRef,
		DependsOnMessageRefs: append([]string(nil), req.DependsOnMessageRefs...),
		Payload:              req.Payload,
		Constraints:          req.Constraints,
		CorrelationRef:       req.CorrelationRef,
		ReadbackRequired:     req.ReadbackRequired,
		State:                domain.MessageStateAvailable,
	}
	qualityDecision := quality.ValidateMessageWithGate(quality.ValidationInput{
		DecisionRef: s.refs.NextRef("quality-decision"),
		MessageRef:  message.MessageRef,
		Request:     req,
		CreatedAt:   s.now(),
		CreatedBy:   req.SenderEndpointRef,
	}, req.QualityGate)
	if err := s.quality.SaveQualityDecision(qualityDecision); err != nil {
		return contracts.MessageIngressResponse{}, err
	}
	if qualityDecision.Decision == contracts.QualityDecisionRejected {
		return contracts.MessageIngressResponse{}, QualityRejectedError{Decision: qualityDecision}
	}
	if err := s.messages.SaveMessage(message); err != nil {
		return contracts.MessageIngressResponse{}, err
	}
	if s.delivery != nil {
		if _, err := s.delivery.PublishAcceptedMessage(message); err != nil {
			return contracts.MessageIngressResponse{}, err
		}
	}
	return contracts.MessageIngressResponse{
		MessageRef:         message.MessageRef,
		DeliveryRef:        message.DeliveryRef,
		State:              message.State,
		QualityDecisionRef: qualityDecision.QualityDecisionRef,
	}, nil
}

func (s *Service) ClaimMessage(messageRef string, req contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error) {
	endpoint, ok := s.endpoints.GetEndpoint(req.EndpointRef)
	if !ok {
		return contracts.ClaimMessageResponse{}, domain.ErrEndpointNotFound
	}
	message, ok := s.messages.GetMessage(messageRef)
	if !ok {
		return contracts.ClaimMessageResponse{}, domain.ErrMessageNotFound
	}
	if !domain.MessageRoutableToEndpoint(message, endpoint) {
		return contracts.ClaimMessageResponse{}, domain.ErrCapabilityNotFound
	}
	if !s.messageDependenciesCompleted(message) {
		return contracts.ClaimMessageResponse{}, domain.ErrMessageUnavailable
	}
	if s.endpointIsStale(endpoint) {
		return contracts.ClaimMessageResponse{}, domain.ErrEndpointStale
	}
	leaseExpiresAt := s.now().Add(s.settings.AttemptLeaseDuration)
	result, err := s.processing.ClaimMessage(domain.ClaimRequest{
		MessageRef:     message.MessageRef,
		Endpoint:       endpoint,
		AttemptRef:     s.refs.NextRef("attempt"),
		DeadLetterRef:  s.refs.NextRef("dead-letter"),
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
	endpoint, ok := s.endpoints.GetEndpoint(req.EndpointRef)
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
	s.processing.ReconcileExpiredClaims(now)
	if s.delivery != nil {
		return s.claimNextMessageWithDeliveryAdapter(endpoint, now)
	}
	result, found, err := s.processing.ClaimNextMessage(domain.ClaimNextRequest{
		Endpoint:       endpoint,
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

func (s *Service) claimNextMessageWithDeliveryAdapter(endpoint domain.Endpoint, now time.Time) (contracts.ClaimNextMessageResponse, error) {
	delivery, found, err := s.delivery.FetchNextDelivery(DeliveryFetchRequest{
		EndpointRef:  endpoint.EndpointRef,
		Capabilities: capabilityNames(endpoint.Capabilities),
	})
	if err != nil {
		return contracts.ClaimNextMessageResponse{}, err
	}
	if !found {
		return contracts.ClaimNextMessageResponse{Empty: true}, nil
	}
	message, ok := s.messages.GetMessage(delivery.MessageRef)
	if !ok {
		if nakErr := s.delivery.NakDelivery(delivery.DeliveryRef, domain.ErrMessageNotFound.Error()); nakErr != nil {
			return contracts.ClaimNextMessageResponse{}, nakErr
		}
		return contracts.ClaimNextMessageResponse{}, domain.ErrMessageNotFound
	}
	if !domain.MessageRoutableToEndpoint(message, endpoint) || !s.messageDependenciesCompleted(message) {
		if nakErr := s.delivery.NakDelivery(delivery.DeliveryRef, domain.ErrMessageUnavailable.Error()); nakErr != nil {
			return contracts.ClaimNextMessageResponse{}, nakErr
		}
		return contracts.ClaimNextMessageResponse{Empty: true}, nil
	}
	result, err := s.processing.ClaimMessage(domain.ClaimRequest{
		MessageRef:     delivery.MessageRef,
		Endpoint:       endpoint,
		AttemptRef:     s.refs.NextRef("attempt"),
		DeadLetterRef:  s.refs.NextRef("dead-letter"),
		LeaseExpiresAt: now.Add(s.settings.AttemptLeaseDuration),
		Now:            now,
		MaxRedelivery:  s.settings.MaxRedelivery,
	})
	if err != nil {
		if errors.Is(err, domain.ErrMessageDeadLettered) {
			if ackErr := s.delivery.AckDelivery(delivery.DeliveryRef); ackErr != nil {
				return contracts.ClaimNextMessageResponse{}, errors.Join(err, ackErr)
			}
			return contracts.ClaimNextMessageResponse{Empty: true}, nil
		}
		if nakErr := s.delivery.NakDelivery(delivery.DeliveryRef, err.Error()); nakErr != nil {
			return contracts.ClaimNextMessageResponse{}, errors.Join(err, nakErr)
		}
		return contracts.ClaimNextMessageResponse{}, err
	}
	claim := claimResponseFromResult(result, endpoint.EndpointRef)
	return contracts.ClaimNextMessageResponse{Claim: &claim}, nil
}

func (s *Service) ingressRouteAvailable(req contracts.MessageIngressRequest) bool {
	if req.TargetEndpointRef != "" {
		endpoint, ok := s.endpoints.GetEndpoint(req.TargetEndpointRef)
		return ok && domain.EndpointCanHandle(endpoint, req.TargetCapability)
	}
	for _, endpoint := range s.Snapshot().Endpoints {
		if domain.EndpointCanHandle(endpoint, req.TargetCapability) {
			return true
		}
	}
	return false
}

func (s *Service) messageDependenciesCompleted(message domain.Message) bool {
	if len(message.DependsOnMessageRefs) == 0 {
		return true
	}
	states := make(map[string]string, len(message.DependsOnMessageRefs))
	for _, ref := range message.DependsOnMessageRefs {
		if dependency, ok := s.messages.GetMessage(ref); ok {
			states[ref] = dependency.State
		}
	}
	return domain.MessageDependenciesCompleted(message, states)
}

func (s *Service) SubmitCheckpoint(attemptRef string, req contracts.CheckpointRequest) (contracts.CheckpointResponse, error) {
	if err := domain.ValidateCheckpoint(req); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	attempt, ok := s.processing.GetAttempt(attemptRef)
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
		CheckpointRef:     s.refs.NextRef("checkpoint"),
		AttemptRef:        attempt.AttemptRef,
		EndpointRef:       req.EndpointRef,
		State:             req.State,
		Summary:           req.Summary,
		ArtifactRefs:      req.ArtifactRefs,
		FailureReasonCode: req.FailureReasonCode,
		MissingFields:     req.MissingFields,
		MissingReasons:    req.MissingReasons,
	}
	accepted, err := s.processing.SaveCheckpoint(checkpoint)
	if err != nil {
		return contracts.CheckpointResponse{}, err
	}
	attempt.State = req.State
	attempt.Revision = accepted.Revision
	if err := s.processing.UpdateAttempt(attempt); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	if err := s.updateMessageStateForCheckpoint(attempt.MessageRef, req.State); err != nil {
		return contracts.CheckpointResponse{}, err
	}
	if err := s.ackDeliveryForTerminalCheckpoint(attempt.MessageRef, req.State); err != nil {
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
	attempt, ok := s.processing.GetAttempt(attemptRef)
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
		ReadbackRef:    s.refs.NextRef("readback"),
		AttemptRef:     attempt.AttemptRef,
		EndpointRef:    req.EndpointRef,
		Summary:        req.Summary,
		Understanding:  req.Understanding,
		Questions:      req.Questions,
		MissingFields:  req.MissingFields,
		MissingReasons: req.MissingReasons,
	}
	accepted, err := s.readbacks.SaveReadback(readback)
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
	attempt, ok := s.processing.GetAttempt(attemptRef)
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
	if err := s.processing.UpdateAttempt(attempt); err != nil {
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
	attempt, ok := s.processing.GetAttempt(attemptRef)
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
	attempt, _ = s.processing.GetAttempt(attemptRef)
	attempt.State = domain.AttemptStateCompleted
	message, _ := s.messages.GetMessage(attempt.MessageRef)
	message.State = domain.MessageStateCompleted
	if err := s.processing.UpdateAttempt(attempt); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	if err := s.messages.UpdateMessage(message); err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	followUpRefs, err := s.createFollowUpMessages(req, message)
	if err != nil {
		return contracts.CompleteAttemptResponse{}, err
	}
	return contracts.CompleteAttemptResponse{
		AttemptRef:          attempt.AttemptRef,
		State:               attempt.State,
		FollowUpMessageRefs: followUpRefs,
	}, nil
}

func (s *Service) createFollowUpMessages(req contracts.CompleteAttemptRequest, parent domain.Message) ([]string, error) {
	refs := make([]string, 0, len(req.FollowUpMessages))
	for _, followUp := range req.FollowUpMessages {
		dependsOn := append([]string(nil), followUp.DependsOnMessageRefs...)
		if len(dependsOn) == 0 {
			dependsOn = []string{parent.MessageRef}
		}
		constraints := append([]contracts.Constraint(nil), followUp.Constraints...)
		if constraints == nil {
			constraints = []contracts.Constraint{}
		}
		response, err := s.IngressMessage(contracts.MessageIngressRequest{
			MessageRef:           followUp.MessageRef,
			SenderEndpointRef:    req.EndpointRef,
			TargetCapability:     followUp.TargetCapability,
			TargetEndpointRef:    followUp.TargetEndpointRef,
			DependsOnMessageRefs: dependsOn,
			Payload: contracts.Payload{
				Summary:    followUp.Summary,
				Body:       followUp.Body,
				References: []contracts.Reference{},
			},
			Constraints:      constraints,
			CorrelationRef:   parent.CorrelationRef,
			ReadbackRequired: followUp.ReadbackRequired,
			QualityGate:      followUp.QualityGate,
		})
		if err != nil {
			return nil, err
		}
		refs = append(refs, response.MessageRef)
	}
	return refs, nil
}

func (s *Service) Snapshot() domain.Snapshot {
	snapshot := s.projections.Snapshot()
	for index := range snapshot.Endpoints {
		endpoint := snapshot.Endpoints[index]
		if endpoint.ConnectionState == domain.EndpointStateOnline && s.endpointIsStale(endpoint) {
			snapshot.Endpoints[index].ConnectionState = domain.EndpointStateStale
		}
	}
	return snapshot
}

func (s *Service) ReconcileExpiredClaims() int {
	return s.processing.ReconcileExpiredClaims(s.now())
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
	message, ok := s.messages.GetMessage(messageRef)
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
	return s.messages.UpdateMessage(message)
}

func (s *Service) ackDeliveryForTerminalCheckpoint(messageRef string, attemptState string) error {
	if s.delivery == nil || !attemptClosed(attemptState) || attemptState == domain.AttemptStateOrphaned {
		return nil
	}
	message, ok := s.messages.GetMessage(messageRef)
	if !ok {
		return domain.ErrMessageNotFound
	}
	return s.delivery.AckDelivery(message.DeliveryRef)
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
		MessageRef:           result.Message.MessageRef,
		AttemptRef:           result.Attempt.AttemptRef,
		EndpointRef:          endpointRef,
		State:                result.Attempt.State,
		LeaseExpiresAt:       formatTime(result.Attempt.LeaseExpiresAt),
		Takeover:             result.Takeover,
		RedeliveryCount:      result.Message.RedeliveryCount,
		LastCheckpointRef:    result.LastCheckpointRef,
		ResumeSummary:        result.ResumeSummary,
		ResumeArtifactRefs:   result.ResumeArtifactRefs,
		ReadbackRequired:     result.Message.ReadbackRequired,
		TargetCapability:     result.Message.TargetCapability,
		TargetEndpointRef:    result.Message.TargetEndpointRef,
		DependsOnMessageRefs: append([]string(nil), result.Message.DependsOnMessageRefs...),
		CorrelationRef:       result.Message.CorrelationRef,
		Payload:              result.Message.Payload,
		Constraints:          result.Message.Constraints,
		PayloadSummary:       result.Message.Payload.Summary,
		ConstraintSummary:    constraintSummary(result.Message.Constraints),
	}
}

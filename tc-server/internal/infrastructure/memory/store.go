package memory

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

type Store struct {
	mu            sync.Mutex
	sequences     map[string]int
	endpoints     map[string]domain.Endpoint
	messages      map[string]domain.Message
	attempts      map[string]domain.Attempt
	checkpoints   map[string][]domain.Checkpoint
	readbacks     map[string][]domain.Readback
	artifacts     map[string]domain.ArtifactVersion
	finalizations map[string]domain.ArtifactFinalization
	deadLetters   map[string]domain.DeadLetter
	approvals     map[string]domain.ApprovalDecision
	sideEffects   map[string]domain.SideEffectExecution
}

func NewStore() *Store {
	return &Store{
		sequences:     map[string]int{},
		endpoints:     map[string]domain.Endpoint{},
		messages:      map[string]domain.Message{},
		attempts:      map[string]domain.Attempt{},
		checkpoints:   map[string][]domain.Checkpoint{},
		readbacks:     map[string][]domain.Readback{},
		artifacts:     map[string]domain.ArtifactVersion{},
		finalizations: map[string]domain.ArtifactFinalization{},
		deadLetters:   map[string]domain.DeadLetter{},
		approvals:     map[string]domain.ApprovalDecision{},
		sideEffects:   map[string]domain.SideEffectExecution{},
	}
}

func (s *Store) SaveEndpoint(endpoint domain.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[endpoint.EndpointRef] = endpoint
	return nil
}

func (s *Store) GetEndpoint(endpointRef string) (domain.Endpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoint, ok := s.endpoints[endpointRef]
	return endpoint, ok
}

func (s *Store) UpdateCapabilities(endpointRef string, capabilities map[string]contracts.Capability) (domain.Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoint, ok := s.endpoints[endpointRef]
	if !ok {
		return domain.Endpoint{}, domain.ErrEndpointNotFound
	}
	endpoint.Capabilities = capabilities
	s.endpoints[endpointRef] = endpoint
	return endpoint, nil
}

func (s *Store) UpdateEndpoint(endpoint domain.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.endpoints[endpoint.EndpointRef]; !ok {
		return domain.ErrEndpointNotFound
	}
	s.endpoints[endpoint.EndpointRef] = endpoint
	return nil
}

func (s *Store) CapabilityEndpoints(capability string) []domain.Endpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoints := make([]domain.Endpoint, 0)
	for _, endpoint := range s.endpoints {
		if _, ok := endpoint.Capabilities[capability]; ok {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (s *Store) SaveMessage(message domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.messages[message.MessageRef]; exists {
		return domain.ErrInvalidInput
	}
	s.messages[message.MessageRef] = message
	return nil
}

func (s *Store) GetMessage(messageRef string) (domain.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message, ok := s.messages[messageRef]
	return message, ok
}

func (s *Store) UpdateMessage(message domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.messages[message.MessageRef]; !ok {
		return domain.ErrMessageNotFound
	}
	s.messages[message.MessageRef] = message
	return nil
}

func (s *Store) ClaimMessage(claim domain.ClaimRequest) (domain.ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message, ok := s.messages[claim.MessageRef]
	if !ok {
		return domain.ClaimResult{}, domain.ErrMessageNotFound
	}
	if message.State == domain.MessageStateAvailable {
		return s.claimAvailableMessage(message, claim), nil
	}
	if message.State == domain.MessageStateDeadLettered {
		return domain.ClaimResult{}, domain.ErrMessageDeadLettered
	}
	if message.State == domain.MessageStateTakeoverCandidate {
		current, ok := s.attempts[message.AttemptRef]
		if !ok {
			return domain.ClaimResult{}, domain.ErrMessageUnavailable
		}
		if message.RedeliveryCount >= claim.MaxRedelivery {
			s.deadLetterMessage(message, current, claim)
			return domain.ClaimResult{}, domain.ErrMessageDeadLettered
		}
		return s.takeoverMessage(message, claim), nil
	}
	if message.State != domain.MessageStateClaimed && message.State != domain.MessageStateProcessing {
		return domain.ClaimResult{}, domain.ErrMessageUnavailable
	}
	current, ok := s.attempts[message.AttemptRef]
	if !ok || claim.Now.Before(current.LeaseExpiresAt) || claim.Now.Equal(current.LeaseExpiresAt) {
		return domain.ClaimResult{}, domain.ErrMessageUnavailable
	}
	if message.RedeliveryCount >= claim.MaxRedelivery {
		s.deadLetterMessage(message, current, claim)
		return domain.ClaimResult{}, domain.ErrMessageDeadLettered
	}
	current.State = domain.AttemptStateOrphaned
	s.attempts[current.AttemptRef] = current
	return s.takeoverMessage(message, claim), nil
}

func (s *Store) SaveAttempt(attempt domain.Attempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[attempt.AttemptRef] = attempt
	return nil
}

func (s *Store) claimAvailableMessage(message domain.Message, claim domain.ClaimRequest) domain.ClaimResult {
	attempt := domain.Attempt{
		AttemptRef:     claim.AttemptRef,
		MessageRef:     message.MessageRef,
		EndpointRef:    claim.Endpoint.EndpointRef,
		State:          domain.AttemptStateClaimed,
		LeaseExpiresAt: claim.LeaseExpiresAt,
		Revision:       1,
		AttemptNo:      message.RedeliveryCount + 1,
		ClaimEpoch:     message.RedeliveryCount + 1,
	}
	message.State = domain.MessageStateClaimed
	message.AttemptRef = attempt.AttemptRef
	s.attempts[attempt.AttemptRef] = attempt
	s.messages[message.MessageRef] = message
	return domain.ClaimResult{Message: message, Attempt: attempt}
}

func (s *Store) takeoverMessage(message domain.Message, claim domain.ClaimRequest) domain.ClaimResult {
	previousAttemptRef := message.AttemptRef
	latest := latestCheckpoint(s.checkpoints[previousAttemptRef])
	message.RedeliveryCount++
	result := s.claimAvailableMessage(message, claim)
	result.Takeover = true
	if latest != nil {
		result.LastCheckpointRef = latest.CheckpointRef
		result.ResumeSummary = latest.Summary
		result.ResumeArtifactRefs = append([]string(nil), latest.ArtifactRefs...)
	}
	return result
}

func (s *Store) deadLetterMessage(message domain.Message, attempt domain.Attempt, claim domain.ClaimRequest) {
	latest := latestCheckpoint(s.checkpoints[attempt.AttemptRef])
	deadLetter := domain.DeadLetter{
		DeadLetterRef:   claim.DeadLetterRef,
		MessageRef:      message.MessageRef,
		LastAttemptRef:  attempt.AttemptRef,
		Reason:          "max_redelivery_exceeded",
		RedeliveryCount: message.RedeliveryCount,
		CreatedAt:       claim.Now,
	}
	if latest != nil {
		deadLetter.LastCheckpointRef = latest.CheckpointRef
	}
	attempt.State = domain.AttemptStateOrphaned
	message.State = domain.MessageStateDeadLettered
	s.attempts[attempt.AttemptRef] = attempt
	s.messages[message.MessageRef] = message
	s.deadLetters[deadLetter.DeadLetterRef] = deadLetter
}

func (s *Store) GetAttempt(attemptRef string) (domain.Attempt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[attemptRef]
	return attempt, ok
}

func (s *Store) UpdateAttempt(attempt domain.Attempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.attempts[attempt.AttemptRef]; !ok {
		return domain.ErrAttemptNotFound
	}
	s.attempts[attempt.AttemptRef] = attempt
	return nil
}

func (s *Store) SaveCheckpoint(checkpoint domain.Checkpoint) (domain.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision := len(s.checkpoints[checkpoint.AttemptRef]) + 1
	checkpoint.Revision = revision
	s.checkpoints[checkpoint.AttemptRef] = append(s.checkpoints[checkpoint.AttemptRef], checkpoint)
	return checkpoint, nil
}

func (s *Store) SaveReadback(readback domain.Readback) (domain.Readback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision := len(s.readbacks[readback.AttemptRef]) + 1
	readback.Revision = revision
	s.readbacks[readback.AttemptRef] = append(s.readbacks[readback.AttemptRef], readback)
	return readback, nil
}

func (s *Store) SaveArtifactVersion(version domain.ArtifactVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.artifacts[version.ArtifactVersionRef]; exists {
		return domain.ErrArtifactExists
	}
	s.artifacts[version.ArtifactVersionRef] = version
	return nil
}

func (s *Store) GetArtifactVersion(artifactVersionRef string) (domain.ArtifactVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, ok := s.artifacts[artifactVersionRef]
	return version, ok
}

func (s *Store) SaveArtifactFinalization(finalization domain.ArtifactFinalization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finalizations[finalization.ArtifactVersionRef] = finalization
	return nil
}

func (s *Store) GetArtifactFinalization(artifactVersionRef string) (domain.ArtifactFinalization, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	finalization, ok := s.finalizations[artifactVersionRef]
	return finalization, ok
}

func (s *Store) SaveApprovalDecision(decision domain.ApprovalDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.approvals[decision.ApprovalRef]; exists {
		return domain.ErrSideEffectConflict
	}
	s.approvals[decision.ApprovalRef] = decision
	return nil
}

func (s *Store) GetApprovalDecision(approvalRef string) (domain.ApprovalDecision, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	decision, ok := s.approvals[approvalRef]
	return decision, ok
}

func (s *Store) SaveSideEffectExecution(execution domain.SideEffectExecution) (domain.SideEffectExecution, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.sideEffects {
		if existing.IdempotencyKey == execution.IdempotencyKey && existing.ProtectedScope == execution.ProtectedScope {
			return existing, true, nil
		}
	}
	if _, exists := s.sideEffects[execution.SideEffectExecutionRef]; exists {
		return domain.SideEffectExecution{}, false, domain.ErrSideEffectConflict
	}
	s.sideEffects[execution.SideEffectExecutionRef] = execution
	return execution, false, nil
}

func (s *Store) GetSideEffectExecution(executionRef string) (domain.SideEffectExecution, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	execution, ok := s.sideEffects[executionRef]
	return execution, ok
}

func (s *Store) UpdateSideEffectExecution(execution domain.SideEffectExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sideEffects[execution.SideEffectExecutionRef]; !ok {
		return domain.ErrSideEffectNotFound
	}
	s.sideEffects[execution.SideEffectExecutionRef] = execution
	return nil
}

func (s *Store) ReconcileExpiredClaims(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	reconciled := 0
	for _, message := range s.messages {
		if message.State != domain.MessageStateClaimed && message.State != domain.MessageStateProcessing {
			continue
		}
		attempt, ok := s.attempts[message.AttemptRef]
		if !ok || now.Before(attempt.LeaseExpiresAt) || now.Equal(attempt.LeaseExpiresAt) {
			continue
		}
		attempt.State = domain.AttemptStateOrphaned
		message.State = domain.MessageStateTakeoverCandidate
		s.attempts[attempt.AttemptRef] = attempt
		s.messages[message.MessageRef] = message
		reconciled++
	}
	return reconciled
}

func (s *Store) NextRef(kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextRefLocked(kind)
}

func (s *Store) nextRefLocked(kind string) string {
	s.sequences[kind]++
	return fmt.Sprintf("tc://%s/%s_%06d", kind, kindPrefix(kind), s.sequences[kind])
}

func (s *Store) Snapshot() domain.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.Snapshot{
		Endpoints:     sortedMapValues(s.endpoints, func(item domain.Endpoint) string { return item.EndpointRef }),
		Messages:      sortedMapValues(s.messages, func(item domain.Message) string { return item.MessageRef }),
		Attempts:      sortedMapValues(s.attempts, func(item domain.Attempt) string { return item.AttemptRef }),
		Checkpoints:   flattenCheckpoints(s.checkpoints),
		Readbacks:     flattenReadbacks(s.readbacks),
		Artifacts:     sortedMapValues(s.artifacts, func(item domain.ArtifactVersion) string { return item.ArtifactVersionRef }),
		Finalizations: sortedMapValues(s.finalizations, func(item domain.ArtifactFinalization) string { return item.ArtifactVersionRef }),
		DeadLetters:   sortedMapValues(s.deadLetters, func(item domain.DeadLetter) string { return item.DeadLetterRef }),
		Approvals:     sortedMapValues(s.approvals, func(item domain.ApprovalDecision) string { return item.ApprovalRef }),
		SideEffects:   sortedMapValues(s.sideEffects, func(item domain.SideEffectExecution) string { return item.SideEffectExecutionRef }),
	}
}

func kindPrefix(kind string) string {
	switch kind {
	case "message":
		return "msg"
	case "delivery":
		return "dlv"
	case "attempt":
		return "att"
	case "checkpoint":
		return "ckp"
	case "readback":
		return "rdb"
	case "dead-letter":
		return "dlq"
	case "side-effect":
		return "sfx"
	case "accepted":
		return "acc"
	default:
		return "ref"
	}
}

func sortedMapValues[T any](items map[string]T, key func(T) string) []T {
	values := make([]T, 0, len(items))
	for _, item := range items {
		values = append(values, item)
	}
	sort.Slice(values, func(i, j int) bool {
		return key(values[i]) < key(values[j])
	})
	return values
}

func flattenCheckpoints(items map[string][]domain.Checkpoint) []domain.Checkpoint {
	values := make([]domain.Checkpoint, 0)
	for _, checkpoints := range items {
		values = append(values, checkpoints...)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].AttemptRef == values[j].AttemptRef {
			return values[i].Revision < values[j].Revision
		}
		return values[i].AttemptRef < values[j].AttemptRef
	})
	return values
}

func latestCheckpoint(items []domain.Checkpoint) *domain.Checkpoint {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.Revision > latest.Revision {
			latest = item
		}
	}
	return &latest
}

func flattenReadbacks(items map[string][]domain.Readback) []domain.Readback {
	values := make([]domain.Readback, 0)
	for _, readbacks := range items {
		values = append(values, readbacks...)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].AttemptRef == values[j].AttemptRef {
			return values[i].Revision < values[j].Revision
		}
		return values[i].AttemptRef < values[j].AttemptRef
	})
	return values
}

package memory

import (
	"sort"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) ClaimNextMessage(claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range []string{domain.MessageStateTakeoverCandidate, domain.MessageStateAvailable} {
		message, ok := s.nextEligibleMessage(state, claim)
		if !ok {
			continue
		}
		result, found, err := s.claimNextEligibleMessage(message, claim)
		return result, found, err
	}
	return domain.ClaimResult{}, false, nil
}

func (s *Store) nextEligibleMessage(state string, claim domain.ClaimNextRequest) (domain.Message, bool) {
	refs := make([]string, 0, len(s.messages))
	for messageRef, message := range s.messages {
		if message.State == state {
			refs = append(refs, messageRef)
		}
	}
	sort.Strings(refs)
	states := make(map[string]string, len(s.messages))
	for ref, message := range s.messages {
		states[ref] = message.State
	}
	for _, messageRef := range refs {
		message := s.messages[messageRef]
		if domain.MessageClaimableByEndpoint(message, claim.Endpoint, domain.PreferredEndpointOnlineForMessage(message, claim.PreferredEndpoints)) &&
			domain.MessageDependenciesCompleted(message, states) {
			return message, true
		}
	}
	return domain.Message{}, false
}

func (s *Store) claimNextEligibleMessage(message domain.Message, claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error) {
	attemptRef := claim.AttemptRef
	if attemptRef == "" {
		attemptRef = s.nextRefLocked("attempt")
	}
	deadLetterRef := claim.DeadLetterRef
	if deadLetterRef == "" {
		deadLetterRef = s.nextRefLocked("dead-letter")
	}
	request := domain.ClaimRequest{
		MessageRef:     message.MessageRef,
		Endpoint:       claim.Endpoint,
		AttemptRef:     attemptRef,
		DeadLetterRef:  deadLetterRef,
		LeaseExpiresAt: claim.LeaseExpiresAt,
		Now:            claim.Now,
		MaxRedelivery:  claim.MaxRedelivery,
	}
	if message.State == domain.MessageStateAvailable {
		return s.claimAvailableMessage(message, request), true, nil
	}
	current, ok := s.attempts[message.AttemptRef]
	if !ok {
		return domain.ClaimResult{}, false, nil
	}
	if message.RedeliveryCount >= claim.MaxRedelivery {
		s.deadLetterMessage(message, current, request)
		return domain.ClaimResult{}, false, nil
	}
	return s.takeoverMessage(message, request), true, nil
}

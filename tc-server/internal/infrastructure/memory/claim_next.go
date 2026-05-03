package memory

import (
	"sort"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) ClaimNextMessage(claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range []string{domain.MessageStateTakeoverCandidate, domain.MessageStateAvailable} {
		message, ok := s.nextEligibleMessage(state, claim.Endpoint)
		if !ok {
			continue
		}
		result, found, err := s.claimNextEligibleMessage(message, claim)
		return result, found, err
	}
	return domain.ClaimResult{}, false, nil
}

func (s *Store) nextEligibleMessage(state string, endpoint domain.Endpoint) (domain.Message, bool) {
	refs := make([]string, 0, len(s.messages))
	for messageRef, message := range s.messages {
		if message.State == state {
			refs = append(refs, messageRef)
		}
	}
	sort.Strings(refs)
	for _, messageRef := range refs {
		message := s.messages[messageRef]
		if _, ok := endpoint.Capabilities[message.TargetCapability]; ok {
			return message, true
		}
	}
	return domain.Message{}, false
}

func (s *Store) claimNextEligibleMessage(message domain.Message, claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error) {
	request := domain.ClaimRequest{
		MessageRef:     message.MessageRef,
		Endpoint:       claim.Endpoint,
		AttemptRef:     claim.AttemptRef,
		DeadLetterRef:  claim.DeadLetterRef,
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

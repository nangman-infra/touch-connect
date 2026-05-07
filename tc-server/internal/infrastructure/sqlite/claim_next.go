package sqlite

import (
	"database/sql"
	"errors"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) ClaimNextMessage(claim domain.ClaimNextRequest) (domain.ClaimResult, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.ClaimResult{}, false, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
SELECT body FROM messages
ORDER BY CASE state WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END, message_ref`,
		domain.MessageStateTakeoverCandidate,
		domain.MessageStateAvailable,
	)
	if err != nil {
		return domain.ClaimResult{}, false, err
	}
	messages, err := decodeRows[domain.Message](rows)
	if err != nil {
		return domain.ClaimResult{}, false, err
	}
	message, ok := nextEligibleMessage(messages, claim)
	if !ok {
		if err := tx.Commit(); err != nil {
			return domain.ClaimResult{}, false, err
		}
		return domain.ClaimResult{}, false, nil
	}
	request, err := claimRequestFromNextTx(tx, message, claim)
	if err != nil {
		return domain.ClaimResult{}, false, err
	}
	result, err := s.claimMessageTx(tx, message, request)
	if err != nil {
		if errors.Is(err, domain.ErrMessageDeadLettered) {
			if err := tx.Commit(); err != nil {
				return domain.ClaimResult{}, false, err
			}
			return domain.ClaimResult{}, false, nil
		}
		return domain.ClaimResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ClaimResult{}, false, err
	}
	return result, true, nil
}

func nextEligibleMessage(messages []domain.Message, claim domain.ClaimNextRequest) (domain.Message, bool) {
	states := messageStates(messages)
	for _, message := range messages {
		if !claimNextStateEligible(message.State) {
			continue
		}
		if domain.MessageClaimableByEndpoint(message, claim.Endpoint, domain.PreferredEndpointOnlineForMessage(message, claim.PreferredEndpoints)) &&
			domain.MessageDependenciesCompleted(message, states) {
			return message, true
		}
	}
	return domain.Message{}, false
}

func claimNextStateEligible(state string) bool {
	return state == domain.MessageStateTakeoverCandidate || state == domain.MessageStateAvailable
}

func messageStates(messages []domain.Message) map[string]string {
	states := make(map[string]string, len(messages))
	for _, message := range messages {
		states[message.MessageRef] = message.State
	}
	return states
}

func claimRequestFromNextTx(tx *sql.Tx, message domain.Message, claim domain.ClaimNextRequest) (domain.ClaimRequest, error) {
	attemptRef := claim.AttemptRef
	if attemptRef == "" {
		var err error
		attemptRef, err = nextRefTx(tx, "attempt")
		if err != nil {
			return domain.ClaimRequest{}, err
		}
	}
	deadLetterRef := claim.DeadLetterRef
	if deadLetterRef == "" {
		var err error
		deadLetterRef, err = nextRefTx(tx, "dead-letter")
		if err != nil {
			return domain.ClaimRequest{}, err
		}
	}
	return domain.ClaimRequest{
		MessageRef:     message.MessageRef,
		Endpoint:       claim.Endpoint,
		AttemptRef:     attemptRef,
		DeadLetterRef:  deadLetterRef,
		LeaseExpiresAt: claim.LeaseExpiresAt,
		Now:            claim.Now,
		MaxRedelivery:  claim.MaxRedelivery,
	}, nil
}

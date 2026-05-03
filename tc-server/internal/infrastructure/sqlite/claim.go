package sqlite

import (
	"database/sql"
	"errors"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) ClaimMessage(claim domain.ClaimRequest) (domain.ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.ClaimResult{}, err
	}
	defer tx.Rollback()
	message, ok, err := getMessageTx(tx, claim.MessageRef)
	if err != nil {
		return domain.ClaimResult{}, err
	}
	if !ok {
		return domain.ClaimResult{}, domain.ErrMessageNotFound
	}
	result, err := s.claimMessageTx(tx, message, claim)
	if err != nil {
		if errors.Is(err, domain.ErrMessageDeadLettered) {
			if commitErr := tx.Commit(); commitErr != nil {
				return domain.ClaimResult{}, commitErr
			}
		}
		return domain.ClaimResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ClaimResult{}, err
	}
	return result, nil
}

func (s *Store) claimMessageTx(tx *sql.Tx, message domain.Message, claim domain.ClaimRequest) (domain.ClaimResult, error) {
	switch message.State {
	case domain.MessageStateAvailable:
		return s.claimAvailableMessageTx(tx, message, claim)
	case domain.MessageStateDeadLettered:
		return domain.ClaimResult{}, domain.ErrMessageDeadLettered
	case domain.MessageStateTakeoverCandidate:
		return s.claimTakeoverCandidateTx(tx, message, claim)
	case domain.MessageStateClaimed, domain.MessageStateProcessing:
		return s.claimExpiredMessageTx(tx, message, claim)
	default:
		return domain.ClaimResult{}, domain.ErrMessageUnavailable
	}
}

func (s *Store) claimTakeoverCandidateTx(tx *sql.Tx, message domain.Message, claim domain.ClaimRequest) (domain.ClaimResult, error) {
	current, ok, err := getAttemptTx(tx, message.AttemptRef)
	if err != nil {
		return domain.ClaimResult{}, err
	}
	if !ok {
		return domain.ClaimResult{}, domain.ErrMessageUnavailable
	}
	if message.RedeliveryCount >= claim.MaxRedelivery {
		if err := s.deadLetterMessageTx(tx, message, current, claim); err != nil {
			return domain.ClaimResult{}, err
		}
		return domain.ClaimResult{}, domain.ErrMessageDeadLettered
	}
	return s.takeoverMessageTx(tx, message, claim)
}

func (s *Store) claimExpiredMessageTx(tx *sql.Tx, message domain.Message, claim domain.ClaimRequest) (domain.ClaimResult, error) {
	current, ok, err := getAttemptTx(tx, message.AttemptRef)
	if err != nil {
		return domain.ClaimResult{}, err
	}
	if !ok || claim.Now.Before(current.LeaseExpiresAt) || claim.Now.Equal(current.LeaseExpiresAt) {
		return domain.ClaimResult{}, domain.ErrMessageUnavailable
	}
	if message.RedeliveryCount >= claim.MaxRedelivery {
		if err := s.deadLetterMessageTx(tx, message, current, claim); err != nil {
			return domain.ClaimResult{}, err
		}
		return domain.ClaimResult{}, domain.ErrMessageDeadLettered
	}
	current.State = domain.AttemptStateOrphaned
	if err := s.updateAttemptTx(tx, current); err != nil {
		return domain.ClaimResult{}, err
	}
	return s.takeoverMessageTx(tx, message, claim)
}

func (s *Store) claimAvailableMessageTx(tx *sql.Tx, message domain.Message, claim domain.ClaimRequest) (domain.ClaimResult, error) {
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
	if err := s.saveAttempt(tx, attempt); err != nil {
		return domain.ClaimResult{}, err
	}
	if err := s.updateMessage(tx, message); err != nil {
		return domain.ClaimResult{}, err
	}
	return domain.ClaimResult{Message: message, Attempt: attempt}, nil
}

func (s *Store) takeoverMessageTx(tx *sql.Tx, message domain.Message, claim domain.ClaimRequest) (domain.ClaimResult, error) {
	previousAttemptRef := message.AttemptRef
	latest, err := latestCheckpointTx(tx, previousAttemptRef)
	if err != nil {
		return domain.ClaimResult{}, err
	}
	message.RedeliveryCount++
	result, err := s.claimAvailableMessageTx(tx, message, claim)
	if err != nil {
		return domain.ClaimResult{}, err
	}
	result.Takeover = true
	if latest != nil {
		result.LastCheckpointRef = latest.CheckpointRef
		result.ResumeSummary = latest.Summary
		result.ResumeArtifactRefs = append([]string(nil), latest.ArtifactRefs...)
	}
	return result, nil
}

func (s *Store) deadLetterMessageTx(tx *sql.Tx, message domain.Message, attempt domain.Attempt, claim domain.ClaimRequest) error {
	latest, err := latestCheckpointTx(tx, attempt.AttemptRef)
	if err != nil {
		return err
	}
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
	if err := s.updateAttemptTx(tx, attempt); err != nil {
		return err
	}
	if err := s.updateMessage(tx, message); err != nil {
		return err
	}
	return saveDeadLetterTx(tx, deadLetter)
}

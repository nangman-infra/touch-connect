package sqlite

import (
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) ReconcileExpiredClaims(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT body FROM messages WHERE state IN (?, ?)`, domain.MessageStateClaimed, domain.MessageStateProcessing)
	if err != nil {
		return 0
	}
	messages, err := decodeRows[domain.Message](rows)
	if err != nil {
		return 0
	}
	reconciled := 0
	for _, message := range messages {
		attempt, ok, err := getAttemptTx(tx, message.AttemptRef)
		if err != nil || !ok || now.Before(attempt.LeaseExpiresAt) || now.Equal(attempt.LeaseExpiresAt) {
			continue
		}
		attempt.State = domain.AttemptStateOrphaned
		message.State = domain.MessageStateTakeoverCandidate
		if err := s.updateAttemptTx(tx, attempt); err != nil {
			return reconciled
		}
		if err := s.updateMessage(tx, message); err != nil {
			return reconciled
		}
		reconciled++
	}
	if err := tx.Commit(); err != nil {
		return 0
	}
	return reconciled
}

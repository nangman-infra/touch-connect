package sqlite

import (
	"database/sql"
	"errors"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveQualityDecision(decision contracts.QualityDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if decision.QualityDecisionRef == "" || decision.MessageRef == "" {
		return domain.ErrInvalidInput
	}
	body, err := encode(decision)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO quality_decisions(quality_decision_ref, message_ref, created_at, body) VALUES(?, ?, ?, ?)`,
		decision.QualityDecisionRef,
		decision.MessageRef,
		decision.CreatedAt,
		body,
	)
	if err != nil {
		return domain.ErrInvalidInput
	}
	return nil
}

func (s *Store) QualityDecisions(messageRef string) []contracts.QualityDecision {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT body FROM quality_decisions WHERE message_ref = ? ORDER BY created_at, quality_decision_ref`, messageRef)
	if err != nil {
		return nil
	}
	decisions, err := decodeRows[contracts.QualityDecision](rows)
	if err != nil {
		return nil
	}
	return decisions
}

func allQualityDecisions(s *Store) []contracts.QualityDecision {
	rows, err := s.db.Query(`SELECT body FROM quality_decisions ORDER BY message_ref, created_at, quality_decision_ref`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return nil
	}
	decisions, err := decodeRows[contracts.QualityDecision](rows)
	if err != nil {
		return nil
	}
	return decisions
}

package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveApprovalDecision(decision domain.ApprovalDecision) error {
	body, err := encode(decision)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO approval_decisions(approval_ref, body) VALUES(?, ?)`, decision.ApprovalRef, body)
	if err != nil {
		return domain.ErrSideEffectConflict
	}
	return nil
}

func (s *Store) GetApprovalDecision(approvalRef string) (domain.ApprovalDecision, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM approval_decisions WHERE approval_ref = ?`, approvalRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.ApprovalDecision{}, false
	}
	if err != nil {
		return domain.ApprovalDecision{}, false
	}
	decision, err := decode[domain.ApprovalDecision](body)
	return decision, err == nil
}

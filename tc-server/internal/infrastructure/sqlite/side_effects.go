package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveSideEffectExecution(execution domain.SideEffectExecution) (domain.SideEffectExecution, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.SideEffectExecution{}, false, err
	}
	defer tx.Rollback()
	existing, ok, err := getSideEffectByBoundaryTx(tx, execution.IdempotencyKey, execution.ProtectedScope)
	if err != nil {
		return domain.SideEffectExecution{}, false, err
	}
	if ok {
		return existing, true, tx.Commit()
	}
	body, err := encode(execution)
	if err != nil {
		return domain.SideEffectExecution{}, false, err
	}
	if _, err := tx.Exec(`INSERT INTO side_effect_executions(side_effect_execution_ref, idempotency_key, protected_scope, body) VALUES(?, ?, ?, ?)`,
		execution.SideEffectExecutionRef, execution.IdempotencyKey, execution.ProtectedScope, body); err != nil {
		return domain.SideEffectExecution{}, false, domain.ErrSideEffectConflict
	}
	return execution, false, tx.Commit()
}

func (s *Store) GetSideEffectExecution(executionRef string) (domain.SideEffectExecution, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM side_effect_executions WHERE side_effect_execution_ref = ?`, executionRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.SideEffectExecution{}, false
	}
	if err != nil {
		return domain.SideEffectExecution{}, false
	}
	execution, err := decode[domain.SideEffectExecution](body)
	return execution, err == nil
}

func (s *Store) UpdateSideEffectExecution(execution domain.SideEffectExecution) error {
	body, err := encode(execution)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE side_effect_executions SET body = ? WHERE side_effect_execution_ref = ?`, body, execution.SideEffectExecutionRef)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrSideEffectNotFound
	}
	return nil
}

func getSideEffectByBoundaryTx(tx *sql.Tx, idempotencyKey string, protectedScope string) (domain.SideEffectExecution, bool, error) {
	var body string
	err := tx.QueryRow(`SELECT body FROM side_effect_executions WHERE idempotency_key = ? AND protected_scope = ?`, idempotencyKey, protectedScope).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.SideEffectExecution{}, false, nil
	}
	if err != nil {
		return domain.SideEffectExecution{}, false, err
	}
	execution, err := decode[domain.SideEffectExecution](body)
	return execution, err == nil, err
}

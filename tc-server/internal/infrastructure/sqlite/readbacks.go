package sqlite

import "github.com/nangman-infra/touch-connect/tc-server/internal/domain"

func (s *Store) SaveReadback(readback domain.Readback) (domain.Readback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Readback{}, err
	}
	defer tx.Rollback()
	var revision int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM readbacks WHERE attempt_ref = ?`, readback.AttemptRef).Scan(&revision); err != nil {
		return domain.Readback{}, err
	}
	readback.Revision = revision + 1
	body, err := encode(readback)
	if err != nil {
		return domain.Readback{}, err
	}
	if _, err := tx.Exec(`INSERT INTO readbacks(attempt_ref, revision, readback_ref, body) VALUES(?, ?, ?, ?)`,
		readback.AttemptRef, readback.Revision, readback.ReadbackRef, body); err != nil {
		return domain.Readback{}, err
	}
	return readback, tx.Commit()
}

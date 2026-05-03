package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveCheckpoint(checkpoint domain.Checkpoint) (domain.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Checkpoint{}, err
	}
	defer tx.Rollback()
	var revision int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM checkpoints WHERE attempt_ref = ?`, checkpoint.AttemptRef).Scan(&revision); err != nil {
		return domain.Checkpoint{}, err
	}
	checkpoint.Revision = revision + 1
	body, err := encode(checkpoint)
	if err != nil {
		return domain.Checkpoint{}, err
	}
	if _, err := tx.Exec(`INSERT INTO checkpoints(attempt_ref, revision, checkpoint_ref, body) VALUES(?, ?, ?, ?)`,
		checkpoint.AttemptRef, checkpoint.Revision, checkpoint.CheckpointRef, body); err != nil {
		return domain.Checkpoint{}, err
	}
	return checkpoint, tx.Commit()
}

func latestCheckpointTx(tx *sql.Tx, attemptRef string) (*domain.Checkpoint, error) {
	var body string
	err := tx.QueryRow(`SELECT body FROM checkpoints WHERE attempt_ref = ? ORDER BY revision DESC LIMIT 1`, attemptRef).Scan(&body)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	checkpoint, err := decode[domain.Checkpoint](body)
	if err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveAttempt(attempt domain.Attempt) error {
	return s.saveAttempt(nil, attempt)
}

func (s *Store) GetAttempt(attemptRef string) (domain.Attempt, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM attempts WHERE attempt_ref = ?`, attemptRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.Attempt{}, false
	}
	if err != nil {
		return domain.Attempt{}, false
	}
	attempt, err := decode[domain.Attempt](body)
	return attempt, err == nil
}

func (s *Store) UpdateAttempt(attempt domain.Attempt) error {
	body, err := encode(attempt)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE attempts SET message_ref = ?, endpoint_ref = ?, state = ?, lease_expires_at = ?, body = ? WHERE attempt_ref = ?`,
		attempt.MessageRef, attempt.EndpointRef, attempt.State, attempt.LeaseExpiresAt.Format(timeFormat), body, attempt.AttemptRef)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrAttemptNotFound
	}
	return nil
}

func (s *Store) saveAttempt(tx *sql.Tx, attempt domain.Attempt) error {
	body, err := encode(attempt)
	if err != nil {
		return err
	}
	query := `INSERT INTO attempts(attempt_ref, message_ref, endpoint_ref, state, lease_expires_at, body) VALUES(?, ?, ?, ?, ?, ?)`
	args := []any{attempt.AttemptRef, attempt.MessageRef, attempt.EndpointRef, attempt.State, attempt.LeaseExpiresAt.Format(timeFormat), body}
	if tx != nil {
		_, err = tx.Exec(query, args...)
	} else {
		_, err = s.db.Exec(query, args...)
	}
	return err
}

func (s *Store) updateAttemptTx(tx *sql.Tx, attempt domain.Attempt) error {
	body, err := encode(attempt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE attempts SET message_ref = ?, endpoint_ref = ?, state = ?, lease_expires_at = ?, body = ? WHERE attempt_ref = ?`,
		attempt.MessageRef, attempt.EndpointRef, attempt.State, attempt.LeaseExpiresAt.Format(timeFormat), body, attempt.AttemptRef)
	return err
}

func getAttemptTx(tx *sql.Tx, attemptRef string) (domain.Attempt, bool, error) {
	var body string
	err := tx.QueryRow(`SELECT body FROM attempts WHERE attempt_ref = ?`, attemptRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.Attempt{}, false, nil
	}
	if err != nil {
		return domain.Attempt{}, false, err
	}
	attempt, err := decode[domain.Attempt](body)
	return attempt, err == nil, err
}

const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

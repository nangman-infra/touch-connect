package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveMessage(message domain.Message) error {
	body, err := encode(message)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO messages(message_ref, state, attempt_ref, body) VALUES(?, ?, ?, ?)`,
		message.MessageRef, message.State, message.AttemptRef, body)
	if err != nil {
		return domain.ErrInvalidInput
	}
	return nil
}

func (s *Store) GetMessage(messageRef string) (domain.Message, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM messages WHERE message_ref = ?`, messageRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.Message{}, false
	}
	if err != nil {
		return domain.Message{}, false
	}
	message, err := decode[domain.Message](body)
	return message, err == nil
}

func (s *Store) UpdateMessage(message domain.Message) error {
	return s.updateMessage(nil, message)
}

func (s *Store) updateMessage(tx *sql.Tx, message domain.Message) error {
	body, err := encode(message)
	if err != nil {
		return err
	}
	query := `UPDATE messages SET state = ?, attempt_ref = ?, body = ? WHERE message_ref = ?`
	var result sql.Result
	if tx != nil {
		result, err = tx.Exec(query, message.State, message.AttemptRef, body, message.MessageRef)
	} else {
		result, err = s.db.Exec(query, message.State, message.AttemptRef, body, message.MessageRef)
	}
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrMessageNotFound
	}
	return nil
}

func getMessageTx(tx *sql.Tx, messageRef string) (domain.Message, bool, error) {
	var body string
	err := tx.QueryRow(`SELECT body FROM messages WHERE message_ref = ?`, messageRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.Message{}, false, nil
	}
	if err != nil {
		return domain.Message{}, false, err
	}
	message, err := decode[domain.Message](body)
	return message, err == nil, err
}

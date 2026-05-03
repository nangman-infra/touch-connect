package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func saveDeadLetterTx(tx *sql.Tx, deadLetter domain.DeadLetter) error {
	body, err := encode(deadLetter)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO dead_letters(dead_letter_ref, body) VALUES(?, ?)`, deadLetter.DeadLetterRef, body)
	return err
}

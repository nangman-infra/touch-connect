package sqlite

import "fmt"

func (s *Store) NextRef(kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fallbackRef(kind)
	}
	defer tx.Rollback()
	var sequence int
	err = tx.QueryRow(`SELECT sequence FROM refs WHERE kind = ?`, kind).Scan(&sequence)
	if err != nil {
		sequence = 0
	}
	sequence++
	if _, err := tx.Exec(`INSERT INTO refs(kind, sequence) VALUES(?, ?)
ON CONFLICT(kind) DO UPDATE SET sequence = excluded.sequence`, kind, sequence); err != nil {
		return fallbackRef(kind)
	}
	if err := tx.Commit(); err != nil {
		return fallbackRef(kind)
	}
	return fmt.Sprintf("tc://%s/%s_%06d", kind, kindPrefix(kind), sequence)
}

func fallbackRef(kind string) string {
	return fmt.Sprintf("tc://%s/%s_000000", kind, kindPrefix(kind))
}

func kindPrefix(kind string) string {
	switch kind {
	case "message":
		return "msg"
	case "delivery":
		return "dlv"
	case "attempt":
		return "att"
	case "checkpoint":
		return "ckp"
	case "readback":
		return "rdb"
	case "dead-letter":
		return "dlq"
	case "side-effect":
		return "sfx"
	case "accepted":
		return "acc"
	default:
		return "ref"
	}
}

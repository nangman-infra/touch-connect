package sqlite

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS refs (
  kind TEXT PRIMARY KEY,
  sequence INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS endpoints (
  endpoint_ref TEXT PRIMARY KEY,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
  message_ref TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  attempt_ref TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS attempts (
  attempt_ref TEXT PRIMARY KEY,
  message_ref TEXT NOT NULL,
  endpoint_ref TEXT NOT NULL,
  state TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS checkpoints (
  attempt_ref TEXT NOT NULL,
  revision INTEGER NOT NULL,
  checkpoint_ref TEXT NOT NULL UNIQUE,
  body TEXT NOT NULL,
  PRIMARY KEY (attempt_ref, revision)
);
CREATE TABLE IF NOT EXISTS readbacks (
  attempt_ref TEXT NOT NULL,
  revision INTEGER NOT NULL,
  readback_ref TEXT NOT NULL UNIQUE,
  body TEXT NOT NULL,
  PRIMARY KEY (attempt_ref, revision)
);
CREATE TABLE IF NOT EXISTS artifact_versions (
  artifact_version_ref TEXT PRIMARY KEY,
  message_ref TEXT NOT NULL,
  attempt_ref TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS dead_letters (
  dead_letter_ref TEXT PRIMARY KEY,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS approval_decisions (
  approval_ref TEXT PRIMARY KEY,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS side_effect_executions (
  side_effect_execution_ref TEXT PRIMARY KEY,
  idempotency_key TEXT NOT NULL,
  protected_scope TEXT NOT NULL,
  body TEXT NOT NULL,
  UNIQUE (idempotency_key, protected_scope)
);
`

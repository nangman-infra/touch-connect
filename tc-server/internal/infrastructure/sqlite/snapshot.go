package sqlite

import "github.com/nangman-infra/touch-connect/tc-server/internal/domain"

func (s *Store) Snapshot() domain.Snapshot {
	return domain.Snapshot{
		Endpoints:     all[domain.Endpoint](s, `SELECT body FROM endpoints ORDER BY endpoint_ref`),
		Messages:      all[domain.Message](s, `SELECT body FROM messages ORDER BY message_ref`),
		Attempts:      all[domain.Attempt](s, `SELECT body FROM attempts ORDER BY attempt_ref`),
		Checkpoints:   all[domain.Checkpoint](s, `SELECT body FROM checkpoints ORDER BY attempt_ref, revision`),
		Readbacks:     all[domain.Readback](s, `SELECT body FROM readbacks ORDER BY attempt_ref, revision`),
		Artifacts:     all[domain.ArtifactVersion](s, `SELECT body FROM artifact_versions ORDER BY artifact_version_ref`),
		Finalizations: all[domain.ArtifactFinalization](s, `SELECT body FROM artifact_finalizations ORDER BY artifact_version_ref`),
		DeadLetters:   all[domain.DeadLetter](s, `SELECT body FROM dead_letters ORDER BY dead_letter_ref`),
		Approvals:     all[domain.ApprovalDecision](s, `SELECT body FROM approval_decisions ORDER BY approval_ref`),
		SideEffects:   all[domain.SideEffectExecution](s, `SELECT body FROM side_effect_executions ORDER BY side_effect_execution_ref`),
	}
}

func all[T any](s *Store, query string) []T {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil
	}
	values, err := decodeRows[T](rows)
	if err != nil {
		return nil
	}
	return values
}

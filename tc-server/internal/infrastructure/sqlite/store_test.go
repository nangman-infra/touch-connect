package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func TestStoreAttemptAndQualityDecisionLifecycle(t *testing.T) {
	store := newTestStore(t)
	defer closeStore(t, store)

	attempt := domain.Attempt{
		AttemptRef:     "tc://attempt/att_000001",
		MessageRef:     "tc://message/msg_000001",
		EndpointRef:    "tc://endpoint/worker",
		State:          domain.AttemptStateClaimed,
		LeaseExpiresAt: time.Date(2026, 5, 6, 1, 0, 0, 0, time.UTC),
		Revision:       1,
		AttemptNo:      1,
		ClaimEpoch:     1,
	}
	if err := store.SaveAttempt(attempt); err != nil {
		t.Fatalf("SaveAttempt returned error: %v", err)
	}
	gotAttempt, ok := store.GetAttempt(attempt.AttemptRef)
	if !ok || gotAttempt.AttemptRef != attempt.AttemptRef {
		t.Fatalf("GetAttempt returned %+v ok=%v", gotAttempt, ok)
	}
	gotAttempt.State = domain.AttemptStateCompleted
	if err := store.UpdateAttempt(gotAttempt); err != nil {
		t.Fatalf("UpdateAttempt returned error: %v", err)
	}

	decision := contracts.QualityDecision{
		QualityDecisionRef: "tc://quality-decision/qdc_000001",
		MessageRef:         attempt.MessageRef,
		Decision:           contracts.QualityDecisionPassed,
		PolicyRef:          "tc://quality-policy/default",
		PolicyVersion:      "v0",
		CreatedAt:          "2026-05-06T01:00:00Z",
	}
	if err := store.SaveQualityDecision(decision); err != nil {
		t.Fatalf("SaveQualityDecision returned error: %v", err)
	}
	decisions := store.QualityDecisions(attempt.MessageRef)
	if len(decisions) != 1 || decisions[0].QualityDecisionRef != decision.QualityDecisionRef {
		t.Fatalf("QualityDecisions returned %+v", decisions)
	}
	snapshot := store.Snapshot()
	if len(snapshot.Attempts) != 1 || len(snapshot.QualityDecisions) != 1 {
		t.Fatalf("Snapshot missing attempt or quality decision: %+v", snapshot)
	}
}

func TestStoreRejectsInvalidQualityDecisionAndMissingAttemptUpdate(t *testing.T) {
	store := newTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveQualityDecision(contracts.QualityDecision{MessageRef: "tc://message/msg_000001"}); err != domain.ErrInvalidInput {
		t.Fatalf("missing quality_decision_ref should return invalid input, got %v", err)
	}
	if err := store.UpdateAttempt(domain.Attempt{AttemptRef: "tc://attempt/missing"}); err != domain.ErrAttemptNotFound {
		t.Fatalf("missing attempt update should return attempt_not_found, got %v", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "touch-connect.db"))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	return store
}

func closeStore(t *testing.T, store *Store) {
	t.Helper()
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

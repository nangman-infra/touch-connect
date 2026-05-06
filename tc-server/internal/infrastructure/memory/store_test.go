package memory

import (
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func TestStoreAttemptAndQualityDecisionLifecycle(t *testing.T) {
	store := NewStore()
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
	decisions[0].Decision = contracts.QualityDecisionRejected
	if store.QualityDecisions(attempt.MessageRef)[0].Decision != contracts.QualityDecisionPassed {
		t.Fatal("QualityDecisions should return a defensive copy")
	}

	snapshot := store.Snapshot()
	if len(snapshot.Attempts) != 1 || len(snapshot.QualityDecisions) != 1 {
		t.Fatalf("Snapshot missing attempt or quality decision: %+v", snapshot)
	}
}

func TestStoreRejectsDuplicateOrIncompleteQualityDecision(t *testing.T) {
	store := NewStore()
	if err := store.SaveQualityDecision(contracts.QualityDecision{MessageRef: "tc://message/msg_000001"}); err != domain.ErrInvalidInput {
		t.Fatalf("missing quality_decision_ref should return invalid input, got %v", err)
	}
	decision := contracts.QualityDecision{
		QualityDecisionRef: "tc://quality-decision/qdc_000001",
		MessageRef:         "tc://message/msg_000001",
	}
	if err := store.SaveQualityDecision(decision); err != nil {
		t.Fatalf("SaveQualityDecision returned error: %v", err)
	}
	if err := store.SaveQualityDecision(decision); err != domain.ErrInvalidInput {
		t.Fatalf("duplicate quality decision should return invalid input, got %v", err)
	}
}

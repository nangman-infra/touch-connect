package application

import (
	"context"
	"errors"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type fakeServerClient struct {
	snapshot contracts.SnapshotResponse
	err      error
}

func (f fakeServerClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{Status: "ok", Component: "tc-server", Version: "server-version"}, f.err
}

func (f fakeServerClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{Version: "server-version", MinimumWorker: "0.1.0", ContractVersion: "contract"}, f.err
}

func (f fakeServerClient) Snapshot(context.Context) (contracts.SnapshotResponse, error) {
	return f.snapshot, f.err
}

func (f fakeServerClient) SendMessage(context.Context, contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	return contracts.MessageIngressResponse{MessageRef: "tc://message/new"}, f.err
}

func (f fakeServerClient) RecordApproval(context.Context, contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	return contracts.ApprovalDecisionResponse{ApprovalRef: "tc://approval/a"}, f.err
}

func (f fakeServerClient) CancelTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{TaskRef: "tc://task/t", State: "canceled"}, f.err
}

func (f fakeServerClient) RetryTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{TaskRef: "tc://task/t", State: "available"}, f.err
}

func (f fakeServerClient) ReplayDeadLetter(context.Context, contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	return contracts.DLQReplayResponse{DeadLetterRef: "tc://dead-letter/d", MessageRef: "tc://message/replayed"}, f.err
}

func (f fakeServerClient) FinalizeArtifact(context.Context, contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	return contracts.ArtifactFinalizeResponse{ArtifactVersionRef: "tc://artifact-version/a2", FinalizationRef: "tc://finalization/f"}, f.err
}

func TestServiceDelegatesHealthVersionAndCommands(t *testing.T) {
	ctx := context.Background()
	service, err := NewService(fakeServerClient{snapshot: controlSnapshot()}, "control-version")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if got := service.Health(); got.Component != "tc-control" || got.Version != "control-version" {
		t.Fatalf("unexpected health: %+v", got)
	}
	if got, err := service.Ready(ctx); err != nil || got.Status != "ready" || got.Version != "server-version" {
		t.Fatalf("unexpected ready response: %+v err=%v", got, err)
	}
	if got, err := service.Version(ctx); err != nil || got.Version != "control-version" || got.MinimumWorker != "0.1.0" {
		t.Fatalf("unexpected version response: %+v err=%v", got, err)
	}
	if got, err := service.SendMessage(ctx, contracts.MessageIngressRequest{}); err != nil || got.MessageRef == "" {
		t.Fatalf("send message did not delegate: %+v err=%v", got, err)
	}
	if got, err := service.RecordApproval(ctx, contracts.ApprovalCommandRequest{}); err != nil || got.ApprovalRef == "" {
		t.Fatalf("record approval did not delegate: %+v err=%v", got, err)
	}
	if got, err := service.CancelTask(ctx, contracts.TaskCommandRequest{}); err != nil || got.State != "canceled" {
		t.Fatalf("cancel task did not delegate: %+v err=%v", got, err)
	}
	if got, err := service.RetryTask(ctx, contracts.TaskCommandRequest{}); err != nil || got.State != "available" {
		t.Fatalf("retry task did not delegate: %+v err=%v", got, err)
	}
	if got, err := service.ReplayDeadLetter(ctx, contracts.DLQReplayRequest{}); err != nil || got.MessageRef == "" {
		t.Fatalf("replay did not delegate: %+v err=%v", got, err)
	}
	if got, err := service.FinalizeArtifact(ctx, contracts.ArtifactFinalizeRequest{}); err != nil || got.FinalizationRef == "" {
		t.Fatalf("finalize did not delegate: %+v err=%v", got, err)
	}
}

func TestServiceQueriesSnapshot(t *testing.T) {
	ctx := context.Background()
	service, err := NewService(fakeServerClient{snapshot: controlSnapshot()}, "")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if got, err := service.Capabilities(ctx); err != nil || len(got["code.change"]) != 1 {
		t.Fatalf("unexpected capabilities: %+v err=%v", got, err)
	}
	if got, ok, err := service.Endpoint(ctx, "tc://endpoint/worker"); err != nil || !ok || got.DisplayName != "Worker" {
		t.Fatalf("unexpected endpoint lookup: %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, err := service.Endpoint(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing endpoint should not be found: ok=%v err=%v", ok, err)
	}
	if got, err := service.Messages(ctx, "tc://task/t"); err != nil || len(got) != 1 {
		t.Fatalf("unexpected task messages: %+v err=%v", got, err)
	}
	if got, ok, err := service.Message(ctx, "tc://message/m"); err != nil || !ok || got.LatestQualityDecision == nil {
		t.Fatalf("message should include quality decisions: %+v ok=%v err=%v", got, ok, err)
	}
	if got, err := service.Attempts(ctx, "tc://task/t"); err != nil || len(got) != 1 {
		t.Fatalf("unexpected task attempts: %+v err=%v", got, err)
	}
	if got, ok, err := service.TaskStatus(ctx, "tc://task/t"); err != nil || !ok || got["message_count"].(int) != 1 {
		t.Fatalf("unexpected task status: %+v ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := service.TaskHistory(ctx, "tc://task/t"); err != nil || !ok || len(got["artifacts"].([]contracts.ArtifactRecord)) != 2 {
		t.Fatalf("unexpected task history: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestServiceQueriesGovernanceRecords(t *testing.T) {
	ctx := context.Background()
	service, err := NewService(fakeServerClient{snapshot: controlSnapshot()}, "")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if got, ok, err := service.Artifact(ctx, "tc://artifact-version/a2"); err != nil || !ok || got.ArtifactRef == "" {
		t.Fatalf("unexpected artifact lookup: %+v ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := service.ArtifactLineage(ctx, "tc://artifact-version/a2"); err != nil || !ok || len(got.Versions) != 2 {
		t.Fatalf("unexpected lineage: %+v ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := service.Approval(ctx, "tc://approval/a"); err != nil || !ok || got.Status != "approved" {
		t.Fatalf("unexpected approval lookup: %+v ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := service.ApprovalChain(ctx, "tc://approval/a"); err != nil || !ok || got.Current == nil {
		t.Fatalf("unexpected approval chain: %+v ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := service.DeadLetter(ctx, "tc://dead-letter/d"); err != nil || !ok || got.MessageRef == "" {
		t.Fatalf("unexpected dead letter: %+v ok=%v err=%v", got, ok, err)
	}
	if got, err := service.SideEffects(ctx, "tc://task/t"); err != nil || len(got) != 1 {
		t.Fatalf("unexpected side effects: %+v err=%v", got, err)
	}
}

func TestServiceReturnsServerErrors(t *testing.T) {
	want := errors.New("server unavailable")
	service, err := NewService(fakeServerClient{err: want}, "")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	if _, err := service.Ready(context.Background()); !errors.Is(err, want) {
		t.Fatalf("expected server error, got %v", err)
	}
	if _, ok, err := service.TaskStatus(context.Background(), "tc://task/t"); !errors.Is(err, want) || ok {
		t.Fatalf("expected task status error, ok=%v err=%v", ok, err)
	}
}

func TestNewServiceRejectsNilClient(t *testing.T) {
	if _, err := NewService(nil, ""); err == nil {
		t.Fatal("expected nil server client to fail")
	}
}

func controlSnapshot() contracts.SnapshotResponse {
	return contracts.SnapshotResponse{
		Endpoints: []contracts.EndpointRecord{{
			EndpointRef:  "tc://endpoint/worker",
			DisplayName:  "Worker",
			Capabilities: map[string]contracts.Capability{"code.change": {Name: "code.change"}},
		}},
		Messages: []contracts.MessageRecord{{
			MessageRef:     "tc://message/m",
			CorrelationRef: "tc://task/t",
			State:          "completed",
		}},
		Attempts: []contracts.AttemptRecord{{
			AttemptRef: "tc://attempt/a",
			MessageRef: "tc://message/m",
			State:      "completed",
		}},
		Artifacts: []contracts.ArtifactRecord{
			{
				ArtifactRef:        "tc://artifact/a",
				ArtifactVersionRef: "tc://artifact-version/a1",
				TaskRef:            "tc://task/t",
				MessageRef:         "tc://message/m",
				CreatedAt:          "2026-05-06T00:00:00Z",
			},
			{
				ArtifactRef:                "tc://artifact/a",
				ArtifactVersionRef:         "tc://artifact-version/a2",
				TaskRef:                    "tc://task/t",
				MessageRef:                 "tc://message/m",
				BasedOnArtifactVersionRefs: []string{"tc://artifact-version/a1"},
				CreatedAt:                  "2026-05-06T00:01:00Z",
			},
		},
		Approvals: []contracts.ApprovalRecord{{
			ApprovalRef: "tc://approval/a",
			MessageRef:  "tc://message/m",
			TargetRef:   "tc://side-effect/s",
			Status:      "approved",
			RequestedAt: "2026-05-06T00:00:00Z",
			DecidedAt:   "2026-05-06T00:01:00Z",
		}},
		DeadLetters: []contracts.DeadLetterRecord{{
			DeadLetterRef: "tc://dead-letter/d",
			MessageRef:    "tc://message/m",
			Reason:        "failed",
		}},
		SideEffects: []contracts.SideEffectRecord{{
			SideEffectExecutionRef: "tc://side-effect/s",
			TaskRef:                "tc://task/t",
			MessageRef:             "tc://message/m",
			Status:                 "succeeded",
		}},
		QualityDecisions: []contracts.QualityDecision{{
			QualityDecisionRef: "tc://quality-decision/q",
			MessageRef:         "tc://message/m",
			Decision:           contracts.QualityDecisionPassed,
		}},
	}
}

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestAttemptPathUsesSharedPrefix(t *testing.T) {
	if got := attemptPath("tc://attempt/att_123", "checkpoint"); got != "/v1/attempts/tc://attempt/att_123/checkpoint" {
		t.Fatalf("unexpected attempt path: %s", got)
	}
	if got := attemptPath("plain_attempt", "complete"); got != "/v1/attempts/plain_attempt/complete" {
		t.Fatalf("unexpected attempt path without suffix: %s", got)
	}
}

func TestAttemptScopedPostMethodsUseAttemptPath(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewHTTPClient(server.URL, server.Client())
	ctx := context.Background()

	if _, err := client.SubmitCheckpoint(ctx, "att", contracts.CheckpointRequest{}); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if _, err := client.SubmitReadback(ctx, "att", contracts.ReadbackRequest{}); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if _, err := client.RefreshLease(ctx, "att", contracts.RefreshLeaseRequest{}); err != nil {
		t.Fatalf("lease: %v", err)
	}
	if _, err := client.RegisterArtifactVersion(ctx, "att", contracts.ArtifactVersionRequest{}); err != nil {
		t.Fatalf("artifact: %v", err)
	}
	if _, err := client.RecordApprovalDecision(ctx, "att", contracts.ApprovalDecisionRequest{}); err != nil {
		t.Fatalf("approval: %v", err)
	}
	if _, err := client.StartSideEffectExecution(ctx, "att", contracts.SideEffectExecutionRequest{}); err != nil {
		t.Fatalf("side effect: %v", err)
	}
	if _, err := client.CompleteAttempt(ctx, "att", contracts.CompleteAttemptRequest{}); err != nil {
		t.Fatalf("complete attempt: %v", err)
	}
	expected := []string{
		"/v1/attempts/att/checkpoints",
		"/v1/attempts/att/readback",
		"/v1/attempts/att/lease",
		"/v1/attempts/att/artifacts",
		"/v1/attempts/att/approvals",
		"/v1/attempts/att/side-effects",
		"/v1/attempts/att/complete",
	}
	for index, path := range expected {
		if paths[index] != path {
			t.Fatalf("path %d got %s want %s", index, paths[index], path)
		}
	}
}

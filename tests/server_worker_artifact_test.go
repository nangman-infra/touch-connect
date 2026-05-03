package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestArtifactVersionCanBeRegisteredAndReferencedByCheckpoint(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)

	artifact := artifactRequest("tc://artifact-version/code_patch_v1")
	registered, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, artifact)
	if err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	if registered.MessageRef != message.MessageRef || registered.AttemptRef != claim.AttemptRef {
		t.Fatalf("expected artifact lineage to match message and attempt, got %+v", registered)
	}
	if err := worker.SubmitCheckpoint(context.Background(), claim.AttemptRef, "in_progress", "artifact produced", []string{artifact.ArtifactVersionRef}); err != nil {
		t.Fatalf("submit checkpoint with artifact: %v", err)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Artifacts) != 1 {
		t.Fatalf("expected one artifact version, got %+v", snapshot.Artifacts)
	}
	if got := snapshot.Checkpoints[len(snapshot.Checkpoints)-1].ArtifactRefs; len(got) != 1 || got[0] != artifact.ArtifactVersionRef {
		t.Fatalf("expected checkpoint artifact ref, got %+v", got)
	}
}

func TestCheckpointRejectsUnknownArtifactRef(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)

	req := contracts.CheckpointRequest{
		EndpointRef:  tcworker.DefaultConfig().EndpointRef,
		State:        "in_progress",
		Summary:      "unknown artifact should fail",
		ArtifactRefs: []string{"tc://artifact-version/missing"},
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/checkpoints", httpServer.Client(), req, http.StatusNotFound, &apiErr)
	if apiErr.Code != "artifact_not_found" {
		t.Fatalf("expected artifact_not_found, got %+v", apiErr)
	}
}

func TestCheckpointRejectsArtifactFromDifferentMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	firstMessage := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), firstMessage.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	artifact := artifactRequest("tc://artifact-version/foreign_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), firstClaim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}

	secondMessage := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	secondClaim := claimMessage(t, httpServer.URL, httpServer.Client(), secondMessage.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	req := contracts.CheckpointRequest{
		EndpointRef:  tcworker.DefaultConfig().EndpointRef,
		State:        "in_progress",
		Summary:      "foreign artifact should fail",
		ArtifactRefs: []string{artifact.ArtifactVersionRef},
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+secondClaim.AttemptRef+"/checkpoints", httpServer.Client(), req, http.StatusNotFound, &apiErr)
	if apiErr.Code != "artifact_not_found" {
		t.Fatalf("expected artifact_not_found, got %+v", apiErr)
	}
}

func TestArtifactRegistrationRequiresCurrentLease(t *testing.T) {
	_, httpServer := newLeaseTestServer(t, 5*time.Millisecond, 3)
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	req := artifactRequest("tc://artifact-version/late_v1")
	req.EndpointRef = tcworker.DefaultConfig().EndpointRef
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/artifacts", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "lease_expired" {
		t.Fatalf("expected lease_expired, got %+v", apiErr)
	}
}

func TestArtifactVersionIsImmutable(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)

	req := artifactRequest("tc://artifact-version/immutable_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, req); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	req.EndpointRef = tcworker.DefaultConfig().EndpointRef
	req.Checksum = "sha256:different"
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/artifacts", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "artifact_version_exists" {
		t.Fatalf("expected artifact_version_exists, got %+v", apiErr)
	}
}

func TestTakeoverClaimReturnsResumeArtifactRefs(t *testing.T) {
	server, httpServer := newLeaseTestServer(t, 5*time.Millisecond, 3)
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_artifact_takeover", "actor.takeover")
	registerWorkers(t, httpServer, firstConfig, secondConfig)

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), firstConfig)
	artifact := artifactRequest("tc://artifact-version/resume_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), firstClaim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	checkpointAttemptWithArtifacts(t, httpServer, firstClaim.AttemptRef, firstConfig.EndpointRef, []string{artifact.ArtifactVersionRef})

	time.Sleep(10 * time.Millisecond)
	if reconciled := server.ReconcileExpiredClaims(); reconciled != 1 {
		t.Fatalf("expected one takeover candidate reconciliation, got %d", reconciled)
	}
	secondClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, secondConfig.EndpointRef, http.StatusAccepted)
	if len(secondClaim.ResumeArtifactRefs) != 1 || secondClaim.ResumeArtifactRefs[0] != artifact.ArtifactVersionRef {
		t.Fatalf("expected takeover resume artifact refs, got %+v", secondClaim)
	}
}

func artifactRequest(versionRef string) contracts.ArtifactVersionRequest {
	return contracts.ArtifactVersionRequest{
		ArtifactRef:        "tc://artifact/code_patch",
		ArtifactVersionRef: versionRef,
		RoomRef:            "tc://room/dev",
		TaskRef:            "tc://task/task_001",
		TaskRevision:       1,
		Kind:               "code_patch",
		MediaType:          "text/plain",
		SizeBytes:          128,
		Checksum:           "sha256:abc123",
		StorageRef:         "memory://artifacts/code_patch",
		RetentionClass:     "operational",
		AccessScope:        "task",
	}
}

func checkpointAttemptWithArtifacts(t *testing.T, server *httptest.Server, attemptRef string, endpointRef string, artifactRefs []string) contracts.CheckpointResponse {
	t.Helper()
	req := contracts.CheckpointRequest{
		EndpointRef:  endpointRef,
		State:        "in_progress",
		Summary:      "checkpoint with artifact refs",
		ArtifactRefs: artifactRefs,
	}
	var res contracts.CheckpointResponse
	postJSON(t, server.URL+"/v1/attempts/"+attemptRef+"/checkpoints", server.Client(), req, http.StatusAccepted, &res)
	return res
}

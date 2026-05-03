package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestSQLiteStoreRecoversLedgersAfterRestart(t *testing.T) {
	dbPath := sqlitePath(t)
	server := newSQLiteServer(t, dbPath, tcserver.DefaultSettings())
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), true)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	checkpointAttempt(t, httpServer, claim.AttemptRef, tcworker.DefaultConfig().EndpointRef, "claimed", http.StatusAccepted)
	submitReadback(t, httpServer, claim.AttemptRef)
	artifact := artifactRequest("tc://artifact-version/sqlite_recovery_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	if err := worker.SubmitCheckpoint(context.Background(), claim.AttemptRef, "in_progress", "sqlite recovery checkpoint", []string{artifact.ArtifactVersionRef}); err != nil {
		t.Fatalf("submit checkpoint: %v", err)
	}
	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-sqlite")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)
	started, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-sqlite", "hash-sqlite"))
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}
	if _, err := worker.CompleteSideEffectExecution(context.Background(), started.SideEffectExecutionRef, contracts.CompleteSideEffectExecutionRequest{
		Status:    "succeeded",
		ResultRef: "tc://artifact-version/sqlite_result",
	}); err != nil {
		t.Fatalf("complete side effect: %v", err)
	}
	if _, err := worker.ProcessMessage(context.Background(), message.MessageRef); err == nil {
		t.Fatalf("expected already-claimed message to reject second processing")
	}

	restarted := newSQLiteServer(t, dbPath, tcserver.DefaultSettings())
	snapshot := restarted.Snapshot()
	if len(snapshot.Endpoints) != 1 || len(snapshot.Messages) != 1 || len(snapshot.Attempts) != 1 {
		t.Fatalf("expected endpoint/message/attempt recovery, got %+v", snapshot)
	}
	if len(snapshot.Readbacks) != 1 || len(snapshot.Artifacts) != 1 || len(snapshot.Approvals) != 1 || len(snapshot.SideEffects) != 1 {
		t.Fatalf("expected readback/artifact/approval/side-effect recovery, got %+v", snapshot)
	}
	if len(snapshot.Checkpoints) != 2 {
		t.Fatalf("expected claimed and in_progress checkpoints, got %+v", snapshot.Checkpoints)
	}
}

func TestSQLiteStorePersistsDLQAfterRestart(t *testing.T) {
	dbPath := sqlitePath(t)
	settings := leaseSettings(50 * time.Millisecond)
	settings.MaxRedelivery = 0
	server := newSQLiteServer(t, dbPath, settings)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_sqlite_dlq", "actor.sqlite.dlq")
	registerWorkers(t, httpServer, firstConfig, secondConfig)
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	checkpointAttempt(t, httpServer, claim.AttemptRef, firstConfig.EndpointRef, "in_progress", http.StatusAccepted)

	time.Sleep(60 * time.Millisecond)
	server.ReconcileExpiredClaims()
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/messages/"+message.MessageRef+"/claim", httpServer.Client(), contracts.ClaimMessageRequest{
		EndpointRef: secondConfig.EndpointRef,
	}, http.StatusConflict, &apiErr)
	if apiErr.Code != "message_dead_lettered" {
		t.Fatalf("expected message_dead_lettered, got %+v", apiErr)
	}

	restarted := newSQLiteServer(t, dbPath, settings)
	if snapshot := restarted.Snapshot(); len(snapshot.DeadLetters) != 1 || snapshot.Messages[0].State != "dead_lettered" {
		t.Fatalf("expected persisted DLQ state, got %+v", snapshot)
	}
}

func TestSQLiteStoreAllowsOnlyOneConcurrentClaim(t *testing.T) {
	dbPath := sqlitePath(t)
	server := newSQLiteServer(t, dbPath, tcserver.DefaultSettings())
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_sqlite_second", "actor.sqlite.second")
	registerWorkers(t, httpServer, firstConfig, secondConfig)
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)

	start := make(chan struct{})
	outcomes := make(chan claimOutcome, 2)
	go asyncClaim(httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, start, outcomes)
	go asyncClaim(httpServer.URL, httpServer.Client(), message.MessageRef, secondConfig.EndpointRef, start, outcomes)
	close(start)

	accepted := 0
	conflicted := 0
	for range 2 {
		outcome := <-outcomes
		if outcome.Err != nil {
			t.Fatalf("claim request failed: %v", outcome.Err)
		}
		if outcome.Status == http.StatusAccepted {
			accepted++
		}
		if outcome.Status == http.StatusConflict {
			conflicted++
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("expected one claim winner and one conflict, got accepted=%d conflicted=%d", accepted, conflicted)
	}
}

func TestSQLiteStorePersistsIdempotencyAndArtifactUniqueness(t *testing.T) {
	dbPath := sqlitePath(t)
	server := newSQLiteServer(t, dbPath, tcserver.DefaultSettings())
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	artifact := artifactRequest("tc://artifact-version/sqlite_unique_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact: %v", err)
	}
	recordApproval(t, httpServer, claim.AttemptRef, approvalRequest("tc://approval/apr_001", "approved", "hash-unique"), http.StatusAccepted)
	first, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-sqlite-unique", "hash-unique"))
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}

	restartedHTTP := httptest.NewServer(newSQLiteServer(t, dbPath, tcserver.DefaultSettings()).Handler())
	defer restartedHTTP.Close()
	restartedWorker := tcworker.NewHTTPRuntime(restartedHTTP.URL, restartedHTTP.Client(), tcworker.DefaultConfig())
	second, err := restartedWorker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-sqlite-unique", "hash-unique"))
	if err != nil {
		t.Fatalf("dedupe side effect after restart: %v", err)
	}
	if second.SideEffectExecutionRef != first.SideEffectExecutionRef || !second.Deduped {
		t.Fatalf("expected persisted idempotency dedupe, first=%+v second=%+v", first, second)
	}
	var apiErr contracts.ErrorResponse
	artifact.EndpointRef = tcworker.DefaultConfig().EndpointRef
	postJSON(t, restartedHTTP.URL+"/v1/attempts/"+claim.AttemptRef+"/artifacts", restartedHTTP.Client(), artifact, http.StatusConflict, &apiErr)
	if apiErr.Code != "artifact_version_exists" {
		t.Fatalf("expected persisted artifact uniqueness, got %+v", apiErr)
	}
}

func sqlitePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "touch-connect.db")
}

func newSQLiteServer(t *testing.T, path string, settings tcserver.Settings) *tcserver.Server {
	t.Helper()
	server, err := tcserver.NewSQLiteServer(path, settings)
	if err != nil {
		t.Fatalf("create sqlite server: %v", err)
	}
	return server
}

func submitReadback(t *testing.T, server *httptest.Server, attemptRef string) {
	t.Helper()
	req := contracts.ReadbackRequest{
		EndpointRef:   tcworker.DefaultConfig().EndpointRef,
		Summary:       "readback before durable storage test",
		Understanding: "worker understands the durable storage test goal",
	}
	var res contracts.ReadbackResponse
	postJSON(t, server.URL+"/v1/attempts/"+attemptRef+"/readback", server.Client(), req, http.StatusAccepted, &res)
}

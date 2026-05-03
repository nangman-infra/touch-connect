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

func TestExpiredLeaseCanBeTakenOverBySecondWorker(t *testing.T) {
	server, httpServer := newLeaseTestServer(t, 5*time.Millisecond, 3)
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_second_worker", "actor.second")
	registerWorkers(t, httpServer, firstConfig, secondConfig)

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	firstCheckpoint := checkpointAttempt(t, httpServer, firstClaim.AttemptRef, firstConfig.EndpointRef, "in_progress", http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	if reconciled := server.ReconcileExpiredClaims(); reconciled != 1 {
		t.Fatalf("expected one takeover candidate reconciliation, got %d", reconciled)
	}
	if snapshot := server.Snapshot(); snapshot.Messages[0].State != "takeover_candidate" {
		t.Fatalf("expected takeover_candidate before new claim, got %+v", snapshot.Messages[0])
	}
	secondClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, secondConfig.EndpointRef, http.StatusAccepted)
	if !secondClaim.Takeover {
		t.Fatalf("expected takeover claim, got %+v", secondClaim)
	}
	if secondClaim.LastCheckpointRef != firstCheckpoint.CheckpointRef || secondClaim.ResumeSummary == "" {
		t.Fatalf("expected latest checkpoint resume data, got %+v", secondClaim)
	}

	var apiErr contracts.ErrorResponse
	checkpoint := contracts.CheckpointRequest{
		EndpointRef: firstConfig.EndpointRef,
		State:       "in_progress",
		Summary:     "old attempt should not continue",
	}
	postJSON(t, httpServer.URL+"/v1/attempts/"+firstClaim.AttemptRef+"/checkpoints", httpServer.Client(), checkpoint, http.StatusConflict, &apiErr)
	if apiErr.Code != "stale_attempt" {
		t.Fatalf("expected stale_attempt for old owner, got %+v", apiErr)
	}

	snapshot := server.Snapshot()
	firstState := ""
	secondState := ""
	for _, attempt := range snapshot.Attempts {
		if attempt.AttemptRef == firstClaim.AttemptRef {
			firstState = attempt.State
		}
		if attempt.AttemptRef == secondClaim.AttemptRef {
			secondState = attempt.State
		}
	}
	if firstState != "orphaned" || secondState != "claimed" {
		t.Fatalf("expected orphaned old attempt and claimed new attempt, got old=%s new=%s", firstState, secondState)
	}
	if snapshot.Messages[0].RedeliveryCount != 1 {
		t.Fatalf("expected one redelivery, got %+v", snapshot.Messages[0])
	}
}

func TestExpiredLeaseMovesToDLQWhenRedeliveryLimitIsExceeded(t *testing.T) {
	server, httpServer := newLeaseTestServer(t, 5*time.Millisecond, 0)
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_second_worker", "actor.second")
	registerWorkers(t, httpServer, firstConfig, secondConfig)

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	checkpointAttempt(t, httpServer, firstClaim.AttemptRef, firstConfig.EndpointRef, "in_progress", http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	if reconciled := server.ReconcileExpiredClaims(); reconciled != 1 {
		t.Fatalf("expected one DLQ candidate reconciliation, got %d", reconciled)
	}
	var apiErr contracts.ErrorResponse
	req := contracts.ClaimMessageRequest{EndpointRef: secondConfig.EndpointRef}
	postJSON(t, httpServer.URL+"/v1/messages/"+message.MessageRef+"/claim", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "message_dead_lettered" {
		t.Fatalf("expected message_dead_lettered, got %+v", apiErr)
	}

	snapshot := server.Snapshot()
	if len(snapshot.DeadLetters) != 1 {
		t.Fatalf("expected one dead letter, got %+v", snapshot.DeadLetters)
	}
	if snapshot.Messages[0].State != "dead_lettered" {
		t.Fatalf("expected dead_lettered message, got %+v", snapshot.Messages[0])
	}
	if snapshot.DeadLetters[0].LastCheckpointRef == "" || snapshot.DeadLetters[0].Reason != "max_redelivery_exceeded" {
		t.Fatalf("expected DLQ reason and last checkpoint, got %+v", snapshot.DeadLetters[0])
	}
}

func TestOnlyOneWorkerCanTakeOverExpiredClaim(t *testing.T) {
	server, httpServer := newLeaseTestServer(t, 5*time.Millisecond, 3)
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_second_worker", "actor.second")
	thirdConfig := workerConfig("tc://endpoint/ep_third_worker", "actor.third")
	registerWorkers(t, httpServer, firstConfig, secondConfig, thirdConfig)

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	checkpointAttempt(t, httpServer, firstClaim.AttemptRef, firstConfig.EndpointRef, "in_progress", http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	if reconciled := server.ReconcileExpiredClaims(); reconciled != 1 {
		t.Fatalf("expected one concurrent takeover candidate reconciliation, got %d", reconciled)
	}
	start := make(chan struct{})
	outcomes := make(chan claimOutcome, 2)
	go asyncClaim(httpServer.URL, httpServer.Client(), message.MessageRef, secondConfig.EndpointRef, start, outcomes)
	go asyncClaim(httpServer.URL, httpServer.Client(), message.MessageRef, thirdConfig.EndpointRef, start, outcomes)
	close(start)

	accepted := 0
	conflicted := 0
	for range 2 {
		outcome := <-outcomes
		if outcome.Err != nil {
			t.Fatalf("takeover request failed: %v", outcome.Err)
		}
		switch outcome.Status {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			conflicted++
			if outcome.ErrorCode != "message_unavailable" {
				t.Fatalf("expected message_unavailable conflict, got %+v", outcome)
			}
		default:
			t.Fatalf("unexpected takeover outcome: %+v", outcome)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("expected one takeover winner and one conflict, got accepted=%d conflicted=%d", accepted, conflicted)
	}
	if len(server.Snapshot().Attempts) != 2 {
		t.Fatalf("expected old and takeover attempts only, got %+v", server.Snapshot().Attempts)
	}
}

func newLeaseTestServer(t *testing.T, lease time.Duration, maxRedelivery int) (*tcserver.Server, *httptest.Server) {
	t.Helper()
	settings := tcserver.DefaultSettings()
	settings.AttemptLeaseDuration = lease
	settings.MaxRedelivery = maxRedelivery
	server, err := tcserver.NewInMemoryServerWithSettings(settings)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	return server, httptest.NewServer(server.Handler())
}

func workerConfig(endpointRef string, actorID string) tcworker.Config {
	config := tcworker.DefaultConfig()
	config.EndpointRef = endpointRef
	config.DisplayName = endpointRef
	config.ActorID = actorID
	return config
}

func checkpointAttempt(t *testing.T, server *httptest.Server, attemptRef string, endpointRef string, state string, status int) contracts.CheckpointResponse {
	t.Helper()
	req := contracts.CheckpointRequest{
		EndpointRef: endpointRef,
		State:       state,
		Summary:     "checkpoint for takeover test",
	}
	var res contracts.CheckpointResponse
	postJSON(t, server.URL+"/v1/attempts/"+attemptRef+"/checkpoints", server.Client(), req, status, &res)
	return res
}

func registerWorkers(t *testing.T, server *httptest.Server, configs ...tcworker.Config) {
	t.Helper()
	for _, config := range configs {
		worker := tcworker.NewHTTPRuntime(server.URL, server.Client(), config)
		if err := worker.Register(context.Background()); err != nil {
			t.Fatalf("register worker %s: %v", config.EndpointRef, err)
		}
	}
}

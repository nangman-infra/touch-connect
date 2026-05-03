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

func TestWorkerProcessNextClaimsAvailableMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process next: %v", err)
	}
	if result.Empty || result.MessageRef != message.MessageRef || result.AttemptRef == "" || !result.Completed {
		t.Fatalf("expected completed claim-next result, got %+v", result)
	}
	snapshot := server.Snapshot()
	if snapshot.Messages[0].State != "completed" || snapshot.Attempts[0].State != "completed" {
		t.Fatalf("expected completed message and attempt, got %+v", snapshot)
	}
	if len(snapshot.Checkpoints) != 3 {
		t.Fatalf("expected claimed/in_progress/completed checkpoints, got %+v", snapshot.Checkpoints)
	}
}

func TestWorkerProcessNextWaitsWhenQueueIsEmpty(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process empty queue: %v", err)
	}
	if !result.Empty || len(server.Snapshot().Attempts) != 0 {
		t.Fatalf("expected empty queue without attempts, result=%+v snapshot=%+v", result, server.Snapshot())
	}
}

func TestWorkerRunMarksEndpointOfflineAfterMaxMessages(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	seed := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := seed.Register(context.Background()); err != nil {
		t.Fatalf("seed register worker: %v", err)
	}
	ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())

	err := worker.Run(context.Background(), tcworker.LoopOptions{
		PollInterval:      time.Millisecond,
		HeartbeatInterval: time.Millisecond,
		MaxMessages:       1,
	})
	if err != nil {
		t.Fatalf("run worker loop: %v", err)
	}
	snapshot := server.Snapshot()
	if snapshot.Endpoints[0].ConnectionState != "offline" {
		t.Fatalf("expected offline endpoint after bounded loop, got %+v", snapshot.Endpoints[0])
	}
	if snapshot.Messages[0].State != "completed" {
		t.Fatalf("expected message completed by loop, got %+v", snapshot.Messages[0])
	}
}

func TestClaimNextTakesOverExpiredClaim(t *testing.T) {
	settings := tcserver.DefaultSettings()
	settings.AttemptLeaseDuration = 5 * time.Millisecond
	server, err := tcserver.NewInMemoryServerWithSettings(settings)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := workerConfig("tc://endpoint/ep_claim_next_second", "actor.claim-next.second")
	registerWorkers(t, httpServer, firstConfig, secondConfig)
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	firstClaim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	checkpointAttempt(t, httpServer, firstClaim.AttemptRef, firstConfig.EndpointRef, "in_progress", http.StatusAccepted)
	time.Sleep(10 * time.Millisecond)

	second := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), secondConfig)
	result, err := second.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process takeover through claim-next: %v", err)
	}
	if result.Empty || result.MessageRef != message.MessageRef {
		t.Fatalf("expected takeover result for message, got %+v", result)
	}
	snapshot := server.Snapshot()
	if len(snapshot.Attempts) != 2 || snapshot.Messages[0].State != "completed" {
		t.Fatalf("expected old and takeover attempts with completed message, got %+v", snapshot)
	}
}

func TestWorkerExecutorCanBlockForMissingFields(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	ingressWorkerMessage(t, httpServer, []contracts.Constraint{
		{
			Code:      "worker.missing_field",
			Summary:   "target repository is required",
			SourceRef: "target_repository",
			Details:   "worker cannot safely act without a repository target",
		},
	}, false)

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process missing field result: %v", err)
	}
	if !result.Blocked || result.Outcome != tcworker.ExecutionOutcomeMissingFields {
		t.Fatalf("expected blocked missing field result, got %+v", result)
	}
	snapshot := server.Snapshot()
	if len(snapshot.Readbacks) != 1 || snapshot.Readbacks[0].MissingFields[0] != "target_repository" {
		t.Fatalf("expected readback with missing field, got %+v", snapshot.Readbacks)
	}
	if snapshot.Messages[0].State != "input_required" {
		t.Fatalf("expected input_required message, got %+v", snapshot.Messages[0])
	}
}

func TestWorkerExecutorCanFailMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	ingressWorkerMessage(t, httpServer, []contracts.Constraint{
		{Code: "worker.fail", Summary: "executor declined the task", Details: "executor_declined"},
	}, false)

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process failed result: %v", err)
	}
	if !result.Failed || result.Outcome != tcworker.ExecutionOutcomeFailed {
		t.Fatalf("expected failed execution result, got %+v", result)
	}
	snapshot := server.Snapshot()
	if snapshot.Messages[0].State != "failed" {
		t.Fatalf("expected failed message, got %+v", snapshot.Messages[0])
	}
	lastCheckpoint := snapshot.Checkpoints[len(snapshot.Checkpoints)-1]
	if lastCheckpoint.State != "failed" || lastCheckpoint.FailureReasonCode != "executor_declined" {
		t.Fatalf("expected failed checkpoint reason, got %+v", lastCheckpoint)
	}
}

func TestCustomExecutorReceivesPayloadAndConstraints(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	var seen tcworker.ExecutionInput
	executor := executorFunc(func(_ context.Context, input tcworker.ExecutionInput) (tcworker.ExecutionResult, error) {
		seen = input
		return tcworker.ExecutionResult{
			Outcome: tcworker.ExecutionOutcomeCompleted,
			Summary: "custom executor completed message",
		}, nil
	})
	worker := tcworker.NewHTTPRuntimeWithExecutor(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig(), executor)
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressWorkerMessage(t, httpServer, []contracts.Constraint{
		{Code: "preserve_contract", Summary: "Do not bypass the message contract."},
	}, true)

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process custom executor result: %v", err)
	}
	if !result.Completed || seen.MessageRef != message.MessageRef || seen.Payload.Body == "" || len(seen.Constraints) != 1 {
		t.Fatalf("expected executor to receive message payload and constraints, result=%+v input=%+v", result, seen)
	}
	if snapshot := server.Snapshot(); len(snapshot.Readbacks) != 1 {
		t.Fatalf("expected readback for readback-required custom execution, got %+v", snapshot.Readbacks)
	}
}

type executorFunc func(context.Context, tcworker.ExecutionInput) (tcworker.ExecutionResult, error)

func (f executorFunc) Execute(ctx context.Context, input tcworker.ExecutionInput) (tcworker.ExecutionResult, error) {
	return f(ctx, input)
}

func ingressWorkerMessage(t *testing.T, server *httptest.Server, constraints []contracts.Constraint, readbackRequired bool) contracts.MessageIngressResponse {
	t.Helper()
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/ep_local_worker",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "worker execution message",
			Body:       "Executor must inspect this payload.",
			References: []contracts.Reference{},
		},
		Constraints:      constraints,
		ReadbackRequired: readbackRequired,
	}
	var accepted contracts.MessageIngressResponse
	postJSON(t, server.URL+"/v1/messages", server.Client(), req, http.StatusAccepted, &accepted)
	return accepted
}

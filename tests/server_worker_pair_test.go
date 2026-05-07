package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestServerWorkerPairProcessesMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	attemptRef, err := worker.ProcessMessage(context.Background(), message.MessageRef)
	if err != nil {
		t.Fatalf("process message: %v", err)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(snapshot.Endpoints))
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].State != "completed" {
		t.Fatalf("expected completed message, got %+v", snapshot.Messages)
	}
	if len(snapshot.Attempts) != 1 || snapshot.Attempts[0].AttemptRef != attemptRef {
		t.Fatalf("expected one matching attempt, got %+v", snapshot.Attempts)
	}
	if snapshot.Attempts[0].State != "completed" {
		t.Fatalf("expected completed attempt, got %+v", snapshot.Attempts[0])
	}
	if len(snapshot.Checkpoints) != 3 {
		t.Fatalf("expected claimed/in_progress/completed checkpoints, got %d", len(snapshot.Checkpoints))
	}
}

func TestWorkerHeartbeatControlsEndpointLiveness(t *testing.T) {
	settings := tcserver.DefaultSettings()
	settings.EndpointHeartbeatTimeout = 5 * time.Millisecond
	server, err := tcserver.NewInMemoryServerWithSettings(settings)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	if endpoint := server.Snapshot().Endpoints[0]; endpoint.ConnectionState != "online" || endpoint.LastHeartbeatAt.IsZero() {
		t.Fatalf("expected online endpoint with heartbeat, got %+v", endpoint)
	}

	time.Sleep(10 * time.Millisecond)
	if endpoint := server.Snapshot().Endpoints[0]; endpoint.ConnectionState != "stale" {
		t.Fatalf("expected stale endpoint after timeout, got %+v", endpoint)
	}
	if err := worker.Heartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat worker: %v", err)
	}
	if endpoint := server.Snapshot().Endpoints[0]; endpoint.ConnectionState != "online" {
		t.Fatalf("expected heartbeat to restore online state, got %+v", endpoint)
	}
}

func TestLeaseBlocksStaleAttemptCheckpoint(t *testing.T) {
	settings := tcserver.DefaultSettings()
	settings.AttemptLeaseDuration = 5 * time.Millisecond
	server, err := tcserver.NewInMemoryServerWithSettings(settings)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	checkpoint := contracts.CheckpointRequest{
		EndpointRef: tcworker.DefaultConfig().EndpointRef,
		State:       "in_progress",
		Summary:     "checkpoint after lease expiry",
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/checkpoints", httpServer.Client(), checkpoint, http.StatusConflict, &apiErr)
	if apiErr.Code != "lease_expired" {
		t.Fatalf("expected lease_expired, got %+v", apiErr)
	}
}

func TestOnlyOneWorkerCanClaimMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	firstConfig := tcworker.DefaultConfig()
	secondConfig := tcworker.DefaultConfig()
	secondConfig.EndpointRef = "tc://endpoint/ep_second_worker"
	secondConfig.DisplayName = "second worker"
	secondConfig.ActorID = "actor.second"

	first := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), firstConfig)
	second := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), secondConfig)
	if err := first.Register(context.Background()); err != nil {
		t.Fatalf("register first worker: %v", err)
	}
	if err := second.Register(context.Background()); err != nil {
		t.Fatalf("register second worker: %v", err)
	}
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
		switch outcome.Status {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			conflicted++
			if outcome.ErrorCode != "message_unavailable" {
				t.Fatalf("expected message_unavailable conflict, got %+v", outcome)
			}
		default:
			t.Fatalf("unexpected claim outcome: %+v", outcome)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("expected one accepted and one conflict, got accepted=%d conflicted=%d", accepted, conflicted)
	}
}

func TestTargetEndpointRoutesMessageToSelectedWorker(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	firstConfig := tcworker.DefaultConfig()
	secondConfig := tcworker.DefaultConfig()
	secondConfig.EndpointRef = "tc://endpoint/ep_target_worker"
	secondConfig.DisplayName = "target worker"
	secondConfig.ActorID = "actor.target"
	first := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), firstConfig)
	second := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), secondConfig)
	if err := first.Register(context.Background()); err != nil {
		t.Fatalf("register first worker: %v", err)
	}
	if err := second.Register(context.Background()); err != nil {
		t.Fatalf("register second worker: %v", err)
	}

	message := ingressTargetedMessage(t, httpServer, secondConfig.EndpointRef)
	firstResult, err := first.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("first process next: %v", err)
	}
	if !firstResult.Empty {
		t.Fatalf("expected non-target worker to skip targeted message, got %+v", firstResult)
	}
	secondResult, err := second.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("second process next: %v", err)
	}
	if !secondResult.Completed || secondResult.MessageRef != message.MessageRef {
		t.Fatalf("expected target worker to complete message, got %+v", secondResult)
	}
}

func TestDependsOnMessageBlocksClaimUntilDependencyCompletes(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	first := ingressWorkerMessageWithRef(t, httpServer, "tc://message/msg_dep_001", nil)
	second := ingressWorkerMessageWithRef(t, httpServer, "tc://message/msg_dep_002", []string{first.MessageRef})

	firstResult, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process first dependency: %v", err)
	}
	if !firstResult.Completed || firstResult.MessageRef != first.MessageRef {
		t.Fatalf("expected first dependency message, got %+v", firstResult)
	}
	secondResult, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process dependent message: %v", err)
	}
	if !secondResult.Completed || secondResult.MessageRef != second.MessageRef {
		t.Fatalf("expected dependent message after dependency completion, got %+v", secondResult)
	}
}

func TestDuplicateOnlineEndpointRegistrationIsRejected(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register first worker: %v", err)
	}
	if err := worker.Register(context.Background()); err == nil {
		t.Fatalf("expected duplicate online endpoint registration to fail")
	}
}

func TestReadbackRequiredMessageCanBlockForMissingFields(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), true)
	attemptRef, err := worker.BlockMessageForMissingFields(context.Background(), message.MessageRef, []tcworker.MissingField{
		{Name: "target_repository", Reason: "worker cannot safely act without a repository target"},
	})
	if err != nil {
		t.Fatalf("block message: %v", err)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Readbacks) != 1 || snapshot.Readbacks[0].AttemptRef != attemptRef {
		t.Fatalf("expected one readback for attempt, got %+v", snapshot.Readbacks)
	}
	if snapshot.Messages[0].State != "input_required" {
		t.Fatalf("expected input_required message, got %+v", snapshot.Messages[0])
	}
	lastCheckpoint := snapshot.Checkpoints[len(snapshot.Checkpoints)-1]
	if lastCheckpoint.State != "blocked_missing_fields" || len(lastCheckpoint.MissingFields) != 1 {
		t.Fatalf("expected blocked_missing_fields checkpoint, got %+v", lastCheckpoint)
	}
}

func ingressMessage(t *testing.T, baseURL string, client *http.Client, readbackRequired bool) contracts.MessageIngressResponse {
	t.Helper()
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/ep_local_worker",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "change Go code",
			Body:       "Implement a minimal contract test.",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "preserve_contract", Summary: "Do not bypass the message contract."},
		},
		ReadbackRequired: readbackRequired,
	}
	var accepted contracts.MessageIngressResponse
	postJSON(t, baseURL+"/v1/messages", client, req, http.StatusAccepted, &accepted)
	return accepted
}

func ingressTargetedMessage(t *testing.T, server *httptest.Server, endpointRef string) contracts.MessageIngressResponse {
	t.Helper()
	req := baseWorkerIngressRequest("tc://message/msg_targeted", nil)
	req.TargetEndpointRef = endpointRef
	var accepted contracts.MessageIngressResponse
	postJSON(t, server.URL+"/v1/messages", server.Client(), req, http.StatusAccepted, &accepted)
	return accepted
}

func ingressWorkerMessageWithRef(t *testing.T, server *httptest.Server, messageRef string, dependsOn []string) contracts.MessageIngressResponse {
	t.Helper()
	req := baseWorkerIngressRequest(messageRef, dependsOn)
	var accepted contracts.MessageIngressResponse
	postJSON(t, server.URL+"/v1/messages", server.Client(), req, http.StatusAccepted, &accepted)
	return accepted
}

func baseWorkerIngressRequest(messageRef string, dependsOn []string) contracts.MessageIngressRequest {
	return contracts.MessageIngressRequest{
		MessageRef:           messageRef,
		SenderEndpointRef:    "tc://endpoint/ep_local_worker",
		TargetCapability:     "code.change",
		DependsOnMessageRefs: dependsOn,
		Payload: contracts.Payload{
			Summary:    "change Go code",
			Body:       "Implement a minimal contract test.",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "preserve_contract", Summary: "Do not bypass the message contract."},
		},
	}
}

func claimMessage(t *testing.T, baseURL string, client *http.Client, messageRef string, endpointRef string, status int) contracts.ClaimMessageResponse {
	t.Helper()
	req := contracts.ClaimMessageRequest{EndpointRef: endpointRef}
	var claim contracts.ClaimMessageResponse
	postJSON(t, baseURL+"/v1/messages/"+messageRef+"/claim", client, req, status, &claim)
	return claim
}

func postJSON(t *testing.T, url string, client *http.Client, req any, status int, target any) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	res, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != status {
		t.Fatalf("expected status %d, got status %d", status, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

type claimOutcome struct {
	Status    int
	ErrorCode string
	Err       error
}

func asyncClaim(baseURL string, client *http.Client, messageRef string, endpointRef string, start <-chan struct{}, outcomes chan<- claimOutcome) {
	<-start
	req := contracts.ClaimMessageRequest{EndpointRef: endpointRef}
	body, err := json.Marshal(req)
	if err != nil {
		outcomes <- claimOutcome{Err: err}
		return
	}
	res, err := client.Post(baseURL+"/v1/messages/"+messageRef+"/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		outcomes <- claimOutcome{Err: err}
		return
	}
	defer res.Body.Close()
	outcome := claimOutcome{Status: res.StatusCode}
	if res.StatusCode == http.StatusConflict {
		payload, err := io.ReadAll(res.Body)
		if err != nil {
			outcomes <- claimOutcome{Err: err}
			return
		}
		var apiErr contracts.ErrorResponse
		if err := json.Unmarshal(payload, &apiErr); err != nil {
			outcomes <- claimOutcome{Err: err}
			return
		}
		outcome.ErrorCode = apiErr.Code
	}
	outcomes <- outcome
}

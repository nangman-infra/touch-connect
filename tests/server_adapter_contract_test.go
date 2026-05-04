package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

type serverAdapterContract struct {
	Name      string
	ID        string
	NewServer func(t *testing.T, settings tcserver.Settings) *tcserver.Server
}

func TestServerAdaptersSatisfyPortContract(t *testing.T) {
	adapters := []serverAdapterContract{
		{
			Name: "memory",
			ID:   "memory",
			NewServer: func(t *testing.T, settings tcserver.Settings) *tcserver.Server {
				t.Helper()
				server, err := tcserver.NewInMemoryServerWithSettings(settings)
				if err != nil {
					t.Fatalf("create in-memory server: %v", err)
				}
				return server
			},
		},
		{
			Name: "sqlite",
			ID:   "sqlite",
			NewServer: func(t *testing.T, settings tcserver.Settings) *tcserver.Server {
				t.Helper()
				return newSQLiteServer(t, sqlitePath(t), settings)
			},
		},
	}

	for _, adapter := range adapters {
		t.Run(adapter.Name, func(t *testing.T) {
			runServerAdapterPortContract(t, adapter)
		})
	}
}

func runServerAdapterPortContract(t *testing.T, adapter serverAdapterContract) {
	t.Helper()
	server := adapter.NewServer(t, tcserver.DefaultSettings())
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	if endpointCount := len(server.Snapshot().Endpoints); endpointCount != 1 {
		t.Fatalf("expected endpoint registry to persist one endpoint, got %d", endpointCount)
	}

	emptyClaim := claimNextMessage(t, httpServer, tcworker.DefaultConfig().EndpointRef)
	if !emptyClaim.Empty || emptyClaim.Claim != nil {
		t.Fatalf("expected empty claim-next before message, got %+v", emptyClaim)
	}

	message := ingressMessage(t, httpServer.URL, httpServer.Client(), true)
	if message.MessageRef != "tc://message/msg_000001" {
		t.Fatalf("expected empty claim-next not to consume message refs, got %s", message.MessageRef)
	}
	if messageCount := len(server.Snapshot().Messages); messageCount != 1 {
		t.Fatalf("expected message ledger to persist one message, got %d", messageCount)
	}

	claimed := claimNextMessage(t, httpServer, tcworker.DefaultConfig().EndpointRef)
	if claimed.Empty || claimed.Claim == nil {
		t.Fatalf("expected claim-next to return a claim, got %+v", claimed)
	}
	claim := *claimed.Claim
	if claim.AttemptRef != "tc://attempt/att_000001" {
		t.Fatalf("expected empty claim-next not to consume attempt refs, got %s", claim.AttemptRef)
	}
	if claim.MessageRef != message.MessageRef || claim.ReadbackRequired != true {
		t.Fatalf("expected processing ledger claim for message with readback flag, got %+v", claim)
	}

	checkpointAttempt(t, httpServer, claim.AttemptRef, tcworker.DefaultConfig().EndpointRef, "claimed", http.StatusAccepted)
	submitReadback(t, httpServer, claim.AttemptRef)

	artifact := artifactRequest("tc://artifact-version/adapter_contract_" + adapter.ID + "_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	checkpointAttemptWithArtifacts(t, httpServer, claim.AttemptRef, tcworker.DefaultConfig().EndpointRef, []string{artifact.ArtifactVersionRef})

	var finalized contracts.ArtifactFinalizeResponse
	postJSON(t, httpServer.URL+"/v1/control/artifacts/finalize", httpServer.Client(), contracts.ArtifactFinalizeRequest{
		ArtifactVersionRef: artifact.ArtifactVersionRef,
		ActorID:            "actor.adapter.contract",
		Reason:             "adapter contract finalization",
	}, http.StatusAccepted, &finalized)
	if finalized.State != "finalized" || finalized.FinalizationRef == "" {
		t.Fatalf("expected finalized artifact, got %+v", finalized)
	}

	recordApproval(t, httpServer, claim.AttemptRef, approvalRequest("tc://approval/apr_001", "approved", "hash-adapter-"+adapter.ID), http.StatusAccepted)
	started, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-adapter-"+adapter.ID, "hash-adapter-"+adapter.ID))
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}
	deduped, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-adapter-"+adapter.ID, "hash-adapter-"+adapter.ID))
	if err != nil {
		t.Fatalf("dedupe side effect: %v", err)
	}
	if deduped.SideEffectExecutionRef != started.SideEffectExecutionRef || !deduped.Deduped {
		t.Fatalf("expected governance ledger idempotency dedupe, started=%+v deduped=%+v", started, deduped)
	}
	completed, err := worker.CompleteSideEffectExecution(context.Background(), started.SideEffectExecutionRef, contracts.CompleteSideEffectExecutionRequest{
		Status:    "succeeded",
		ResultRef: "tc://artifact-version/adapter_contract_" + adapter.ID + "_result",
	})
	if err != nil {
		t.Fatalf("complete side effect: %v", err)
	}
	if completed.Status != "succeeded" {
		t.Fatalf("expected succeeded side effect completion, got %+v", completed)
	}

	snapshot := server.Snapshot()
	if len(snapshot.Endpoints) != 1 || len(snapshot.Messages) != 1 || len(snapshot.Attempts) != 1 {
		t.Fatalf("expected endpoint/message/attempt projection, got %+v", snapshot)
	}
	if len(snapshot.Checkpoints) != 2 || len(snapshot.Readbacks) != 1 {
		t.Fatalf("expected processing/readback projection, checkpoints=%+v readbacks=%+v", snapshot.Checkpoints, snapshot.Readbacks)
	}
	if len(snapshot.Artifacts) != 1 || len(snapshot.Finalizations) != 1 {
		t.Fatalf("expected artifact projection, artifacts=%+v finalizations=%+v", snapshot.Artifacts, snapshot.Finalizations)
	}
	if len(snapshot.Approvals) != 1 || len(snapshot.SideEffects) != 1 {
		t.Fatalf("expected governance projection, approvals=%+v sideEffects=%+v", snapshot.Approvals, snapshot.SideEffects)
	}
}

func claimNextMessage(t *testing.T, server *httptest.Server, endpointRef string) contracts.ClaimNextMessageResponse {
	t.Helper()
	var response contracts.ClaimNextMessageResponse
	postJSON(t, server.URL+"/v1/messages/claim-next", server.Client(), contracts.ClaimNextMessageRequest{
		EndpointRef: endpointRef,
	}, http.StatusAccepted, &response)
	return response
}

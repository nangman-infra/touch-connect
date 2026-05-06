package serverapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestClientGetAndPostMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			writeTestJSON(t, w, contracts.HealthResponse{Status: "ok", Component: "tc-server", Version: "server-version"})
		case "/version":
			writeTestJSON(t, w, contracts.VersionResponse{Version: "server-version"})
		case "/v1/control/snapshot":
			writeTestJSON(t, w, contracts.SnapshotResponse{Messages: []contracts.MessageRecord{{MessageRef: "tc://message/m"}}})
		case "/v1/messages":
			writeTestJSON(t, w, contracts.MessageIngressResponse{MessageRef: "tc://message/new"})
		case "/v1/attempts/tc://attempt/a/approvals":
			writeTestJSON(t, w, contracts.ApprovalDecisionResponse{ApprovalRef: "tc://approval/a"})
		case "/v1/control/tasks/cancel":
			writeTestJSON(t, w, contracts.TaskCommandResponse{State: "canceled"})
		case "/v1/control/tasks/retry":
			writeTestJSON(t, w, contracts.TaskCommandResponse{State: "available"})
		case "/v1/control/dlq/replay":
			writeTestJSON(t, w, contracts.DLQReplayResponse{MessageRef: "tc://message/replayed"})
		case "/v1/control/artifacts/finalize":
			writeTestJSON(t, w, contracts.ArtifactFinalizeResponse{FinalizationRef: "tc://finalization/f"})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/", server.Client())
	if got, err := client.Health(t.Context()); err != nil || got.Status != "ok" {
		t.Fatalf("Health returned %+v err=%v", got, err)
	}
	if got, err := client.Version(t.Context()); err != nil || got.Version != "server-version" {
		t.Fatalf("Version returned %+v err=%v", got, err)
	}
	if got, err := client.Snapshot(t.Context()); err != nil || len(got.Messages) != 1 {
		t.Fatalf("Snapshot returned %+v err=%v", got, err)
	}
	if got, err := client.SendMessage(t.Context(), contracts.MessageIngressRequest{}); err != nil || got.MessageRef == "" {
		t.Fatalf("SendMessage returned %+v err=%v", got, err)
	}
	approvalReq := contracts.ApprovalCommandRequest{AttemptRef: "tc://attempt/a"}
	if got, err := client.RecordApproval(t.Context(), approvalReq); err != nil || got.ApprovalRef == "" {
		t.Fatalf("RecordApproval returned %+v err=%v", got, err)
	}
	if got, err := client.CancelTask(t.Context(), contracts.TaskCommandRequest{}); err != nil || got.State != "canceled" {
		t.Fatalf("CancelTask returned %+v err=%v", got, err)
	}
	if got, err := client.RetryTask(t.Context(), contracts.TaskCommandRequest{}); err != nil || got.State != "available" {
		t.Fatalf("RetryTask returned %+v err=%v", got, err)
	}
	if got, err := client.ReplayDeadLetter(t.Context(), contracts.DLQReplayRequest{}); err != nil || got.MessageRef == "" {
		t.Fatalf("ReplayDeadLetter returned %+v err=%v", got, err)
	}
	if got, err := client.FinalizeArtifact(t.Context(), contracts.ArtifactFinalizeRequest{}); err != nil || got.FinalizationRef == "" {
		t.Fatalf("FinalizeArtifact returned %+v err=%v", got, err)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		writeTestJSON(t, w, contracts.ErrorResponse{Code: "lease_expired", Message: "lease expired"})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.Health(t.Context())
	var apiErr contracts.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || apiErr.Response.Code != "lease_expired" {
		t.Fatalf("expected APIError, got %T %v", err, err)
	}
}

func TestClientReturnsSyntheticAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.Health(t.Context())
	var apiErr contracts.APIError
	if !errors.As(err, &apiErr) || apiErr.Response.Code != "server_status_502" {
		t.Fatalf("expected synthetic APIError, got %T %v", err, err)
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

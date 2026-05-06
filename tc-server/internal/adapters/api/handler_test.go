package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/memory"
)

func TestHandlerRoutesThroughMessageLifecycle(t *testing.T) {
	handler := newServerAPIHandler(t)

	postServerJSON(t, handler, "/v1/endpoints/register", contracts.EndpointRegistrationRequest{
		EndpointRef:     "tc://endpoint/worker",
		DisplayName:     "Worker",
		ActorID:         "actor.worker",
		WorkspaceID:     "workspace.local",
		ConnectionState: domain.EndpointStateOnline,
		Capabilities:    []contracts.Capability{{Name: "code.change"}},
		WorkerVersion:   "0.1.0-dev",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/endpoints/tc://endpoint/worker/capabilities", contracts.CapabilityAdvertisementRequest{
		Capabilities: []contracts.Capability{{Name: "code.change"}, {Name: "ai.review"}},
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/endpoints/tc://endpoint/worker/heartbeat", contracts.EndpointHeartbeatRequest{
		EndpointRef:     "tc://endpoint/worker",
		ConnectionState: domain.EndpointStateOnline,
	}, http.StatusAccepted, nil)

	var accepted contracts.MessageIngressResponse
	postServerJSON(t, handler, "/v1/messages", contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/manager",
		TargetCapability:  "code.change",
		CorrelationRef:    "tc://task/t",
		Payload: contracts.Payload{
			Summary:    "Change code",
			Body:       "Update the requested file",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{},
		QualityGate: contracts.QualityGateSkip,
	}, http.StatusAccepted, &accepted)

	var claimNext contracts.ClaimNextMessageResponse
	postServerJSON(t, handler, "/v1/messages/claim-next", contracts.ClaimNextMessageRequest{
		EndpointRef: "tc://endpoint/worker",
	}, http.StatusAccepted, &claimNext)
	if claimNext.Claim == nil || claimNext.Claim.AttemptRef == "" {
		t.Fatalf("expected claim-next to claim message, got %+v", claimNext)
	}

	postServerJSON(t, handler, "/v1/attempts/"+claimNext.Claim.AttemptRef+"/readback", contracts.ReadbackRequest{
		EndpointRef:   "tc://endpoint/worker",
		Summary:       "readback",
		Understanding: "I will update the requested file",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/attempts/"+claimNext.Claim.AttemptRef+"/lease", contracts.RefreshLeaseRequest{
		EndpointRef: "tc://endpoint/worker",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/attempts/"+claimNext.Claim.AttemptRef+"/checkpoints", contracts.CheckpointRequest{
		EndpointRef: "tc://endpoint/worker",
		State:       domain.AttemptStateInProgress,
		Summary:     "working",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/control/tasks/cancel", contracts.TaskCommandRequest{
		TaskRef: "tc://task/t",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/control/tasks/retry", contracts.TaskCommandRequest{
		TaskRef: "tc://task/t",
	}, http.StatusAccepted, nil)
	postServerJSON(t, handler, "/v1/messages/"+accepted.MessageRef+"/claim", contracts.ClaimMessageRequest{
		EndpointRef: "tc://endpoint/worker",
	}, http.StatusAccepted, nil)
}

func TestHandlerGetAndA2ARoutes(t *testing.T) {
	handler := newServerAPIHandler(t)
	for _, route := range []string{"/healthz", "/readyz", "/version", "/v1/control/snapshot", "/.well-known/agent.json"} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", route, res.Code, res.Body.String())
		}
	}

	postServerJSON(t, handler, "/a2a/rpc", map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "GetTask",
		"params": map[string]any{
			"id": "tc://message/missing",
		},
	}, http.StatusOK, nil)
	postServerJSON(t, handler, "/v1/a2a/rpc", map[string]any{
		"jsonrpc": "2.0",
		"id":      "2",
		"method":  "Unknown",
	}, http.StatusOK, nil)
}

func TestHandlerRejectsBadRequests(t *testing.T) {
	handler := newServerAPIHandler(t)
	cases := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/missing", nil),
		httptest.NewRequest(http.MethodDelete, "/healthz", nil),
		httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString("{")),
	}
	for _, req := range cases {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code < http.StatusBadRequest {
			t.Fatalf("%s %s should fail, got %d", req.Method, req.URL.Path, res.Code)
		}
	}
}

func TestWriteResultClassifiesErrors(t *testing.T) {
	notFoundErrors := []error{
		domain.ErrEndpointNotFound,
		domain.ErrCapabilityNotFound,
		domain.ErrMessageNotFound,
		domain.ErrAttemptNotFound,
		domain.ErrArtifactNotFound,
		domain.ErrApprovalNotFound,
		domain.ErrSideEffectNotFound,
	}
	for _, err := range notFoundErrors {
		if !isNotFoundError(err) {
			t.Fatalf("%v should be classified as not found", err)
		}
		res := httptest.NewRecorder()
		writeResult(res, http.StatusAccepted, nil, err)
		if res.Code != http.StatusNotFound {
			t.Fatalf("%v returned %d, want 404", err, res.Code)
		}
	}

	conflictErrors := []error{
		domain.ErrMessageUnavailable,
		domain.ErrStaleAttempt,
		domain.ErrEndpointStale,
		domain.ErrLeaseExpired,
		domain.ErrMessageDeadLettered,
		domain.ErrArtifactExists,
		domain.ErrApprovalRequired,
		domain.ErrApprovalRejected,
		domain.ErrApprovalExpired,
		domain.ErrApprovalHashMismatch,
		domain.ErrSelfApproval,
		domain.ErrSideEffectConflict,
	}
	for _, err := range conflictErrors {
		if !isConflictError(err) {
			t.Fatalf("%v should be classified as conflict", err)
		}
		res := httptest.NewRecorder()
		writeResult(res, http.StatusAccepted, nil, err)
		if res.Code != http.StatusConflict {
			t.Fatalf("%v returned %d, want 409", err, res.Code)
		}
	}

	res := httptest.NewRecorder()
	writeResult(res, http.StatusAccepted, nil, application.QualityRejectedError{Decision: contracts.QualityDecision{QualityDecisionRef: "tc://quality-decision/qdc_000001"}})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("quality rejected returned %d, want 400", res.Code)
	}

	res = httptest.NewRecorder()
	writeResult(res, http.StatusAccepted, nil, errors.New("other"))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("unclassified error returned %d, want 400", res.Code)
	}
}

func TestA2AResponseHelpersAndBaseURL(t *testing.T) {
	res := httptest.NewRecorder()
	writeA2AError(res, "1", domain.ErrMessageNotFound)
	if res.Code != http.StatusOK {
		t.Fatalf("A2A errors should be JSON-RPC 200 responses, got %d", res.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "http://internal.example/.well-known/agent.json", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "touch.example")
	if got := a2aBaseURL(req); got != "https://touch.example" {
		t.Fatalf("a2aBaseURL = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "https://direct.example/.well-known/agent.json", nil)
	if got := a2aBaseURL(req); got != "https://direct.example" {
		t.Fatalf("a2aBaseURL TLS fallback = %q", got)
	}
}

func TestPathExtractors(t *testing.T) {
	if got := endpointRefFromPath("v1/endpoints/tc://endpoint/worker/heartbeat", "/heartbeat"); got != "tc://endpoint/worker" {
		t.Fatalf("endpointRefFromPath = %q", got)
	}
	if got := messageRefFromPath("v1/messages/tc://message/msg_000001/claim"); got != "tc://message/msg_000001" {
		t.Fatalf("messageRefFromPath = %q", got)
	}
	if got := attemptRefFromPath("v1/attempts/tc://attempt/att_000001/complete", "/complete"); got != "tc://attempt/att_000001" {
		t.Fatalf("attemptRefFromPath = %q", got)
	}
	if got := sideEffectRefFromPath("v1/side-effects/tc://side-effect/sfx_000001/complete"); got != "tc://side-effect/sfx_000001" {
		t.Fatalf("sideEffectRefFromPath = %q", got)
	}
}

func newServerAPIHandler(t *testing.T) *Handler {
	t.Helper()
	store := memory.NewStore()
	service, err := application.NewService(application.PortsFromStore(store), application.DefaultSettings())
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return NewHandler(service)
}

func postServerJSON(t *testing.T, handler *Handler, route string, body any, wantStatus int, target any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("POST %s returned %d, want %d: %s", route, res.Code, wantStatus, res.Body.String())
	}
	if target != nil {
		if err := json.Unmarshal(res.Body.Bytes(), target); err != nil {
			t.Fatalf("decode response from %s: %v body=%s", route, err, res.Body.String())
		}
	}
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-control/internal/application"
)

type apiFakeServerClient struct{}

func (apiFakeServerClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{Status: "ok", Component: "tc-server", Version: "server-version"}, nil
}

func (apiFakeServerClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{Version: "server-version", MinimumWorker: "0.1.0", ContractVersion: "contract"}, nil
}

func (apiFakeServerClient) Snapshot(context.Context) (contracts.SnapshotResponse, error) {
	return contracts.SnapshotResponse{
		Endpoints: []contracts.EndpointRecord{{EndpointRef: "tc://endpoint/worker", Capabilities: map[string]contracts.Capability{"code.change": {Name: "code.change"}}}},
		Messages:  []contracts.MessageRecord{{MessageRef: "tc://message/m", CorrelationRef: "tc://task/t", State: "completed"}},
		Attempts:  []contracts.AttemptRecord{{AttemptRef: "tc://attempt/a", MessageRef: "tc://message/m"}},
		Artifacts: []contracts.ArtifactRecord{{ArtifactRef: "tc://artifact/a", ArtifactVersionRef: "tc://artifact-version/a", TaskRef: "tc://task/t"}},
		Approvals: []contracts.ApprovalRecord{{ApprovalRef: "tc://approval/a", TargetRef: "tc://side-effect/s"}},
		DeadLetters: []contracts.DeadLetterRecord{{
			DeadLetterRef: "tc://dead-letter/d",
			MessageRef:    "tc://message/m",
		}},
		SideEffects: []contracts.SideEffectRecord{{SideEffectExecutionRef: "tc://side-effect/s", TaskRef: "tc://task/t"}},
	}, nil
}

func (apiFakeServerClient) SendMessage(context.Context, contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	return contracts.MessageIngressResponse{MessageRef: "tc://message/new", State: "available"}, nil
}

func (apiFakeServerClient) RecordApproval(context.Context, contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	return contracts.ApprovalDecisionResponse{ApprovalRef: "tc://approval/a"}, nil
}

func (apiFakeServerClient) CancelTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{TaskRef: "tc://task/t", State: "canceled"}, nil
}

func (apiFakeServerClient) RetryTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{TaskRef: "tc://task/t", State: "available"}, nil
}

func (apiFakeServerClient) ReplayDeadLetter(context.Context, contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	return contracts.DLQReplayResponse{DeadLetterRef: "tc://dead-letter/d", MessageRef: "tc://message/replayed"}, nil
}

func (apiFakeServerClient) FinalizeArtifact(context.Context, contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	return contracts.ArtifactFinalizeResponse{ArtifactVersionRef: "tc://artifact-version/a", FinalizationRef: "tc://finalization/f"}, nil
}

type apiErrorServerClient struct{}

func (apiErrorServerClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{}, errors.New("server down")
}

func (apiErrorServerClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{}, errors.New("server down")
}

func (apiErrorServerClient) Snapshot(context.Context) (contracts.SnapshotResponse, error) {
	return contracts.SnapshotResponse{}, errors.New("server down")
}

func (apiErrorServerClient) SendMessage(context.Context, contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	return contracts.MessageIngressResponse{}, contracts.APIError{
		StatusCode: http.StatusUnprocessableEntity,
		Response: contracts.ErrorResponse{
			Code:    contracts.ErrorCodeQualityRejected,
			Message: "quality rejected",
		},
	}
}

func (apiErrorServerClient) RecordApproval(context.Context, contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	return contracts.ApprovalDecisionResponse{}, errors.New("server down")
}

func (apiErrorServerClient) CancelTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{}, errors.New("server down")
}

func (apiErrorServerClient) RetryTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return contracts.TaskCommandResponse{}, errors.New("server down")
}

func (apiErrorServerClient) ReplayDeadLetter(context.Context, contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	return contracts.DLQReplayResponse{}, errors.New("server down")
}

func (apiErrorServerClient) FinalizeArtifact(context.Context, contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	return contracts.ArtifactFinalizeResponse{}, errors.New("server down")
}

func TestHandlerGetRoutes(t *testing.T) {
	handler := newTestHandler(t)
	routes := []string{
		"/healthz",
		"/readyz",
		"/version",
		"/v1/snapshot",
		"/v1/endpoints",
		"/v1/endpoints/inspect?ref=tc://endpoint/worker",
		"/v1/capabilities",
		"/v1/messages",
		"/v1/messages/inspect?ref=tc://message/m",
		"/v1/messages/history?task=tc://task/t",
		"/v1/tasks/status?task=tc://task/t",
		"/v1/tasks/history?task=tc://task/t",
		"/v1/artifacts",
		"/v1/artifacts/lineage?ref=tc://artifact-version/a",
		"/v1/artifacts/inspect?ref=tc://artifact-version/a",
		"/v1/approvals",
		"/v1/approvals/chain?ref=tc://approval/a",
		"/v1/approvals/inspect?ref=tc://approval/a",
		"/v1/dlq",
		"/v1/dlq/inspect?ref=tc://dead-letter/d",
		"/v1/side-effects",
	}
	for _, route := range routes {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", route, res.Code, res.Body.String())
		}
	}
}

func TestHandlerPostRoutes(t *testing.T) {
	handler := newTestHandler(t)
	routes := map[string]any{
		"/v1/messages":           contracts.MessageIngressRequest{},
		"/v1/tasks/cancel":       contracts.TaskCommandRequest{},
		"/v1/tasks/retry":        contracts.TaskCommandRequest{},
		"/v1/artifacts/finalize": contracts.ArtifactFinalizeRequest{},
		"/v1/approvals/decide":   contracts.ApprovalCommandRequest{},
		"/v1/dlq/replay":         contracts.DLQReplayRequest{},
	}
	for route, body := range routes {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, jsonBody(t, body))
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusAccepted {
			t.Fatalf("POST %s returned %d: %s", route, res.Code, res.Body.String())
		}
	}
}

func TestHandlerErrors(t *testing.T) {
	handler := newTestHandler(t)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/missing", nil),
		httptest.NewRequest(http.MethodDelete, "/healthz", nil),
		httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString("{")),
	} {
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code < http.StatusBadRequest {
			t.Fatalf("%s %s should fail, got %d", req.Method, req.URL.Path, res.Code)
		}
	}
}

func TestHandlerLookupMissesAndServerErrors(t *testing.T) {
	handler := newTestHandler(t)
	notFoundRoutes := []string{
		"/v1/endpoints/inspect?ref=tc://endpoint/missing",
		"/v1/messages/inspect?ref=tc://message/missing",
		"/v1/tasks/status?task=tc://task/missing",
		"/v1/tasks/history?task=tc://task/missing",
		"/v1/artifacts/inspect?ref=tc://artifact-version/missing",
		"/v1/artifacts/lineage?ref=tc://artifact-version/missing",
		"/v1/approvals/inspect?ref=tc://approval/missing",
		"/v1/approvals/chain?ref=tc://approval/missing",
		"/v1/dlq/inspect?ref=tc://dead-letter/missing",
	}
	for _, route := range notFoundRoutes {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404: %s", route, res.Code, res.Body.String())
		}
	}

	errorHandler := newErrorHandler(t)
	errorRoutes := []string{
		"/readyz",
		"/version",
		"/v1/snapshot",
		"/v1/endpoints",
		"/v1/capabilities",
		"/v1/messages",
		"/v1/tasks/status?task=tc://task/t",
		"/v1/artifacts",
		"/v1/approvals",
		"/v1/dlq",
		"/v1/side-effects",
	}
	for _, route := range errorRoutes {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, route, nil)
		errorHandler.ServeHTTP(res, req)
		if res.Code != http.StatusBadGateway {
			t.Fatalf("GET %s returned %d, want 502: %s", route, res.Code, res.Body.String())
		}
	}
	postServerJSONToControl(t, errorHandler, "/v1/messages", contracts.MessageIngressRequest{}, http.StatusUnprocessableEntity)
	postServerJSONToControl(t, errorHandler, "/v1/tasks/cancel", contracts.TaskCommandRequest{}, http.StatusBadGateway)
	postServerJSONToControl(t, errorHandler, "/v1/tasks/retry", contracts.TaskCommandRequest{}, http.StatusBadGateway)
	postServerJSONToControl(t, errorHandler, "/v1/artifacts/finalize", contracts.ArtifactFinalizeRequest{}, http.StatusBadGateway)
	postServerJSONToControl(t, errorHandler, "/v1/approvals/decide", contracts.ApprovalCommandRequest{}, http.StatusBadGateway)
	postServerJSONToControl(t, errorHandler, "/v1/dlq/replay", contracts.DLQReplayRequest{}, http.StatusBadGateway)
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	service, err := application.NewService(apiFakeServerClient{}, "control-version")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return NewHandler(service)
}

func newErrorHandler(t *testing.T) *Handler {
	t.Helper()
	service, err := application.NewService(apiErrorServerClient{}, "control-version")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return NewHandler(service)
}

func postServerJSONToControl(t *testing.T, handler *Handler, route string, body any, wantStatus int) {
	t.Helper()
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, jsonBody(t, body))
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("POST %s returned %d, want %d: %s", route, res.Code, wantStatus, res.Body.String())
	}
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return bytes.NewReader(body)
}

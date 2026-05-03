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

func TestProtectedSideEffectRequiresApprovedDecision(t *testing.T) {
	httpServer, worker, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
	defer httpServer.Close()

	req := sideEffectRequest("idem-no-approval", "hash-1")
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/side-effects", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "approval_required" {
		t.Fatalf("expected approval_required, got %+v", apiErr)
	}

	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-1")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)
	started, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, req)
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}
	if started.Status != "executing" || started.Deduped {
		t.Fatalf("expected executing side effect, got %+v", started)
	}
}

func TestApprovalRejectsSelfApproval(t *testing.T) {
	httpServer, _, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
	defer httpServer.Close()

	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-self")
	approval.DecidedByActorID = approval.RequestedByActorID
	var apiErr contracts.ErrorResponse
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusConflict, &apiErr)
	if apiErr.Code != "self_approval_forbidden" {
		t.Fatalf("expected self_approval_forbidden, got %+v", apiErr)
	}
}

func TestSideEffectRejectsApprovalHashMismatch(t *testing.T) {
	httpServer, _, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
	defer httpServer.Close()

	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-approved")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)
	req := sideEffectRequest("idem-hash", "hash-mutated")
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/side-effects", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "approval_hash_mismatch" {
		t.Fatalf("expected approval_hash_mismatch, got %+v", apiErr)
	}
}

func TestSideEffectRejectsNonApprovedOrExpiredApproval(t *testing.T) {
	cases := []struct {
		name        string
		status      string
		expiresAt   string
		expectedErr string
	}{
		{name: "rejected", status: "rejected", expectedErr: "approval_not_approved"},
		{name: "expired", status: "approved", expiresAt: time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), expectedErr: "approval_expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			httpServer, _, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
			defer httpServer.Close()
			approval := approvalRequest("tc://approval/apr_001", tc.status, "hash-status")
			approval.ExpiresAt = tc.expiresAt
			recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)

			req := sideEffectRequest("idem-"+tc.name, "hash-status")
			var apiErr contracts.ErrorResponse
			postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/side-effects", httpServer.Client(), req, http.StatusConflict, &apiErr)
			if apiErr.Code != tc.expectedErr {
				t.Fatalf("expected %s, got %+v", tc.expectedErr, apiErr)
			}
		})
	}
}

func TestSideEffectRequiresCurrentLease(t *testing.T) {
	server, err := tcserver.NewInMemoryServerWithSettings(leaseSettings(5 * time.Millisecond))
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	httpServer, _, claim := sideEffectFixture(t, server)
	defer httpServer.Close()
	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-lease")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)

	time.Sleep(10 * time.Millisecond)
	req := sideEffectRequest("idem-lease", "hash-lease")
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/attempts/"+claim.AttemptRef+"/side-effects", httpServer.Client(), req, http.StatusConflict, &apiErr)
	if apiErr.Code != "lease_expired" {
		t.Fatalf("expected lease_expired, got %+v", apiErr)
	}
}

func TestSideEffectCompletionRequiresCurrentLease(t *testing.T) {
	server, err := tcserver.NewInMemoryServerWithSettings(leaseSettings(5 * time.Millisecond))
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	httpServer, worker, claim := sideEffectFixture(t, server)
	defer httpServer.Close()
	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-complete-lease")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)
	started, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-complete-lease", "hash-complete-lease"))
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	complete := contracts.CompleteSideEffectExecutionRequest{
		EndpointRef: tcworker.DefaultConfig().EndpointRef,
		Status:      "succeeded",
		ResultRef:   "tc://artifact-version/result_after_lease",
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/side-effects/"+started.SideEffectExecutionRef+"/complete", httpServer.Client(), complete, http.StatusConflict, &apiErr)
	if apiErr.Code != "lease_expired" {
		t.Fatalf("expected lease_expired, got %+v", apiErr)
	}
}

func TestSideEffectIdempotencyDedupe(t *testing.T) {
	httpServer, worker, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
	defer httpServer.Close()
	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-dedupe")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)

	req := sideEffectRequest("idem-dedupe", "hash-dedupe")
	first, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, req)
	if err != nil {
		t.Fatalf("start first side effect: %v", err)
	}
	second, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, req)
	if err != nil {
		t.Fatalf("start duplicate side effect: %v", err)
	}
	if first.SideEffectExecutionRef != second.SideEffectExecutionRef || !second.Deduped || second.Status != "deduped" {
		t.Fatalf("expected deduped existing execution, first=%+v second=%+v", first, second)
	}
}

func TestSideEffectCompletionRequiresExplicitSuccessResult(t *testing.T) {
	httpServer, worker, claim := sideEffectFixture(t, tcserver.NewInMemoryServer())
	defer httpServer.Close()
	approval := approvalRequest("tc://approval/apr_001", "approved", "hash-complete")
	recordApproval(t, httpServer, claim.AttemptRef, approval, http.StatusAccepted)
	started, err := worker.StartSideEffectExecution(context.Background(), claim.AttemptRef, sideEffectRequest("idem-complete", "hash-complete"))
	if err != nil {
		t.Fatalf("start side effect: %v", err)
	}

	invalid := contracts.CompleteSideEffectExecutionRequest{
		EndpointRef: tcworker.DefaultConfig().EndpointRef,
		Status:      "succeeded",
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, httpServer.URL+"/v1/side-effects/"+started.SideEffectExecutionRef+"/complete", httpServer.Client(), invalid, http.StatusBadRequest, &apiErr)
	if apiErr.Code != "invalid_input" {
		t.Fatalf("expected invalid_input, got %+v", apiErr)
	}

	completed, err := worker.CompleteSideEffectExecution(context.Background(), started.SideEffectExecutionRef, contracts.CompleteSideEffectExecutionRequest{
		Status:    "succeeded",
		ResultRef: "tc://artifact-version/side_effect_result",
	})
	if err != nil {
		t.Fatalf("complete side effect: %v", err)
	}
	if completed.Status != "succeeded" || completed.CompletedAt == "" {
		t.Fatalf("expected succeeded completion, got %+v", completed)
	}
}

func sideEffectFixture(t *testing.T, server *tcserver.Server) (*httptest.Server, *tcworker.Runtime, contracts.ClaimMessageResponse) {
	t.Helper()
	httpServer := httptest.NewServer(server.Handler())
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, httpServer.URL, httpServer.Client(), false)
	claim := claimMessage(t, httpServer.URL, httpServer.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	return httpServer, worker, claim
}

func approvalRequest(approvalRef string, status string, approvalHash string) contracts.ApprovalDecisionRequest {
	return contracts.ApprovalDecisionRequest{
		ApprovalRef:             approvalRef,
		TargetType:              "side_effect",
		TargetRef:               "tc://protected-scope/repo-write",
		RequestedByActorID:      "actor.requester",
		ApproverSubjectsOrRoles: []string{"role.owner"},
		ApprovalScope:           "workspace",
		ApprovalHash:            approvalHash,
		Status:                  status,
		Reason:                  "protected repo write",
		DecidedByActorID:        "actor.approver",
		DecisionNote:            "approved for test",
	}
}

func sideEffectRequest(idempotencyKey string, approvalHash string) contracts.SideEffectExecutionRequest {
	return contracts.SideEffectExecutionRequest{
		EndpointRef:        tcworker.DefaultConfig().EndpointRef,
		IdempotencyKey:     idempotencyKey,
		ProtectedScope:     "workspace:repo-write",
		ApprovalRef:        "tc://approval/apr_001",
		ApprovalHash:       approvalHash,
		TaskRef:            "tc://task/task_001",
		OperationKind:      "git_push",
		ExternalTarget:     "git@github.com:nangman-infra/touch-connect.git",
		RequestedByActorID: "actor.requester",
	}
}

func recordApproval(t *testing.T, server *httptest.Server, attemptRef string, req contracts.ApprovalDecisionRequest, status int, target ...*contracts.ErrorResponse) contracts.ApprovalDecisionResponse {
	t.Helper()
	var res contracts.ApprovalDecisionResponse
	if len(target) > 0 {
		postJSON(t, server.URL+"/v1/attempts/"+attemptRef+"/approvals", server.Client(), req, status, target[0])
		return res
	}
	postJSON(t, server.URL+"/v1/attempts/"+attemptRef+"/approvals", server.Client(), req, status, &res)
	return res
}

func leaseSettings(lease time.Duration) tcserver.Settings {
	settings := tcserver.DefaultSettings()
	settings.AttemptLeaseDuration = lease
	return settings
}

package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestRunProcessingLoopProcessesOneMessageAndStops(t *testing.T) {
	client := &successfulLoopClient{
		claim: contracts.ClaimMessageResponse{
			MessageRef:       "tc://message/msg_loop",
			AttemptRef:       "tc://attempt/att_loop",
			EndpointRef:      "tc://endpoint/worker_loop",
			LeaseExpiresAt:   time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano),
			TargetCapability: "code.change",
			Payload: contracts.Payload{
				Summary: "loop claim",
				Body:    "process exactly once",
			},
		},
	}
	worker := NewWithExecutor(client, Config{
		EndpointRef:   "tc://endpoint/worker_loop",
		DisplayName:   "loop worker",
		ActorID:       "actor.loop",
		WorkspaceID:   "workspace.loop",
		WorkerVersion: "0.1.0-dev",
		Capabilities:  []contracts.Capability{{Name: "code.change"}},
	}, EchoExecutor{})

	err := worker.runProcessingLoop(context.Background(), LoopOptions{
		PollInterval: time.Millisecond,
		MaxMessages:  1,
	}, make(chan error))
	if err != nil {
		t.Fatalf("processing loop: %v", err)
	}
	if client.claims != 1 || client.completed != 1 {
		t.Fatalf("expected one claim and completion, got claims=%d completed=%d", client.claims, client.completed)
	}
}

func TestLoopHelpersHandleInterruptsAndValidation(t *testing.T) {
	defaults := DefaultLoopOptions()
	if defaults.PollInterval != time.Second || defaults.HeartbeatInterval != 10*time.Second {
		t.Fatalf("unexpected default loop options: %+v", defaults)
	}
	accepted, err := (LoopOptions{}).Validated()
	if err != nil {
		t.Fatalf("validate empty loop options: %v", err)
	}
	if accepted.PollInterval != time.Second || accepted.HeartbeatInterval != 10*time.Second {
		t.Fatalf("unexpected accepted defaults: %+v", accepted)
	}
	if _, err := (LoopOptions{PollInterval: -time.Second}).Validated(); err == nil {
		t.Fatalf("expected negative loop option to fail")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	interrupted, err := processingLoopInterrupted(ctx, make(chan error))
	if !interrupted || err != nil {
		t.Fatalf("expected context interruption, interrupted=%t err=%v", interrupted, err)
	}

	heartbeatErrors := make(chan error, 1)
	expected := errors.New("heartbeat failed")
	heartbeatErrors <- expected
	interrupted, err = processingLoopInterrupted(context.Background(), heartbeatErrors)
	if interrupted || !errors.Is(err, expected) {
		t.Fatalf("expected heartbeat error, interrupted=%t err=%v", interrupted, err)
	}
	if !maxMessagesReached(2, 2) || maxMessagesReached(1, 2) || maxMessagesReached(10, 0) {
		t.Fatalf("max message helper mismatch")
	}
	if err := waitForNextPoll(ctx, time.Hour, make(chan error)); err != nil {
		t.Fatalf("cancelled poll should exit cleanly: %v", err)
	}
	heartbeatErrors <- expected
	if err := waitForNextPoll(context.Background(), time.Hour, heartbeatErrors); !errors.Is(err, expected) {
		t.Fatalf("expected heartbeat error from poll, got %v", err)
	}
}

func TestHeartbeatIncludesCurrentAttemptProgress(t *testing.T) {
	client := &successfulLoopClient{}
	worker := NewWithExecutor(client, Config{
		EndpointRef:   "tc://endpoint/worker_progress",
		DisplayName:   "progress worker",
		ActorID:       "actor.progress",
		WorkspaceID:   "workspace.progress",
		WorkerVersion: "0.1.0-dev",
		Capabilities:  []contracts.Capability{{Name: "code.change"}},
	}, EchoExecutor{})
	worker.markProgress("tc://attempt/att_progress", "still working")

	if err := worker.Heartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if client.lastHeartbeat.CurrentAttemptRef != "tc://attempt/att_progress" ||
		client.lastHeartbeat.ProgressSummary != "still working" ||
		client.lastHeartbeat.LastActivityAt == "" {
		t.Fatalf("expected heartbeat progress fields, got %+v", client.lastHeartbeat)
	}
}

func TestHeartbeatRefreshesDynamicCapabilities(t *testing.T) {
	client := &successfulLoopClient{}
	executor := dynamicCapabilityExecutor{
		capabilities: []contracts.Capability{{Name: "ai.review"}},
		result:       ExecutionResult{Outcome: ExecutionOutcomeCompleted, Summary: "done"},
	}
	worker := NewWithExecutor(client, Config{
		EndpointRef:   "tc://endpoint/worker_dynamic_capabilities",
		DisplayName:   "dynamic capability worker",
		ActorID:       "actor.dynamic",
		WorkspaceID:   "workspace.dynamic",
		WorkerVersion: "0.1.0-dev",
		Capabilities:  []contracts.Capability{{Name: "code.change"}},
	}, executor)

	if err := worker.Heartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if len(client.advertisedCapabilities) != 1 || client.advertisedCapabilities[0].Name != "ai.review" {
		t.Fatalf("expected re-advertised dynamic capability, got %+v", client.advertisedCapabilities)
	}
	if len(worker.config.Capabilities) != 1 || worker.config.Capabilities[0].Name != "ai.review" {
		t.Fatalf("expected worker config to use refreshed capability, got %+v", worker.config.Capabilities)
	}
	if err := worker.Heartbeat(context.Background()); err != nil {
		t.Fatalf("second heartbeat: %v", err)
	}
	if client.advertiseCalls != 1 {
		t.Fatalf("expected unchanged dynamic capabilities to skip duplicate advertise, got %d calls", client.advertiseCalls)
	}
}

func TestFinishClaimBranchesForExecutorFailuresAndTerminalOutcomes(t *testing.T) {
	claim := contracts.ClaimMessageResponse{
		MessageRef:       "tc://message/msg_finish",
		AttemptRef:       "tc://attempt/att_finish",
		EndpointRef:      "tc://endpoint/worker_finish",
		LeaseExpiresAt:   time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano),
		TargetCapability: "code.change",
		Payload:          contracts.Payload{Summary: "finish claim", Body: "finish claim"},
	}
	for _, testCase := range []struct {
		name    string
		result  ExecutionResult
		err     error
		outcome string
	}{
		{name: "executor_error", err: errors.New("boom"), outcome: ExecutionOutcomeFailed},
		{name: "invalid_result", result: ExecutionResult{Outcome: "unknown"}, outcome: ExecutionOutcomeFailed},
		{name: "missing_fields", result: ExecutionResult{Outcome: ExecutionOutcomeMissingFields, MissingFields: []MissingField{{Name: "target", Reason: "missing"}}}, outcome: ExecutionOutcomeMissingFields},
		{name: "failed", result: ExecutionResult{Outcome: ExecutionOutcomeFailed, Summary: "failed", FailureReasonCode: "worker_failed"}, outcome: ExecutionOutcomeFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := &successfulLoopClient{claim: claim}
			worker := NewWithExecutor(client, Config{
				EndpointRef:   "tc://endpoint/worker_finish",
				DisplayName:   "finish worker",
				ActorID:       "actor.finish",
				WorkspaceID:   "workspace.finish",
				WorkerVersion: "0.1.0-dev",
				Capabilities:  []contracts.Capability{{Name: "code.change"}},
			}, staticExecutor{result: testCase.result, err: testCase.err})
			_, outcome, err := worker.finishClaimAfterAck(context.Background(), claim)
			if testCase.err != nil {
				if !errors.Is(err, testCase.err) {
					t.Fatalf("expected executor error, got %v", err)
				}
				return
			}
			if err == nil && outcome != testCase.outcome {
				t.Fatalf("unexpected outcome %s", outcome)
			}
		})
	}
}

func TestProcessNextDropsRecoverableAttemptError(t *testing.T) {
	client := &recoverableDropClient{
		claim: contracts.ClaimMessageResponse{
			MessageRef:       "tc://message/msg_drop",
			AttemptRef:       "tc://attempt/att_drop",
			EndpointRef:      "tc://endpoint/worker_drop",
			LeaseExpiresAt:   time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano),
			TargetCapability: "code.change",
			Payload: contracts.Payload{
				Summary:    "drop attempt",
				Body:       "drop attempt after checkpoint conflict",
				References: []contracts.Reference{},
			},
			Constraints: []contracts.Constraint{},
		},
	}
	worker := NewWithExecutor(client, Config{
		EndpointRef:   "tc://endpoint/worker_drop",
		DisplayName:   "drop worker",
		ActorID:       "actor.drop",
		WorkspaceID:   "workspace.drop",
		WorkerVersion: "0.1.0-dev",
		Capabilities:  []contracts.Capability{{Name: "code.change"}},
	}, EchoExecutor{})

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("expected recoverable attempt error to be dropped, got %v", err)
	}
	if !result.Dropped || result.DropReason != "lease_expired" || result.Outcome != ExecutionOutcomeDropped {
		t.Fatalf("expected dropped attempt result, got %+v", result)
	}
	if client.checkpointCalls != 2 {
		t.Fatalf("expected claimed checkpoint plus terminal checkpoint attempt, got %d", client.checkpointCalls)
	}
}

type recoverableDropClient struct {
	claim           contracts.ClaimMessageResponse
	checkpointCalls int
}

type successfulLoopClient struct {
	claim                  contracts.ClaimMessageResponse
	claims                 int
	completed              int
	advertiseCalls         int
	advertisedCapabilities []contracts.Capability
	lastHeartbeat          contracts.EndpointHeartbeatRequest
	checkpointRequests     []contracts.CheckpointRequest
}

type staticExecutor struct {
	result ExecutionResult
	err    error
}

func (e staticExecutor) Execute(context.Context, ExecutionInput) (ExecutionResult, error) {
	return e.result, e.err
}

type dynamicCapabilityExecutor struct {
	capabilities []contracts.Capability
	result       ExecutionResult
}

func (e dynamicCapabilityExecutor) Execute(context.Context, ExecutionInput) (ExecutionResult, error) {
	return e.result, nil
}

func (e dynamicCapabilityExecutor) RefreshCapabilities(context.Context) ([]contracts.Capability, error) {
	return append([]contracts.Capability(nil), e.capabilities...), nil
}

func (c *successfulLoopClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{}, nil
}

func (c *successfulLoopClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{}, nil
}

func (c *successfulLoopClient) Snapshot(context.Context) (contracts.SnapshotResponse, error) {
	return contracts.SnapshotResponse{}, nil
}

func (c *successfulLoopClient) RegisterEndpoint(context.Context, contracts.EndpointRegistrationRequest) (contracts.EndpointRegistrationResponse, error) {
	return contracts.EndpointRegistrationResponse{}, nil
}

func (c *successfulLoopClient) HeartbeatEndpoint(_ context.Context, _ string, req contracts.EndpointHeartbeatRequest) (contracts.EndpointHeartbeatResponse, error) {
	c.lastHeartbeat = req
	return contracts.EndpointHeartbeatResponse{}, nil
}

func (c *successfulLoopClient) AdvertiseCapabilities(_ context.Context, _ string, req contracts.CapabilityAdvertisementRequest) (contracts.CapabilityAdvertisementResponse, error) {
	c.advertiseCalls++
	c.advertisedCapabilities = append([]contracts.Capability(nil), req.Capabilities...)
	return contracts.CapabilityAdvertisementResponse{}, nil
}

func (c *successfulLoopClient) ClaimMessage(context.Context, string, contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error) {
	return c.claim, nil
}

func (c *successfulLoopClient) ClaimNextMessage(context.Context, contracts.ClaimNextMessageRequest) (contracts.ClaimNextMessageResponse, error) {
	c.claims++
	if c.claims > 1 {
		return contracts.ClaimNextMessageResponse{Empty: true}, nil
	}
	return contracts.ClaimNextMessageResponse{Claim: &c.claim}, nil
}

func (c *successfulLoopClient) SubmitCheckpoint(_ context.Context, _ string, req contracts.CheckpointRequest) (contracts.CheckpointResponse, error) {
	c.checkpointRequests = append(c.checkpointRequests, req)
	return contracts.CheckpointResponse{}, nil
}

func (c *successfulLoopClient) SubmitReadback(context.Context, string, contracts.ReadbackRequest) (contracts.ReadbackResponse, error) {
	return contracts.ReadbackResponse{}, nil
}

func (c *successfulLoopClient) RefreshLease(context.Context, string, contracts.RefreshLeaseRequest) (contracts.RefreshLeaseResponse, error) {
	return contracts.RefreshLeaseResponse{
		AttemptRef:     c.claim.AttemptRef,
		State:          "claimed",
		LeaseExpiresAt: time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano),
	}, nil
}

func (c *successfulLoopClient) RegisterArtifactVersion(context.Context, string, contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error) {
	return contracts.ArtifactVersionResponse{}, nil
}

func (c *successfulLoopClient) RecordApprovalDecision(context.Context, string, contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error) {
	return contracts.ApprovalDecisionResponse{}, nil
}

func (c *successfulLoopClient) StartSideEffectExecution(context.Context, string, contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error) {
	return contracts.SideEffectExecutionResponse{}, nil
}

func (c *successfulLoopClient) CompleteSideEffectExecution(context.Context, string, contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error) {
	return contracts.CompleteSideEffectExecutionResponse{}, nil
}

func (c *successfulLoopClient) CompleteAttempt(context.Context, string, contracts.CompleteAttemptRequest) (contracts.CompleteAttemptResponse, error) {
	c.completed++
	return contracts.CompleteAttemptResponse{
		AttemptRef: c.claim.AttemptRef,
		State:      "completed",
	}, nil
}

func (c *recoverableDropClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{}, nil
}

func (c *recoverableDropClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{}, nil
}

func (c *recoverableDropClient) Snapshot(context.Context) (contracts.SnapshotResponse, error) {
	return contracts.SnapshotResponse{}, nil
}

func (c *recoverableDropClient) RegisterEndpoint(context.Context, contracts.EndpointRegistrationRequest) (contracts.EndpointRegistrationResponse, error) {
	return contracts.EndpointRegistrationResponse{}, nil
}

func (c *recoverableDropClient) HeartbeatEndpoint(context.Context, string, contracts.EndpointHeartbeatRequest) (contracts.EndpointHeartbeatResponse, error) {
	return contracts.EndpointHeartbeatResponse{}, nil
}

func (c *recoverableDropClient) AdvertiseCapabilities(context.Context, string, contracts.CapabilityAdvertisementRequest) (contracts.CapabilityAdvertisementResponse, error) {
	return contracts.CapabilityAdvertisementResponse{}, nil
}

func (c *recoverableDropClient) ClaimMessage(context.Context, string, contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error) {
	return c.claim, nil
}

func (c *recoverableDropClient) ClaimNextMessage(context.Context, contracts.ClaimNextMessageRequest) (contracts.ClaimNextMessageResponse, error) {
	return contracts.ClaimNextMessageResponse{Claim: &c.claim}, nil
}

func (c *recoverableDropClient) SubmitCheckpoint(_ context.Context, _ string, _ contracts.CheckpointRequest) (contracts.CheckpointResponse, error) {
	c.checkpointCalls++
	if c.checkpointCalls == 1 {
		return contracts.CheckpointResponse{}, nil
	}
	return contracts.CheckpointResponse{}, contracts.APIError{
		StatusCode: 409,
		Response: contracts.ErrorResponse{
			Code:    "lease_expired",
			Message: "lease_expired",
		},
	}
}

func (c *recoverableDropClient) SubmitReadback(context.Context, string, contracts.ReadbackRequest) (contracts.ReadbackResponse, error) {
	return contracts.ReadbackResponse{}, nil
}

func (c *recoverableDropClient) RefreshLease(context.Context, string, contracts.RefreshLeaseRequest) (contracts.RefreshLeaseResponse, error) {
	return contracts.RefreshLeaseResponse{
		AttemptRef:     c.claim.AttemptRef,
		State:          "claimed",
		LeaseExpiresAt: time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano),
	}, nil
}

func (c *recoverableDropClient) RegisterArtifactVersion(context.Context, string, contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error) {
	return contracts.ArtifactVersionResponse{}, nil
}

func (c *recoverableDropClient) RecordApprovalDecision(context.Context, string, contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error) {
	return contracts.ApprovalDecisionResponse{}, nil
}

func (c *recoverableDropClient) StartSideEffectExecution(context.Context, string, contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error) {
	return contracts.SideEffectExecutionResponse{}, nil
}

func (c *recoverableDropClient) CompleteSideEffectExecution(context.Context, string, contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error) {
	return contracts.CompleteSideEffectExecutionResponse{}, nil
}

func (c *recoverableDropClient) CompleteAttempt(context.Context, string, contracts.CompleteAttemptRequest) (contracts.CompleteAttemptResponse, error) {
	return contracts.CompleteAttemptResponse{}, nil
}

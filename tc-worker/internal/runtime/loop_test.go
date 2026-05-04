package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

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

func (c *recoverableDropClient) Health(context.Context) (contracts.HealthResponse, error) {
	return contracts.HealthResponse{}, nil
}

func (c *recoverableDropClient) Version(context.Context) (contracts.VersionResponse, error) {
	return contracts.VersionResponse{}, nil
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

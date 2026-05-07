package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type ServerClient interface {
	Health(context.Context) (contracts.HealthResponse, error)
	Version(context.Context) (contracts.VersionResponse, error)
	Snapshot(context.Context) (contracts.SnapshotResponse, error)
	RegisterEndpoint(context.Context, contracts.EndpointRegistrationRequest) (contracts.EndpointRegistrationResponse, error)
	HeartbeatEndpoint(context.Context, string, contracts.EndpointHeartbeatRequest) (contracts.EndpointHeartbeatResponse, error)
	AdvertiseCapabilities(context.Context, string, contracts.CapabilityAdvertisementRequest) (contracts.CapabilityAdvertisementResponse, error)
	ClaimMessage(context.Context, string, contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error)
	ClaimNextMessage(context.Context, contracts.ClaimNextMessageRequest) (contracts.ClaimNextMessageResponse, error)
	SubmitCheckpoint(context.Context, string, contracts.CheckpointRequest) (contracts.CheckpointResponse, error)
	SubmitReadback(context.Context, string, contracts.ReadbackRequest) (contracts.ReadbackResponse, error)
	RefreshLease(context.Context, string, contracts.RefreshLeaseRequest) (contracts.RefreshLeaseResponse, error)
	RegisterArtifactVersion(context.Context, string, contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error)
	RecordApprovalDecision(context.Context, string, contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error)
	StartSideEffectExecution(context.Context, string, contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error)
	CompleteSideEffectExecution(context.Context, string, contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error)
	CompleteAttempt(context.Context, string, contracts.CompleteAttemptRequest) (contracts.CompleteAttemptResponse, error)
}

type Config struct {
	EndpointRef      string
	DisplayName      string
	ActorID          string
	WorkspaceID      string
	WorkerVersion    string
	Capabilities     []contracts.Capability
	ExecutionHints   []string
	ProgressInterval time.Duration
}

type Runtime struct {
	client                     ServerClient
	config                     Config
	executor                   WorkerExecutor
	artifactStore              ExecutionArtifactStore
	progressMu                 sync.Mutex
	currentAttemptRef          string
	lastActivityAt             time.Time
	progressSummary            string
	readbackSubmittedByAttempt map[string]bool
}

type MissingField struct {
	Name   string
	Reason string
}

func New(client ServerClient, config Config) *Runtime {
	return NewWithExecutor(client, config, EchoExecutor{})
}

func NewWithExecutor(client ServerClient, config Config, executor WorkerExecutor) *Runtime {
	return NewWithExecutorAndArtifacts(client, config, executor, nil)
}

func NewWithExecutorAndArtifacts(client ServerClient, config Config, executor WorkerExecutor, artifactStore ExecutionArtifactStore) *Runtime {
	if executor == nil {
		executor = EchoExecutor{}
	}
	return &Runtime{
		client:                     client,
		config:                     config,
		executor:                   executor,
		artifactStore:              artifactStore,
		readbackSubmittedByAttempt: map[string]bool{},
	}
}

func (r *Runtime) Register(ctx context.Context) error {
	if err := r.config.Validate(); err != nil {
		return err
	}
	if _, err := r.client.Health(ctx); err != nil {
		return err
	}
	version, err := r.client.Version(ctx)
	if err != nil {
		return err
	}
	if err := r.ensureCompatible(version); err != nil {
		return err
	}
	_, err = r.client.RegisterEndpoint(ctx, contracts.EndpointRegistrationRequest{
		EndpointRef:     r.config.EndpointRef,
		DisplayName:     r.config.DisplayName,
		ActorID:         r.config.ActorID,
		WorkspaceID:     r.config.WorkspaceID,
		ConnectionState: "online",
		Capabilities:    r.config.Capabilities,
		ExecutionHints:  r.config.ExecutionHints,
		WorkerVersion:   r.config.WorkerVersion,
		StartedAt:       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := r.Heartbeat(ctx); err != nil {
		return err
	}
	_, err = r.client.AdvertiseCapabilities(ctx, r.config.EndpointRef, contracts.CapabilityAdvertisementRequest{
		Capabilities: r.config.Capabilities,
	})
	return err
}

func (r *Runtime) Heartbeat(ctx context.Context) error {
	return r.sendHeartbeat(ctx, "online")
}

func (r *Runtime) sendHeartbeat(ctx context.Context, state string) error {
	attemptRef, lastActivityAt, progressSummary := r.progressSnapshot()
	_, err := r.client.HeartbeatEndpoint(ctx, r.config.EndpointRef, contracts.EndpointHeartbeatRequest{
		EndpointRef:       r.config.EndpointRef,
		ConnectionState:   state,
		ObservedAt:        time.Now().UTC().Format(time.RFC3339),
		CurrentAttemptRef: attemptRef,
		LastActivityAt:    formatOptionalWorkerTime(lastActivityAt),
		ProgressSummary:   progressSummary,
	})
	return err
}

func (r *Runtime) ProcessMessage(ctx context.Context, messageRef string) (string, error) {
	claim, err := r.claimAndAcknowledge(ctx, messageRef)
	if err != nil {
		return "", err
	}
	attemptRef, _, err := r.finishClaimAfterAck(ctx, claim)
	return attemptRef, err
}

func (r *Runtime) finishClaimAfterAck(ctx context.Context, claim contracts.ClaimMessageResponse) (attemptRef string, outcome string, err error) {
	lease, err := r.refreshLease(ctx, claim.AttemptRef)
	if err != nil {
		return "", "", err
	}
	r.markProgress(claim.AttemptRef, "executing message "+claim.MessageRef)
	defer r.clearProgress(claim.AttemptRef)
	executionCtx, cancelExecution := context.WithCancel(ctx)
	keeper := r.startLeaseKeeper(ctx, claim.AttemptRef, lease.LeaseExpiresAt, cancelExecution)
	defer func() {
		cancelExecution()
		if leaseErr := keeper.stop(); leaseErr != nil {
			if drop, ok := recoverableAttemptDrop(leaseErr); ok {
				attemptRef = claim.AttemptRef
				outcome = ExecutionOutcomeDropped
				err = drop
				return
			}
			attemptRef = claim.AttemptRef
			outcome = ExecutionOutcomeFailed
			err = leaseErr
		}
	}()
	input := r.executionInputForClaim(ctx, claim)
	progressReporter := r.startProgressReporter(executionCtx, claim)
	result, err := r.executeAndValidateClaim(executionCtx, ctx, claim, input)
	progressReporter.stop()
	if err != nil {
		return "", ExecutionOutcomeFailed, err
	}
	accepted, err := r.recordResultArtifactWhenNeeded(ctx, claim, result)
	if err != nil {
		return "", ExecutionOutcomeFailed, err
	}
	return r.finishAcceptedClaim(ctx, claim, accepted)
}

func (r *Runtime) executeAndValidateClaim(executionCtx context.Context, ctx context.Context, claim contracts.ClaimMessageResponse, input ExecutionInput) (ExecutionResult, error) {
	result, err := r.executor.Execute(executionCtx, input)
	if err != nil {
		r.recordAndFailClaim(ctx, claim, ExecutionResult{
			Outcome:           ExecutionOutcomeFailed,
			Summary:           "worker executor returned an error",
			FailureReasonCode: "executor_error",
		})
		return ExecutionResult{}, err
	}
	accepted, err := result.validated()
	if err != nil {
		r.recordAndFailClaim(ctx, claim, ExecutionResult{
			Outcome:           ExecutionOutcomeFailed,
			Summary:           "worker executor returned an invalid result",
			FailureReasonCode: "invalid_executor_result",
		})
		return ExecutionResult{}, err
	}
	return accepted, nil
}

func (r *Runtime) recordAndFailClaim(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) {
	recorded, err := r.recordExecutionArtifact(ctx, claim, result)
	if err == nil {
		result = recorded
	}
	_ = r.failClaim(ctx, claim, result)
}

func (r *Runtime) recordResultArtifactWhenNeeded(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) (ExecutionResult, error) {
	if result.Outcome == ExecutionOutcomeMissingFields {
		return result, nil
	}
	return r.recordExecutionArtifact(ctx, claim, result)
}

func (r *Runtime) finishAcceptedClaim(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) (string, string, error) {
	switch result.Outcome {
	case ExecutionOutcomeMissingFields:
		return claim.AttemptRef, ExecutionOutcomeMissingFields, r.blockClaimForMissingFields(ctx, claim, result.MissingFields)
	case ExecutionOutcomeFailed:
		return claim.AttemptRef, ExecutionOutcomeFailed, r.failClaim(ctx, claim, result)
	default:
		if isCanonicalScenario(claim) {
			return claim.AttemptRef, ExecutionOutcomeCompleted, r.completeCanonicalClaim(ctx, claim, result)
		}
		return claim.AttemptRef, ExecutionOutcomeCompleted, r.completeClaim(ctx, claim, result)
	}
}

func (r *Runtime) completeClaim(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) error {
	if err := r.submitReadbackWhenRequired(ctx, claim, nil); err != nil {
		return err
	}
	if _, err := r.client.SubmitCheckpoint(ctx, claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef:  r.config.EndpointRef,
		State:        "in_progress",
		Summary:      "processing message",
		ArtifactRefs: result.ArtifactRefs,
	}); err != nil {
		return err
	}
	if _, err := r.client.CompleteAttempt(ctx, claim.AttemptRef, contracts.CompleteAttemptRequest{
		EndpointRef:      r.config.EndpointRef,
		Summary:          result.Summary,
		ArtifactRefs:     result.ArtifactRefs,
		FollowUpMessages: result.FollowUpMessages,
	}); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) BlockMessageForMissingFields(ctx context.Context, messageRef string, fields []MissingField) (string, error) {
	claim, err := r.claimAndAcknowledge(ctx, messageRef)
	if err != nil {
		return "", err
	}
	return claim.AttemptRef, r.blockClaimForMissingFields(ctx, claim, fields)
}

func (r *Runtime) blockClaimForMissingFields(ctx context.Context, claim contracts.ClaimMessageResponse, fields []MissingField) error {
	if err := r.submitReadback(ctx, claim, fields); err != nil {
		return err
	}
	missingFields, missingReasons := splitMissingFields(fields)
	if _, err := r.client.SubmitCheckpoint(ctx, claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef:    r.config.EndpointRef,
		State:          "blocked_missing_fields",
		Summary:        "processing blocked because required information is missing",
		MissingFields:  missingFields,
		MissingReasons: missingReasons,
	}); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) failClaim(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) error {
	_, err := r.client.SubmitCheckpoint(ctx, claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef:       r.config.EndpointRef,
		State:             "failed",
		Summary:           result.Summary,
		ArtifactRefs:      result.ArtifactRefs,
		FailureReasonCode: result.FailureReasonCode,
	})
	return err
}

func (r *Runtime) RefreshLease(ctx context.Context, attemptRef string) error {
	_, err := r.refreshLease(ctx, attemptRef)
	return err
}

func (r *Runtime) refreshLease(ctx context.Context, attemptRef string) (contracts.RefreshLeaseResponse, error) {
	return r.client.RefreshLease(ctx, attemptRef, contracts.RefreshLeaseRequest{EndpointRef: r.config.EndpointRef})
}

func (r *Runtime) RegisterArtifactVersion(ctx context.Context, attemptRef string, req contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error) {
	req.EndpointRef = r.config.EndpointRef
	return r.client.RegisterArtifactVersion(ctx, attemptRef, req)
}

func (r *Runtime) RecordApprovalDecision(ctx context.Context, attemptRef string, req contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error) {
	return r.client.RecordApprovalDecision(ctx, attemptRef, req)
}

func (r *Runtime) StartSideEffectExecution(ctx context.Context, attemptRef string, req contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error) {
	req.EndpointRef = r.config.EndpointRef
	return r.client.StartSideEffectExecution(ctx, attemptRef, req)
}

func (r *Runtime) CompleteSideEffectExecution(ctx context.Context, executionRef string, req contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error) {
	req.EndpointRef = r.config.EndpointRef
	return r.client.CompleteSideEffectExecution(ctx, executionRef, req)
}

func (r *Runtime) SubmitCheckpoint(ctx context.Context, attemptRef string, state string, summary string, artifactRefs []string) error {
	r.markProgress(attemptRef, summary)
	_, err := r.client.SubmitCheckpoint(ctx, attemptRef, contracts.CheckpointRequest{
		EndpointRef:  r.config.EndpointRef,
		State:        state,
		Summary:      summary,
		ArtifactRefs: artifactRefs,
	})
	return err
}

func (r *Runtime) acknowledgeClaim(ctx context.Context, claim contracts.ClaimMessageResponse) error {
	r.markProgress(claim.AttemptRef, "message claimed")
	_, err := r.client.SubmitCheckpoint(ctx, claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef: r.config.EndpointRef,
		State:       "claimed",
		Summary:     "message claimed",
	})
	return err
}

func (r *Runtime) claimAndAcknowledge(ctx context.Context, messageRef string) (contracts.ClaimMessageResponse, error) {
	claim, err := r.client.ClaimMessage(ctx, messageRef, contracts.ClaimMessageRequest{
		EndpointRef: r.config.EndpointRef,
	})
	if err != nil {
		return contracts.ClaimMessageResponse{}, err
	}
	return claim, r.acknowledgeClaim(ctx, claim)
}

func (r *Runtime) submitReadbackWhenRequired(ctx context.Context, claim contracts.ClaimMessageResponse, fields []MissingField) error {
	if !claim.ReadbackRequired {
		return nil
	}
	if r.readbackAlreadySubmitted(claim.AttemptRef) {
		return nil
	}
	return r.submitReadback(ctx, claim, fields)
}

func (r *Runtime) submitReadback(ctx context.Context, claim contracts.ClaimMessageResponse, fields []MissingField) error {
	missingFields, missingReasons := splitMissingFields(fields)
	r.markProgress(claim.AttemptRef, "readback submitted")
	_, err := r.client.SubmitReadback(ctx, claim.AttemptRef, contracts.ReadbackRequest{
		EndpointRef:    r.config.EndpointRef,
		Summary:        "readback submitted before execution",
		Understanding:  "target capability=" + claim.TargetCapability + "; payload=" + claim.PayloadSummary,
		MissingFields:  missingFields,
		MissingReasons: missingReasons,
	})
	if err == nil {
		r.markReadbackSubmitted(claim.AttemptRef)
	}
	return err
}

func executionInputFromClaim(claim contracts.ClaimMessageResponse) ExecutionInput {
	return ExecutionInput{
		MessageRef:         claim.MessageRef,
		AttemptRef:         claim.AttemptRef,
		TargetCapability:   claim.TargetCapability,
		CorrelationRef:     claim.CorrelationRef,
		Payload:            claim.Payload,
		Constraints:        claim.Constraints,
		Takeover:           claim.Takeover,
		RedeliveryCount:    claim.RedeliveryCount,
		LastCheckpointRef:  claim.LastCheckpointRef,
		ResumeSummary:      claim.ResumeSummary,
		ResumeArtifactRefs: claim.ResumeArtifactRefs,
	}
}

func (r *Runtime) executionInputForClaim(ctx context.Context, claim contracts.ClaimMessageResponse) ExecutionInput {
	input := executionInputFromClaim(claim)
	input.Progress = r.progressCallback(ctx, claim)
	snapshot, err := r.client.Snapshot(ctx)
	if err != nil {
		return input
	}
	input.HandoffContext = handoffContextFromSnapshot(claim, snapshot)
	return input
}

func (r *Runtime) progressCallback(ctx context.Context, claim contracts.ClaimMessageResponse) func(ExecutionProgress) {
	return func(progress ExecutionProgress) {
		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			summary = strings.TrimSpace(progress.Line)
		}
		if summary == "" {
			return
		}
		summary = compactWorkerProgress(summary)
		r.markProgress(claim.AttemptRef, summary)
		if progress.Kind != "readback" || !claim.ReadbackRequired || r.readbackAlreadySubmitted(claim.AttemptRef) {
			return
		}
		_, err := r.client.SubmitReadback(ctx, claim.AttemptRef, contracts.ReadbackRequest{
			EndpointRef:   r.config.EndpointRef,
			Summary:       "readback marker observed during execution",
			Understanding: summary,
		})
		if err == nil {
			r.markReadbackSubmitted(claim.AttemptRef)
		}
	}
}

func (r *Runtime) recordExecutionArtifact(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) (ExecutionResult, error) {
	if r.artifactStore == nil {
		return result, nil
	}
	request, err := r.artifactStore.StoreExecutionLog(ctx, executionInputFromClaim(claim), result)
	if err != nil {
		return ExecutionResult{}, err
	}
	response, err := r.RegisterArtifactVersion(ctx, claim.AttemptRef, request)
	if err != nil {
		return ExecutionResult{}, err
	}
	result.ArtifactRefs = append(result.ArtifactRefs, response.ArtifactVersionRef)
	return result, nil
}

func (r *Runtime) ensureCompatible(version contracts.VersionResponse) error {
	if version.MinimumWorker == "" || version.MinimumWorker == r.config.WorkerVersion {
		return nil
	}
	return errors.New("worker version is below server minimum")
}

func (c Config) Validate() error {
	if blank(c.EndpointRef) || blank(c.DisplayName) || blank(c.ActorID) || blank(c.WorkspaceID) || blank(c.WorkerVersion) {
		return errors.New("worker identity settings are required")
	}
	if len(c.Capabilities) == 0 {
		return errors.New("worker capabilities are required")
	}
	if c.ProgressInterval < 0 {
		return errors.New("worker progress interval must not be negative")
	}
	seen := map[string]struct{}{}
	for _, capability := range c.Capabilities {
		if blank(capability.Name) {
			return errors.New("worker capability name is required")
		}
		if _, ok := seen[capability.Name]; ok {
			return errors.New("worker capability names must be unique")
		}
		seen[capability.Name] = struct{}{}
	}
	return nil
}

func splitMissingFields(fields []MissingField) ([]string, []string) {
	names := make([]string, 0, len(fields))
	reasons := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
		reasons = append(reasons, field.Reason)
	}
	return names, reasons
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

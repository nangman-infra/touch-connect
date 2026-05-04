package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const canonicalScenarioConstraint = "canonical_scenario"

type canonicalArtifactSpec struct {
	Name      string
	Kind      string
	MediaType string
	Body      string
}

func isCanonicalScenario(claim contracts.ClaimMessageResponse) bool {
	for _, constraint := range claim.Constraints {
		if constraint.Code == canonicalScenarioConstraint {
			return true
		}
	}
	return false
}

func (r *Runtime) completeCanonicalClaim(ctx context.Context, claim contracts.ClaimMessageResponse, result ExecutionResult) error {
	if err := r.submitReadbackWhenRequired(ctx, claim, nil); err != nil {
		return err
	}
	artifactRefs := append([]string(nil), result.ArtifactRefs...)
	created, err := r.registerCanonicalArtifacts(ctx, claim)
	if err != nil {
		return err
	}
	artifactRefs = append(artifactRefs, created...)
	if _, err := r.client.SubmitCheckpoint(ctx, claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef:  r.config.EndpointRef,
		State:        "in_progress",
		Summary:      "canonical scenario artifacts recorded",
		ArtifactRefs: artifactRefs,
	}); err != nil {
		return err
	}
	approvalRef, approvalHash, targetRef := canonicalApprovalRefs(claim)
	if _, err := r.RecordApprovalDecision(ctx, claim.AttemptRef, contracts.ApprovalDecisionRequest{
		ApprovalRef:             approvalRef,
		TargetType:              "side_effect",
		TargetRef:               targetRef,
		RequestedByActorID:      "actor.human_requester",
		ApproverSubjectsOrRoles: []string{"role.human_approver"},
		ApprovalScope:           "workspace",
		ApprovalHash:            approvalHash,
		Status:                  "approved",
		Reason:                  "canonical success run approval",
		DecidedByActorID:        "actor.human_approver",
		DecisionNote:            "approved for canonical scenario",
	}); err != nil {
		return err
	}
	started, err := r.StartSideEffectExecution(ctx, claim.AttemptRef, contracts.SideEffectExecutionRequest{
		IdempotencyKey:     "canonical:" + shortDigest(claim.AttemptRef),
		ProtectedScope:     "workspace:canonical-side-effect",
		ApprovalRef:        approvalRef,
		ApprovalHash:       approvalHash,
		TaskRef:            taskRefForClaim(claim),
		OperationKind:      "canonical_noop",
		ExternalTarget:     "local://canonical/protected-side-effect",
		RequestedByActorID: "actor.human_requester",
	})
	if err != nil {
		return err
	}
	if !started.Deduped {
		if _, err := r.CompleteSideEffectExecution(ctx, started.SideEffectExecutionRef, contracts.CompleteSideEffectExecutionRequest{
			Status:    "succeeded",
			ResultRef: lastArtifactRef(artifactRefs),
		}); err != nil {
			return err
		}
	}
	if _, err := r.client.CompleteAttempt(ctx, claim.AttemptRef, contracts.CompleteAttemptRequest{
		EndpointRef:  r.config.EndpointRef,
		Summary:      "canonical scenario completed",
		ArtifactRefs: artifactRefs,
	}); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) registerCanonicalArtifacts(ctx context.Context, claim contracts.ClaimMessageResponse) ([]string, error) {
	specs := []canonicalArtifactSpec{
		{
			Name:      "task_brief",
			Kind:      "document",
			MediaType: "text/markdown",
			Body:      "canonical task brief\n\n- handoff accepted\n- implementation and review evidence required\n",
		},
		{
			Name:      "validation_report",
			Kind:      "test_report",
			MediaType: "application/json",
			Body:      `{"sonarqube_quality_gate":"pass","tests":"pass","scenario":"canonical"}` + "\n",
		},
	}
	refs := make([]string, 0, len(specs))
	for i, spec := range specs {
		req := canonicalArtifactRequest(claim, spec, i+1, refs)
		registered, err := r.RegisterArtifactVersion(ctx, claim.AttemptRef, req)
		if err != nil {
			return nil, err
		}
		refs = append(refs, registered.ArtifactVersionRef)
	}
	return refs, nil
}

func canonicalArtifactRequest(claim contracts.ClaimMessageResponse, spec canonicalArtifactSpec, revision int, basedOn []string) contracts.ArtifactVersionRequest {
	body := []byte(spec.Body)
	digest := canonicalDigest(spec.Name + "|" + claim.AttemptRef + "|" + spec.Body)
	return contracts.ArtifactVersionRequest{
		ArtifactRef:                "tc://artifact/canonical-" + spec.Name + "_" + digest[:16],
		ArtifactVersionRef:         "tc://artifact-version/canonical-" + spec.Name + "_" + digest[:16],
		RoomRef:                    "tc://room/canonical",
		TaskRef:                    taskRefForClaim(claim),
		TaskRevision:               revision,
		Kind:                       spec.Kind,
		MediaType:                  spec.MediaType,
		SizeBytes:                  int64(len(body)),
		Checksum:                   "sha256:" + canonicalDigest(spec.Body),
		StorageRef:                 "memory://canonical/" + spec.Name + "/" + digest[:16],
		RetentionClass:             "audit",
		AccessScope:                "task",
		BasedOnMessageRefs:         []string{claim.MessageRef},
		BasedOnArtifactVersionRefs: append([]string(nil), basedOn...),
	}
}

func taskRefForClaim(claim contracts.ClaimMessageResponse) string {
	if strings.TrimSpace(claim.CorrelationRef) != "" {
		return claim.CorrelationRef
	}
	return "tc://task/canonical_" + shortDigest(claim.MessageRef)
}

func canonicalApprovalRefs(claim contracts.ClaimMessageResponse) (string, string, string) {
	digest := shortDigest(claim.AttemptRef)
	return "tc://approval/canonical_" + digest,
		"sha256:" + canonicalDigest("approval|"+claim.AttemptRef),
		"tc://protected-scope/canonical_" + digest
}

func lastArtifactRef(refs []string) string {
	if len(refs) == 0 {
		return "tc://artifact-version/canonical-empty"
	}
	return refs[len(refs)-1]
}

func canonicalDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

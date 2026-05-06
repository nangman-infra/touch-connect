package domain

import (
	"errors"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestValidateEndpointRegistration(t *testing.T) {
	valid := contracts.EndpointRegistrationRequest{
		EndpointRef:     "tc://endpoint/worker",
		DisplayName:     "Worker",
		ActorID:         "actor.worker",
		WorkspaceID:     "workspace.local",
		ConnectionState: EndpointStateOnline,
		Capabilities:    []contracts.Capability{{Name: "code.change"}},
	}
	if err := ValidateEndpointRegistration(valid); err != nil {
		t.Fatalf("valid registration rejected: %v", err)
	}
	valid.ConnectionState = ""
	if err := ValidateEndpointRegistration(valid); err != nil {
		t.Fatalf("empty connection state should be accepted: %v", err)
	}
	valid.ConnectionState = "bad"
	if !errors.Is(ValidateEndpointRegistration(valid), ErrInvalidInput) {
		t.Fatal("invalid connection state should be rejected")
	}
}

func TestValidateCapabilities(t *testing.T) {
	if err := ValidateCapabilities([]contracts.Capability{{Name: "a"}, {Name: "b"}}); err != nil {
		t.Fatalf("valid capabilities rejected: %v", err)
	}
	for _, caps := range [][]contracts.Capability{
		nil,
		{{Name: ""}},
		{{Name: "a"}, {Name: "a"}},
	} {
		if !errors.Is(ValidateCapabilities(caps), ErrInvalidInput) {
			t.Fatalf("capabilities %+v should be invalid", caps)
		}
	}
}

func TestValidateMessage(t *testing.T) {
	valid := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/sender",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "summary",
			Body:       "body",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{},
		QualityGate: contracts.QualityGateEnforce,
	}
	if err := ValidateMessage(valid); err != nil {
		t.Fatalf("valid message rejected: %v", err)
	}
	valid.QualityGate = "bad"
	if !errors.Is(ValidateMessage(valid), ErrInvalidInput) {
		t.Fatal("invalid quality gate should be rejected")
	}
	valid.QualityGate = ""
	valid.Payload.References = nil
	if !errors.Is(ValidateMessage(valid), ErrInvalidInput) {
		t.Fatal("nil references should be rejected")
	}
}

func TestValidateCheckpointReadbackAndCompletion(t *testing.T) {
	checkpoint := contracts.CheckpointRequest{EndpointRef: "tc://endpoint/worker", State: AttemptStateFailed, Summary: "failed"}
	if !errors.Is(ValidateCheckpoint(checkpoint), ErrInvalidInput) {
		t.Fatal("failed checkpoint without reason should be rejected")
	}
	checkpoint.FailureReasonCode = "command_failed"
	if err := ValidateCheckpoint(checkpoint); err != nil {
		t.Fatalf("failed checkpoint with reason rejected: %v", err)
	}
	checkpoint.State = AttemptStateBlockedMissingFields
	checkpoint.MissingFields = []string{"goal"}
	checkpoint.MissingReasons = []string{"missing"}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		t.Fatalf("blocked checkpoint with missing fields rejected: %v", err)
	}
	if err := ValidateReadback(contracts.ReadbackRequest{EndpointRef: "tc://endpoint/worker", Summary: "ok", Understanding: "understood"}); err != nil {
		t.Fatalf("valid readback rejected: %v", err)
	}
	if err := ValidateCompletion(contracts.CompleteAttemptRequest{EndpointRef: "tc://endpoint/worker", Summary: "done"}); err != nil {
		t.Fatalf("valid completion rejected: %v", err)
	}
}

func TestValidateArtifactApprovalAndSideEffects(t *testing.T) {
	artifact := contracts.ArtifactVersionRequest{
		EndpointRef:        "tc://endpoint/worker",
		ArtifactRef:        "tc://artifact/a",
		ArtifactVersionRef: "tc://artifact-version/a1",
		RoomRef:            "tc://room/r",
		TaskRef:            "tc://task/t",
		TaskRevision:       1,
		Kind:               "document",
		MediaType:          "text/plain",
		Checksum:           "sha256:abc",
		SizeBytes:          1,
		StorageRef:         "file:///tmp/a",
		RetentionClass:     "audit",
		AccessScope:        "task",
	}
	if err := ValidateArtifactVersion(artifact); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
	artifact.Kind = "bad"
	if !errors.Is(ValidateArtifactVersion(artifact), ErrInvalidInput) {
		t.Fatal("invalid artifact kind should be rejected")
	}

	approval := contracts.ApprovalDecisionRequest{
		ApprovalRef:             "tc://approval/a",
		TargetType:              "side_effect",
		TargetRef:               "tc://side-effect/s",
		RequestedByActorID:      "actor.requester",
		ApproverSubjectsOrRoles: []string{"role.admin"},
		ApprovalScope:           "task",
		ApprovalHash:            "hash",
		Status:                  ApprovalStatusApproved,
		DecidedByActorID:        "actor.approver",
	}
	if err := ValidateApprovalDecision(approval); err != nil {
		t.Fatalf("valid approval rejected: %v", err)
	}
	approval.DecidedByActorID = approval.RequestedByActorID
	if !errors.Is(ValidateApprovalDecision(approval), ErrSelfApproval) {
		t.Fatal("self approval should be rejected")
	}

	sideEffect := contracts.SideEffectExecutionRequest{
		EndpointRef:        "tc://endpoint/worker",
		IdempotencyKey:     "idem",
		ProtectedScope:     "task",
		ApprovalRef:        "tc://approval/a",
		ApprovalHash:       "hash",
		TaskRef:            "tc://task/t",
		OperationKind:      "shell",
		ExternalTarget:     "local",
		RequestedByActorID: "actor.worker",
	}
	if err := ValidateSideEffectExecution(sideEffect); err != nil {
		t.Fatalf("valid side effect rejected: %v", err)
	}
	if err := ValidateSideEffectCompletion(contracts.CompleteSideEffectExecutionRequest{EndpointRef: "tc://endpoint/worker", Status: SideEffectStatusSucceeded, ResultRef: "tc://result/r"}); err != nil {
		t.Fatalf("valid side effect completion rejected: %v", err)
	}
	if !errors.Is(ValidateSideEffectCompletion(contracts.CompleteSideEffectExecutionRequest{EndpointRef: "tc://endpoint/worker", Status: SideEffectStatusFailed}), ErrInvalidInput) {
		t.Fatal("failed side effect without reason should be rejected")
	}
}

func TestValidateArtifactVersionRejectsEachRequiredFieldAndEnum(t *testing.T) {
	valid := contracts.ArtifactVersionRequest{
		EndpointRef:        "tc://endpoint/worker",
		ArtifactRef:        "tc://artifact/a",
		ArtifactVersionRef: "tc://artifact-version/a1",
		RoomRef:            "tc://room/r",
		TaskRef:            "tc://task/t",
		TaskRevision:       1,
		Kind:               "document",
		MediaType:          "text/plain",
		Checksum:           "sha256:abc",
		SizeBytes:          1,
		StorageRef:         "file:///tmp/a",
		RetentionClass:     "audit",
		AccessScope:        "task",
	}

	cases := map[string]func(*contracts.ArtifactVersionRequest){
		"endpoint_ref":         func(req *contracts.ArtifactVersionRequest) { req.EndpointRef = "" },
		"artifact_ref":         func(req *contracts.ArtifactVersionRequest) { req.ArtifactRef = "" },
		"artifact_version_ref": func(req *contracts.ArtifactVersionRequest) { req.ArtifactVersionRef = "" },
		"room_ref":             func(req *contracts.ArtifactVersionRequest) { req.RoomRef = "" },
		"task_ref":             func(req *contracts.ArtifactVersionRequest) { req.TaskRef = "" },
		"kind":                 func(req *contracts.ArtifactVersionRequest) { req.Kind = "" },
		"media_type":           func(req *contracts.ArtifactVersionRequest) { req.MediaType = "" },
		"checksum":             func(req *contracts.ArtifactVersionRequest) { req.Checksum = "" },
		"storage_ref":          func(req *contracts.ArtifactVersionRequest) { req.StorageRef = "" },
		"retention_class":      func(req *contracts.ArtifactVersionRequest) { req.RetentionClass = "" },
		"access_scope":         func(req *contracts.ArtifactVersionRequest) { req.AccessScope = "" },
		"task_revision":        func(req *contracts.ArtifactVersionRequest) { req.TaskRevision = 0 },
		"size_bytes":           func(req *contracts.ArtifactVersionRequest) { req.SizeBytes = -1 },
		"bad_kind":             func(req *contracts.ArtifactVersionRequest) { req.Kind = "binary" },
		"bad_retention":        func(req *contracts.ArtifactVersionRequest) { req.RetentionClass = "forever" },
		"bad_access":           func(req *contracts.ArtifactVersionRequest) { req.AccessScope = "public" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			if !errors.Is(ValidateArtifactVersion(req), ErrInvalidInput) {
				t.Fatalf("%s should be invalid", name)
			}
		})
	}
}

func TestValidateApprovalDecisionRejectsMissingAndInvalidFields(t *testing.T) {
	valid := contracts.ApprovalDecisionRequest{
		ApprovalRef:             "tc://approval/a",
		TargetType:              "side_effect",
		TargetRef:               "tc://side-effect/s",
		RequestedByActorID:      "actor.requester",
		ApproverSubjectsOrRoles: []string{"role.admin"},
		ApprovalScope:           "task",
		ApprovalHash:            "hash",
		Status:                  ApprovalStatusPending,
	}
	if err := ValidateApprovalDecision(valid); err != nil {
		t.Fatalf("pending approval rejected: %v", err)
	}

	cases := map[string]func(*contracts.ApprovalDecisionRequest){
		"approval_ref": func(req *contracts.ApprovalDecisionRequest) { req.ApprovalRef = "" },
		"target_type":  func(req *contracts.ApprovalDecisionRequest) { req.TargetType = "" },
		"target_ref":   func(req *contracts.ApprovalDecisionRequest) { req.TargetRef = "" },
		"requester":    func(req *contracts.ApprovalDecisionRequest) { req.RequestedByActorID = "" },
		"scope":        func(req *contracts.ApprovalDecisionRequest) { req.ApprovalScope = "" },
		"hash":         func(req *contracts.ApprovalDecisionRequest) { req.ApprovalHash = "" },
		"status":       func(req *contracts.ApprovalDecisionRequest) { req.Status = "" },
		"approvers":    func(req *contracts.ApprovalDecisionRequest) { req.ApproverSubjectsOrRoles = nil },
		"bad_target":   func(req *contracts.ApprovalDecisionRequest) { req.TargetType = "unknown" },
		"bad_status":   func(req *contracts.ApprovalDecisionRequest) { req.Status = "unknown" },
		"no_decider": func(req *contracts.ApprovalDecisionRequest) {
			req.Status = ApprovalStatusApproved
			req.DecidedByActorID = ""
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			if !errors.Is(ValidateApprovalDecision(req), ErrInvalidInput) {
				t.Fatalf("%s should be invalid", name)
			}
		})
	}
}

func TestValidateSideEffectsRejectInvalidInputs(t *testing.T) {
	validExecution := contracts.SideEffectExecutionRequest{
		EndpointRef:        "tc://endpoint/worker",
		IdempotencyKey:     "idem",
		ProtectedScope:     "task",
		ApprovalRef:        "tc://approval/a",
		ApprovalHash:       "hash",
		TaskRef:            "tc://task/t",
		OperationKind:      "shell",
		ExternalTarget:     "local",
		RequestedByActorID: "actor.worker",
	}
	cases := map[string]func(*contracts.SideEffectExecutionRequest){
		"endpoint_ref":    func(req *contracts.SideEffectExecutionRequest) { req.EndpointRef = "" },
		"idempotency_key": func(req *contracts.SideEffectExecutionRequest) { req.IdempotencyKey = "" },
		"protected_scope": func(req *contracts.SideEffectExecutionRequest) { req.ProtectedScope = "" },
		"approval_ref":    func(req *contracts.SideEffectExecutionRequest) { req.ApprovalRef = "" },
		"approval_hash":   func(req *contracts.SideEffectExecutionRequest) { req.ApprovalHash = "" },
		"task_ref":        func(req *contracts.SideEffectExecutionRequest) { req.TaskRef = "" },
		"operation_kind":  func(req *contracts.SideEffectExecutionRequest) { req.OperationKind = "" },
		"external_target": func(req *contracts.SideEffectExecutionRequest) { req.ExternalTarget = "" },
		"requested_actor": func(req *contracts.SideEffectExecutionRequest) { req.RequestedByActorID = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validExecution
			mutate(&req)
			if !errors.Is(ValidateSideEffectExecution(req), ErrInvalidInput) {
				t.Fatalf("%s should be invalid", name)
			}
		})
	}

	completionCases := []contracts.CompleteSideEffectExecutionRequest{
		{EndpointRef: "", Status: SideEffectStatusSucceeded, ResultRef: "tc://result/r"},
		{EndpointRef: "tc://endpoint/worker", Status: SideEffectStatusSucceeded},
		{EndpointRef: "tc://endpoint/worker", Status: "unknown"},
	}
	for _, req := range completionCases {
		if !errors.Is(ValidateSideEffectCompletion(req), ErrInvalidInput) {
			t.Fatalf("completion %+v should be invalid", req)
		}
	}
	if err := ValidateSideEffectCompletion(contracts.CompleteSideEffectExecutionRequest{EndpointRef: "tc://endpoint/worker", Status: SideEffectStatusCanceled}); err != nil {
		t.Fatalf("canceled side effect completion rejected: %v", err)
	}
}

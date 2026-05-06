package a2a

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestMessageIngressRequestMapsA2AMessage(t *testing.T) {
	req := SendMessageRequest{
		Message: Message{
			MessageID: "a2a-msg-1",
			ContextID: "ctx-1",
			Role:      RoleUser,
			Parts: []Part{
				{Text: "Change the Go code", MediaType: "text/plain"},
				{URL: "https://example.test/evidence", Filename: "evidence.html", MediaType: "text/html"},
			},
			ReferenceTaskIDs: []string{"task-prior"},
			Metadata: map[string]any{
				"target_capability": "code.change",
				"summary":           "A2A handoff",
				"quality_gate":      "warn",
			},
		},
	}
	ingress, err := MessageIngressRequest(req)
	if err != nil {
		t.Fatalf("map ingress request: %v", err)
	}
	if ingress.TargetCapability != "code.change" || ingress.CorrelationRef != "ctx-1" || ingress.QualityGate != contracts.QualityGateWarn {
		t.Fatalf("unexpected ingress mapping: %+v", ingress)
	}
	if ingress.Payload.Summary != "A2A handoff" || ingress.Payload.Body == "" {
		t.Fatalf("expected payload summary and body, got %+v", ingress.Payload)
	}
	if len(ingress.Payload.References) != 2 {
		t.Fatalf("expected url and reference task refs, got %+v", ingress.Payload.References)
	}
}

func TestMessageIngressRequestRequiresCapability(t *testing.T) {
	_, err := MessageIngressRequest(SendMessageRequest{
		Message: Message{
			MessageID: "a2a-msg-1",
			Role:      RoleUser,
			Parts:     []Part{{Text: "body"}},
		},
	})
	if err != ErrCapabilityRequired {
		t.Fatalf("expected capability error, got %v", err)
	}
}

func TestTaskFromSnapshotMapsMessageStateAndArtifacts(t *testing.T) {
	snapshot := contracts.SnapshotResponse{
		Messages: []contracts.MessageRecord{{
			MessageRef:       "tc://message/msg_1",
			DeliveryRef:      "tc://delivery/dlv_1",
			TargetCapability: "code.change",
			Payload: contracts.Payload{
				Summary:    "done",
				Body:       "completed body",
				References: []contracts.Reference{},
			},
			CorrelationRef: "ctx-1",
			State:          "completed",
		}},
		Artifacts: []contracts.ArtifactRecord{{
			ArtifactRef:        "tc://artifact/art_1",
			ArtifactVersionRef: "tc://artifact-version/art_1_v1",
			MessageRef:         "tc://message/msg_1",
			TaskRef:            "ctx-1",
			Kind:               "code_patch",
			MediaType:          "text/plain",
			Checksum:           "sha256:test",
			StorageRef:         "memory://artifact",
		}},
	}
	task, ok := TaskFromSnapshot("ctx-1", snapshot)
	if !ok {
		t.Fatalf("expected task from snapshot")
	}
	if task.ID != "tc://message/msg_1" || task.Status.State != TaskStateCompleted || len(task.Artifacts) != 1 {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestJSONRPCResponseShapes(t *testing.T) {
	response := ResultResponse("1", SendMessageResponse{Task: &Task{ID: "tc://message/msg_1", Status: TaskStatus{State: TaskStateSubmitted}}})
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("expected valid json response")
	}
}

func TestJSONRPCValidationAndDecode(t *testing.T) {
	if err := ValidateJSONRPCRequest(JSONRPCRequest{JSONRPC: "2.0", Method: MethodGetTask}); err != nil {
		t.Fatalf("valid rpc rejected: %v", err)
	}
	if err := ValidateJSONRPCRequest(JSONRPCRequest{JSONRPC: "1.0", Method: MethodGetTask}); err != ErrInvalidRequest {
		t.Fatalf("expected invalid rpc error, got %v", err)
	}
	send, err := DecodeSendMessageRequest(json.RawMessage(`{"message":{"messageId":"m","role":"user","parts":[{"data":{"k":"v"}}]}}`))
	if err != nil || send.Message.MessageID != "m" {
		t.Fatalf("decode send failed: %+v err=%v", send, err)
	}
	if _, err := DecodeSendMessageRequest(nil); err != ErrInvalidParams {
		t.Fatalf("expected missing params error, got %v", err)
	}
	task, err := DecodeGetTaskRequest(json.RawMessage(`{"id":"tc://message/m"}`))
	if err != nil || task.ID != "tc://message/m" {
		t.Fatalf("decode task failed: %+v err=%v", task, err)
	}
	if _, err := DecodeGetTaskRequest(json.RawMessage(`{"id":""}`)); err != ErrInvalidParams {
		t.Fatalf("expected invalid task params, got %v", err)
	}
}

func TestA2AHelperMappings(t *testing.T) {
	if !VersionSupported("") || !VersionSupported("1.0") || !VersionSupported("1.1") || VersionSupported("2.0") {
		t.Fatal("unexpected version support mapping")
	}
	if got := ErrorCode(ErrInvalidParams); got != ErrorInvalidParams {
		t.Fatalf("invalid params mapped to %d", got)
	}
	if got := ErrorCode(ErrTaskNotFound); got != ErrorTaskNotFound {
		t.Fatalf("task not found mapped to %d", got)
	}
	if got := ErrorCode(ErrVersionNotSupported); got != ErrorVersionUnsupported {
		t.Fatalf("version unsupported mapped to %d", got)
	}
	if got := ErrorCode(ErrUnsupportedA2AMethod); got != ErrorMethodNotFound {
		t.Fatalf("unsupported method mapped to %d", got)
	}
	if got := ErrorCode(errors.New("unknown")); got != ErrorInternal {
		t.Fatalf("unknown error mapped to %d", got)
	}
	states := map[string]string{
		"available":          TaskStateSubmitted,
		"claimed":            TaskStateWorking,
		"takeover_candidate": TaskStateWorking,
		"completed":          TaskStateCompleted,
		"failed":             TaskStateFailed,
		"dead_lettered":      TaskStateFailed,
		"canceled":           TaskStateCanceled,
		"input_required":     TaskStateInputRequired,
		"other":              TaskStateUnknown,
	}
	for state, want := range states {
		if got := taskStateFromMessageState(state); got != want {
			t.Fatalf("state %q mapped to %q, want %q", state, got, want)
		}
	}
}

func TestA2APayloadMetadataAndSummaryHelpers(t *testing.T) {
	body, refs, err := partsToPayload(Message{
		Parts: []Part{
			{Data: map[string]any{"key": "value"}},
			{URL: "https://example.test/result", Filename: "result.txt"},
			{Text: "plain text"},
		},
		ReferenceTaskIDs: []string{"", "tc://message/msg_1"},
	})
	if err != nil {
		t.Fatalf("partsToPayload returned error: %v", err)
	}
	if body == "" || len(refs) != 2 {
		t.Fatalf("unexpected payload body=%q refs=%+v", body, refs)
	}
	if _, _, err := partsToPayload(Message{Parts: []Part{{}}}); !errors.Is(err, ErrMessagePartRequired) {
		t.Fatalf("empty message parts should require content, got %v", err)
	}

	metadata := map[string]any{
		"readback_required": "true",
		"custom_stringer":   stringerValue("custom"),
	}
	if !metadataBool(metadata, "readback_required") {
		t.Fatal("string true should parse as metadata bool")
	}
	if metadataBool(map[string]any{"readback_required": "false"}, "readback_required") {
		t.Fatal("string false should not parse as true")
	}
	if got := metadataString(metadata, "missing", "custom_stringer"); got != "custom" {
		t.Fatalf("metadataString stringer = %q", got)
	}

	longLine := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"
	if got := summaryFromMetadataOrBody(nil, longLine); len(got) != 120 {
		t.Fatalf("summary should be truncated to 120 chars, got %d", len(got))
	}
	if validClientRole("assistant") {
		t.Fatal("assistant should not be accepted as a client role")
	}
	if compactRef("tc://message/msg_1") != "message_msg_1" {
		t.Fatal("compactRef should remove tc scheme and replace separators")
	}
}

type stringerValue string

func (s stringerValue) String() string {
	return string(s)
}

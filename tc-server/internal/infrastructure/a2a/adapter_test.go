package a2a

import (
	"encoding/json"
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

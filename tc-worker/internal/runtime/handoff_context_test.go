package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestHandoffContextIncludesTaskMessagesAndArtifacts(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "execution.json")
	if err := os.WriteFile(logPath, []byte(`{"summary":"done","stdout":"ok","stderr":"hidden","outcome":"completed","used_skill_refs":["tc://skill/demo"]}`), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	rawPath := filepath.Join(dir, "raw.txt")
	if err := os.WriteFile(rawPath, []byte("raw artifact content"), 0o644); err != nil {
		t.Fatalf("write raw artifact: %v", err)
	}

	claim := contracts.ClaimMessageResponse{
		MessageRef:       "tc://message/current",
		CorrelationRef:   "tc://task/demo",
		TargetCapability: "code.change",
		Payload: contracts.Payload{
			References: []contracts.Reference{
				{Ref: "tc://message/direct", Type: "message"},
				{Ref: "tc://artifact-version/direct", Type: "artifact_version"},
			},
		},
	}
	context := handoffContextFromSnapshot(claim, contracts.SnapshotResponse{
		Messages: []contracts.MessageRecord{
			{MessageRef: "tc://message/current", State: "completed", CorrelationRef: "tc://task/demo"},
			{MessageRef: "tc://message/direct", State: "completed", TargetCapability: "ai.review", Payload: contracts.Payload{Summary: "direct", Body: "direct body"}},
			{MessageRef: "tc://message/task", State: "completed", CorrelationRef: "tc://task/demo", Payload: contracts.Payload{Summary: "task", Body: "task body"}},
			{MessageRef: "tc://message/open", State: "claimed", CorrelationRef: "tc://task/demo"},
		},
		Artifacts: []contracts.ArtifactRecord{
			{ArtifactVersionRef: "tc://artifact-version/by-message", MessageRef: "tc://message/task", StorageRef: "file://" + logPath},
			{ArtifactVersionRef: "tc://artifact-version/direct", StorageRef: "file://" + rawPath},
			{ArtifactVersionRef: "tc://artifact-version/ignored", MessageRef: "tc://message/other"},
		},
	})
	if context.TaskRef != "tc://task/demo" {
		t.Fatalf("unexpected task ref: %+v", context)
	}
	if len(context.Messages) != 2 {
		t.Fatalf("expected direct and task messages, got %+v", context.Messages)
	}
	if len(context.Artifacts) != 2 {
		t.Fatalf("expected two artifacts, got %+v", context.Artifacts)
	}
	if context.Artifacts[0].Summary != "done" || context.Artifacts[0].Stdout != "ok" || len(context.Artifacts[0].UsedSkillRefs) != 1 {
		t.Fatalf("expected parsed execution log artifact, got %+v", context.Artifacts[0])
	}
	if !strings.Contains(context.Artifacts[1].Content, "raw artifact") {
		t.Fatalf("expected raw artifact content, got %+v", context.Artifacts[1])
	}
}

func TestHandoffContextIncludesResumeArtifacts(t *testing.T) {
	dir := t.TempDir()
	partialPath := filepath.Join(dir, "partial.json")
	if err := os.WriteFile(partialPath, []byte(`{"summary":"partial work","stdout":"already changed files A and B","stderr":"timeout","outcome":"partial_completed"}`), 0o644); err != nil {
		t.Fatalf("write partial artifact: %v", err)
	}
	claim := contracts.ClaimMessageResponse{
		MessageRef:           "tc://message/current",
		CorrelationRef:       "tc://task/resume",
		Takeover:             true,
		ResumeSummary:        "previous attempt timed out after partial work",
		ResumeArtifactRefs:   []string{"tc://artifact-version/partial"},
		TargetCapability:     "code.change",
		RedeliveryCount:      1,
		LastCheckpointRef:    "tc://checkpoint/previous",
		ReadbackRequired:     true,
		TargetEndpointRef:    "",
		PreferredEndpointRef: "",
	}
	context := handoffContextFromSnapshot(claim, contracts.SnapshotResponse{
		Artifacts: []contracts.ArtifactRecord{
			{ArtifactVersionRef: "tc://artifact-version/partial", StorageRef: "file://" + partialPath},
			{ArtifactVersionRef: "tc://artifact-version/unrelated"},
		},
	})
	if len(context.Artifacts) != 1 {
		t.Fatalf("expected resume artifact, got %+v", context.Artifacts)
	}
	if context.Artifacts[0].Summary != "partial work" || !strings.Contains(context.Artifacts[0].Stdout, "already changed") {
		t.Fatalf("expected parsed resume artifact content, got %+v", context.Artifacts[0])
	}
}

func TestHandoffReferenceAndStorageHelpers(t *testing.T) {
	messages, artifacts := referencedHandoffRefs([]contracts.Reference{
		{Ref: " tc://message/one ", Type: ""},
		{Ref: "tc://artifact/two", Type: ""},
		{Ref: "tc://artifact-version/three", Type: "artifact_version_ref"},
		{Ref: "ignored", Type: "unknown"},
		{Ref: "", Type: "message"},
	})
	if _, ok := messages["tc://message/one"]; !ok {
		t.Fatalf("expected message ref detection: %+v", messages)
	}
	if _, ok := artifacts["tc://artifact/two"]; !ok {
		t.Fatalf("expected artifact ref detection: %+v", artifacts)
	}
	if _, ok := artifacts["tc://artifact-version/three"]; !ok {
		t.Fatalf("expected artifact version ref detection: %+v", artifacts)
	}
	if sameHandoffTask("", contracts.MessageRecord{CorrelationRef: "tc://task/demo"}) {
		t.Fatalf("empty task ref should not match")
	}
	if path, ok := localFilePathFromStorageRef("file:///tmp/demo.txt"); !ok || path != "/tmp/demo.txt" {
		t.Fatalf("expected file URL path, path=%s ok=%t", path, ok)
	}
	if _, ok := localFilePathFromStorageRef("relative.txt"); ok {
		t.Fatalf("relative path should not be accepted")
	}
	long := strings.Repeat("x", maxHandoffPromptFieldChars+5)
	if got := trimPromptField(long); !strings.Contains(got, "[truncated]") {
		t.Fatalf("expected trimmed prompt marker")
	}
}

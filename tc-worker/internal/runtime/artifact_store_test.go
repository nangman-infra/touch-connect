package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalArtifactStoreWritesExecutionLog(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalArtifactStore(LocalArtifactStoreOptions{
		Dir:            dir,
		RoomRef:        "tc://room/test",
		TaskRef:        "tc://task/test",
		TaskRevision:   3,
		RetentionClass: "debug",
		AccessScope:    "task",
	})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	request, err := store.StoreExecutionLog(context.Background(), ExecutionInput{
		MessageRef:        "tc://message/msg/with/slashes",
		AttemptRef:        "tc://attempt/att_test",
		TargetCapability:  "code.change",
		CorrelationRef:    "tc://task/correlation",
		ResumeArtifactRefs: []string{"tc://artifact-version/previous"},
	}, ExecutionResult{
		Outcome:           ExecutionOutcomeCompleted,
		Summary:           "WORKER_RESULT_READY",
		UsedSkillRefs:     []string{"tc://skill/local-ai-worker"},
		FailureReasonCode: "",
		Stdout:            "done",
		Stderr:            "debug",
		ExitCode:          0,
		DurationMS:        42,
	})
	if err != nil {
		t.Fatalf("store execution log: %v", err)
	}
	if request.TaskRef != "tc://task/test" || request.TaskRevision != 3 || request.RetentionClass != "debug" {
		t.Fatalf("unexpected artifact request: %+v", request)
	}
	if !strings.HasPrefix(request.ArtifactRef, "tc://artifact/execution-log_") || !strings.HasPrefix(request.ArtifactVersionRef, "tc://artifact-version/execution-log_") {
		t.Fatalf("unexpected artifact refs: %+v", request)
	}
	if request.Checksum == "" || request.SizeBytes == 0 || len(request.BasedOnMessageRefs) != 1 {
		t.Fatalf("missing checksum or lineage: %+v", request)
	}

	path := strings.TrimPrefix(request.StorageRef, "file://")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact body: %v", err)
	}
	var log ExecutionLogArtifact
	if err := json.Unmarshal(body, &log); err != nil {
		t.Fatalf("decode artifact body: %v", err)
	}
	if log.Summary != "WORKER_RESULT_READY" || log.Stdout != "done" || log.DurationMS != 42 {
		t.Fatalf("unexpected execution log: %+v", log)
	}
}

func TestLocalArtifactStoreDefaultsAndValidation(t *testing.T) {
	if _, err := NewLocalArtifactStore(LocalArtifactStoreOptions{Dir: "relative"}); err == nil {
		t.Fatalf("expected relative artifact dir to fail")
	}
	if _, err := NewLocalArtifactStore(LocalArtifactStoreOptions{Dir: t.TempDir(), TaskRevision: -1}); err == nil {
		t.Fatalf("expected negative revision to fail")
	}

	store, err := NewLocalArtifactStore(LocalArtifactStoreOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("new default store: %v", err)
	}
	request, err := store.StoreExecutionLog(context.Background(), ExecutionInput{
		MessageRef:     "tc://message/msg_default",
		AttemptRef:     "tc://attempt/att_default",
		CorrelationRef: "tc://task/from_correlation",
	}, ExecutionResult{Outcome: ExecutionOutcomeCompleted, Summary: "done"})
	if err != nil {
		t.Fatalf("store default log: %v", err)
	}
	if request.TaskRef != "tc://task/from_correlation" || request.RoomRef != "tc://room/worker-execution" || request.RetentionClass != "operational" {
		t.Fatalf("unexpected defaults: %+v", request)
	}

	if got := safePathPart(" tc://message/a b "); !strings.Contains(got, "tc___message_a_b") {
		t.Fatalf("unexpected safe path part: %q", got)
	}
	if got := executionLogFileName(ExecutionInput{MessageRef: "", AttemptRef: ""}); !strings.HasSuffix(filepath.Base(got), "__execution-log.json") {
		t.Fatalf("unexpected empty ref filename: %q", got)
	}
}

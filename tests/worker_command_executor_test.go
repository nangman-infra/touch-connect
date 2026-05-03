package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestCommandExecutorCapturesSuccessfulCommandOutput(t *testing.T) {
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	body := commandBody(t, tcworker.CommandRequest{Command: "echo", Args: []string{"hello"}})
	result, err := executor.Execute(context.Background(), commandInput(body))
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeCompleted || result.ExitCode != 0 || result.Stdout != "hello\n" {
		t.Fatalf("expected completed echo output, got %+v", result)
	}
}

func TestCommandExecutorBlocksMissingCommandRequest(t *testing.T) {
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	result, err := executor.Execute(context.Background(), commandInput(""))
	if err != nil {
		t.Fatalf("execute missing command: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeMissingFields || result.MissingFields[0].Name != "payload.body" {
		t.Fatalf("expected missing payload body result, got %+v", result)
	}
}

func TestCommandExecutorRejectsCommandOutsideAllowlist(t *testing.T) {
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	body := commandBody(t, tcworker.CommandRequest{Command: "pwd"})
	result, err := executor.Execute(context.Background(), commandInput(body))
	if err != nil {
		t.Fatalf("execute disallowed command: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeFailed || result.FailureReasonCode != "command_not_allowed" {
		t.Fatalf("expected command_not_allowed result, got %+v", result)
	}
}

func TestCommandExecutorCapturesNonZeroExit(t *testing.T) {
	executor := newTestCommandExecutor(t, []string{"false"}, 0)
	body := commandBody(t, tcworker.CommandRequest{Command: "false"})
	result, err := executor.Execute(context.Background(), commandInput(body))
	if err != nil {
		t.Fatalf("execute non-zero command: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeFailed || result.FailureReasonCode != "command_exit_nonzero" || result.ExitCode != 1 {
		t.Fatalf("expected command_exit_nonzero result, got %+v", result)
	}
}

func TestCommandExecutorTimesOutLongRunningCommand(t *testing.T) {
	executor := newTestCommandExecutor(t, []string{"sleep"}, 20*time.Millisecond)
	body := commandBody(t, tcworker.CommandRequest{Command: "sleep", Args: []string{"1"}})
	result, err := executor.Execute(context.Background(), commandInput(body))
	if err != nil {
		t.Fatalf("execute timeout command: %v", err)
	}
	if result.Outcome != tcworker.ExecutionOutcomeFailed || result.FailureReasonCode != "command_timeout" {
		t.Fatalf("expected command_timeout result, got %+v", result)
	}
}

func TestWorkerCommandExecutorCompletesMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	artifactStore := newTestArtifactStore(t)
	worker := tcworker.NewHTTPRuntimeWithExecutorAndArtifacts(
		httpServer.URL,
		httpServer.Client(),
		tcworker.DefaultConfig(),
		executor,
		artifactStore,
	)
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	ingressCommandMessage(t, httpServer, tcworker.CommandRequest{Command: "echo", Args: []string{"worker"}})

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process command message: %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected completed command message, got %+v", result)
	}
	if snapshot := server.Snapshot(); snapshot.Messages[0].State != "completed" {
		t.Fatalf("expected completed message, got %+v", snapshot.Messages[0])
	}
}

func TestWorkerCommandExecutorRegistersExecutionLogArtifact(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	artifactStore := newTestArtifactStore(t)
	worker := tcworker.NewHTTPRuntimeWithExecutorAndArtifacts(
		httpServer.URL,
		httpServer.Client(),
		tcworker.DefaultConfig(),
		executor,
		artifactStore,
	)
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	ingressCommandMessage(t, httpServer, tcworker.CommandRequest{Command: "echo", Args: []string{"artifact"}})

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process command with artifact store: %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected completed command result, got %+v", result)
	}
	snapshot := server.Snapshot()
	if len(snapshot.Artifacts) != 1 {
		t.Fatalf("expected one execution log artifact, got %+v", snapshot.Artifacts)
	}
	lastCheckpoint := snapshot.Checkpoints[len(snapshot.Checkpoints)-1]
	if lastCheckpoint.State != "completed" || len(lastCheckpoint.ArtifactRefs) != 1 {
		t.Fatalf("expected completed checkpoint with artifact ref, got %+v", lastCheckpoint)
	}
	if lastCheckpoint.ArtifactRefs[0] != snapshot.Artifacts[0].ArtifactVersionRef {
		t.Fatalf("expected checkpoint to reference artifact version, checkpoint=%+v artifact=%+v", lastCheckpoint, snapshot.Artifacts[0])
	}
	path := strings.TrimPrefix(snapshot.Artifacts[0].StorageRef, "file://")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read execution log artifact: %v", err)
	}
	if !strings.Contains(string(body), `"stdout": "artifact\n"`) {
		t.Fatalf("expected stdout in execution log artifact, got %s", string(body))
	}
}

func TestLocalArtifactStoreRejectsRelativeDir(t *testing.T) {
	_, err := tcworker.NewLocalArtifactStore(tcworker.LocalArtifactStoreOptions{Dir: "relative/artifacts"})
	if err == nil {
		t.Fatalf("expected relative artifact dir to be rejected")
	}
}

func TestWorkerCommandExecutorFailsDisallowedCommand(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	executor := newTestCommandExecutor(t, []string{"echo"}, 0)
	artifactStore := newTestArtifactStore(t)
	worker := tcworker.NewHTTPRuntimeWithExecutorAndArtifacts(
		httpServer.URL,
		httpServer.Client(),
		tcworker.DefaultConfig(),
		executor,
		artifactStore,
	)
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	ingressCommandMessage(t, httpServer, tcworker.CommandRequest{Command: "pwd"})

	result, err := worker.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process disallowed command message: %v", err)
	}
	if !result.Failed {
		t.Fatalf("expected failed command message, got %+v", result)
	}
	snapshot := server.Snapshot()
	if len(snapshot.Artifacts) != 1 {
		t.Fatalf("expected one failed execution log artifact, got %+v", snapshot.Artifacts)
	}
	lastCheckpoint := snapshot.Checkpoints[len(snapshot.Checkpoints)-1]
	if lastCheckpoint.State != "failed" || lastCheckpoint.FailureReasonCode != "command_not_allowed" || len(lastCheckpoint.ArtifactRefs) != 1 {
		t.Fatalf("expected command_not_allowed checkpoint, got %+v", lastCheckpoint)
	}
}

func newTestCommandExecutor(t *testing.T, allowed []string, timeout time.Duration) *tcworker.CommandExecutor {
	t.Helper()
	executor, err := tcworker.NewCommandExecutor(tcworker.CommandExecutorOptions{
		AllowedCommands: allowed,
		WorkDir:         t.TempDir(),
		Timeout:         timeout,
	})
	if err != nil {
		t.Fatalf("create command executor: %v", err)
	}
	return executor
}

func newTestArtifactStore(t *testing.T) *tcworker.LocalArtifactStore {
	t.Helper()
	store, err := tcworker.NewLocalArtifactStore(tcworker.LocalArtifactStoreOptions{
		Dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create local artifact store: %v", err)
	}
	return store
}

func commandInput(body string) tcworker.ExecutionInput {
	return tcworker.ExecutionInput{
		MessageRef:       "tc://message/msg_command_test",
		AttemptRef:       "tc://attempt/att_command_test",
		TargetCapability: "code.change",
		Payload: contracts.Payload{
			Summary:    "command test",
			Body:       body,
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{},
	}
}

func ingressCommandMessage(t *testing.T, server *httptest.Server, request tcworker.CommandRequest) contracts.MessageIngressResponse {
	t.Helper()
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/ep_local_worker",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "run command",
			Body:       commandBody(t, request),
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "command.allowlisted", Summary: "worker must enforce command allowlist"},
		},
	}
	var accepted contracts.MessageIngressResponse
	postJSON(t, server.URL+"/v1/messages", server.Client(), req, http.StatusAccepted, &accepted)
	return accepted
}

func commandBody(t *testing.T, request tcworker.CommandRequest) string {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal command request: %v", err)
	}
	return string(body)
}

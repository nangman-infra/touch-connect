package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	tccontrol "github.com/nangman-infra/touch-connect/tc-control"
	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
	tcctl "github.com/nangman-infra/touch-connect/tcctl"
)

func TestControlAPIExposesServerProjection(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var endpoints []contracts.EndpointRecord
	getJSON(t, controlHTTP.URL+"/v1/endpoints", controlHTTP.Client(), http.StatusOK, &endpoints)
	if len(endpoints) != 1 || endpoints[0].EndpointRef != tcworker.DefaultConfig().EndpointRef {
		t.Fatalf("expected worker endpoint through control projection, got %+v", endpoints)
	}
}

func TestTCCTLListsEndpointsAndSendsMessageThroughControl(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"endpoint", "list",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl endpoint list failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), tcworker.DefaultConfig().EndpointRef) {
		t.Fatalf("expected endpoint ref in CLI output, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"message", "send",
		"--capability", "code.change",
		"--summary", "CLI message",
		"--body", "message sent through tcctl and tc-control",
		"--task", "task.cli.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message send failed: %v stderr=%s", err, stderr.String())
	}
	var sent contracts.MessageIngressResponse
	if err := json.Unmarshal(stdout.Bytes(), &sent); err != nil {
		t.Fatalf("decode tcctl message send output: %v\n%s", err, stdout.String())
	}
	if sent.State != "available" || sent.MessageRef == "" {
		t.Fatalf("expected available message from tcctl send, got %+v", sent)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"message", "history",
		"--task", "task.cli.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message history failed: %v stderr=%s", err, stderr.String())
	}
	var messages []contracts.MessageRecord
	if err := json.Unmarshal(stdout.Bytes(), &messages); err != nil {
		t.Fatalf("decode tcctl message history output: %v\n%s", err, stdout.String())
	}
	if len(messages) != 1 || messages[0].MessageRef != sent.MessageRef {
		t.Fatalf("expected one task message in history, got %+v", messages)
	}
}

func TestTCCTLRejectsIncompatibleContractVersion(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--contract-version", "incompatible",
		"server", "health",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected incompatible contract error")
	}
	exitErr, ok := err.(tcctl.ExitError)
	if !ok || exitErr.Code != 2 {
		t.Fatalf("expected usage exit code for incompatible contract, got %#v", err)
	}
}

func TestTCCTLRecordsApprovalThroughControl(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, serverHTTP.URL, serverHTTP.Client(), false)
	claim := claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"approval", "approve", "approval.cli.1",
		"--attempt-ref", claim.AttemptRef,
		"--target-ref", "side-effect.cli.1",
		"--requested-by", "actor.requester",
		"--approvers", "role.approver",
		"--scope", "protected.side_effect",
		"--hash", "hash.cli.1",
		"--decided-by", "actor.approver",
		"--note", "approved by tcctl test",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl approval approve failed: %v stderr=%s", err, stderr.String())
	}
	var approved contracts.ApprovalDecisionResponse
	if err := json.Unmarshal(stdout.Bytes(), &approved); err != nil {
		t.Fatalf("decode approval output: %v\n%s", err, stdout.String())
	}
	if approved.Status != "approved" || approved.DecidedByActorID != "actor.approver" {
		t.Fatalf("expected approved decision, got %+v", approved)
	}
}

func TestTCCTLTaskCancelThroughControl(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"message", "send",
		"--capability", "code.change",
		"--summary", "cancel me",
		"--body", "this message should be canceled",
		"--task", "task.cancel.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message send failed: %v stderr=%s", err, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"task", "cancel", "task.cancel.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl task cancel failed: %v stderr=%s", err, stderr.String())
	}
	var canceled contracts.TaskCommandResponse
	if err := json.Unmarshal(stdout.Bytes(), &canceled); err != nil {
		t.Fatalf("decode cancel output: %v\n%s", err, stdout.String())
	}
	if canceled.State != "canceled" || canceled.AffectedMessages != 1 {
		t.Fatalf("expected one canceled message, got %+v", canceled)
	}
	if snapshot := server.Snapshot(); snapshot.Messages[0].State != "canceled" {
		t.Fatalf("expected canceled server message, got %+v", snapshot.Messages[0])
	}
}

func TestTCCTLTaskCreateCreatesInitialHandoffMessage(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"task", "create", "task.create.1",
		"--capability", "code.change",
		"--summary", "initial task handoff",
		"--body", "create task through tcctl",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl task create failed: %v stderr=%s", err, stderr.String())
	}
	var created contracts.MessageIngressResponse
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode task create output: %v\n%s", err, stdout.String())
	}
	if created.MessageRef == "" || created.State != "available" {
		t.Fatalf("expected task create to return available message, got %+v", created)
	}
	if snapshot := server.Snapshot(); len(snapshot.Messages) != 1 || snapshot.Messages[0].CorrelationRef != "task.create.1" {
		t.Fatalf("expected task correlation ref on server message, got %+v", snapshot.Messages)
	}
}

func TestTCCTLDLQReplayThroughControl(t *testing.T) {
	settings := tcserver.DefaultSettings()
	settings.AttemptLeaseDuration = 1
	settings.MaxRedelivery = 0
	server, err := tcserver.NewInMemoryServerWithSettings(settings)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	firstConfig := tcworker.DefaultConfig()
	secondConfig := tcworker.DefaultConfig()
	secondConfig.EndpointRef = "tc://endpoint/dlq_replay_second"
	secondConfig.ActorID = "actor.dlq.second"
	registerWorkers(t, serverHTTP, firstConfig, secondConfig)
	message := ingressMessage(t, serverHTTP.URL, serverHTTP.Client(), false)
	claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, firstConfig.EndpointRef, http.StatusAccepted)
	claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, secondConfig.EndpointRef, http.StatusConflict)
	if len(server.Snapshot().DeadLetters) != 1 {
		t.Fatalf("expected one dead letter, got %+v", server.Snapshot().DeadLetters)
	}
	deadLetterRef := server.Snapshot().DeadLetters[0].DeadLetterRef
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"dlq", "replay", deadLetterRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl dlq replay failed: %v stderr=%s", err, stderr.String())
	}
	var replayed contracts.DLQReplayResponse
	if err := json.Unmarshal(stdout.Bytes(), &replayed); err != nil {
		t.Fatalf("decode replay output: %v\n%s", err, stdout.String())
	}
	if replayed.State != "available" || replayed.MessageRef == "" || replayed.MessageRef == message.MessageRef {
		t.Fatalf("expected new available replay message, got %+v", replayed)
	}
}

func TestTCCTLArtifactFinalizeThroughControl(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, serverHTTP.URL, serverHTTP.Client(), false)
	claim := claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	artifact := artifactRequest("tc://artifact-version/finalize_cli_v1")
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, artifact); err != nil {
		t.Fatalf("register artifact version: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"artifact", "finalize", artifact.ArtifactVersionRef,
		"--actor", "actor.finalizer",
		"--reason", "ready for handoff",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl artifact finalize failed: %v stderr=%s", err, stderr.String())
	}
	var finalized contracts.ArtifactFinalizeResponse
	if err := json.Unmarshal(stdout.Bytes(), &finalized); err != nil {
		t.Fatalf("decode finalize output: %v\n%s", err, stdout.String())
	}
	if finalized.State != "finalized" || finalized.FinalizationRef == "" {
		t.Fatalf("expected finalized artifact response, got %+v", finalized)
	}
	if snapshot := server.Snapshot(); len(snapshot.Finalizations) != 1 {
		t.Fatalf("expected one finalization in server snapshot, got %+v", snapshot.Finalizations)
	}
}

func TestTCCTLCanonicalScenarioRunAndVerify(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	control, err := tccontrol.New(serverHTTP.URL, serverHTTP.Client(), "test-control")
	if err != nil {
		t.Fatalf("create control: %v", err)
	}
	controlHTTP := httptest.NewServer(control.Handler())
	defer controlHTTP.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"scenario", "run", "canonical",
		"--task", "task.scenario.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl scenario run failed: %v stderr=%s", err, stderr.String())
	}
	var sent contracts.MessageIngressResponse
	if err := json.Unmarshal(stdout.Bytes(), &sent); err != nil {
		t.Fatalf("decode scenario run output: %v\n%s", err, stdout.String())
	}
	if sent.MessageRef == "" {
		t.Fatalf("expected scenario run to create message, got %+v", sent)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"scenario", "verify", "canonical",
		"--task", "task.scenario.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl scenario verify failed: %v stderr=%s", err, stderr.String())
	}
	var report struct {
		Passed bool `json:"passed"`
		Checks []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode scenario verify output: %v\n%s", err, stdout.String())
	}
	if report.Passed || len(report.Checks) != 5 || !report.Checks[0].Passed {
		t.Fatalf("expected partial canonical verification failure with readback handoff present, got %+v", report)
	}
}

func getJSON(t *testing.T, url string, client *http.Client, status int, target any) {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != status {
		t.Fatalf("expected status %d, got %d", status, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

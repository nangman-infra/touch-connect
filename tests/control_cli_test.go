package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/bridge"
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

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"message", "inspect", sent.MessageRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message inspect json failed: %v stderr=%s", err, stderr.String())
	}
	var inspected contracts.MessageRecord
	if err := json.Unmarshal(stdout.Bytes(), &inspected); err != nil {
		t.Fatalf("decode tcctl message inspect output: %v\n%s", err, stdout.String())
	}
	if inspected.LatestQualityDecision == nil || inspected.LatestQualityDecision.QualityDecisionRef != sent.QualityDecisionRef {
		t.Fatalf("expected inspect to expose latest quality decision %q, got %+v", sent.QualityDecisionRef, inspected.LatestQualityDecision)
	}
	if len(inspected.QualityDecisions) != 1 {
		t.Fatalf("expected inspect to include one quality decision, got %+v", inspected.QualityDecisions)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"message", "inspect", sent.MessageRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message inspect failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "quality=") || !strings.Contains(stdout.String(), "quality_decision="+sent.QualityDecisionRef) {
		t.Fatalf("expected quality details in message inspect output, got %q", stdout.String())
	}
}

func TestTCCTLBodyFilePreservesMultilinePayload(t *testing.T) {
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

	body := "goal: preserve exact body text\nconstraints: keep quotes \"as-is\", backticks `ok`, and $VAR\nnext_action: worker should read this body\n"
	bodyFile := filepath.Join(t.TempDir(), "handoff.md")
	if err := os.WriteFile(bodyFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"message", "send",
		"--capability", "code.change",
		"--summary", "body file message",
		"--body-file", bodyFile,
		"--task", "task.body-file.1",
		"--readback-required",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message send with body-file failed: %v stderr=%s", err, stderr.String())
	}
	var sent contracts.MessageIngressResponse
	if err := json.Unmarshal(stdout.Bytes(), &sent); err != nil {
		t.Fatalf("decode message send output: %v\n%s", err, stdout.String())
	}
	snapshot := server.Snapshot()
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Payload.Body != body {
		t.Fatalf("expected message body to match file exactly, got %+v", snapshot.Messages)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"task", "create", "task.body-file.2",
		"--capability", "code.change",
		"--summary", "body file task",
		"--body-file", bodyFile,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl task create with body-file failed: %v stderr=%s", err, stderr.String())
	}
	snapshot = server.Snapshot()
	if len(snapshot.Messages) != 2 || snapshot.Messages[1].Payload.Body != body || snapshot.Messages[1].CorrelationRef != "task.body-file.2" {
		t.Fatalf("expected task create body to match file exactly, got %+v", snapshot.Messages)
	}
}

func TestTCCTLWatchAndTailExposeLiveFlow(t *testing.T) {
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
		"message", "send",
		"--capability", "code.change",
		"--summary", "watch flow",
		"--body", "message for watch flow",
		"--task", "tc://task/watch-flow",
		"--quality-gate", "skip",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message send failed: %v stderr=%s", err, stderr.String())
	}
	if _, err := worker.ProcessNext(context.Background()); err != nil {
		t.Fatalf("process message: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"task", "watch", "tc://task/watch-flow",
		"--once",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl task watch failed: %v stderr=%s", err, stderr.String())
	}
	watchOutput := stdout.String()
	for _, expected := range []string{"message ref=", "attempt ref=", "checkpoint ref=", "state=completed"} {
		if !strings.Contains(watchOutput, expected) {
			t.Fatalf("expected %q in task watch output, got %q", expected, watchOutput)
		}
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"message", "tail",
		"--capability", "code.change",
		"--once",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message tail failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cap=code.change") || !strings.Contains(stdout.String(), "state=completed") {
		t.Fatalf("expected message tail to include completed code.change flow, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"monitor",
		"--once",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl monitor failed: %v stderr=%s", err, stderr.String())
	}
	monitorOutput := stdout.String()
	for _, expected := range []string{"touch-connect monitor", "workers online=1", "messages total=1", "tasks total=1", "quality total=", "artifacts total="} {
		if !strings.Contains(monitorOutput, expected) {
			t.Fatalf("expected %q in monitor output, got %q", expected, monitorOutput)
		}
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"manager",
		"--task", "tc://task/watch-flow",
		"--once",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl manager failed: %v stderr=%s", err, stderr.String())
	}
	managerOutput := stdout.String()
	for _, expected := range []string{"touch-connect manager", "System task=tc://task/watch-flow", "Workers", "Tasks", "Timeline", "Next", "message completed", "checkpoint completed"} {
		if !strings.Contains(managerOutput, expected) {
			t.Fatalf("expected %q in manager output, got %q", expected, managerOutput)
		}
	}
}

func TestTCCTLManagerSendCreatesHandoffAndCockpit(t *testing.T) {
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

	body := "goal: manager cockpit should send from a body file\nnext_action: worker reads the manager task\n"
	bodyFile := filepath.Join(t.TempDir(), "manager-body.md")
	if err := os.WriteFile(bodyFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"manager",
		"--send",
		"--task", "tc://task/manager-cli-send",
		"--capability", "code.change",
		"--summary", "manager send",
		"--body-file", bodyFile,
		"--quality-gate", "skip",
		"--once",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl manager send failed: %v stderr=%s", err, stderr.String())
	}
	output := stdout.String()
	for _, expected := range []string{"sent message=", "task=tc://task/manager-cli-send", "touch-connect manager", "Workers", "Timeline", "manager send"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in manager send output, got %q", expected, output)
		}
	}
	snapshot := server.Snapshot()
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Payload.Body != body {
		t.Fatalf("expected manager send body to match file exactly, got %+v", snapshot.Messages)
	}
}

func TestTCCTLSkillRegisterListInspectUsesLocalRegistry(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, "review-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	skillBody := `---
skill_ref: tc://skill/review
name: Review Skill
kind: guidance
capabilities:
  - ai.review
---
# Review Skill

Read the previous AI output and check whether the handoff is auditable.
`
	if err := os.WriteFile(skillPath, []byte(skillBody), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	registryPath := filepath.Join(dir, "registry.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := tcctl.Run(context.Background(), []string{
		"--json",
		"skill", "register", skillPath,
		"--registry", registryPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl skill register failed: %v stderr=%s", err, stderr.String())
	}
	var registered contracts.SkillDefinition
	if err := json.Unmarshal(stdout.Bytes(), &registered); err != nil {
		t.Fatalf("decode registered skill: %v\n%s", err, stdout.String())
	}
	if registered.SkillRef != "tc://skill/review" || len(registered.Capabilities) != 1 {
		t.Fatalf("unexpected registered skill: %+v", registered)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--json",
		"skill", "list",
		"--registry", registryPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl skill list failed: %v stderr=%s", err, stderr.String())
	}
	var listed []contracts.SkillDefinition
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("decode skill list: %v\n%s", err, stdout.String())
	}
	if len(listed) != 1 || listed[0].SkillRef != registered.SkillRef {
		t.Fatalf("expected listed skill, got %+v", listed)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--json",
		"skill", "inspect", "tc://skill/review",
		"--registry", registryPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl skill inspect failed: %v stderr=%s", err, stderr.String())
	}
	var inspected contracts.SkillDefinition
	if err := json.Unmarshal(stdout.Bytes(), &inspected); err != nil {
		t.Fatalf("decode skill inspect: %v\n%s", err, stdout.String())
	}
	if inspected.Body == "" || !strings.Contains(inspected.Body, "handoff is auditable") {
		t.Fatalf("expected inspect to include skill body, got %+v", inspected)
	}
}

func TestTouchBrowserEvidenceFlowsIntoMessageReferenceAndQualityInspect(t *testing.T) {
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

	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/tcctl",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "browser-backed handoff",
			Body:       "use grounded evidence from the browser run",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{},
	}
	req, err = bridge.AttachTouchBrowserEvidence(req, bridge.TouchBrowserEvidence{
		EvidenceRef: "tc://evidence/browser-live-1",
		Title:       "Browser run evidence",
		URL:         "https://example.test/browser-run",
		SourceRisk:  contracts.SourceRiskHostile,
	})
	if err != nil {
		t.Fatalf("attach touch-browser evidence: %v", err)
	}

	var accepted contracts.MessageIngressResponse
	postJSON(t, controlHTTP.URL+"/v1/messages", controlHTTP.Client(), req, http.StatusAccepted, &accepted)

	var inspected contracts.MessageRecord
	getJSON(t, controlHTTP.URL+"/v1/messages/inspect?ref="+url.QueryEscape(accepted.MessageRef), controlHTTP.Client(), http.StatusOK, &inspected)
	if len(inspected.Payload.References) != 1 {
		t.Fatalf("expected one browser evidence reference, got %+v", inspected.Payload.References)
	}
	reference := inspected.Payload.References[0]
	if reference.Type != bridge.ReferenceTypeEvidence || reference.SourceRisk != contracts.SourceRiskHostile {
		t.Fatalf("expected hostile browser evidence source risk, got %+v", reference)
	}
	if inspected.LatestQualityDecision == nil || inspected.LatestQualityDecision.QualityDecisionRef != accepted.QualityDecisionRef {
		t.Fatalf("expected inspect to include quality decision %q, got %+v", accepted.QualityDecisionRef, inspected.LatestQualityDecision)
	}
}

func TestControlPreservesQualityRejectedEnvelope(t *testing.T) {
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

	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/tcctl",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "quality rejected",
			Body:       "message should fail the quality gate",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{},
		PhraseologyPolicy: &contracts.PhraseologyPolicy{
			PolicyRef:      "tc://quality-policy/rejecting",
			PolicyVersion:  "1",
			ScopeKind:      "task",
			RequiredFields: []string{"constraints"},
			FallbackAction: contracts.QualityFallbackReject,
			Severity:       contracts.QualitySeverityBlocking,
		},
	}
	var apiErr contracts.ErrorResponse
	postJSON(t, controlHTTP.URL+"/v1/messages", controlHTTP.Client(), req, http.StatusBadRequest, &apiErr)
	if apiErr.Code != contracts.ErrorCodeQualityRejected || apiErr.QualityDecision == nil {
		t.Fatalf("expected quality rejected envelope with decision, got %+v", apiErr)
	}
	if apiErr.QualityDecision.Decision != contracts.QualityDecisionRejected || apiErr.QualityDecision.QualityDecisionRef == "" {
		t.Fatalf("expected rejected quality decision details, got %+v", apiErr.QualityDecision)
	}
	if snapshot := server.Snapshot(); len(snapshot.Messages) != 0 || len(snapshot.QualityDecisions) != 1 {
		t.Fatalf("expected rejected message to record only quality decision, got %+v", snapshot)
	}
}

func TestTCCTLMessageSendQualityGateSkipRecordsSkippedDecision(t *testing.T) {
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
		"--summary", "skip quality gate",
		"--body", "replace the prior artifact with this version",
		"--quality-gate", "skip",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl message send with quality skip failed: %v stderr=%s", err, stderr.String())
	}
	var sent contracts.MessageIngressResponse
	if err := json.Unmarshal(stdout.Bytes(), &sent); err != nil {
		t.Fatalf("decode tcctl message send output: %v\n%s", err, stdout.String())
	}
	snapshot := server.Snapshot()
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].MessageRef != sent.MessageRef {
		t.Fatalf("expected skipped gate message to be dispatched, got %+v", snapshot.Messages)
	}
	if len(snapshot.QualityDecisions) != 1 || snapshot.QualityDecisions[0].Decision != contracts.QualityDecisionSkipped {
		t.Fatalf("expected skipped quality decision, got %+v", snapshot.QualityDecisions)
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

func TestTCCTLCommandHelpDoesNotRequireControl(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := tcctl.Run(context.Background(), []string{"message", "send", "-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("tcctl message send help failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "usage: tcctl message send") || !strings.Contains(stderr.String(), "-capability") || !strings.Contains(stderr.String(), "-quality-gate") || !strings.Contains(stderr.String(), "-body-file") {
		t.Fatalf("expected message send help in stderr, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := tcctl.Run(context.Background(), []string{"help", "task", "create"}, &stdout, &stderr); err != nil {
		t.Fatalf("tcctl help task create failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "usage: tcctl task create") || !strings.Contains(stderr.String(), "-summary") || !strings.Contains(stderr.String(), "-body-file") {
		t.Fatalf("expected task create help in stderr, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := tcctl.Run(context.Background(), []string{"manager", "-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("tcctl manager help failed: %v", err)
	}
	if !strings.Contains(stderr.String(), "usage: tcctl manager") || !strings.Contains(stderr.String(), "-send") || !strings.Contains(stderr.String(), "-body-file") {
		t.Fatalf("expected manager help in stderr, stdout=%q stderr=%q", stdout.String(), stderr.String())
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

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"--json",
		"approval", "chain", claim.AttemptRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl approval chain failed: %v stderr=%s", err, stderr.String())
	}
	var chain contracts.ApprovalChain
	if err := json.Unmarshal(stdout.Bytes(), &chain); err != nil {
		t.Fatalf("decode approval chain output: %v\n%s", err, stdout.String())
	}
	if chain.Current == nil || chain.Current.ApprovalRef != "approval.cli.1" || len(chain.Decisions) != 1 {
		t.Fatalf("expected approval chain to expose current decision, got %+v", chain)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"approval", "chain", "side-effect.cli.1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl approval chain text failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "chain=") || !strings.Contains(stdout.String(), "decision\tapproval.cli.1\tapproved") {
		t.Fatalf("expected approval chain details in output, got %q", stdout.String())
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

func TestTCCTLArtifactLineageThroughControl(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, serverHTTP.URL, serverHTTP.Client(), false)
	claim := claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	parent := artifactRequest("tc://artifact-version/lineage_cli_v1")
	parent.ArtifactRef = "tc://artifact/lineage_cli"
	parent.BasedOnMessageRefs = []string{message.MessageRef}
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, parent); err != nil {
		t.Fatalf("register parent artifact version: %v", err)
	}
	child := artifactRequest("tc://artifact-version/lineage_cli_v2")
	child.ArtifactRef = parent.ArtifactRef
	child.BasedOnArtifactVersionRefs = []string{parent.ArtifactVersionRef}
	if _, err := worker.RegisterArtifactVersion(context.Background(), claim.AttemptRef, child); err != nil {
		t.Fatalf("register child artifact version: %v", err)
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
		"artifact", "lineage", parent.ArtifactVersionRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl artifact lineage failed: %v stderr=%s", err, stderr.String())
	}
	var lineage contracts.ArtifactLineage
	if err := json.Unmarshal(stdout.Bytes(), &lineage); err != nil {
		t.Fatalf("decode artifact lineage output: %v\n%s", err, stdout.String())
	}
	if lineage.CurrentVersionRef != child.ArtifactVersionRef || len(lineage.Versions) != 2 {
		t.Fatalf("expected lineage to include parent and child, got %+v", lineage)
	}
	if !lineageHasEdge(lineage.Edges, parent.ArtifactVersionRef, child.ArtifactVersionRef, "derived_from") {
		t.Fatalf("expected derived_from edge, got %+v", lineage.Edges)
	}

	stdout.Reset()
	stderr.Reset()
	err = tcctl.Run(context.Background(), []string{
		"--control-url", controlHTTP.URL,
		"artifact", "lineage", parent.ArtifactRef,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl artifact lineage text failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "lineage=") || !strings.Contains(stdout.String(), "edge\tderived_from\t"+parent.ArtifactVersionRef+"\t"+child.ArtifactVersionRef) {
		t.Fatalf("expected artifact lineage details in output, got %q", stdout.String())
	}
}

func lineageHasEdge(edges []contracts.ArtifactLineageEdge, from string, to string, relation string) bool {
	for _, edge := range edges {
		if edge.FromRef == from && edge.ToRef == to && edge.Relation == relation {
			return true
		}
	}
	return false
}

func TestTCCTLCanonicalScenarioRunAndVerify(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	workerDone := make(chan error, 1)
	go func() {
		for {
			result, err := worker.ProcessNext(context.Background())
			if err != nil {
				workerDone <- err
				return
			}
			if !result.Empty {
				workerDone <- nil
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
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
		"--wait-timeout", "2s",
		"--poll-interval", "10ms",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("tcctl scenario run failed: %v stderr=%s", err, stderr.String())
	}
	var runReport struct {
		Message contracts.MessageIngressResponse `json:"message"`
		Passed  bool                             `json:"passed"`
		Checks  []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &runReport); err != nil {
		t.Fatalf("decode scenario run output: %v\n%s", err, stdout.String())
	}
	if runReport.Message.MessageRef == "" || !runReport.Passed {
		t.Fatalf("expected scenario run to pass with message, got %+v", runReport)
	}
	if err := <-workerDone; err != nil {
		t.Fatalf("worker process next failed: %v", err)
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
	if !report.Passed || len(report.Checks) != 5 {
		t.Fatalf("expected complete canonical verification success, got %+v", report)
	}
	for _, check := range report.Checks {
		if !check.Passed {
			t.Fatalf("expected all canonical checks to pass, got %+v", report)
		}
	}
}

func TestInMemoryRefsUsePerKindSequences(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	serverHTTP := httptest.NewServer(server.Handler())
	defer serverHTTP.Close()
	worker := tcworker.NewHTTPRuntime(serverHTTP.URL, serverHTTP.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	message := ingressMessage(t, serverHTTP.URL, serverHTTP.Client(), false)
	claim := claimMessage(t, serverHTTP.URL, serverHTTP.Client(), message.MessageRef, tcworker.DefaultConfig().EndpointRef, http.StatusAccepted)
	if !strings.HasSuffix(message.MessageRef, "msg_000001") || !strings.HasSuffix(message.DeliveryRef, "dlv_000001") || !strings.HasSuffix(claim.AttemptRef, "att_000001") {
		t.Fatalf("expected per-kind ref sequences, message=%+v claim=%+v", message, claim)
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

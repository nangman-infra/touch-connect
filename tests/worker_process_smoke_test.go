package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestWorkerCLILoopProcessesMessageFromServerProcess(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "process-smoke.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	workerOutput := new(bytes.Buffer)
	workerCmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	workerCmd.Dir = root
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	workerCmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
	)
	workerCmd.Stdout = workerOutput
	workerCmd.Stderr = workerOutput
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	ingressMessageEventually(t, serverURL, workerOutput)
	if err := workerCmd.Wait(); err != nil {
		t.Fatalf("worker process failed: %v\n%s", err, workerOutput.String())
	}
}

func TestWorkerCLICommandExecutorProcessesMessageFromServerProcess(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "command-process-smoke.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	workerOutput := new(bytes.Buffer)
	workerCmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	workerCmd.Dir = root
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	workerCmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
		"TC_WORKER_ALLOWED_COMMANDS=echo",
		"TC_WORKER_WORKDIR="+root,
		"TC_WORKER_COMMAND_TIMEOUT=2s",
		"TC_WORKER_ARTIFACT_DIR="+t.TempDir(),
	)
	workerCmd.Stdout = workerOutput
	workerCmd.Stderr = workerOutput
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	postCommandIngressMessageEventually(t, serverURL, workerOutput)
	if err := workerCmd.Wait(); err != nil {
		t.Fatalf("worker process failed: %v\n%s", err, workerOutput.String())
	}
}

func TestWorkerCLILLMExecutorProcessesMessageFromServerProcess(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	providerCalled := make(chan struct{}, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case providerCalled <- struct{}{}:
		default:
		}
		writeOpenAIResponsesText(w, "LLM worker completed process smoke")
	}))
	defer provider.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "llm-process-smoke.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	workerOutput := new(bytes.Buffer)
	workerCmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	workerCmd.Dir = root
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	workerCmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
		"TC_WORKER_EXECUTOR=llm",
		"TC_WORKER_CAPABILITIES=ai.generate",
		"TC_WORKER_LLM_BASE_URL="+provider.URL,
		"TC_WORKER_LLM_API_KEY=test-key",
		"TC_WORKER_LLM_MODEL=test-model",
		"TC_WORKER_LLM_TIMEOUT=2s",
	)
	workerCmd.Stdout = workerOutput
	workerCmd.Stderr = workerOutput
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	postLLMIngressMessageEventually(t, serverURL, workerOutput)
	if err := workerCmd.Wait(); err != nil {
		t.Fatalf("worker process failed: %v\n%s", err, workerOutput.String())
	}
	select {
	case <-providerCalled:
	default:
		t.Fatalf("expected LLM provider to be called\nworker output:\n%s", workerOutput.String())
	}
}

func TestWorkerCLISkillExecutorProcessesMessageFromServerProcess(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	skillsDir := filepath.Join(t.TempDir(), "skills")
	skillPath := filepath.Join(skillsDir, "ai-generate", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(`---
skill_ref: tc://skill/ai-generate
name: AI Generate
kind: guidance
capabilities:
  - ai.generate
---
# AI Generate

Read the handoff request, preserve user intent, and produce an auditable completion summary.
`), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	providerCalled := make(chan struct{}, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		input, _ := request["input"].(string)
		if !strings.Contains(input, "SKILL.md instructions") || !strings.Contains(input, "preserve user intent") {
			t.Fatalf("expected provider input to include skill guidance, got %q", input)
		}
		select {
		case providerCalled <- struct{}{}:
		default:
		}
		writeOpenAIResponsesText(w, "skill-guided AI completed process smoke")
	}))
	defer provider.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "skill-process-smoke.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	workerOutput := new(bytes.Buffer)
	workerCmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	workerCmd.Dir = root
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	workerCmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
		"TC_WORKER_EXECUTOR=skill",
		"TC_WORKER_SKILL_BACKEND=llm",
		"TC_WORKER_SKILLS_DIR="+skillsDir,
		"TC_WORKER_LLM_BASE_URL="+provider.URL,
		"TC_WORKER_LLM_API_KEY=test-key",
		"TC_WORKER_LLM_MODEL=test-model",
		"TC_WORKER_LLM_TIMEOUT=2s",
	)
	workerCmd.Stdout = workerOutput
	workerCmd.Stderr = workerOutput
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	postLLMIngressMessageEventually(t, serverURL, workerOutput)
	if err := workerCmd.Wait(); err != nil {
		t.Fatalf("worker process failed: %v\n%s", err, workerOutput.String())
	}
	select {
	case <-providerCalled:
	default:
		t.Fatalf("expected skill-guided provider to be called\nworker output:\n%s", workerOutput.String())
	}
}

func TestWorkerCLISkillExecutorUsesLocalAICLIProcess(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	skillsDir := filepath.Join(t.TempDir(), "skills")
	skillPath := filepath.Join(skillsDir, "ai-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(`---
skill_ref: tc://skill/ai-review
name: AI Review
kind: guidance
capabilities:
  - ai.review
---
# AI Review

Read the handoff, verify intent preservation, and produce an auditable review.
`), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	aiCLIPath := filepath.Join(t.TempDir(), "fake-ai-cli.sh")
	if err := os.WriteFile(aiCLIPath, []byte(`#!/bin/sh
set -eu
input=$(cat)
printf '%s' "$input" | grep -q "tc://skill/ai-review" || { echo "missing skill guidance" >&2; exit 9; }
printf '%s' "$input" | grep -q "Original message body" || { echo "missing original message body" >&2; exit 10; }
echo "skill AI CLI reviewed handoff"
`), 0o700); err != nil {
		t.Fatalf("write fake AI CLI: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "skill-ai-cli-process-smoke.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()
	workerOutput := new(bytes.Buffer)
	workerCmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	workerCmd.Dir = root
	workerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	workerCmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
		"TC_WORKER_EXECUTOR=skill",
		"TC_WORKER_SKILL_BACKEND=ai-cli",
		"TC_WORKER_SKILLS_DIR="+skillsDir,
		"TC_WORKER_AI_CLI_COMMAND="+aiCLIPath,
		"TC_WORKER_AI_CLI_TIMEOUT=2s",
	)
	workerCmd.Stdout = workerOutput
	workerCmd.Stderr = workerOutput
	if err := workerCmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	postReviewIngressMessageEventually(t, serverURL, workerOutput)
	if err := workerCmd.Wait(); err != nil {
		t.Fatalf("worker process failed: %v\n%s", err, workerOutput.String())
	}
}

func TestWorkerCLISkillExecutorsSharePriorAIHandoffContext(t *testing.T) {
	root := repositoryRoot(t)
	addr := freeLocalAddress(t)
	serverURL := "http://" + addr
	skillsDir := filepath.Join(t.TempDir(), "skills")
	writeProcessSkill(t, skillsDir, "ai-generate", `---
skill_ref: tc://skill/ai-generate
name: AI Generate
kind: guidance
capabilities:
  - ai.generate
---
# AI Generate

Produce an auditable implementation handoff for the next AI reviewer.
`)
	writeProcessSkill(t, skillsDir, "ai-review", `---
skill_ref: tc://skill/ai-review
name: AI Review
kind: guidance
capabilities:
  - ai.review
---
# AI Review

Review the previous AI worker output before deciding whether the handoff is auditable.
`)
	artifactDir := filepath.Join(t.TempDir(), "artifacts")
	implementerCLI := filepath.Join(t.TempDir(), "fake-implementer-ai-cli.sh")
	if err := os.WriteFile(implementerCLI, []byte(`#!/bin/sh
set -eu
input=$(cat)
printf '%s' "$input" | grep -q "tc://skill/ai-generate" || { echo "missing implementer skill" >&2; exit 21; }
echo "IMPLEMENTER_OUTPUT_OK: wrote auditable handoff for reviewer"
`), 0o700); err != nil {
		t.Fatalf("write implementer AI CLI: %v", err)
	}
	reviewerCLI := filepath.Join(t.TempDir(), "fake-reviewer-ai-cli.sh")
	if err := os.WriteFile(reviewerCLI, []byte(`#!/bin/sh
set -eu
input=$(cat)
printf '%s' "$input" | grep -q "tc://skill/ai-review" || { echo "missing reviewer skill" >&2; exit 31; }
printf '%s' "$input" | grep -q "handoff_context:" || { echo "missing handoff context" >&2; exit 32; }
printf '%s' "$input" | grep -q "IMPLEMENTER_OUTPUT_OK" || { echo "missing prior AI output" >&2; exit 33; }
echo "REVIEWER_OUTPUT_OK: prior AI handoff context was available"
`), 0o700); err != nil {
		t.Fatalf("write reviewer AI CLI: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverOutput := new(bytes.Buffer)
	serverCmd := exec.CommandContext(ctx, "go", "run", "./tc-server/cmd/tc-server")
	serverCmd.Dir = root
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	serverCmd.Env = append(os.Environ(),
		"TC_SERVER_BIND_ADDR="+addr,
		"TC_SERVER_STORAGE=sqlite",
		"TC_SERVER_SQLITE_PATH="+filepath.Join(t.TempDir(), "skill-ai-cli-handoff.db"),
	)
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start server process: %v", err)
	}
	defer waitForCanceledProcess(serverCmd)
	waitForHealth(t, serverURL+"/healthz", serverOutput)

	implementerOutput := new(bytes.Buffer)
	implementerCmd := skillWorkerProcess(t, root, serverURL, skillsDir, artifactDir, implementerCLI, "tc://endpoint/ai_cli_implementer", "ai.generate", implementerOutput)
	if err := implementerCmd.Start(); err != nil {
		t.Fatalf("start implementer worker: %v", err)
	}
	postSkillHandoffMessageEventually(t, serverURL, "ai.generate", "tc://task/skill_ai_cli_tikitaka", "Create an implementation handoff for the reviewer.", implementerOutput)
	if err := implementerCmd.Wait(); err != nil {
		t.Fatalf("implementer worker failed: %v\n%s", err, implementerOutput.String())
	}

	reviewerOutput := new(bytes.Buffer)
	reviewerCmd := skillWorkerProcess(t, root, serverURL, skillsDir, artifactDir, reviewerCLI, "tc://endpoint/ai_cli_reviewer", "ai.review", reviewerOutput)
	if err := reviewerCmd.Start(); err != nil {
		t.Fatalf("start reviewer worker: %v", err)
	}
	postSkillHandoffMessageEventually(t, serverURL, "ai.review", "tc://task/skill_ai_cli_tikitaka", "Review the previous AI output and confirm handoff quality.", reviewerOutput)
	if err := reviewerCmd.Wait(); err != nil {
		t.Fatalf("reviewer worker failed: %v\n%s", err, reviewerOutput.String())
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Dir(wd)
}

func writeProcessSkill(t *testing.T, skillsDir string, name string, body string) {
	t.Helper()
	skillPath := filepath.Join(skillsDir, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func skillWorkerProcess(t *testing.T, root string, serverURL string, skillsDir string, artifactDir string, aiCLIPath string, endpointRef string, capability string, output *bytes.Buffer) *exec.Cmd {
	t.Helper()
	workerCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(workerCancel)
	cmd := exec.CommandContext(workerCtx, "go", "run", "./tc-worker/cmd/tc-worker")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(),
		"TC_WORKER_SERVER_URL="+serverURL,
		"TC_WORKER_MAX_MESSAGES=1",
		"TC_WORKER_POLL_INTERVAL=20ms",
		"TC_WORKER_HEARTBEAT_INTERVAL=50ms",
		"TC_WORKER_ENDPOINT_REF="+endpointRef,
		"TC_WORKER_DISPLAY_NAME="+capability+" worker",
		"TC_WORKER_EXECUTOR=skill",
		"TC_WORKER_SKILL_BACKEND=ai-cli",
		"TC_WORKER_SKILLS_DIR="+skillsDir,
		"TC_WORKER_CAPABILITIES="+capability,
		"TC_WORKER_AI_CLI_COMMAND="+aiCLIPath,
		"TC_WORKER_AI_CLI_TIMEOUT=2s",
		"TC_WORKER_ARTIFACT_DIR="+artifactDir,
	)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd
}

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate local address: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func waitForHealth(t *testing.T, url string, output *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(url)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server health check timed out\n%s", output.String())
}

func ingressMessageEventually(t *testing.T, serverURL string, workerOutput *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = postIngressMessage(client, serverURL)
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("ingress message was not accepted: %v\nworker output:\n%s", lastErr, workerOutput.String())
}

func postIngressMessage(client *http.Client, serverURL string) error {
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/process_smoke_sender",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "process smoke message",
			Body:       "Verify tc-worker can poll and process from tc-server.",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "process_smoke", Summary: "worker process must exit after one completed message"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := client.Post(serverURL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	return nil
}

func postCommandIngressMessageEventually(t *testing.T, serverURL string, workerOutput *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = postCommandIngressMessage(client, serverURL)
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("command ingress message was not accepted: %v\nworker output:\n%s", lastErr, workerOutput.String())
}

func postCommandIngressMessage(client *http.Client, serverURL string) error {
	commandBody, err := json.Marshal(map[string]any{
		"command": "echo",
		"args":    []string{"process-command-smoke"},
	})
	if err != nil {
		return err
	}
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/process_command_smoke_sender",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "process command smoke message",
			Body:       string(commandBody),
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "process_command_smoke", Summary: "worker must run an allowlisted command"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := client.Post(serverURL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	return nil
}

func postLLMIngressMessageEventually(t *testing.T, serverURL string, workerOutput *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = postLLMIngressMessage(client, serverURL)
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("LLM ingress message was not accepted: %v\nworker output:\n%s", lastErr, workerOutput.String())
}

func postLLMIngressMessage(client *http.Client, serverURL string) error {
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/process_llm_smoke_sender",
		TargetCapability:  "ai.generate",
		Payload: contracts.Payload{
			Summary:    "process LLM smoke message",
			Body:       "Produce a short handoff summary for the next AI reviewer.",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "process_llm_smoke", Summary: "worker must call the configured LLM provider"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := client.Post(serverURL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	return nil
}

func postReviewIngressMessageEventually(t *testing.T, serverURL string, workerOutput *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = postReviewIngressMessage(client, serverURL)
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("review ingress message was not accepted: %v\nworker output:\n%s", lastErr, workerOutput.String())
}

func postReviewIngressMessage(client *http.Client, serverURL string) error {
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/process_skill_ai_cli_sender",
		TargetCapability:  "ai.review",
		Payload: contracts.Payload{
			Summary:    "process skill AI CLI smoke message",
			Body:       "Review the previous AI handoff and confirm it is auditable.",
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "process_skill_ai_cli_smoke", Summary: "worker must run a local AI CLI with skill guidance"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := client.Post(serverURL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	return nil
}

func postSkillHandoffMessageEventually(t *testing.T, serverURL string, capability string, correlationRef string, bodyText string, workerOutput *bytes.Buffer) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = postSkillHandoffMessage(client, serverURL, capability, correlationRef, bodyText)
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("skill handoff message was not accepted: %v\nworker output:\n%s", lastErr, workerOutput.String())
}

func postSkillHandoffMessage(client *http.Client, serverURL string, capability string, correlationRef string, bodyText string) error {
	req := contracts.MessageIngressRequest{
		SenderEndpointRef: "tc://endpoint/process_skill_handoff_sender",
		TargetCapability:  capability,
		CorrelationRef:    correlationRef,
		Payload: contracts.Payload{
			Summary:    "process skill handoff message",
			Body:       bodyText,
			References: []contracts.Reference{},
		},
		Constraints: []contracts.Constraint{
			{Code: "process_skill_handoff_smoke", Summary: "local AI CLI workers must preserve prior handoff context"},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := client.Post(serverURL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	return nil
}

func waitForCanceledProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	_ = cmd.Wait()
}

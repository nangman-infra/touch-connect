package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func repositoryRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Dir(wd)
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

func waitForCanceledProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	_ = cmd.Wait()
}

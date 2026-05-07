package runtime

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestAICLIExecutorExecutesWithPromptOnStdin(t *testing.T) {
	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestAICLIExecutorHelper", "--", "stdin_echo"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	result, err := executor.Execute(context.Background(), ExecutionInput{
		MessageRef:       "tc://message/msg_test",
		AttemptRef:       "tc://attempt/att_test",
		TargetCapability: "code.change",
		Payload: contracts.Payload{
			Summary: "Summarize release readiness",
			Body:    "Return WORKER_READBACK and WORKER_RESULT_READY.",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Outcome != ExecutionOutcomeCompleted || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Stdout, "Summarize release readiness") || !strings.Contains(result.Stdout, "WORKER_RESULT_READY") {
		t.Fatalf("prompt was not sent to stdin:\n%s", result.Stdout)
	}
}

func TestAICLIExecutorUsesPromptPlaceholder(t *testing.T) {
	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestAICLIExecutorHelper", "--", "arg_echo", "{{prompt}}"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	result, err := executor.Execute(context.Background(), ExecutionInput{
		MessageRef:       "tc://message/msg_placeholder",
		AttemptRef:       "tc://attempt/att_placeholder",
		TargetCapability: "ai.review",
		Payload:          contracts.Payload{Summary: "Review prompt placeholder"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(result.Stdout, "Review prompt placeholder") {
		t.Fatalf("placeholder prompt was not passed in args:\n%s", result.Stdout)
	}
}

func TestAICLIExecutorHelper(t *testing.T) {
	marker := -1
	for index, arg := range os.Args {
		if arg == "--" {
			marker = index
			break
		}
	}
	if marker < 0 || len(os.Args) <= marker+1 {
		return
	}

	switch os.Args[marker+1] {
	case "stdin_echo":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(data)
	case "arg_echo":
		_, _ = os.Stdout.WriteString(strings.Join(os.Args[marker+2:], "\n"))
	default:
		return
	}
	os.Exit(0)
}

func TestAICLIExecutorValidationAndFailureResults(t *testing.T) {
	if _, err := NewAICLIExecutor(AICLIExecutorOptions{}); err == nil {
		t.Fatalf("expected missing command to fail")
	}
	if _, err := NewAICLIExecutor(AICLIExecutorOptions{Command: "/bin/sh", Timeout: -time.Second}); err == nil {
		t.Fatalf("expected negative timeout to fail")
	}
	if _, err := NewAICLIExecutor(AICLIExecutorOptions{Command: "/bin/sh", WorkDir: "relative"}); err == nil {
		t.Fatalf("expected relative workdir to fail")
	}

	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: "/bin/sh",
		Args:    []string{"-c", "echo failed >&2; exit 7"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), ExecutionInput{Payload: contracts.Payload{Summary: "fail"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Outcome != ExecutionOutcomeFailed || result.ExitCode != 7 || result.FailureReasonCode != "ai_cli_exit_nonzero" {
		t.Fatalf("unexpected failure result: %+v", result)
	}
}

func TestAICLIExecutorStreamsProgressAndReadbackMarker(t *testing.T) {
	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: "/bin/sh",
		Args:    []string{"-c", "printf 'WORKER_READBACK ready\\nWORKER_RESULT_READY done\\n'"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	var progress []ExecutionProgress
	result, err := executor.Execute(context.Background(), ExecutionInput{
		Payload: contracts.Payload{Summary: "stream"},
		Progress: func(item ExecutionProgress) {
			progress = append(progress, item)
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Outcome != ExecutionOutcomeCompleted {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if len(progress) != 2 || progress[0].Kind != "readback" || !strings.Contains(progress[1].Summary, "WORKER_RESULT_READY") {
		t.Fatalf("expected streamed progress with readback marker, got %+v", progress)
	}
}

func TestAICLIExecutorReturnsPartialCompletedOnTimeout(t *testing.T) {
	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: "/bin/sh",
		Args:    []string{"-c", "printf 'partial line\\n'; sleep 2"},
		Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	result, err := executor.Execute(context.Background(), ExecutionInput{Payload: contracts.Payload{Summary: "timeout"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Outcome != ExecutionOutcomePartialCompleted || !strings.Contains(result.Stdout, "partial line") || result.FailureReasonCode != "ai_cli_timeout" {
		t.Fatalf("expected partial completed timeout with stdout preserved, got %+v", result)
	}
}

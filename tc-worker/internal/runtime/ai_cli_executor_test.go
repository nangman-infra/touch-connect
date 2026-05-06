package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestAICLIExecutorExecutesWithPromptOnStdin(t *testing.T) {
	executor, err := NewAICLIExecutor(AICLIExecutorOptions{
		Command: "/bin/sh",
		Args:    []string{"-c", "cat"},
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
		Command: "/bin/sh",
		Args:    []string{"-c", "printf '%s' \"$1\"", "sh", "{{prompt}}"},
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

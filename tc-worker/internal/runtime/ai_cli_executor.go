package runtime

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultAICLITimeout = 10 * time.Minute
)

type AICLIExecutorOptions struct {
	Command string
	Args    []string
	WorkDir string
	Timeout time.Duration
}

type AICLIExecutor struct {
	command string
	args    []string
	workDir string
	timeout time.Duration
}

func NewAICLIExecutor(options AICLIExecutorOptions) (*AICLIExecutor, error) {
	accepted, err := options.validated()
	if err != nil {
		return nil, err
	}
	return &AICLIExecutor{
		command: accepted.Command,
		args:    accepted.Args,
		workDir: accepted.WorkDir,
		timeout: accepted.Timeout,
	}, nil
}

func (e *AICLIExecutor) Execute(ctx context.Context, input ExecutionInput) (ExecutionResult, error) {
	startedAt := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, e.command, e.args...)
	if e.workDir != "" {
		command.Dir = e.workDir
	}
	command.Stdin = strings.NewReader(llmPromptFromInput(input))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	durationMS := time.Since(startedAt).Milliseconds()
	output := collectCommandOutput(stdout.String(), stderr.String())
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return failedCommandOutput("ai_cli_timeout", "AI CLI timed out", output, -1, durationMS), nil
	}
	if err != nil {
		exitCode := exitCodeFromError(err)
		if exitCode >= 0 {
			return failedCommandOutput("ai_cli_exit_nonzero", "AI CLI exited with non-zero status", output, exitCode, durationMS), nil
		}
		return failedCommandOutput("ai_cli_start_failed", err.Error(), output, exitCode, durationMS), nil
	}
	summary := strings.TrimSpace(output.Stdout)
	if summary == "" {
		summary = "AI CLI completed without stdout"
	}
	return ExecutionResult{
		Outcome:    ExecutionOutcomeCompleted,
		Summary:    summary,
		Stdout:     output.Stdout,
		Stderr:     output.Stderr,
		ExitCode:   0,
		DurationMS: durationMS,
	}, nil
}

func (o AICLIExecutorOptions) validated() (AICLIExecutorOptions, error) {
	o.Command = strings.TrimSpace(o.Command)
	if o.Command == "" {
		return AICLIExecutorOptions{}, errors.New("AI CLI command is required")
	}
	if o.Timeout == 0 {
		o.Timeout = defaultAICLITimeout
	}
	if o.Timeout < 0 {
		return AICLIExecutorOptions{}, errors.New("AI CLI timeout must not be negative")
	}
	if o.WorkDir != "" && !filepath.IsAbs(o.WorkDir) {
		return AICLIExecutorOptions{}, errors.New("AI CLI workdir must be an absolute path")
	}
	args := make([]string, 0, len(o.Args))
	for _, arg := range o.Args {
		arg = strings.TrimSpace(arg)
		if arg != "" {
			args = append(args, arg)
		}
	}
	o.Args = args
	return o, nil
}

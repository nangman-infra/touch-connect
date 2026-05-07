package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultAICLITimeout     = 10 * time.Minute
	defaultAICLIGracePeriod = 30 * time.Second
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
	prompt := llmPromptFromInput(input)
	args, promptInArgs := aiCLIArgsWithPrompt(e.args, prompt)
	command := exec.Command(e.command, args...)
	if e.workDir != "" {
		command.Dir = e.workDir
	}
	if !promptInArgs {
		command.Stdin = strings.NewReader(prompt)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return failedCommandOutput("ai_cli_start_failed", err.Error(), commandOutput{}, -1, 0), nil
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return failedCommandOutput("ai_cli_start_failed", err.Error(), commandOutput{}, -1, 0), nil
	}
	if err := command.Start(); err != nil {
		return failedCommandOutput("ai_cli_start_failed", err.Error(), commandOutput{}, -1, 0), nil
	}
	var scanWG sync.WaitGroup
	scanWG.Add(2)
	go scanAICLIOutput(stdoutPipe, &stdout, input.Progress, true, &scanWG)
	go scanAICLIOutput(stderrPipe, &stderr, input.Progress, false, &scanWG)
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()
	err, interrupted := waitForAICLICommand(runCtx, command, waitDone)
	scanWG.Wait()
	durationMS := time.Since(startedAt).Milliseconds()
	output := collectCommandOutput(stdout.String(), stderr.String())
	if interrupted && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return partialCommandOutput("ai_cli_timeout", "AI CLI timed out; partial output preserved", output, -1, durationMS), nil
	}
	if interrupted {
		return failedCommandOutput("ai_cli_canceled", "AI CLI canceled", output, -1, durationMS), nil
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

func scanAICLIOutput(reader io.Reader, buffer *bytes.Buffer, progress func(ExecutionProgress), stdout bool, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		buffer.WriteString(line)
		buffer.WriteByte('\n')
		if !stdout || progress == nil {
			continue
		}
		kind := "stdout"
		if strings.Contains(line, "WORKER_READBACK") {
			kind = "readback"
		}
		progress(ExecutionProgress{
			Kind:    kind,
			Summary: line,
			Line:    line,
		})
	}
}

func waitForAICLICommand(ctx context.Context, command *exec.Cmd, waitDone <-chan error) (error, bool) {
	select {
	case err := <-waitDone:
		return err, false
	case <-ctx.Done():
		terminateAICLIProcess(command)
		select {
		case err := <-waitDone:
			return err, true
		case <-time.After(defaultAICLIGracePeriod):
			if command.Process != nil {
				_ = command.Process.Kill()
			}
			return <-waitDone, true
		}
	}
}

func terminateAICLIProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		_ = command.Process.Kill()
	}
}

func aiCLIArgsWithPrompt(args []string, prompt string) ([]string, bool) {
	out := make([]string, 0, len(args))
	promptInArgs := false
	for _, arg := range args {
		next := strings.ReplaceAll(arg, "{{prompt}}", prompt)
		next = strings.ReplaceAll(next, "{{PROMPT}}", prompt)
		if next != arg {
			promptInArgs = true
		}
		out = append(out, next)
	}
	return out, promptInArgs
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

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommandExecutorOptions struct {
	AllowedCommands []string
	WorkDir         string
	Timeout         time.Duration
}

type CommandExecutor struct {
	allowed map[string]struct{}
	workDir string
	timeout time.Duration
}

type CommandRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"workdir,omitempty"`
}

func NewCommandExecutor(options CommandExecutorOptions) (*CommandExecutor, error) {
	accepted, err := options.validated()
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(accepted.AllowedCommands))
	for _, command := range accepted.AllowedCommands {
		allowed[command] = struct{}{}
	}
	return &CommandExecutor{allowed: allowed, workDir: accepted.WorkDir, timeout: accepted.Timeout}, nil
}

func (e *CommandExecutor) Execute(ctx context.Context, input ExecutionInput) (ExecutionResult, error) {
	request, missing, err := decodeCommandRequest(input.Payload.Body)
	if len(missing) > 0 {
		return missingFieldResult(missing[0], missing[1]), nil
	}
	if err != nil {
		return failedCommandResult("command_request_invalid_json", err.Error()), nil
	}
	if _, ok := e.allowed[request.Command]; !ok {
		return failedCommandResult("command_not_allowed", "command is not in worker allowlist"), nil
	}
	workDir := request.WorkDir
	if workDir == "" {
		workDir = e.workDir
	}
	if workDir == "" {
		return missingFieldResult("workdir", "command execution requires an absolute workdir"), nil
	}
	if !filepath.IsAbs(workDir) {
		return failedCommandResult("command_workdir_not_absolute", "command workdir must be an absolute path"), nil
	}
	return e.runCommand(ctx, request, workDir), nil
}

func (e *CommandExecutor) runCommand(ctx context.Context, request CommandRequest, workDir string) ExecutionResult {
	startedAt := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, request.Command, request.Args...)
	command.Dir = workDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	durationMS := time.Since(startedAt).Milliseconds()
	output := collectCommandOutput(stdout.String(), stderr.String())
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return failedCommandOutput("command_timeout", "command timed out", output, -1, durationMS)
	}
	if err != nil {
		exitCode := exitCodeFromError(err)
		if exitCode >= 0 {
			return failedCommandOutput("command_exit_nonzero", "command exited with non-zero status", output, exitCode, durationMS)
		}
		return failedCommandOutput("command_start_failed", err.Error(), output, exitCode, durationMS)
	}
	return ExecutionResult{
		Outcome:    ExecutionOutcomeCompleted,
		Summary:    "command completed successfully",
		Stdout:     output.Stdout,
		Stderr:     output.Stderr,
		ExitCode:   0,
		DurationMS: durationMS,
	}
}

func (o CommandExecutorOptions) validated() (CommandExecutorOptions, error) {
	if len(o.AllowedCommands) == 0 {
		return CommandExecutorOptions{}, errors.New("allowed commands are required")
	}
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.Timeout < 0 {
		return CommandExecutorOptions{}, errors.New("command timeout must not be negative")
	}
	if o.WorkDir != "" && !filepath.IsAbs(o.WorkDir) {
		return CommandExecutorOptions{}, errors.New("command workdir must be an absolute path")
	}
	commands := make([]string, 0, len(o.AllowedCommands))
	seen := map[string]struct{}{}
	for _, command := range o.AllowedCommands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		if _, ok := seen[command]; ok {
			continue
		}
		seen[command] = struct{}{}
		commands = append(commands, command)
	}
	if len(commands) == 0 {
		return CommandExecutorOptions{}, errors.New("allowed commands are required")
	}
	o.AllowedCommands = commands
	return o, nil
}

func decodeCommandRequest(body string) (CommandRequest, []string, error) {
	if strings.TrimSpace(body) == "" {
		return CommandRequest{}, []string{"payload.body", "payload body must contain a command request JSON object"}, nil
	}
	var request CommandRequest
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		return CommandRequest{}, nil, err
	}
	request.Command = strings.TrimSpace(request.Command)
	request.WorkDir = strings.TrimSpace(request.WorkDir)
	if request.Command == "" {
		return CommandRequest{}, []string{"command", "command request must include command"}, nil
	}
	return request, nil, nil
}

type commandOutput struct {
	Stdout string
	Stderr string
}

func collectCommandOutput(stdout string, stderr string) commandOutput {
	return commandOutput{Stdout: stdout, Stderr: stderr}
}

func failedCommandResult(reason string, summary string) ExecutionResult {
	return failedCommandOutput(reason, summary, commandOutput{}, -1, 0)
}

func failedCommandOutput(reason string, summary string, output commandOutput, exitCode int, durationMS int64) ExecutionResult {
	return ExecutionResult{
		Outcome:           ExecutionOutcomeFailed,
		Summary:           summary,
		FailureReasonCode: reason,
		Stdout:            output.Stdout,
		Stderr:            output.Stderr,
		ExitCode:          exitCode,
		DurationMS:        durationMS,
	}
}

func missingFieldResult(name string, reason string) ExecutionResult {
	return ExecutionResult{
		Outcome: ExecutionOutcomeMissingFields,
		Summary: "processing blocked because required command input is missing",
		MissingFields: []MissingField{
			{Name: name, Reason: reason},
		},
	}
}

func exitCodeFromError(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

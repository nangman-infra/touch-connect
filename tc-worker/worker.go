package tcworker

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-worker/internal/client"
	"github.com/nangman-infra/touch-connect/tc-worker/internal/runtime"
)

type Config = runtime.Config
type CommandExecutor = runtime.CommandExecutor
type CommandExecutorOptions = runtime.CommandExecutorOptions
type CommandRequest = runtime.CommandRequest
type EchoExecutor = runtime.EchoExecutor
type ExecutionArtifactStore = runtime.ExecutionArtifactStore
type ExecutionInput = runtime.ExecutionInput
type ExecutionResult = runtime.ExecutionResult
type LocalArtifactStore = runtime.LocalArtifactStore
type LocalArtifactStoreOptions = runtime.LocalArtifactStoreOptions
type MissingField = runtime.MissingField
type LoopOptions = runtime.LoopOptions
type ProcessResult = runtime.ProcessResult
type Runtime = runtime.Runtime
type WorkerExecutor = runtime.WorkerExecutor

const (
	ExecutionOutcomeCompleted     = runtime.ExecutionOutcomeCompleted
	ExecutionOutcomeMissingFields = runtime.ExecutionOutcomeMissingFields
	ExecutionOutcomeFailed        = runtime.ExecutionOutcomeFailed
	ExecutionOutcomeDropped       = runtime.ExecutionOutcomeDropped
)

func NewHTTPRuntime(serverURL string, httpClient *http.Client, config Config) *runtime.Runtime {
	return runtime.New(client.NewHTTPClient(serverURL, httpClient), config)
}

func NewHTTPRuntimeWithExecutor(serverURL string, httpClient *http.Client, config Config, executor WorkerExecutor) *runtime.Runtime {
	return runtime.NewWithExecutor(client.NewHTTPClient(serverURL, httpClient), config, executor)
}

func NewHTTPRuntimeWithExecutorAndArtifacts(serverURL string, httpClient *http.Client, config Config, executor WorkerExecutor, artifactStore ExecutionArtifactStore) *runtime.Runtime {
	return runtime.NewWithExecutorAndArtifacts(client.NewHTTPClient(serverURL, httpClient), config, executor, artifactStore)
}

func NewCommandExecutor(options CommandExecutorOptions) (*CommandExecutor, error) {
	return runtime.NewCommandExecutor(options)
}

func NewLocalArtifactStore(options LocalArtifactStoreOptions) (*LocalArtifactStore, error) {
	return runtime.NewLocalArtifactStore(options)
}

func DefaultConfig() Config {
	return Config{
		EndpointRef:   "tc://endpoint/ep_local_worker",
		DisplayName:   "local worker",
		ActorID:       "actor.local",
		WorkspaceID:   "workspace.local",
		WorkerVersion: "0.1.0-dev",
		Capabilities: []contracts.Capability{
			{Name: "code.change", ExecutionHints: []string{"checkpoint_progress", "local_execution"}},
		},
		ExecutionHints: []string{"local_cli"},
	}
}

func ConfigFromEnv() Config {
	config := DefaultConfig()
	if value := os.Getenv("TC_WORKER_ENDPOINT_REF"); value != "" {
		config.EndpointRef = value
	}
	if value := os.Getenv("TC_WORKER_DISPLAY_NAME"); value != "" {
		config.DisplayName = value
	}
	if value := os.Getenv("TC_WORKER_ACTOR_ID"); value != "" {
		config.ActorID = value
	}
	if value := os.Getenv("TC_WORKER_WORKSPACE_ID"); value != "" {
		config.WorkspaceID = value
	}
	if value := os.Getenv("TC_WORKER_VERSION"); value != "" {
		config.WorkerVersion = value
	}
	return config
}

func DefaultLoopOptions() LoopOptions {
	return runtime.DefaultLoopOptions()
}

func LoopOptionsFromEnv() (LoopOptions, error) {
	options := runtime.DefaultLoopOptions()
	if value := os.Getenv("TC_WORKER_POLL_INTERVAL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return LoopOptions{}, err
		}
		options.PollInterval = parsed
	}
	if value := os.Getenv("TC_WORKER_HEARTBEAT_INTERVAL"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return LoopOptions{}, err
		}
		options.HeartbeatInterval = parsed
	}
	if value := os.Getenv("TC_WORKER_MAX_MESSAGES"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return LoopOptions{}, err
		}
		options.MaxMessages = parsed
	}
	return options.Validated()
}

func ExecutorFromEnv() (WorkerExecutor, error) {
	allowed := splitCSV(os.Getenv("TC_WORKER_ALLOWED_COMMANDS"))
	if len(allowed) == 0 {
		return EchoExecutor{}, nil
	}
	options := CommandExecutorOptions{
		AllowedCommands: allowed,
		WorkDir:         os.Getenv("TC_WORKER_WORKDIR"),
	}
	if value := os.Getenv("TC_WORKER_COMMAND_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		options.Timeout = parsed
	}
	return NewCommandExecutor(options)
}

func ArtifactStoreFromEnv() (ExecutionArtifactStore, error) {
	dir := os.Getenv("TC_WORKER_ARTIFACT_DIR")
	if dir == "" {
		return nil, nil
	}
	options := LocalArtifactStoreOptions{
		Dir:            dir,
		RoomRef:        os.Getenv("TC_WORKER_ARTIFACT_ROOM_REF"),
		TaskRef:        os.Getenv("TC_WORKER_ARTIFACT_TASK_REF"),
		RetentionClass: os.Getenv("TC_WORKER_ARTIFACT_RETENTION_CLASS"),
		AccessScope:    os.Getenv("TC_WORKER_ARTIFACT_ACCESS_SCOPE"),
	}
	if value := os.Getenv("TC_WORKER_ARTIFACT_TASK_REVISION"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		options.TaskRevision = parsed
	}
	return NewLocalArtifactStore(options)
}

func RegisterAndProcess(ctx context.Context, serverURL string, messageRef string) (string, error) {
	executor, err := ExecutorFromEnv()
	if err != nil {
		return "", err
	}
	artifactStore, err := ArtifactStoreFromEnv()
	if err != nil {
		return "", err
	}
	worker := NewHTTPRuntimeWithExecutorAndArtifacts(serverURL, nil, ConfigFromEnv(), executor, artifactStore)
	if err := worker.Register(ctx); err != nil {
		return "", err
	}
	return worker.ProcessMessage(ctx, messageRef)
}

func RegisterAndRun(ctx context.Context, serverURL string, options LoopOptions) error {
	executor, err := ExecutorFromEnv()
	if err != nil {
		return err
	}
	artifactStore, err := ArtifactStoreFromEnv()
	if err != nil {
		return err
	}
	worker := NewHTTPRuntimeWithExecutorAndArtifacts(serverURL, nil, ConfigFromEnv(), executor, artifactStore)
	return worker.Run(ctx, options)
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

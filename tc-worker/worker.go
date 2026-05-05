package tcworker

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	skillpkg "github.com/nangman-infra/touch-connect/internal/communication/skills"
	"github.com/nangman-infra/touch-connect/tc-worker/internal/client"
	"github.com/nangman-infra/touch-connect/tc-worker/internal/runtime"
)

type Config = runtime.Config
type CommandExecutor = runtime.CommandExecutor
type CommandExecutorOptions = runtime.CommandExecutorOptions
type CommandRequest = runtime.CommandRequest
type AICLIExecutor = runtime.AICLIExecutor
type AICLIExecutorOptions = runtime.AICLIExecutorOptions
type EchoExecutor = runtime.EchoExecutor
type ExecutionArtifactStore = runtime.ExecutionArtifactStore
type HandoffArtifact = runtime.HandoffArtifact
type HandoffContext = runtime.HandoffContext
type HandoffMessage = runtime.HandoffMessage
type ExecutionInput = runtime.ExecutionInput
type ExecutionResult = runtime.ExecutionResult
type LLMExecutor = runtime.LLMExecutor
type LLMExecutorOptions = runtime.LLMExecutorOptions
type LocalArtifactStore = runtime.LocalArtifactStore
type LocalArtifactStoreOptions = runtime.LocalArtifactStoreOptions
type MissingField = runtime.MissingField
type LoopOptions = runtime.LoopOptions
type ProcessResult = runtime.ProcessResult
type Runtime = runtime.Runtime
type SkillDefinition = contracts.SkillDefinition
type SkillExecutor = runtime.SkillExecutor
type SkillExecutorOptions = runtime.SkillExecutorOptions
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

func NewAICLIExecutor(options AICLIExecutorOptions) (*AICLIExecutor, error) {
	return runtime.NewAICLIExecutor(options)
}

func NewLLMExecutor(options LLMExecutorOptions) (*LLMExecutor, error) {
	return runtime.NewLLMExecutor(options)
}

func NewSkillExecutor(options SkillExecutorOptions) (*SkillExecutor, error) {
	return runtime.NewSkillExecutor(options)
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
	if value := os.Getenv("TC_WORKER_CAPABILITIES"); value != "" {
		capabilities := capabilitiesFromCSV(value)
		if len(capabilities) > 0 {
			config.Capabilities = capabilities
		}
	} else if strings.EqualFold(strings.TrimSpace(os.Getenv("TC_WORKER_EXECUTOR")), "skill") {
		if capabilities, err := skillCapabilitiesFromEnv(); err == nil && len(capabilities) > 0 {
			config.Capabilities = capabilities
			config.ExecutionHints = appendUniqueStrings(config.ExecutionHints, "skill_guided")
		}
	}
	if backend := strings.TrimSpace(os.Getenv("TC_WORKER_BACKEND")); backend != "" {
		config.ExecutionHints = appendUniqueStrings(config.ExecutionHints, "backend:"+backend)
	}
	if model := strings.TrimSpace(os.Getenv("TC_WORKER_MODEL")); model != "" {
		config.ExecutionHints = appendUniqueStrings(config.ExecutionHints, "model:"+model)
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
	kind := strings.ToLower(strings.TrimSpace(os.Getenv("TC_WORKER_EXECUTOR")))
	allowed := splitCSV(os.Getenv("TC_WORKER_ALLOWED_COMMANDS"))
	if kind == "" {
		if len(allowed) > 0 {
			kind = "command"
		} else {
			kind = "echo"
		}
	}
	switch kind {
	case "echo":
		return EchoExecutor{}, nil
	case "command":
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
	case "llm":
		return llmExecutorFromEnv()
	case "ai-cli", "aicli", "cli":
		return aiCLIExecutorFromEnv()
	case "skill":
		return skillExecutorFromEnv()
	default:
		return nil, errors.New("unknown TC_WORKER_EXECUTOR")
	}
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

func capabilitiesFromCSV(value string) []contracts.Capability {
	names := splitCSV(value)
	capabilities := make([]contracts.Capability, 0, len(names))
	for _, name := range names {
		capabilities = append(capabilities, contracts.Capability{
			Name:           name,
			ExecutionHints: []string{"checkpoint_progress", "ai_execution"},
		})
	}
	return capabilities
}

func capabilitiesFromSkills(items []contracts.SkillDefinition) []contracts.Capability {
	capabilities := make([]contracts.Capability, 0)
	seen := map[string]struct{}{}
	for _, skill := range items {
		for _, name := range skill.Capabilities {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			capabilities = append(capabilities, contracts.Capability{
				Name:           name,
				ExecutionHints: []string{"checkpoint_progress", "skill_guided", "ai_execution"},
			})
		}
	}
	return capabilities
}

func llmExecutorFromEnv() (WorkerExecutor, error) {
	maxOutputTokens, err := parseOptionalNonNegativeInt(os.Getenv("TC_WORKER_LLM_MAX_OUTPUT_TOKENS"))
	if err != nil {
		return nil, err
	}
	options := LLMExecutorOptions{
		Provider:        os.Getenv("TC_WORKER_LLM_PROVIDER"),
		BaseURL:         os.Getenv("TC_WORKER_LLM_BASE_URL"),
		APIKey:          os.Getenv("TC_WORKER_LLM_API_KEY"),
		Model:           os.Getenv("TC_WORKER_LLM_MODEL"),
		SystemPrompt:    os.Getenv("TC_WORKER_LLM_SYSTEM_PROMPT"),
		MaxOutputTokens: maxOutputTokens,
	}
	if value := os.Getenv("TC_WORKER_LLM_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		options.Timeout = parsed
	}
	return NewLLMExecutor(options)
}

func skillExecutorFromEnv() (WorkerExecutor, error) {
	loaded, err := skillsFromEnv()
	if err != nil {
		return nil, err
	}
	backendKind := strings.ToLower(strings.TrimSpace(os.Getenv("TC_WORKER_SKILL_BACKEND")))
	if backendKind == "" {
		backendKind = "ai-cli"
	}
	backend, err := backendExecutorFromEnv(backendKind)
	if err != nil {
		return nil, err
	}
	return NewSkillExecutor(SkillExecutorOptions{Skills: loaded, Backend: backend})
}

func backendExecutorFromEnv(kind string) (WorkerExecutor, error) {
	switch kind {
	case "ai-cli", "aicli", "cli":
		return aiCLIExecutorFromEnv()
	case "echo":
		return EchoExecutor{}, nil
	case "command":
		options := CommandExecutorOptions{
			AllowedCommands: splitCSV(os.Getenv("TC_WORKER_ALLOWED_COMMANDS")),
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
	case "llm":
		return llmExecutorFromEnv()
	default:
		return nil, errors.New("unknown TC_WORKER_SKILL_BACKEND")
	}
}

func aiCLIExecutorFromEnv() (WorkerExecutor, error) {
	command := strings.TrimSpace(os.Getenv("TC_WORKER_AI_CLI_COMMAND"))
	args := splitCSV(os.Getenv("TC_WORKER_AI_CLI_ARGS"))
	if command == "" {
		var err error
		command, args, err = defaultAICLICommand()
		if err != nil {
			return nil, err
		}
	} else if len(args) == 0 {
		args = defaultArgsForAICLI(command)
	}
	options := AICLIExecutorOptions{
		Command: command,
		Args:    args,
		WorkDir: os.Getenv("TC_WORKER_AI_CLI_WORKDIR"),
	}
	if options.WorkDir == "" {
		options.WorkDir = os.Getenv("TC_WORKER_WORKDIR")
	}
	if value := os.Getenv("TC_WORKER_AI_CLI_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return nil, err
		}
		options.Timeout = parsed
	}
	return NewAICLIExecutor(options)
}

func defaultAICLICommand() (string, []string, error) {
	if command, err := exec.LookPath("codex"); err == nil {
		return command, defaultArgsForAICLI(command), nil
	}
	if command, err := exec.LookPath("claude"); err == nil {
		return command, defaultArgsForAICLI(command), nil
	}
	return "", nil, errors.New("no supported AI CLI found; set TC_WORKER_AI_CLI_COMMAND")
}

func defaultArgsForAICLI(command string) []string {
	base := filepath.Base(command)
	switch base {
	case "codex":
		return []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only", "-c", "approval_policy=\"never\"", "-"}
	case "claude":
		return []string{"-p"}
	default:
		return nil
	}
}

func skillCapabilitiesFromEnv() ([]contracts.Capability, error) {
	loaded, err := skillsFromEnv()
	if err != nil {
		return nil, err
	}
	return capabilitiesFromSkills(loaded), nil
}

func skillsFromEnv() ([]contracts.SkillDefinition, error) {
	var loaded []contracts.SkillDefinition
	for _, path := range splitCSV(os.Getenv("TC_WORKER_SKILL_PATHS")) {
		skill, err := skillpkg.LoadFile(path)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, skill)
	}
	if registryPath := skillRegistryPathFromEnv(); registryPath != "" {
		registry, err := skillpkg.LoadRegistry(registryPath)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, registry.Skills...)
	}
	if dir := strings.TrimSpace(os.Getenv("TC_WORKER_SKILLS_DIR")); dir != "" {
		items, err := skillpkg.LoadDir(dir)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, items...)
	}
	if len(loaded) == 0 {
		return nil, errors.New("skill executor requires TC_WORKER_SKILL_REGISTRY or TC_WORKER_SKILLS_DIR")
	}
	return dedupeSkills(loaded), nil
}

func skillRegistryPathFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("TC_WORKER_SKILL_REGISTRY")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("TC_SKILL_REGISTRY")); value != "" {
		return value
	}
	return defaultSkillRegistryPath()
}

func defaultSkillRegistryPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return home + "/.touch-connect/skills/registry.json"
}

func dedupeSkills(items []contracts.SkillDefinition) []contracts.SkillDefinition {
	out := make([]contracts.SkillDefinition, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if _, ok := seen[item.SkillRef]; ok {
			continue
		}
		seen[item.SkillRef] = struct{}{}
		out = append(out, item)
	}
	return out
}

func appendUniqueStrings(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func parseOptionalNonNegativeInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsed < 0 {
		return 0, errors.New("integer value must not be negative")
	}
	return parsed, nil
}

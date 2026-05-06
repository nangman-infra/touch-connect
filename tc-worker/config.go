package tcworker

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	WorkerConfigVersion       = 1
	DefaultWorkerConfigRel    = ".touch-connect/worker/config.json"
	DefaultWorkerSkillsRel    = ".touch-connect/worker/skills"
	DefaultWorkerArtifactsRel = ".touch-connect/worker/artifacts"
	DefaultWorkerLogsRel      = ".touch-connect/worker/logs"
	DefaultWorkerPermission   = "auto-approve"
	DefaultWorkerRole         = "code-worker"
)

type WorkerConfig struct {
	Version           int      `json:"version"`
	Backend           string   `json:"backend"`
	Model             string   `json:"model,omitempty"`
	ServerURL         string   `json:"server"`
	EndpointRef       string   `json:"endpoint"`
	DisplayName       string   `json:"display_name,omitempty"`
	ActorID           string   `json:"actor_id,omitempty"`
	WorkspaceID       string   `json:"workspace_id,omitempty"`
	Role              string   `json:"role"`
	Capabilities      []string `json:"capabilities"`
	Permission        string   `json:"permission"`
	Command           string   `json:"command,omitempty"`
	Args              []string `json:"args,omitempty"`
	SkillsDir         string   `json:"skills_dir"`
	SkillPaths        []string `json:"skill_paths,omitempty"`
	WorkDir           string   `json:"workdir"`
	ArtifactDir       string   `json:"artifact_dir"`
	Timeout           string   `json:"timeout,omitempty"`
	PollInterval      string   `json:"poll_interval,omitempty"`
	HeartbeatInterval string   `json:"heartbeat_interval,omitempty"`
	Sandbox           string   `json:"sandbox,omitempty"`
}

func DefaultWorkerConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultWorkerConfigRel), nil
}

func DefaultWorkerSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultWorkerSkillsRel), nil
}

func DefaultWorkerArtifactDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultWorkerArtifactsRel), nil
}

func LoadWorkerConfig(path string) (WorkerConfig, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultWorkerConfigPath()
		if err != nil {
			return WorkerConfig{}, err
		}
	}
	body, err := os.ReadFile(expandHome(path))
	if err != nil {
		return WorkerConfig{}, err
	}
	var config WorkerConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return WorkerConfig{}, err
	}
	return config.withDefaults()
}

func SaveWorkerConfig(path string, config WorkerConfig) error {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultWorkerConfigPath()
		if err != nil {
			return err
		}
	}
	config, err := config.withDefaults()
	if err != nil {
		return err
	}
	path = expandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

func WorkerConfigExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		defaultPath, err := DefaultWorkerConfigPath()
		if err != nil {
			return false
		}
		path = defaultPath
	}
	_, err := os.Stat(expandHome(path))
	return err == nil
}

func EnsureDefaultWorkerSkill(skillsDir string, capabilities []string) error {
	if strings.TrimSpace(skillsDir) == "" {
		var err error
		skillsDir, err = DefaultWorkerSkillsDir()
		if err != nil {
			return err
		}
	}
	if len(capabilities) == 0 {
		capabilities = []string{"code.change", "ai.review"}
	}
	dir := filepath.Join(expandHome(skillsDir), "local-ai-worker")
	path := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultWorkerSkillMarkdown(capabilities)), 0o600)
}

func (c WorkerConfig) JoinOptions() (JoinOptions, error) {
	config, err := c.withDefaults()
	if err != nil {
		return JoinOptions{}, err
	}
	timeout, err := parseOptionalDuration(config.Timeout)
	if err != nil {
		return JoinOptions{}, err
	}
	pollInterval, err := parseOptionalDuration(config.PollInterval)
	if err != nil {
		return JoinOptions{}, err
	}
	heartbeatInterval, err := parseOptionalDuration(config.HeartbeatInterval)
	if err != nil {
		return JoinOptions{}, err
	}
	return JoinOptions{
		ServerURL:         config.ServerURL,
		Backend:           config.Backend,
		Model:             config.Model,
		Command:           config.Command,
		Args:              append([]string(nil), config.Args...),
		EndpointRef:       config.EndpointRef,
		DisplayName:       config.DisplayName,
		ActorID:           config.ActorID,
		WorkspaceID:       config.WorkspaceID,
		Role:              config.Role,
		Capabilities:      strings.Join(config.Capabilities, ","),
		Permission:        config.Permission,
		SkillsDir:         config.SkillsDir,
		SkillPaths:        append([]string(nil), config.SkillPaths...),
		WorkDir:           config.WorkDir,
		ArtifactDir:       config.ArtifactDir,
		Timeout:           timeout,
		PollInterval:      pollInterval,
		HeartbeatInterval: heartbeatInterval,
		Sandbox:           config.Sandbox,
	}, nil
}

func WorkerConfigFromJoinOptions(options JoinOptions) (WorkerConfig, error) {
	accepted, err := options.withDefaults()
	if err != nil {
		return WorkerConfig{}, err
	}
	model := accepted.Model
	if strings.TrimSpace(model) == "" {
		if preset, err := presetForBackend(accepted.Backend); err == nil {
			model = preset.DefaultModel
		}
	}
	return WorkerConfig{
		Version:           WorkerConfigVersion,
		Backend:           accepted.Backend,
		Model:             model,
		ServerURL:         accepted.ServerURL,
		EndpointRef:       accepted.EndpointRef,
		DisplayName:       accepted.DisplayName,
		ActorID:           accepted.ActorID,
		WorkspaceID:       accepted.WorkspaceID,
		Role:              accepted.Role,
		Capabilities:      splitCSV(accepted.Capabilities),
		Permission:        accepted.Permission,
		Command:           accepted.Command,
		Args:              append([]string(nil), accepted.Args...),
		SkillsDir:         accepted.SkillsDir,
		SkillPaths:        append([]string(nil), accepted.SkillPaths...),
		WorkDir:           accepted.WorkDir,
		ArtifactDir:       accepted.ArtifactDir,
		Timeout:           accepted.Timeout.String(),
		PollInterval:      accepted.PollInterval.String(),
		HeartbeatInterval: accepted.HeartbeatInterval.String(),
		Sandbox:           accepted.Sandbox,
	}, nil
}

func (c WorkerConfig) withDefaults() (WorkerConfig, error) {
	c = defaultWorkerConfigScalars(c)
	if !supportedWorkerConfigVersion(c.Version) {
		return WorkerConfig{}, errors.New("unsupported worker config version")
	}
	var err error
	if c.SkillsDir, err = defaultConfigSkillsDir(c.SkillsDir, c.SkillPaths); err != nil {
		return WorkerConfig{}, err
	}
	if c.WorkDir, err = defaultConfigWorkDir(c.WorkDir); err != nil {
		return WorkerConfig{}, err
	}
	if c.ArtifactDir, err = defaultConfigArtifactDir(c.ArtifactDir); err != nil {
		return WorkerConfig{}, err
	}
	c.SkillsDir = expandHome(c.SkillsDir)
	c.WorkDir = expandHome(c.WorkDir)
	c.ArtifactDir = expandHome(c.ArtifactDir)
	for index, path := range c.SkillPaths {
		c.SkillPaths[index] = expandHome(path)
	}
	return c, nil
}

func defaultWorkerConfigScalars(c WorkerConfig) WorkerConfig {
	if c.Version == 0 {
		c.Version = WorkerConfigVersion
	}
	if strings.TrimSpace(c.ServerURL) == "" {
		c.ServerURL = "http://127.0.0.1:8080"
	}
	c.Backend = strings.ToLower(strings.TrimSpace(c.Backend))
	if c.Backend == "" {
		c.Backend = BackendAuto
	}
	if strings.TrimSpace(c.Role) == "" {
		c.Role = DefaultWorkerRole
	}
	if len(c.Capabilities) == 0 {
		c.Capabilities = []string{"code.change", "ai.review"}
	}
	if strings.TrimSpace(c.Permission) == "" {
		c.Permission = DefaultWorkerPermission
	}
	if strings.TrimSpace(c.Sandbox) == "" {
		c.Sandbox = "danger-full-access"
	}
	return c
}

func supportedWorkerConfigVersion(version int) bool {
	return version == WorkerConfigVersion
}

func defaultConfigSkillsDir(current string, skillPaths []string) (string, error) {
	if strings.TrimSpace(current) != "" || len(skillPaths) > 0 {
		return current, nil
	}
	return DefaultWorkerSkillsDir()
}

func defaultConfigWorkDir(current string) (string, error) {
	if strings.TrimSpace(current) != "" {
		return current, nil
	}
	return os.Getwd()
}

func defaultConfigArtifactDir(current string) (string, error) {
	if strings.TrimSpace(current) != "" {
		return current, nil
	}
	return DefaultWorkerArtifactDir()
}

func parseOptionalDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func defaultWorkerSkillMarkdown(capabilities []string) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("skill_ref: tc://skill/local-ai-worker\n")
	builder.WriteString("name: Local AI Worker\n")
	builder.WriteString("kind: executable\n")
	builder.WriteString("description: Execute touch-connect handoffs through the selected local AI CLI.\n")
	builder.WriteString("capabilities:\n")
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability != "" {
			builder.WriteString("  - ")
			builder.WriteString(capability)
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("executor_hint: ai-cli\n")
	builder.WriteString("approval_required: false\n")
	builder.WriteString("---\n\n")
	builder.WriteString("# Local AI Worker\n\n")
	builder.WriteString("You are a touch-connect worker. Read the task body, produce a concise readback, do the requested work, and finish with these exact markers:\n\n")
	builder.WriteString("- WORKER_READBACK\n")
	builder.WriteString("- WORKER_ACTION\n")
	builder.WriteString("- WORKER_RESULT_READY\n\n")
	builder.WriteString("Do not wait for interactive permission prompts. Use the trusted local workspace policy configured by the worker.\n")
	return builder.String()
}

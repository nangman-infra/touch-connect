package tcworker

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	BackendAuto   = "auto"
	BackendClaude = "claude"
	BackendCodex  = "codex"
	BackendGemini = "gemini"
	BackendKiro   = "kiro"
)

type JoinOptions struct {
	ServerURL         string
	Backend           string
	Model             string
	Command           string
	Args              []string
	EndpointRef       string
	DisplayName       string
	ActorID           string
	WorkspaceID       string
	Role              string
	Capabilities      string
	Permission        string
	SkillsDir         string
	SkillPaths        []string
	WorkDir           string
	ArtifactDir       string
	Timeout           time.Duration
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	ProgressInterval  time.Duration
	MaxMessages       int
	Sandbox           string
}

type JoinEnvironment struct {
	Backend string
	Model   string
	Command string
	Args    []string
	Env     map[string]string
}

type backendPreset struct {
	Backend      string
	Command      string
	DefaultModel string
	DisplayName  string
	BuildArgs    func(model string, sandbox string) []string
}

func BuildJoinEnvironment(options JoinOptions) (JoinEnvironment, error) {
	accepted, err := options.withDefaults()
	if err != nil {
		return JoinEnvironment{}, err
	}
	preset, err := presetForBackend(accepted.Backend)
	if err != nil {
		return JoinEnvironment{}, err
	}
	command := strings.TrimSpace(accepted.Command)
	if command == "" {
		command, err = exec.LookPath(preset.Command)
		if err != nil {
			return JoinEnvironment{}, fmt.Errorf("%s backend requires %q on PATH; pass --command to override", preset.Backend, preset.Command)
		}
	}
	args := append([]string(nil), accepted.Args...)
	model := strings.TrimSpace(accepted.Model)
	if model == "" {
		model = preset.DefaultModel
	}
	if len(args) == 0 {
		args = preset.BuildArgs(model, accepted.Sandbox)
	}
	env := map[string]string{
		"TC_WORKER_SERVER_URL":         accepted.ServerURL,
		"TC_WORKER_ENDPOINT_REF":       accepted.EndpointRef,
		"TC_WORKER_DISPLAY_NAME":       accepted.DisplayName,
		"TC_WORKER_ACTOR_ID":           accepted.ActorID,
		"TC_WORKER_WORKSPACE_ID":       accepted.WorkspaceID,
		"TC_WORKER_ROLE":               accepted.Role,
		"TC_WORKER_BACKEND":            preset.Backend,
		"TC_WORKER_MODEL":              model,
		"TC_WORKER_PERMISSION":         accepted.Permission,
		"TC_WORKER_EXECUTOR":           "skill",
		"TC_WORKER_SKILL_BACKEND":      "ai-cli",
		"TC_WORKER_AI_CLI_COMMAND":     command,
		"TC_WORKER_AI_CLI_ARGS":        strings.Join(args, ","),
		"TC_WORKER_AI_CLI_WORKDIR":     accepted.WorkDir,
		"TC_WORKER_ARTIFACT_DIR":       accepted.ArtifactDir,
		"TC_WORKER_AI_CLI_TIMEOUT":     accepted.Timeout.String(),
		"TC_WORKER_POLL_INTERVAL":      accepted.PollInterval.String(),
		"TC_WORKER_HEARTBEAT_INTERVAL": accepted.HeartbeatInterval.String(),
		"TC_WORKER_PROGRESS_INTERVAL":  accepted.ProgressInterval.String(),
	}
	if accepted.Capabilities != "" {
		env["TC_WORKER_CAPABILITIES"] = accepted.Capabilities
	}
	if accepted.SkillsDir != "" {
		env["TC_WORKER_SKILLS_DIR"] = accepted.SkillsDir
	}
	if len(accepted.SkillPaths) > 0 {
		env["TC_WORKER_SKILL_PATHS"] = strings.Join(accepted.SkillPaths, ",")
	}
	if accepted.MaxMessages > 0 {
		env["TC_WORKER_MAX_MESSAGES"] = fmt.Sprintf("%d", accepted.MaxMessages)
	}
	return JoinEnvironment{Backend: preset.Backend, Model: model, Command: command, Args: args, Env: env}, nil
}

func (o JoinOptions) withDefaults() (JoinOptions, error) {
	var err error
	if o, err = o.defaultServerAndBackend(); err != nil {
		return JoinOptions{}, err
	}
	if o, err = o.defaultPaths(); err != nil {
		return JoinOptions{}, err
	}
	if o, err = o.defaultIdentityAndTiming(); err != nil {
		return JoinOptions{}, err
	}
	return o, nil
}

func (o JoinOptions) defaultServerAndBackend() (JoinOptions, error) {
	if strings.TrimSpace(o.ServerURL) == "" {
		o.ServerURL = "http://127.0.0.1:8080"
	}
	if err := validateWorkerServerURL(o.ServerURL); err != nil {
		return JoinOptions{}, err
	}
	o.Backend = strings.ToLower(strings.TrimSpace(o.Backend))
	if o.Backend == "" {
		o.Backend = BackendAuto
	}
	if o.Backend == BackendAuto {
		selected, err := detectBackend()
		if err != nil {
			return JoinOptions{}, err
		}
		o.Backend = selected
	}
	return o, nil
}

func validateWorkerServerURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("worker server URL must be an absolute http:// or https:// URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("worker server URL must use http:// or https://")
	}
	return nil
}

func (o JoinOptions) defaultPaths() (JoinOptions, error) {
	workDir, err := resolveJoinWorkDir(o.WorkDir)
	if err != nil {
		return JoinOptions{}, err
	}
	o.WorkDir = workDir

	o.SkillsDir, err = resolveJoinSkillsDir(o.SkillsDir, o.SkillPaths, o.WorkDir)
	if err != nil {
		return JoinOptions{}, err
	}
	o.SkillPaths, err = absoluteJoinSkillPaths(o.SkillPaths)
	if err != nil {
		return JoinOptions{}, err
	}
	if joinHasNoSkills(o) {
		return JoinOptions{}, errors.New("worker join requires --skills-dir or --skill")
	}

	o.ArtifactDir, err = resolveJoinArtifactDir(o.ArtifactDir, o.WorkDir, o.Backend)
	if err != nil {
		return JoinOptions{}, err
	}
	return o, nil
}

func resolveJoinWorkDir(path string) (string, error) {
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = wd
	}
	return filepath.Abs(path)
}

func resolveJoinSkillsDir(skillsDir string, skillPaths []string, workDir string) (string, error) {
	if skillsDir == "" && len(skillPaths) == 0 {
		skillsDir = existingDefaultJoinSkillsDir(workDir)
	}
	if skillsDir == "" {
		return "", nil
	}
	return filepath.Abs(skillsDir)
}

func existingDefaultJoinSkillsDir(workDir string) string {
	defaultSkillsDir := filepath.Join(workDir, "examples", "skills")
	if _, err := os.Stat(defaultSkillsDir); err == nil {
		return defaultSkillsDir
	}
	return ""
}

func absoluteJoinSkillPaths(paths []string) ([]string, error) {
	for index, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		paths[index] = absolute
	}
	return paths, nil
}

func joinHasNoSkills(o JoinOptions) bool {
	return o.SkillsDir == "" && len(o.SkillPaths) == 0
}

func resolveJoinArtifactDir(artifactDir string, workDir string, backend string) (string, error) {
	if artifactDir == "" {
		artifactDir = filepath.Join(workDir, ".touch-connect", "workers", safeJoinPart(backend), "artifacts")
	}
	return filepath.Abs(artifactDir)
}

func (o JoinOptions) defaultIdentityAndTiming() (JoinOptions, error) {
	if o.EndpointRef == "" {
		o.EndpointRef = defaultJoinEndpointRef(o.Backend)
	}
	if o.DisplayName == "" {
		o.DisplayName = joinTitle(o.Backend) + " worker"
	}
	if o.ActorID == "" {
		o.ActorID = "actor." + safeJoinPart(o.Backend) + "-worker"
	}
	if o.WorkspaceID == "" {
		o.WorkspaceID = "workspace.local"
	}
	if o.Role == "" {
		o.Role = DefaultWorkerRole
	}
	if o.Permission == "" {
		o.Permission = DefaultWorkerPermission
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Minute
	}
	if o.Timeout < 0 {
		return JoinOptions{}, errors.New("--timeout must not be negative")
	}
	if o.PollInterval == 0 {
		o.PollInterval = 500 * time.Millisecond
	}
	if o.PollInterval < 0 {
		return JoinOptions{}, errors.New("--poll-interval must not be negative")
	}
	if o.HeartbeatInterval == 0 {
		o.HeartbeatInterval = 5 * time.Second
	}
	if o.HeartbeatInterval < 0 {
		return JoinOptions{}, errors.New("--heartbeat-interval must not be negative")
	}
	if o.ProgressInterval == 0 {
		o.ProgressInterval = 30 * time.Second
	}
	if o.ProgressInterval < 0 {
		return JoinOptions{}, errors.New("--progress-interval must not be negative")
	}
	if o.Sandbox == "" {
		o.Sandbox = "danger-full-access"
	}
	return o, nil
}

func defaultJoinEndpointRef(backend string) string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "local"
	}
	return "tc://endpoint/" + safeJoinPart(backend) + "_" + safeJoinPart(host) + "_" + fmt.Sprintf("%d", os.Getpid())
}

func detectBackend() (string, error) {
	for _, candidate := range []string{BackendClaude, BackendCodex, BackendGemini, BackendKiro} {
		preset, _ := presetForBackend(candidate)
		if _, err := exec.LookPath(preset.Command); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("no supported AI CLI found on PATH; pass --backend and --command")
}

func presetForBackend(backend string) (backendPreset, error) {
	switch backend {
	case BackendClaude:
		return backendPreset{
			Backend:      BackendClaude,
			Command:      "claude",
			DefaultModel: "opus[1m]",
			DisplayName:  "Claude",
			BuildArgs: func(model string, _ string) []string {
				args := []string{"-p", "--permission-mode", "bypassPermissions"}
				if model != "" {
					args = append(args, "--model", model)
				}
				return args
			},
		}, nil
	case BackendCodex:
		return backendPreset{
			Backend:      BackendCodex,
			Command:      "codex",
			DefaultModel: "",
			DisplayName:  "Codex",
			BuildArgs: func(model string, sandbox string) []string {
				args := []string{"exec", "--skip-git-repo-check", "--sandbox", sandbox, "-c", "approval_policy=\"never\""}
				if model != "" {
					args = append(args, "-m", model)
				}
				return append(args, "-")
			},
		}, nil
	case BackendGemini:
		return backendPreset{
			Backend:      BackendGemini,
			Command:      "gemini",
			DefaultModel: "",
			DisplayName:  "Gemini",
			BuildArgs: func(model string, _ string) []string {
				args := []string{"-p", "{{prompt}}", "--approval-mode", "yolo"}
				if model != "" {
					args = append([]string{"--model", model}, args...)
				}
				return args
			},
		}, nil
	case BackendKiro:
		return backendPreset{
			Backend:      BackendKiro,
			Command:      "kiro-cli",
			DefaultModel: "",
			DisplayName:  "Kiro",
			BuildArgs: func(model string, _ string) []string {
				return []string{"chat", "--no-interactive", "--trust-all-tools", "{{prompt}}"}
			},
		}, nil
	default:
		return backendPreset{}, fmt.Errorf("unknown worker backend %q", backend)
	}
}

func safeJoinPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastUnderscore := false
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= '0' && item <= '9' {
			builder.WriteRune(item)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "ai"
	}
	return result
}

func joinTitle(value string) string {
	switch value {
	case BackendClaude:
		return "Claude"
	case BackendCodex:
		return "Codex"
	case BackendGemini:
		return "Gemini"
	case BackendKiro:
		return "Kiro"
	default:
		return "AI"
	}
}

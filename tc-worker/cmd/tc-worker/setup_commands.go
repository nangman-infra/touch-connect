package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

const defaultWorkerReleaseRepo = "nangman-infra/touch-connect"

type setupFlowOptions struct {
	ConfigPath     string
	Base           tcworker.JoinOptions
	AutoAccept     bool
	Plain          bool
	ConfirmLabel   string
	NonInteractive bool
}

func runSetup(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("tc-worker setup", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var options tcworker.JoinOptions
	var skillPaths repeatedFlag
	var configPath string
	var plain bool
	var yes bool
	flags.StringVar(&configPath, "config", "", "worker config path; default is ~/.touch-connect/worker/config.json")
	flags.StringVar(&options.ServerURL, "server-url", os.Getenv("TC_WORKER_SERVER_URL"), "tc-server URL")
	flags.StringVar(&options.ServerURL, "server", os.Getenv("TC_WORKER_SERVER_URL"), "alias for --server-url")
	flags.StringVar(&options.Backend, "backend", os.Getenv("TC_WORKER_BACKEND"), "AI CLI backend: auto, claude, codex, gemini, or kiro")
	flags.StringVar(&options.Model, "model", os.Getenv("TC_WORKER_MODEL"), "model override")
	flags.StringVar(&options.Command, "command", os.Getenv("TC_WORKER_AI_CLI_COMMAND"), "AI CLI command override")
	rawArgs := flags.String("args", os.Getenv("TC_WORKER_AI_CLI_ARGS"), "comma-separated AI CLI args override")
	flags.StringVar(&options.EndpointRef, "endpoint-ref", os.Getenv("TC_WORKER_ENDPOINT_REF"), "worker endpoint ref")
	flags.StringVar(&options.EndpointRef, "endpoint", os.Getenv("TC_WORKER_ENDPOINT_REF"), "alias for --endpoint-ref")
	flags.StringVar(&options.DisplayName, "display-name", os.Getenv("TC_WORKER_DISPLAY_NAME"), "worker display name")
	flags.StringVar(&options.ActorID, "actor-id", os.Getenv("TC_WORKER_ACTOR_ID"), "worker actor id")
	flags.StringVar(&options.WorkspaceID, "workspace-id", os.Getenv("TC_WORKER_WORKSPACE_ID"), "worker workspace id")
	flags.StringVar(&options.Role, "role", os.Getenv("TC_WORKER_ROLE"), "worker role")
	flags.StringVar(&options.Capabilities, "capabilities", os.Getenv("TC_WORKER_CAPABILITIES"), "comma-separated capabilities")
	flags.StringVar(&options.Permission, "permission", os.Getenv("TC_WORKER_PERMISSION"), "permission profile")
	flags.StringVar(&options.SkillsDir, "skills-dir", os.Getenv("TC_WORKER_SKILLS_DIR"), "directory containing SKILL.md files")
	flags.Var(&skillPaths, "skill", "SKILL.md path; repeatable")
	flags.StringVar(&options.WorkDir, "workdir", getenvDefault("TC_WORKER_AI_CLI_WORKDIR", os.Getenv("TC_WORKER_WORKDIR")), "AI CLI working directory")
	flags.StringVar(&options.ArtifactDir, "artifact-dir", os.Getenv("TC_WORKER_ARTIFACT_DIR"), "artifact output directory")
	flags.DurationVar(&options.Timeout, "timeout", durationFromEnv("TC_WORKER_AI_CLI_TIMEOUT"), "AI CLI execution timeout")
	flags.DurationVar(&options.PollInterval, "poll-interval", durationFromEnv("TC_WORKER_POLL_INTERVAL"), "message poll interval")
	flags.DurationVar(&options.HeartbeatInterval, "heartbeat-interval", durationFromEnv("TC_WORKER_HEARTBEAT_INTERVAL"), "endpoint heartbeat interval")
	flags.StringVar(&options.Sandbox, "sandbox", getenvDefault("TC_WORKER_SANDBOX", "danger-full-access"), "backend sandbox/profile hint where supported")
	flags.BoolVar(&plain, "plain", false, "disable interactive TUI-style chooser")
	flags.BoolVar(&yes, "yes", false, "accept defaults without prompting")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: tc-worker setup [flags]")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "creates ~/.touch-connect/worker/config.json and a default local worker SKILL.md")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	options.Args = splitArgList(*rawArgs)
	options.SkillPaths = skillPaths
	config, path, err := runSetupFlow(ctx, setupFlowOptions{
		ConfigPath:     configPath,
		Base:           options,
		AutoAccept:     yes,
		Plain:          plain,
		ConfirmLabel:   "Save worker config?",
		NonInteractive: yes || !isInteractiveTerminal(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "saved worker config: %s\n", path)
	fmt.Fprintf(os.Stdout, "backend=%s model=%s role=%s capabilities=%s\n", config.Backend, printable(config.Model), config.Role, strings.Join(config.Capabilities, ","))
	fmt.Fprintln(os.Stdout, "next: tc-worker join")
	return nil
}

func runSetupFlow(ctx context.Context, options setupFlowOptions) (tcworker.WorkerConfig, string, error) {
	base := options.Base
	base = applySetupDefaults(base)
	needsBackendChooser := strings.TrimSpace(base.Command) == "" && (strings.TrimSpace(base.Backend) == "" || strings.EqualFold(base.Backend, tcworker.BackendAuto) || !options.NonInteractive)
	if needsBackendChooser {
		resolved, err := tcworker.RunJoinWizard(ctx, tcworker.JoinWizardOptions{
			Input:        os.Stdin,
			Output:       os.Stdout,
			Base:         base,
			AutoAccept:   options.AutoAccept,
			UseTUI:       !options.Plain && !options.AutoAccept && isInteractiveTerminal(),
			ConfirmLabel: options.ConfirmLabel,
		})
		if err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		base = resolved
	}
	if !options.NonInteractive {
		reader := bufio.NewReader(os.Stdin)
		writer := os.Stdout
		var err error
		if base.ServerURL, err = promptLineDefault(reader, writer, "Server URL", base.ServerURL); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.Role, err = promptLineDefault(reader, writer, "Worker role", base.Role); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.Capabilities, err = promptLineDefault(reader, writer, "Capabilities", base.Capabilities); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.Permission, err = promptLineDefault(reader, writer, "Permission", base.Permission); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.SkillsDir, err = promptLineDefault(reader, writer, "Skills directory", base.SkillsDir); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.WorkDir, err = promptLineDefault(reader, writer, "Workspace directory", base.WorkDir); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
		if base.ArtifactDir, err = promptLineDefault(reader, writer, "Artifact directory", base.ArtifactDir); err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
	}
	config, err := tcworker.WorkerConfigFromJoinOptions(base)
	if err != nil {
		return tcworker.WorkerConfig{}, "", err
	}
	if err := tcworker.EnsureDefaultWorkerSkill(config.SkillsDir, config.Capabilities); err != nil {
		return tcworker.WorkerConfig{}, "", err
	}
	path := options.ConfigPath
	if strings.TrimSpace(path) == "" {
		path, err = tcworker.DefaultWorkerConfigPath()
		if err != nil {
			return tcworker.WorkerConfig{}, "", err
		}
	}
	if err := tcworker.SaveWorkerConfig(path, config); err != nil {
		return tcworker.WorkerConfig{}, "", err
	}
	return config, path, nil
}

func applySetupDefaults(options tcworker.JoinOptions) tcworker.JoinOptions {
	if strings.TrimSpace(options.ServerURL) == "" {
		options.ServerURL = "http://127.0.0.1:8080"
	}
	if strings.TrimSpace(options.Backend) == "" {
		options.Backend = tcworker.BackendAuto
	}
	if strings.TrimSpace(options.Role) == "" {
		options.Role = tcworker.DefaultWorkerRole
	}
	if strings.TrimSpace(options.Capabilities) == "" {
		options.Capabilities = "code.change,ai.review"
	}
	if strings.TrimSpace(options.Permission) == "" {
		options.Permission = tcworker.DefaultWorkerPermission
	}
	if strings.TrimSpace(options.WorkDir) == "" {
		if wd, err := os.Getwd(); err == nil {
			options.WorkDir = wd
		}
	}
	if strings.TrimSpace(options.SkillsDir) == "" && len(options.SkillPaths) == 0 {
		if skillsDir, err := tcworker.DefaultWorkerSkillsDir(); err == nil {
			options.SkillsDir = skillsDir
		}
	}
	if strings.TrimSpace(options.ArtifactDir) == "" {
		if artifactDir, err := tcworker.DefaultWorkerArtifactDir(); err == nil {
			options.ArtifactDir = artifactDir
		}
	}
	if options.Timeout == 0 {
		options.Timeout = 10 * time.Minute
	}
	if options.PollInterval == 0 {
		options.PollInterval = 500 * time.Millisecond
	}
	if options.HeartbeatInterval == 0 {
		options.HeartbeatInterval = 5 * time.Second
	}
	if strings.TrimSpace(options.Sandbox) == "" {
		options.Sandbox = "danger-full-access"
	}
	return options
}

func promptLineDefault(reader *bufio.Reader, writer io.Writer, label string, fallback string) (string, error) {
	fmt.Fprintf(writer, "%s [%s]: ", label, fallback)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func runDoctor(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("tc-worker doctor", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "worker config path")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	path := *configPath
	if strings.TrimSpace(path) == "" {
		defaultPath, err := tcworker.DefaultWorkerConfigPath()
		if err != nil {
			return err
		}
		path = defaultPath
	}
	fmt.Fprintf(os.Stdout, "tc-worker %s %s\n", version, commit)
	fmt.Fprintf(os.Stdout, "platform %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stdout, "config %s\n", path)
	if tcworker.WorkerConfigExists(path) {
		config, err := tcworker.LoadWorkerConfig(path)
		if err != nil {
			fmt.Fprintf(os.Stdout, "config_status invalid: %v\n", err)
		} else {
			fmt.Fprintf(os.Stdout, "config_status ready backend=%s model=%s role=%s capabilities=%s\n", config.Backend, printable(config.Model), config.Role, strings.Join(config.Capabilities, ","))
		}
	} else {
		fmt.Fprintln(os.Stdout, "config_status missing")
	}
	fmt.Fprintln(os.Stdout, "")
	printDoctorBackendCandidates(os.Stdout, tcworker.DetectJoinBackends(ctx))
	return nil
}

func runInstallOrUpdate(ctx context.Context, action string, args []string) error {
	flags := flag.NewFlagSet("tc-worker "+action, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	versionFlag := flags.String("version", "latest", "release version, for example worker-v0.1.0 or latest")
	dir := flags.String("dir", defaultInstallDir(), "installation directory")
	repo := flags.String("repo", defaultWorkerReleaseRepo, "GitHub owner/repo for worker releases")
	scriptURL := flags.String("script-url", "", "installer script URL override")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	url := strings.TrimSpace(*scriptURL)
	if url == "" {
		url = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/scripts/install-worker.sh", strings.TrimSpace(*repo))
	}
	path, err := downloadInstallerScript(ctx, url)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	command := exec.CommandContext(ctx, "sh", path)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	command.Env = append(os.Environ(),
		"TC_WORKER_VERSION="+strings.TrimSpace(*versionFlag),
		"TC_WORKER_REPO="+strings.TrimSpace(*repo),
		"TC_INSTALL_DIR="+expandInstallPath(*dir),
	)
	return command.Run()
}

func runUninstall(args []string) error {
	flags := flag.NewFlagSet("tc-worker uninstall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	purge := flags.Bool("purge", false, "remove ~/.touch-connect/worker config, skills, logs, and artifacts")
	force := flags.Bool("force", false, "allow removing the current executable even when it does not look installed")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if filepath.Base(executable) != "tc-worker" && !*force {
		return fmt.Errorf("refusing to remove %s; rerun with --force if this is the installed tc-worker binary", executable)
	}
	if err := os.Remove(executable); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintf(os.Stdout, "removed %s\n", executable)
	if *purge {
		configPath, err := tcworker.DefaultWorkerConfigPath()
		if err != nil {
			return err
		}
		dir := filepath.Dir(configPath)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "removed %s\n", dir)
	}
	return nil
}

func downloadInstallerScript(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download installer: %s returned %s", url, resp.Status)
	}
	file, err := os.CreateTemp("", "tc-worker-install-*.sh")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func defaultInstallDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/usr/local/bin"
	}
	return filepath.Join(home, ".local", "bin")
}

func expandInstallPath(path string) string {
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

func printable(value string) string {
	if strings.TrimSpace(value) == "" {
		return "default"
	}
	return value
}

func printDoctorBackendCandidates(w io.Writer, candidates []tcworker.BackendCandidate) {
	fmt.Fprintln(w, "Detected AI CLIs:")
	for index, candidate := range candidates {
		if candidate.Status == tcworker.BackendStatusMissing {
			fmt.Fprintf(w, "  %d. %-12s %-12s command=%s\n", index+1, candidate.DisplayName, candidate.Status, candidate.Command)
			continue
		}
		model := printable(candidate.RecommendedModel)
		detail := candidate.StatusDetail
		if detail == "" {
			detail = "installed"
		}
		fmt.Fprintf(w, "  %d. %-12s %-12s model=%s path=%s (%s)\n", index+1, candidate.DisplayName, candidate.Status, model, candidate.CommandPath, detail)
	}
}

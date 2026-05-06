package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

var (
	version = "0.1.0-dev"
	commit  = "unknown"
)

const (
	flagServerURL   = "server-url"
	flagEndpointRef = "endpoint-ref"
	flagSkillsDir   = "skills-dir"
)

type rootCommand func(context.Context, []string) error

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runRoot(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func runRoot(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return runEnvWorker(ctx)
	}
	if isVersionArg(args[0]) {
		writeVersion(os.Stdout)
		return nil
	}
	command, ok := rootCommands()[args[0]]
	if !ok {
		return runEnvWorker(ctx)
	}
	return command(ctx, args[1:])
}

func rootCommands() map[string]rootCommand {
	return map[string]rootCommand{
		"join":    runJoin,
		"setup":   runSetup,
		"doctor":  runDoctor,
		"install": installWorker,
		"update":  updateWorker,
		"uninstall": func(_ context.Context, args []string) error {
			return runUninstall(args)
		},
		"version": func(_ context.Context, _ []string) error {
			writeVersion(os.Stdout)
			return nil
		},
		"help": func(_ context.Context, _ []string) error {
			writeUsage(os.Stdout)
			return nil
		},
		"-h": func(_ context.Context, _ []string) error {
			writeUsage(os.Stdout)
			return nil
		},
		"--help": func(_ context.Context, _ []string) error {
			writeUsage(os.Stdout)
			return nil
		},
	}
}

func isVersionArg(arg string) bool {
	return arg == "--version" || arg == "-version"
}

func installWorker(ctx context.Context, args []string) error {
	return runInstallOrUpdate(ctx, "install", args)
}

func updateWorker(ctx context.Context, args []string) error {
	return runInstallOrUpdate(ctx, "update", args)
}

func runEnvWorker(ctx context.Context) error {
	serverURL := os.Getenv("TC_WORKER_SERVER_URL")
	messageRef := os.Getenv("TC_WORKER_MESSAGE_REF")
	if serverURL == "" {
		return fmt.Errorf("TC_WORKER_SERVER_URL is required")
	}
	if messageRef != "" {
		attemptRef, err := tcworker.RegisterAndProcess(ctx, serverURL, messageRef)
		if err != nil {
			return err
		}
		log.Printf("completed attempt %s", attemptRef)
		return nil
	}
	options, err := tcworker.LoopOptionsFromEnv()
	if err != nil {
		return err
	}
	return tcworker.RegisterAndRun(ctx, serverURL, options)
}

func runJoin(ctx context.Context, args []string) error {
	parsed, err := parseJoinArgs(args)
	if err != nil {
		return err
	}
	if parsed.Help {
		return nil
	}
	base, err := resolveJoinOptions(ctx, parsed)
	if err != nil {
		return err
	}
	if parsed.Wizard {
		base, err = runExplicitJoinWizard(ctx, base, parsed)
		if err != nil {
			return err
		}
	}
	env, err := tcworker.BuildJoinEnvironment(base)
	if err != nil {
		return err
	}
	if err := applyJoinEnvironment(env); err != nil {
		return err
	}
	if parsed.DryRun {
		printJoinDryRun(env)
		return nil
	}
	return startJoinedWorker(ctx, env, parsed.Plain)
}

type joinRunOptions struct {
	ConfigPath string
	Setup      bool
	Wizard     bool
	Plain      bool
	Yes        bool
	DryRun     bool
	Help       bool
	Visited    map[string]bool
	Options    tcworker.JoinOptions
}

func parseJoinArgs(args []string) (joinRunOptions, error) {
	flags := flag.NewFlagSet("tc-worker join", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var parsed joinRunOptions
	var skillPaths repeatedFlag
	options := &parsed.Options
	flags.StringVar(&parsed.ConfigPath, "config", "", "worker config path; default is ~/.touch-connect/worker/config.json")
	flags.BoolVar(&parsed.Setup, "setup", false, "run setup before joining when config is missing")
	flags.StringVar(&options.ServerURL, flagServerURL, os.Getenv("TC_WORKER_SERVER_URL"), "tc-server URL")
	flags.StringVar(&options.ServerURL, "server", os.Getenv("TC_WORKER_SERVER_URL"), "alias for --server-url")
	flags.StringVar(&options.Backend, "backend", os.Getenv("TC_WORKER_BACKEND"), "AI CLI backend: auto, claude, codex, gemini, or kiro")
	flags.StringVar(&options.Model, "model", os.Getenv("TC_WORKER_MODEL"), "model override passed to the selected backend")
	flags.StringVar(&options.Command, "command", os.Getenv("TC_WORKER_AI_CLI_COMMAND"), "AI CLI command override")
	rawArgs := flags.String("args", os.Getenv("TC_WORKER_AI_CLI_ARGS"), "comma-separated AI CLI args override")
	flags.StringVar(&options.EndpointRef, flagEndpointRef, os.Getenv("TC_WORKER_ENDPOINT_REF"), "worker endpoint ref")
	flags.StringVar(&options.EndpointRef, "endpoint", os.Getenv("TC_WORKER_ENDPOINT_REF"), "alias for --endpoint-ref")
	flags.StringVar(&options.DisplayName, "display-name", os.Getenv("TC_WORKER_DISPLAY_NAME"), "worker display name")
	flags.StringVar(&options.ActorID, "actor-id", os.Getenv("TC_WORKER_ACTOR_ID"), "worker actor id")
	flags.StringVar(&options.WorkspaceID, "workspace-id", os.Getenv("TC_WORKER_WORKSPACE_ID"), "worker workspace id")
	flags.StringVar(&options.Role, "role", os.Getenv("TC_WORKER_ROLE"), "human-assigned worker role, for example code-worker or reviewer")
	flags.StringVar(&options.Capabilities, "capabilities", os.Getenv("TC_WORKER_CAPABILITIES"), "comma-separated capability filter")
	flags.StringVar(&options.Permission, "permission", os.Getenv("TC_WORKER_PERMISSION"), "permission profile: auto-approve or manual")
	flags.StringVar(&options.SkillsDir, flagSkillsDir, os.Getenv("TC_WORKER_SKILLS_DIR"), "directory containing SKILL.md files")
	flags.Var(&skillPaths, "skill", "SKILL.md path; repeatable")
	flags.StringVar(&options.WorkDir, "workdir", getenvDefault("TC_WORKER_AI_CLI_WORKDIR", os.Getenv("TC_WORKER_WORKDIR")), "AI CLI working directory")
	flags.StringVar(&options.ArtifactDir, "artifact-dir", os.Getenv("TC_WORKER_ARTIFACT_DIR"), "artifact output directory")
	flags.DurationVar(&options.Timeout, "timeout", durationFromEnv("TC_WORKER_AI_CLI_TIMEOUT"), "AI CLI execution timeout")
	flags.DurationVar(&options.PollInterval, "poll-interval", durationFromEnv("TC_WORKER_POLL_INTERVAL"), "message poll interval")
	flags.DurationVar(&options.HeartbeatInterval, "heartbeat-interval", durationFromEnv("TC_WORKER_HEARTBEAT_INTERVAL"), "endpoint heartbeat interval")
	flags.IntVar(&options.MaxMessages, "max-messages", intFromEnv("TC_WORKER_MAX_MESSAGES"), "stop after processing this many messages; 0 means run until interrupted")
	flags.StringVar(&options.Sandbox, "sandbox", getenvDefault("TC_WORKER_SANDBOX", "danger-full-access"), "backend sandbox/profile hint where supported")
	flags.BoolVar(&parsed.Wizard, "wizard", false, "choose an installed AI CLI backend and model interactively without saving config")
	flags.BoolVar(&parsed.Plain, "plain", false, "disable the worker TUI and use plain text prompts/logs")
	flags.BoolVar(&parsed.Yes, "yes", false, "accept setup or wizard defaults without prompting")
	flags.BoolVar(&parsed.DryRun, "dry-run", false, "print resolved worker environment and exit")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: tc-worker join [flags]")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "normal path:")
		fmt.Fprintln(flags.Output(), "  tc-worker setup")
		fmt.Fprintln(flags.Output(), "  tc-worker join")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "if no config exists and the terminal is interactive, join runs setup once before starting")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			parsed.Help = true
			return parsed, nil
		}
		return joinRunOptions{}, err
	}
	parsed.Visited = visitedFlags(flags)
	options.Args = splitArgList(*rawArgs)
	options.SkillPaths = skillPaths
	return parsed, nil
}

func runExplicitJoinWizard(ctx context.Context, base tcworker.JoinOptions, parsed joinRunOptions) (tcworker.JoinOptions, error) {
	return tcworker.RunJoinWizard(ctx, tcworker.JoinWizardOptions{
		Input:        os.Stdin,
		Output:       os.Stdout,
		Base:         base,
		AutoAccept:   parsed.Yes,
		UseTUI:       !parsed.Plain && !parsed.Yes && isInteractiveTerminal(),
		ConfirmLabel: "Start worker?",
	})
}

func applyJoinEnvironment(env tcworker.JoinEnvironment) error {
	for key, value := range env.Env {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func printJoinDryRun(env tcworker.JoinEnvironment) {
	fmt.Printf("backend=%s\nmodel=%s\ncommand=%s\nargs=%s\n", env.Backend, env.Model, env.Command, strings.Join(env.Args, ","))
	for key, value := range env.Env {
		fmt.Printf("%s=%s\n", key, value)
	}
}

func startJoinedWorker(ctx context.Context, env tcworker.JoinEnvironment, plain bool) error {
	if !plain && isInteractiveTerminal() {
		return tcworker.RunWorkerStatusTUI(ctx, env, runEnvWorker)
	}
	log.Printf("tc-worker joining backend=%s model=%s endpoint=%s server=%s", env.Backend, env.Model, env.Env["TC_WORKER_ENDPOINT_REF"], env.Env["TC_WORKER_SERVER_URL"])
	return runEnvWorker(ctx)
}

func resolveJoinOptions(ctx context.Context, parsed joinRunOptions) (tcworker.JoinOptions, error) {
	var base tcworker.JoinOptions
	configExists := tcworker.WorkerConfigExists(parsed.ConfigPath)
	switch {
	case shouldSetupBeforeJoin(parsed, configExists):
		config, _, err := runSetupFlow(ctx, setupFlowOptions{
			ConfigPath:     parsed.ConfigPath,
			Base:           parsed.Options,
			AutoAccept:     parsed.Yes,
			Plain:          parsed.Plain,
			ConfirmLabel:   "Save worker config?",
			NonInteractive: parsed.Yes || !isInteractiveTerminal(),
		})
		if err != nil {
			return tcworker.JoinOptions{}, err
		}
		base, err = config.JoinOptions()
		if err != nil {
			return tcworker.JoinOptions{}, err
		}
	case configExists && !parsed.Wizard:
		config, err := tcworker.LoadWorkerConfig(parsed.ConfigPath)
		if err != nil {
			return tcworker.JoinOptions{}, err
		}
		base, err = config.JoinOptions()
		if err != nil {
			return tcworker.JoinOptions{}, err
		}
	case !configExists && !parsed.Wizard && !hasExplicitJoinInput(parsed.Visited):
		return tcworker.JoinOptions{}, fmt.Errorf("worker config not found; run tc-worker setup or pass explicit join flags")
	default:
		base = parsed.Options
	}
	return applyJoinFlagOverrides(base, parsed.Options, parsed.Visited), nil
}

func shouldSetupBeforeJoin(parsed joinRunOptions, configExists bool) bool {
	if parsed.Setup && !configExists {
		return true
	}
	return !parsed.Wizard && !configExists && !hasExplicitJoinInput(parsed.Visited) && isInteractiveTerminal()
}

func hasExplicitJoinInput(visited map[string]bool) bool {
	for _, name := range []string{"backend", "model", "command", "args", "skills-dir", "skill", "server", "server-url", "endpoint", "endpoint-ref", "capabilities", "role", "permission", "dry-run"} {
		if visited[name] {
			return true
		}
	}
	for _, key := range []string{"TC_WORKER_BACKEND", "TC_WORKER_AI_CLI_COMMAND", "TC_WORKER_MODEL", "TC_WORKER_SKILLS_DIR"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func applyJoinFlagOverrides(base tcworker.JoinOptions, flags tcworker.JoinOptions, visited map[string]bool) tcworker.JoinOptions {
	applyStringOverrides(&base, flags, visited)
	applyDurationOverrides(&base, flags, visited)
	applySliceOverrides(&base, flags, visited)
	return base
}

func applyStringOverrides(base *tcworker.JoinOptions, flags tcworker.JoinOptions, visited map[string]bool) {
	overrides := []struct {
		names []string
		set   func()
	}{
		{[]string{flagServerURL, "server"}, func() { base.ServerURL = flags.ServerURL }},
		{[]string{"backend"}, func() { base.Backend = flags.Backend }},
		{[]string{"model"}, func() { base.Model = flags.Model }},
		{[]string{"command"}, func() { base.Command = flags.Command }},
		{[]string{flagEndpointRef, "endpoint"}, func() { base.EndpointRef = flags.EndpointRef }},
		{[]string{"display-name"}, func() { base.DisplayName = flags.DisplayName }},
		{[]string{"actor-id"}, func() { base.ActorID = flags.ActorID }},
		{[]string{"workspace-id"}, func() { base.WorkspaceID = flags.WorkspaceID }},
		{[]string{"role"}, func() { base.Role = flags.Role }},
		{[]string{"capabilities"}, func() { base.Capabilities = flags.Capabilities }},
		{[]string{"permission"}, func() { base.Permission = flags.Permission }},
		{[]string{flagSkillsDir}, func() { base.SkillsDir = flags.SkillsDir }},
		{[]string{"workdir"}, func() { base.WorkDir = flags.WorkDir }},
		{[]string{"artifact-dir"}, func() { base.ArtifactDir = flags.ArtifactDir }},
		{[]string{"sandbox"}, func() { base.Sandbox = flags.Sandbox }},
	}
	for _, item := range overrides {
		if anyVisited(visited, item.names...) {
			item.set()
		}
	}
}

func applyDurationOverrides(base *tcworker.JoinOptions, flags tcworker.JoinOptions, visited map[string]bool) {
	if visited["timeout"] {
		base.Timeout = flags.Timeout
	}
	if visited["poll-interval"] {
		base.PollInterval = flags.PollInterval
	}
	if visited["heartbeat-interval"] {
		base.HeartbeatInterval = flags.HeartbeatInterval
	}
	if visited["max-messages"] {
		base.MaxMessages = flags.MaxMessages
	}
}

func applySliceOverrides(base *tcworker.JoinOptions, flags tcworker.JoinOptions, visited map[string]bool) {
	if visited["args"] {
		base.Args = append([]string(nil), flags.Args...)
	}
	if visited["skill"] {
		base.SkillPaths = append([]string(nil), flags.SkillPaths...)
	}
}

func anyVisited(visited map[string]bool, names ...string) bool {
	for _, name := range names {
		if visited[name] {
			return true
		}
	}
	return false
}

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*f = append(*f, value)
	}
	return nil
}

func splitArgList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func visitedFlags(flags *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	flags.Visit(func(item *flag.Flag) {
		visited[item.Name] = true
	})
	return visited
}

func isInteractiveTerminal() bool {
	return isTerminal(os.Stdin) && isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb"
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func getenvDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(key string) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return parsed
}

func intFromEnv(key string) int {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}
	var parsed int
	_, _ = fmt.Sscanf(value, "%d", &parsed)
	return parsed
}

func writeVersion(w io.Writer) {
	fmt.Fprintf(w, "tc-worker %s %s\n", version, commit)
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: tc-worker <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "normal worker lifecycle:")
	fmt.Fprintln(w, "  tc-worker install        install the latest released tc-worker binary")
	fmt.Fprintln(w, "  tc-worker setup          create ~/.touch-connect/worker/config.json")
	fmt.Fprintln(w, "  tc-worker join           start the configured local AI worker")
	fmt.Fprintln(w, "  tc-worker update         update the installed tc-worker binary")
	fmt.Fprintln(w, "  tc-worker uninstall      remove the installed tc-worker binary")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "advanced:")
	fmt.Fprintln(w, "  tc-worker join --backend claude --model 'opus[1m]' --server http://127.0.0.1:8080 --role code-worker --capabilities code.change,ai.review --permission auto-approve")
	fmt.Fprintln(w, "  tc-worker doctor")
	fmt.Fprintln(w, "  tc-worker version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "without a command, tc-worker keeps the legacy TC_WORKER_* environment contract")
}

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "join":
			if err := runJoin(ctx, os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		case "help", "-h", "--help":
			writeUsage(os.Stdout)
			return
		}
	}
	if err := runEnvWorker(ctx); err != nil {
		log.Fatal(err)
	}
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
	flags := flag.NewFlagSet("tc-worker join", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var skillPaths repeatedFlag
	serverURL := flags.String("server-url", getenvDefault("TC_WORKER_SERVER_URL", "http://127.0.0.1:8080"), "tc-server URL")
	backend := flags.String("backend", getenvDefault("TC_WORKER_BACKEND", tcworker.BackendAuto), "AI CLI backend: auto, claude, codex, gemini, or kiro")
	model := flags.String("model", os.Getenv("TC_WORKER_MODEL"), "model override passed to the selected backend")
	command := flags.String("command", os.Getenv("TC_WORKER_AI_CLI_COMMAND"), "AI CLI command override")
	rawArgs := flags.String("args", os.Getenv("TC_WORKER_AI_CLI_ARGS"), "comma-separated AI CLI args override")
	endpointRef := flags.String("endpoint-ref", os.Getenv("TC_WORKER_ENDPOINT_REF"), "worker endpoint ref")
	displayName := flags.String("display-name", os.Getenv("TC_WORKER_DISPLAY_NAME"), "worker display name")
	actorID := flags.String("actor-id", os.Getenv("TC_WORKER_ACTOR_ID"), "worker actor id")
	workspaceID := flags.String("workspace-id", os.Getenv("TC_WORKER_WORKSPACE_ID"), "worker workspace id")
	capabilities := flags.String("capabilities", os.Getenv("TC_WORKER_CAPABILITIES"), "comma-separated capability filter")
	skillsDir := flags.String("skills-dir", os.Getenv("TC_WORKER_SKILLS_DIR"), "absolute or relative directory containing SKILL.md files")
	flags.Var(&skillPaths, "skill", "absolute or relative SKILL.md path; repeatable")
	workDir := flags.String("workdir", getenvDefault("TC_WORKER_AI_CLI_WORKDIR", os.Getenv("TC_WORKER_WORKDIR")), "AI CLI working directory")
	artifactDir := flags.String("artifact-dir", os.Getenv("TC_WORKER_ARTIFACT_DIR"), "artifact output directory")
	timeout := flags.Duration("timeout", durationFromEnv("TC_WORKER_AI_CLI_TIMEOUT"), "AI CLI execution timeout")
	pollInterval := flags.Duration("poll-interval", durationFromEnv("TC_WORKER_POLL_INTERVAL"), "message poll interval")
	heartbeatInterval := flags.Duration("heartbeat-interval", durationFromEnv("TC_WORKER_HEARTBEAT_INTERVAL"), "endpoint heartbeat interval")
	maxMessages := flags.Int("max-messages", intFromEnv("TC_WORKER_MAX_MESSAGES"), "stop after processing this many messages; 0 means run until interrupted")
	sandbox := flags.String("sandbox", "read-only", "backend sandbox/profile hint where supported")
	wizard := flags.Bool("wizard", false, "choose an installed AI CLI backend and model interactively")
	plain := flags.Bool("plain", false, "disable the worker TUI and use plain text prompts/logs")
	yes := flags.Bool("yes", false, "accept wizard defaults without prompting")
	dryRun := flags.Bool("dry-run", false, "print resolved worker environment and exit")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: tc-worker join [flags]")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "with no explicit backend/model/command in an interactive terminal, join starts the worker wizard")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	visited := visitedFlags(flags)
	options := tcworker.JoinOptions{
		ServerURL:         *serverURL,
		Backend:           *backend,
		Model:             *model,
		Command:           *command,
		Args:              splitArgList(*rawArgs),
		EndpointRef:       *endpointRef,
		DisplayName:       *displayName,
		ActorID:           *actorID,
		WorkspaceID:       *workspaceID,
		Capabilities:      *capabilities,
		SkillsDir:         *skillsDir,
		SkillPaths:        skillPaths,
		WorkDir:           *workDir,
		ArtifactDir:       *artifactDir,
		Timeout:           *timeout,
		PollInterval:      *pollInterval,
		HeartbeatInterval: *heartbeatInterval,
		MaxMessages:       *maxMessages,
		Sandbox:           *sandbox,
	}
	runWizard := shouldRunJoinWizard(*wizard, visited)
	useTUI := runWizard && !*plain && !*yes && isTerminal(os.Stdin) && isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb"
	if runWizard {
		resolved, err := tcworker.RunJoinWizard(ctx, tcworker.JoinWizardOptions{
			Input:      os.Stdin,
			Output:     os.Stdout,
			Base:       options,
			AutoAccept: *yes,
			UseTUI:     useTUI,
		})
		if err != nil {
			return err
		}
		options = resolved
	}
	env, err := tcworker.BuildJoinEnvironment(options)
	if err != nil {
		return err
	}
	for key, value := range env.Env {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if *dryRun {
		fmt.Printf("backend=%s\nmodel=%s\ncommand=%s\nargs=%s\n", env.Backend, env.Model, env.Command, strings.Join(env.Args, ","))
		for key, value := range env.Env {
			fmt.Printf("%s=%s\n", key, value)
		}
		return nil
	}
	if useTUI {
		return tcworker.RunWorkerStatusTUI(ctx, env, runEnvWorker)
	}
	log.Printf("tc-worker joining backend=%s model=%s endpoint=%s server=%s", env.Backend, env.Model, env.Env["TC_WORKER_ENDPOINT_REF"], env.Env["TC_WORKER_SERVER_URL"])
	return runEnvWorker(ctx)
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

func shouldRunJoinWizard(force bool, visited map[string]bool) bool {
	if force {
		return true
	}
	if len(os.Args) > 2 {
		if visited["backend"] || visited["model"] || visited["command"] || visited["args"] || visited["dry-run"] {
			return false
		}
	}
	if os.Getenv("TC_WORKER_BACKEND") != "" || os.Getenv("TC_WORKER_AI_CLI_COMMAND") != "" || os.Getenv("TC_WORKER_MODEL") != "" {
		return false
	}
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
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

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: tc-worker [join] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  join      start a skill-guided local AI CLI worker; interactive terminals can use a backend/model wizard")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "without a command, tc-worker keeps the legacy TC_WORKER_* environment contract")
}

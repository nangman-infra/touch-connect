package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestRunRootVersionHelpAndLegacyEnvFallback(t *testing.T) {
	clearWorkerEnvForTest(t)

	versionOutput := captureStdout(t, func() error {
		return runRoot(context.Background(), []string{"version"})
	})
	if !strings.Contains(versionOutput, "tc-worker") {
		t.Fatalf("expected version output, got %q", versionOutput)
	}

	helpOutput := captureStdout(t, func() error {
		return runRoot(context.Background(), []string{"help"})
	})
	if !strings.Contains(helpOutput, "tc-worker setup") || !strings.Contains(helpOutput, "tc-worker join") {
		t.Fatalf("expected lifecycle help, got %q", helpOutput)
	}
	dashVersionOutput := captureStdout(t, func() error {
		return runRoot(context.Background(), []string{"--version"})
	})
	if !strings.Contains(dashVersionOutput, "tc-worker") {
		t.Fatalf("expected --version output, got %q", dashVersionOutput)
	}
	flagHelpOutput := captureStdout(t, func() error {
		return rootCommands()["--help"](context.Background(), nil)
	})
	if !strings.Contains(flagHelpOutput, "normal worker lifecycle") {
		t.Fatalf("expected --help command output, got %q", flagHelpOutput)
	}
	if err := rootCommands()["uninstall"](context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("uninstall help should not fail: %v", err)
	}
	if err := runSetup(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("setup help should not continue into setup flow: %v", err)
	}
	if err := runJoin(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("join help should not continue into worker startup: %v", err)
	}

	t.Setenv("TC_WORKER_SERVER_URL", "")
	if err := runRoot(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "TC_WORKER_SERVER_URL is required") {
		t.Fatalf("expected no-arg legacy env worker error, got %v", err)
	}
	err := runRoot(context.Background(), []string{"unknown-command"})
	if err == nil || !strings.Contains(err.Error(), "TC_WORKER_SERVER_URL is required") {
		t.Fatalf("expected legacy env worker error, got %v", err)
	}
}

func TestInstallAndUpdateWrappersUseInstaller(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	t.Setenv("TC_WORKER_TEST_MARKER", marker)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#!/bin/sh\nset -eu\nprintf '%s|%s' \"$TC_WORKER_VERSION\" \"$TC_INSTALL_DIR\" >> \"$TC_WORKER_TEST_MARKER\"\n"))
	}))
	defer server.Close()

	if err := installWorker(context.Background(), []string{"--script-url", server.URL, "--version", "worker-v1", "--dir", dir}); err != nil {
		t.Fatalf("install wrapper: %v", err)
	}
	if err := updateWorker(context.Background(), []string{"--script-url", server.URL, "--version", "worker-v2", "--dir", dir}); err != nil {
		t.Fatalf("update wrapper: %v", err)
	}
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if got := string(body); !strings.Contains(got, "worker-v1|"+dir) || !strings.Contains(got, "worker-v2|"+dir) {
		t.Fatalf("unexpected installer marker: %s", got)
	}
}

func TestRunJoinDryRunWithExplicitFlags(t *testing.T) {
	clearWorkerEnvForTest(t)

	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("create skills dir: %v", err)
	}
	skillPath := filepath.Join(dir, "worker.SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# worker\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	output := captureStdout(t, func() error {
		return runJoin(context.Background(), []string{
			"--dry-run",
			"--plain",
			"--server", "http://127.0.0.1:8080",
			"--backend", "claude",
			"--model", "opus[1m]",
			"--command", "/bin/echo",
			"--args", "-p,--model,opus",
			"--endpoint", "tc://endpoint/test_worker",
			"--display-name", "Test Worker",
			"--actor-id", "actor.test",
			"--workspace-id", "workspace.test",
			"--role", "reviewer",
			"--capabilities", "code.change,ai.review",
			"--permission", "auto-approve",
			"--skills-dir", skillsDir,
			"--skill", skillPath,
			"--workdir", dir,
			"--artifact-dir", filepath.Join(dir, "artifacts"),
			"--timeout", "2m",
			"--poll-interval", "250ms",
			"--heartbeat-interval", "3s",
			"--max-messages", "2",
			"--sandbox", "workspace-write",
		})
	})

	for _, expected := range []string{
		"backend=claude",
		"model=opus[1m]",
		"command=/bin/echo",
		"TC_WORKER_ENDPOINT_REF=tc://endpoint/test_worker",
		"TC_WORKER_ROLE=reviewer",
		"TC_WORKER_PERMISSION=auto-approve",
		"TC_WORKER_AI_CLI_ARGS=-p,--model,opus",
		"TC_WORKER_MAX_MESSAGES=2",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to contain %q\n%s", expected, output)
		}
	}
}

func TestJoinSetupBranchAndStartPlainError(t *testing.T) {
	clearWorkerEnvForTest(t)
	dir := t.TempDir()
	options, err := resolveJoinOptions(context.Background(), joinRunOptions{
		ConfigPath: filepath.Join(dir, "config.json"),
		Setup:      true,
		Plain:      true,
		Yes:        true,
		Visited:    map[string]bool{},
		Options: tcworker.JoinOptions{
			ServerURL:    tcworker.DefaultWorkerServerURL,
			Backend:      tcworker.BackendClaude,
			Command:      "/bin/echo",
			SkillsDir:    filepath.Join(dir, "skills"),
			WorkDir:      dir,
			ArtifactDir:  filepath.Join(dir, "artifacts"),
			Capabilities: "code.change",
			Permission:   tcworker.DefaultWorkerPermission,
		},
	})
	if err != nil {
		t.Fatalf("resolve join setup branch: %v", err)
	}
	if options.Backend != tcworker.BackendClaude || options.Command != "/bin/echo" {
		t.Fatalf("unexpected options from setup branch: %+v", options)
	}
	env, err := tcworker.BuildJoinEnvironment(options)
	if err != nil {
		t.Fatalf("build env: %v", err)
	}
	if err := applyJoinEnvironment(env); err != nil {
		t.Fatalf("apply env: %v", err)
	}
	t.Setenv("TC_WORKER_SERVER_URL", "")
	err = startJoinedWorker(context.Background(), env, true)
	if err == nil || !strings.Contains(err.Error(), "TC_WORKER_SERVER_URL is required") {
		t.Fatalf("expected plain worker start to use env contract, got %v", err)
	}
}

func TestParseJoinArgsAndSmallHelpers(t *testing.T) {
	clearWorkerEnvForTest(t)
	t.Setenv("TC_WORKER_AI_CLI_ARGS", " -p , --model , opus ")
	t.Setenv("TC_WORKER_AI_CLI_TIMEOUT", "7s")
	t.Setenv("TC_WORKER_POLL_INTERVAL", "125ms")
	t.Setenv("TC_WORKER_HEARTBEAT_INTERVAL", "2s")
	t.Setenv("TC_WORKER_MAX_MESSAGES", "3")

	parsed, err := parseJoinArgs([]string{"--backend", "codex", "--skill", "a.md", "--skill", "b.md"})
	if err != nil {
		t.Fatalf("parse join args: %v", err)
	}
	if !parsed.Visited["backend"] || !parsed.Visited["skill"] {
		t.Fatalf("expected visited flags, got %+v", parsed.Visited)
	}
	if got := strings.Join(parsed.Options.Args, "|"); got != "-p|--model|opus" {
		t.Fatalf("unexpected parsed args: %q", got)
	}
	if parsed.Options.Timeout != 7*time.Second || parsed.Options.PollInterval != 125*time.Millisecond || parsed.Options.HeartbeatInterval != 2*time.Second {
		t.Fatalf("unexpected durations: %+v", parsed.Options)
	}
	if parsed.Options.MaxMessages != 3 {
		t.Fatalf("unexpected max messages: %d", parsed.Options.MaxMessages)
	}

	if !isVersionArg("--version") || !isVersionArg("-version") || isVersionArg("version") {
		t.Fatalf("version arg helper mismatch")
	}
	if got := getenvDefault("TC_MISSING_FOR_TEST", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback env, got %q", got)
	}
	if got := splitArgList(" a, ,b , c "); strings.Join(got, "|") != "a|b|c" {
		t.Fatalf("unexpected split args: %#v", got)
	}

	var repeated repeatedFlag
	if err := repeated.Set(" alpha "); err != nil {
		t.Fatalf("set repeated flag: %v", err)
	}
	if err := repeated.Set(""); err != nil {
		t.Fatalf("set blank repeated flag: %v", err)
	}
	if repeated.String() != "alpha" {
		t.Fatalf("unexpected repeated flag string: %q", repeated.String())
	}
	if err := runJoin(context.Background(), []string{"--bad-flag"}); err == nil {
		t.Fatalf("expected bad join flag to fail")
	}
}

func TestRunSetupFlowCreatesConfigAndDefaultSkill(t *testing.T) {
	dir := t.TempDir()
	config, path, err := runSetupFlow(context.Background(), setupFlowOptions{
		ConfigPath: filepath.Join(dir, "config.json"),
		Base: tcworker.JoinOptions{
			ServerURL:    "http://127.0.0.1:8080",
			Backend:      tcworker.BackendClaude,
			Command:      "/bin/echo",
			SkillsDir:    filepath.Join(dir, "skills"),
			WorkDir:      dir,
			ArtifactDir:  filepath.Join(dir, "artifacts"),
			Capabilities: "code.change,ai.review",
			Role:         "code-worker",
			Permission:   tcworker.DefaultWorkerPermission,
		},
		AutoAccept:     true,
		NonInteractive: true,
	})
	if err != nil {
		t.Fatalf("setup flow: %v", err)
	}
	if path != filepath.Join(dir, "config.json") || config.Backend != tcworker.BackendClaude || config.Model != "opus[1m]" {
		t.Fatalf("unexpected setup result path=%s config=%+v", path, config)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "local-ai-worker", "SKILL.md")); err != nil {
		t.Fatalf("default skill was not created: %v", err)
	}
}

func TestRunSetupAndDoctorUseConfigSurface(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	skillsDir := filepath.Join(dir, "skills")
	setupOutput := captureStdout(t, func() error {
		return runSetup(context.Background(), []string{
			"--yes",
			"--plain",
			"--config", configPath,
			"--backend", "claude",
			"--model", "opus[1m]",
			"--command", "/bin/echo",
			"--server-url", "http://127.0.0.1:8080",
			"--endpoint-ref", "tc://endpoint/claude_worker",
			"--role", "code-worker",
			"--capabilities", "code.change,ai.review",
			"--permission", "auto-approve",
			"--skills-dir", skillsDir,
			"--workdir", dir,
			"--artifact-dir", filepath.Join(dir, "artifacts"),
			"--timeout", "1m",
			"--poll-interval", "250ms",
			"--heartbeat-interval", "2s",
			"--sandbox", "danger-full-access",
		})
	})
	if !strings.Contains(setupOutput, "saved worker config") || !strings.Contains(setupOutput, "backend=claude") {
		t.Fatalf("unexpected setup output: %s", setupOutput)
	}

	doctorOutput := captureStdout(t, func() error {
		return runDoctor(context.Background(), []string{"--config", configPath})
	})
	if !strings.Contains(doctorOutput, "config_status ready") || !strings.Contains(doctorOutput, "Detected AI CLIs") {
		t.Fatalf("unexpected doctor output: %s", doctorOutput)
	}
}

func TestSetupDefaultsPromptsAndPathHelpers(t *testing.T) {
	dir := t.TempDir()
	options := applySetupDefaults(tcworker.JoinOptions{WorkDir: dir})
	if options.ServerURL != "http://127.0.0.1:8080" || options.Backend != tcworker.BackendAuto {
		t.Fatalf("unexpected setup defaults: %+v", options)
	}
	if options.Timeout != 10*time.Minute || options.PollInterval != 500*time.Millisecond || options.HeartbeatInterval != 5*time.Second {
		t.Fatalf("unexpected timing defaults: %+v", options)
	}
	if !setupNeedsBackendChooser(tcworker.JoinOptions{}, false) {
		t.Fatalf("expected empty setup to need backend chooser")
	}
	if setupNeedsBackendChooser(tcworker.JoinOptions{Backend: tcworker.BackendClaude}, false) {
		t.Fatalf("explicit backend should skip backend chooser")
	}
	if setupNeedsBackendChooser(tcworker.JoinOptions{Command: "/bin/echo"}, false) {
		t.Fatalf("command override should skip backend chooser")
	}
	if got := printable(""); got != "default" {
		t.Fatalf("unexpected printable blank: %q", got)
	}
	if got := printable("opus"); got != "opus" {
		t.Fatalf("unexpected printable value: %q", got)
	}
}

func TestSetupPromptCatalogAndInputHelpers(t *testing.T) {
	fallback, err := promptLineDefault(bufioReader(" \n"), io.Discard, "Role", "code-worker")
	if err != nil || fallback != "code-worker" {
		t.Fatalf("expected fallback prompt value=%q err=%v", fallback, err)
	}
	custom, err := promptLineDefault(bufioReader("reviewer\n"), io.Discard, "Role", "code-worker")
	if err != nil || custom != "reviewer" {
		t.Fatalf("expected custom prompt value=%q err=%v", custom, err)
	}

	if got := expandInstallPath("/opt/bin"); got != "/opt/bin" {
		t.Fatalf("unexpected absolute install path: %q", got)
	}
	if len(setupPrompts()) != 7 {
		t.Fatalf("expected setup prompt catalog to stay stable")
	}
	mutated := tcworker.JoinOptions{}
	for _, prompt := range setupPrompts() {
		prompt.Set(&mutated, "value")
		if prompt.Get(mutated) == "" {
			t.Fatalf("expected prompt %s to read written value", prompt.Label)
		}
	}
}

func TestSetupPathAndParseHelpers(t *testing.T) {
	if defaultWorkDir() == "" || defaultSkillsDir() == "" || defaultArtifactDir() == "" {
		t.Fatalf("expected default path helpers to return non-empty paths")
	}
	if _, err := parseSetupArgs([]string{"--bad-flag"}); err == nil {
		t.Fatalf("expected bad setup flag to fail")
	}
	if err := runSetup(context.Background(), []string{"--bad-flag"}); err == nil {
		t.Fatalf("expected run setup with bad flag to fail")
	}
	if _, err := parseSetupArgs([]string{"--help"}); err != nil {
		t.Fatalf("setup help should not fail: %v", err)
	}
}

func TestResolveJoinOptionsLoadsConfigAndAppliesOverrides(t *testing.T) {
	clearWorkerEnvForTest(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := tcworker.SaveWorkerConfig(configPath, tcworker.WorkerConfig{
		Backend:      tcworker.BackendCodex,
		Model:        "gpt-5.4-mini",
		ServerURL:    "http://127.0.0.1:8080",
		EndpointRef:  "tc://endpoint/codex_worker",
		Role:         "code-worker",
		Capabilities: []string{"code.change"},
		Permission:   tcworker.DefaultWorkerPermission,
		Command:      "/bin/echo",
		SkillsDir:    filepath.Join(dir, "skills"),
		WorkDir:      dir,
		ArtifactDir:  filepath.Join(dir, "artifacts"),
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	options, err := resolveJoinOptions(context.Background(), joinRunOptions{
		ConfigPath: configPath,
		Plain:      true,
		Yes:        true,
		Visited: map[string]bool{
			"role":         true,
			"capabilities": true,
		},
		Options: tcworker.JoinOptions{
			Role:         "reviewer",
			Capabilities: "ai.review",
		},
	})
	if err != nil {
		t.Fatalf("resolve join options: %v", err)
	}
	if options.Role != "reviewer" || options.Capabilities != "ai.review" || options.Backend != tcworker.BackendCodex {
		t.Fatalf("unexpected resolved options: %+v", options)
	}
}

func TestResolveJoinOptionsInitializesDefaultConfigWhenMissing(t *testing.T) {
	clearWorkerEnvForTest(t)
	stubWorkerServerDiscovery(t, "http://192.168.10.34:8080")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	skillsDir := filepath.Join(dir, "skills")
	options, err := resolveJoinOptions(context.Background(), joinRunOptions{
		ConfigPath: configPath,
		Plain:      true,
		Visited:    map[string]bool{},
		Options: tcworker.JoinOptions{
			Backend:     tcworker.BackendClaude,
			Command:     "/bin/echo",
			SkillsDir:   skillsDir,
			WorkDir:     dir,
			ArtifactDir: filepath.Join(dir, "artifacts"),
		},
	})
	if err != nil {
		t.Fatalf("resolve join options: %v", err)
	}
	if options.Backend != tcworker.BackendClaude || options.Command != "/bin/echo" || options.Role != tcworker.DefaultWorkerRole {
		t.Fatalf("unexpected initialized join options: %+v", options)
	}
	if options.ServerURL != "http://192.168.10.34:8080" {
		t.Fatalf("expected discovered server URL, got %+v", options)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected missing config to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "local-ai-worker", "SKILL.md")); err != nil {
		t.Fatalf("expected default worker skill to be created: %v", err)
	}
}

func TestRunSetupFlowDiscoversServerURL(t *testing.T) {
	clearWorkerEnvForTest(t)
	stubWorkerServerDiscovery(t, "http://192.168.10.44:8080")

	dir := t.TempDir()
	config, _, err := runSetupFlow(context.Background(), setupFlowOptions{
		ConfigPath: filepath.Join(dir, "config.json"),
		Base: tcworker.JoinOptions{
			Backend:      tcworker.BackendClaude,
			Command:      "/bin/echo",
			SkillsDir:    filepath.Join(dir, "skills"),
			WorkDir:      dir,
			ArtifactDir:  filepath.Join(dir, "artifacts"),
			Capabilities: "code.change",
			Permission:   tcworker.DefaultWorkerPermission,
		},
		AutoAccept:     true,
		NonInteractive: true,
		Visited:        map[string]bool{},
	})
	if err != nil {
		t.Fatalf("setup flow with discovery: %v", err)
	}
	if config.ServerURL != "http://192.168.10.44:8080" {
		t.Fatalf("expected discovered server URL in config, got %+v", config)
	}
}

func TestRunInstallOrUpdateUsesDownloadedInstaller(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	t.Setenv("TC_WORKER_TEST_MARKER", marker)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#!/bin/sh\nset -eu\nprintf '%s|%s|%s' \"$TC_WORKER_VERSION\" \"$TC_WORKER_REPO\" \"$TC_INSTALL_DIR\" > \"$TC_WORKER_TEST_MARKER\"\n"))
	}))
	defer server.Close()
	if err := runInstallOrUpdate(context.Background(), "install", []string{"--script-url", server.URL, "--version", "worker-v0.1.0", "--repo", "owner/repo", "--dir", dir}); err != nil {
		t.Fatalf("install command: %v", err)
	}
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if got := string(body); got != "worker-v0.1.0|owner/repo|"+dir {
		t.Fatalf("unexpected installer env: %s", got)
	}
}

func TestRunUninstallRefusesGoRunExecutable(t *testing.T) {
	err := runUninstall(nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to remove") {
		t.Fatalf("expected refusal for test executable, got %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	err = fn()
	_ = writer.Close()
	os.Stdout = original
	if err != nil {
		t.Fatalf("captured command failed: %v", err)
	}
	var buffer bytes.Buffer
	if _, copyErr := io.Copy(&buffer, reader); copyErr != nil {
		t.Fatalf("read stdout pipe: %v", copyErr)
	}
	return buffer.String()
}

func bufioReader(value string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(value))
}

func clearWorkerEnvForTest(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TC_WORKER_SERVER_URL",
		"TC_WORKER_BACKEND",
		"TC_WORKER_AI_CLI_COMMAND",
		"TC_WORKER_AI_CLI_ARGS",
		"TC_WORKER_MODEL",
		"TC_WORKER_SKILLS_DIR",
		"TC_WORKER_ENDPOINT_REF",
		"TC_WORKER_DISPLAY_NAME",
		"TC_WORKER_ACTOR_ID",
		"TC_WORKER_WORKSPACE_ID",
		"TC_WORKER_ROLE",
		"TC_WORKER_CAPABILITIES",
		"TC_WORKER_PERMISSION",
		"TC_WORKER_AI_CLI_WORKDIR",
		"TC_WORKER_WORKDIR",
		"TC_WORKER_ARTIFACT_DIR",
		"TC_WORKER_AI_CLI_TIMEOUT",
		"TC_WORKER_POLL_INTERVAL",
		"TC_WORKER_HEARTBEAT_INTERVAL",
		"TC_WORKER_MAX_MESSAGES",
		"TC_WORKER_SANDBOX",
	} {
		t.Setenv(key, "")
	}
}

func stubWorkerServerDiscovery(t *testing.T, url string) {
	t.Helper()
	original := discoverWorkerServerURL
	discoverWorkerServerURL = func(context.Context, tcworker.ServerDiscoveryOptions) (string, []tcworker.ServerCandidate) {
		if url == "" {
			return "", nil
		}
		return url, []tcworker.ServerCandidate{{URL: url, Source: "test", Status: "ok", Component: "tc-server"}}
	}
	t.Cleanup(func() {
		discoverWorkerServerURL = original
	})
}

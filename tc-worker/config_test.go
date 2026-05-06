package tcworker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestWorkerConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	config := WorkerConfig{
		Backend:      BackendClaude,
		Model:        "opus[1m]",
		ServerURL:    "http://127.0.0.1:8080",
		EndpointRef:  "tc://endpoint/claude_worker",
		Role:         "code-worker",
		Capabilities: []string{"code.change", "ai.review"},
		Permission:   DefaultWorkerPermission,
		Command:      "/bin/echo",
		SkillsDir:    filepath.Join(dir, "skills"),
		WorkDir:      dir,
		ArtifactDir:  filepath.Join(dir, "artifacts"),
		Timeout:      (10 * time.Minute).String(),
	}
	if err := SaveWorkerConfig(path, config); err != nil {
		t.Fatalf("save config: %v", err)
	}
	loaded, err := LoadWorkerConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Backend != BackendClaude || loaded.Role != "code-worker" || loaded.Permission != DefaultWorkerPermission {
		t.Fatalf("unexpected config: %+v", loaded)
	}
	options, err := loaded.JoinOptions()
	if err != nil {
		t.Fatalf("join options: %v", err)
	}
	if options.Capabilities != "code.change,ai.review" || options.Timeout != 10*time.Minute {
		t.Fatalf("unexpected join options: %+v", options)
	}
}

func TestEnsureDefaultWorkerSkillCreatesSkillContract(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDefaultWorkerSkill(dir, []string{"code.change"}); err != nil {
		t.Fatalf("ensure skill: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "local-ai-worker", "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	text := string(body)
	for _, want := range []string{"tc://skill/local-ai-worker", "code.change", "WORKER_READBACK", "WORKER_RESULT_READY"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated skill to contain %q, got:\n%s", want, text)
		}
	}
}

func TestWorkerConfigFromJoinOptionsKeepsRoleAndPermission(t *testing.T) {
	dir := t.TempDir()
	config, err := WorkerConfigFromJoinOptions(JoinOptions{
		ServerURL:    "http://127.0.0.1:8080",
		Backend:      BackendCodex,
		Model:        "gpt-5.4-mini",
		Command:      "/bin/echo",
		Role:         "reviewer",
		Capabilities: "ai.review",
		Permission:   "auto-approve",
		SkillsDir:    filepath.Join(dir, "skills"),
		WorkDir:      dir,
		ArtifactDir:  filepath.Join(dir, "artifacts"),
	})
	if err != nil {
		t.Fatalf("config from options: %v", err)
	}
	if config.Role != "reviewer" || config.Permission != "auto-approve" || len(config.Capabilities) != 1 || config.Capabilities[0] != "ai.review" {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestWorkerConfigDefaultsAndValidationBranches(t *testing.T) {
	config, err := (WorkerConfig{}).withDefaults()
	if err != nil {
		t.Fatalf("default worker config: %v", err)
	}
	if config.ServerURL != "http://127.0.0.1:8080" || config.Backend != BackendAuto || len(config.Capabilities) != 2 {
		t.Fatalf("unexpected default worker config: %+v", config)
	}
	if config.SkillsDir == "" || config.WorkDir == "" || config.ArtifactDir == "" {
		t.Fatalf("expected default paths: %+v", config)
	}

	withSkillPath, err := (WorkerConfig{
		SkillPaths: []string{"~/worker.SKILL.md"},
	}).withDefaults()
	if err != nil {
		t.Fatalf("config with skill path: %v", err)
	}
	if strings.Contains(withSkillPath.SkillPaths[0], "~") {
		t.Fatalf("expected skill path home expansion: %+v", withSkillPath.SkillPaths)
	}

	if _, err := (WorkerConfig{Version: 999}).withDefaults(); err == nil {
		t.Fatalf("expected unsupported worker config version to fail")
	}
}

func TestDefaultConfigPathHelpersRespectExistingValues(t *testing.T) {
	if got, err := defaultConfigSkillsDir("/tmp/skills", nil); err != nil || got != "/tmp/skills" {
		t.Fatalf("unexpected skills dir got=%q err=%v", got, err)
	}
	if got, err := defaultConfigWorkDir("/tmp/work"); err != nil || got != "/tmp/work" {
		t.Fatalf("unexpected work dir got=%q err=%v", got, err)
	}
	if got, err := defaultConfigArtifactDir("/tmp/artifacts"); err != nil || got != "/tmp/artifacts" {
		t.Fatalf("unexpected artifact dir got=%q err=%v", got, err)
	}
}

func TestConfigFromEnvAddsIdentityCapabilitiesAndHints(t *testing.T) {
	clearEnvForPackageTest(t)
	t.Setenv("TC_WORKER_ENDPOINT_REF", "tc://endpoint/env_worker")
	t.Setenv("TC_WORKER_DISPLAY_NAME", "Env Worker")
	t.Setenv("TC_WORKER_ACTOR_ID", "actor.env")
	t.Setenv("TC_WORKER_WORKSPACE_ID", "workspace.env")
	t.Setenv("TC_WORKER_VERSION", "worker-test")
	t.Setenv("TC_WORKER_CAPABILITIES", "code.change, ai.review")
	t.Setenv("TC_WORKER_BACKEND", "claude")
	t.Setenv("TC_WORKER_MODEL", "opus[1m]")
	t.Setenv("TC_WORKER_ROLE", "reviewer")
	t.Setenv("TC_WORKER_PERMISSION", "auto-approve")

	config := ConfigFromEnv()
	if config.EndpointRef != "tc://endpoint/env_worker" || config.DisplayName != "Env Worker" || config.WorkerVersion != "worker-test" {
		t.Fatalf("identity env was not applied: %+v", config)
	}
	if len(config.Capabilities) != 2 || config.Capabilities[0].Name != "code.change" || config.Capabilities[1].Name != "ai.review" {
		t.Fatalf("capabilities env was not applied: %+v", config.Capabilities)
	}
	for _, expected := range []string{"backend:claude", "model:opus[1m]", "role:reviewer", "permission:auto-approve"} {
		if !containsString(config.ExecutionHints, expected) {
			t.Fatalf("expected execution hint %q in %+v", expected, config.ExecutionHints)
		}
	}
}

func TestLoopOptionsAndExecutorsFromEnv(t *testing.T) {
	clearEnvForPackageTest(t)
	t.Setenv("TC_WORKER_POLL_INTERVAL", "250ms")
	t.Setenv("TC_WORKER_HEARTBEAT_INTERVAL", "2s")
	t.Setenv("TC_WORKER_MAX_MESSAGES", "4")
	options, err := LoopOptionsFromEnv()
	if err != nil {
		t.Fatalf("loop options from env: %v", err)
	}
	if options.PollInterval != 250*time.Millisecond || options.HeartbeatInterval != 2*time.Second || options.MaxMessages != 4 {
		t.Fatalf("unexpected loop options: %+v", options)
	}

	t.Setenv("TC_WORKER_EXECUTOR", "")
	t.Setenv("TC_WORKER_ALLOWED_COMMANDS", "")
	if _, err := ExecutorFromEnv(); err != nil {
		t.Fatalf("default echo executor: %v", err)
	}

	t.Setenv("TC_WORKER_ALLOWED_COMMANDS", "echo, printf")
	t.Setenv("TC_WORKER_WORKDIR", t.TempDir())
	t.Setenv("TC_WORKER_COMMAND_TIMEOUT", "1s")
	if _, err := ExecutorFromEnv(); err != nil {
		t.Fatalf("command executor from env: %v", err)
	}

	t.Setenv("TC_WORKER_EXECUTOR", "ai-cli")
	t.Setenv("TC_WORKER_AI_CLI_COMMAND", "/bin/echo")
	t.Setenv("TC_WORKER_AI_CLI_ARGS", "-n")
	t.Setenv("TC_WORKER_AI_CLI_TIMEOUT", "1s")
	if _, err := ExecutorFromEnv(); err != nil {
		t.Fatalf("ai cli executor from env: %v", err)
	}

	t.Setenv("TC_WORKER_EXECUTOR", "missing")
	if _, err := ExecutorFromEnv(); err == nil {
		t.Fatalf("expected unknown executor to fail")
	}
}

func TestWorkerEnvHelpers(t *testing.T) {
	if got := splitCSV(" a, , b "); strings.Join(got, "|") != "a|b" {
		t.Fatalf("unexpected split csv: %#v", got)
	}
	capabilities := capabilitiesFromCSV("code.change,ai.review")
	if len(capabilities) != 2 || capabilities[0].ExecutionHints[1] != "ai_execution" {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
	skills := capabilitiesFromSkills([]contracts.SkillDefinition{
		{SkillRef: "skill://one", Capabilities: []string{"code.change", "ai.review"}},
		{SkillRef: "skill://two", Capabilities: []string{"code.change"}},
	})
	if len(skills) != 2 {
		t.Fatalf("expected duplicate capabilities to be removed: %+v", skills)
	}
	if got := defaultArgsForAICLI("/usr/bin/codex"); len(got) == 0 || got[0] != "exec" {
		t.Fatalf("unexpected codex args: %#v", got)
	}
	if got := defaultArgsForAICLI("/usr/bin/claude"); len(got) != 1 || got[0] != "-p" {
		t.Fatalf("unexpected claude args: %#v", got)
	}
	if got := defaultArgsForAICLI("/usr/bin/custom"); got != nil {
		t.Fatalf("unexpected custom args: %#v", got)
	}
	if _, err := parseOptionalNonNegativeInt("-1"); err == nil {
		t.Fatalf("expected negative int parse to fail")
	}
	if parsed, err := parseOptionalNonNegativeInt("7"); err != nil || parsed != 7 {
		t.Fatalf("unexpected int parse parsed=%d err=%v", parsed, err)
	}
	if got := appendUniqueStrings([]string{"a"}, "a"); len(got) != 1 {
		t.Fatalf("expected duplicate append to be ignored: %#v", got)
	}
}

func clearEnvForPackageTest(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TC_WORKER_ENDPOINT_REF",
		"TC_WORKER_DISPLAY_NAME",
		"TC_WORKER_ACTOR_ID",
		"TC_WORKER_WORKSPACE_ID",
		"TC_WORKER_VERSION",
		"TC_WORKER_CAPABILITIES",
		"TC_WORKER_BACKEND",
		"TC_WORKER_MODEL",
		"TC_WORKER_ROLE",
		"TC_WORKER_PERMISSION",
		"TC_WORKER_POLL_INTERVAL",
		"TC_WORKER_HEARTBEAT_INTERVAL",
		"TC_WORKER_MAX_MESSAGES",
		"TC_WORKER_EXECUTOR",
		"TC_WORKER_ALLOWED_COMMANDS",
		"TC_WORKER_WORKDIR",
		"TC_WORKER_COMMAND_TIMEOUT",
		"TC_WORKER_AI_CLI_COMMAND",
		"TC_WORKER_AI_CLI_ARGS",
		"TC_WORKER_AI_CLI_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

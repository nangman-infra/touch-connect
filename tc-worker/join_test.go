package tcworker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildJoinEnvironmentDefaultsAndBackendPresets(t *testing.T) {
	dir := t.TempDir()
	env, err := BuildJoinEnvironment(JoinOptions{
		ServerURL:    "http://127.0.0.1:8080",
		Backend:      BackendCodex,
		Command:      "/bin/echo",
		SkillsDir:    dir,
		WorkDir:      dir,
		ArtifactDir:  filepath.Join(dir, "artifacts"),
		Capabilities: "code.change",
		MaxMessages:  3,
		Sandbox:      "read-only",
	})
	if err != nil {
		t.Fatalf("build join env: %v", err)
	}
	if env.Backend != BackendCodex || !strings.HasPrefix(env.Env["TC_WORKER_ENDPOINT_REF"], "tc://endpoint/codex_") {
		t.Fatalf("unexpected join env: %+v", env)
	}
	if !containsJoinArg(env.Args, "approval_policy=\"never\"") || env.Env["TC_WORKER_MAX_MESSAGES"] != "3" || env.Env["TC_WORKER_PROGRESS_INTERVAL"] != "30s" {
		t.Fatalf("expected codex preset args and max messages, env=%+v args=%+v", env.Env, env.Args)
	}
}

func TestJoinPathAndIdentityDefaults(t *testing.T) {
	dir := t.TempDir()
	examplesSkills := filepath.Join(dir, "examples", "skills")
	if err := os.MkdirAll(examplesSkills, 0o755); err != nil {
		t.Fatalf("create examples skills: %v", err)
	}
	options, err := (JoinOptions{Backend: " Claude ", WorkDir: dir}).defaultServerAndBackend()
	if err != nil {
		t.Fatalf("server backend defaults: %v", err)
	}
	if options.Backend != BackendClaude || options.ServerURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected server/backend defaults: %+v", options)
	}
	options, err = options.defaultPaths()
	if err != nil {
		t.Fatalf("path defaults: %v", err)
	}
	if options.SkillsDir != examplesSkills || !strings.HasSuffix(options.ArtifactDir, filepath.Join(".touch-connect", "workers", "claude", "artifacts")) {
		t.Fatalf("unexpected path defaults: %+v", options)
	}
	options, err = options.defaultIdentityAndTiming()
	if err != nil {
		t.Fatalf("identity defaults: %v", err)
	}
	if options.DisplayName != "Claude worker" || options.ActorID != "actor.claude-worker" || options.Timeout != 10*time.Minute || !strings.HasPrefix(options.EndpointRef, "tc://endpoint/claude_") {
		t.Fatalf("unexpected identity defaults: %+v", options)
	}
}

func TestJoinPresetsAndValidationErrors(t *testing.T) {
	for _, backend := range []string{BackendClaude, BackendCodex, BackendGemini, BackendKiro} {
		preset, err := presetForBackend(backend)
		if err != nil {
			t.Fatalf("preset %s: %v", backend, err)
		}
		if len(preset.BuildArgs("model", "sandbox")) == 0 {
			t.Fatalf("expected preset args for %s", backend)
		}
	}
	if _, err := presetForBackend("unknown"); err == nil {
		t.Fatalf("expected unknown backend to fail")
	}
	if safeJoinPart(" ??? ") != "ai" || safeJoinPart("Claude Max") != "claude_max" {
		t.Fatalf("safe join part mismatch")
	}
	if joinTitle("other") != "AI" {
		t.Fatalf("unexpected default join title")
	}
	if _, err := (JoinOptions{Backend: BackendClaude, SkillsDir: t.TempDir(), WorkDir: t.TempDir(), Timeout: -time.Second}).defaultIdentityAndTiming(); err == nil {
		t.Fatalf("expected negative timeout to fail")
	}
	if _, err := (JoinOptions{Backend: BackendClaude, SkillsDir: t.TempDir(), WorkDir: t.TempDir(), ProgressInterval: -time.Second}).defaultIdentityAndTiming(); err == nil {
		t.Fatalf("expected negative progress interval to fail")
	}
	if err := validateWorkerServerURL("file:///tmp/tc-server.sock"); err == nil {
		t.Fatalf("expected non-http server URL to fail")
	}
	if joinHasNoSkills(JoinOptions{}) != true {
		t.Fatalf("empty join options should have no skills")
	}
}

func containsJoinArg(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

package tcworker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

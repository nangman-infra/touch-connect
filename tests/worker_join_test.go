package tests

import (
	"path/filepath"
	"slices"
	"testing"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestWorkerJoinEnvironmentBuildsBackendModelAndSkillContract(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, "skills", "local-ai-worker", "SKILL.md")
	writeProcessSkill(t, filepath.Join(root, "skills"), "local-ai-worker", `---
skill_ref: tc://skill/local-ai-worker
name: Local AI Worker
kind: guidance
capabilities:
  - code.change
---
# Local AI Worker

Return WORKER_READBACK, WORKER_ACTION, and WORKER_RESULT_READY.
`)

	env, err := tcworker.BuildJoinEnvironment(tcworker.JoinOptions{
		ServerURL:    "http://127.0.0.1:8080",
		Backend:      tcworker.BackendCodex,
		Model:        "gpt-test",
		Command:      "/bin/echo",
		SkillPaths:   []string{skillPath},
		WorkDir:      root,
		Capabilities: "code.change",
		MaxMessages:  1,
	})
	if err != nil {
		t.Fatalf("build join environment: %v", err)
	}
	if env.Backend != tcworker.BackendCodex || env.Model != "gpt-test" || env.Command != "/bin/echo" {
		t.Fatalf("unexpected join environment identity: %+v", env)
	}
	if !slices.Contains(env.Args, "-m") || !slices.Contains(env.Args, "gpt-test") || env.Args[len(env.Args)-1] != "-" {
		t.Fatalf("expected codex model args and stdin marker, got %+v", env.Args)
	}
	if env.Env["TC_WORKER_EXECUTOR"] != "skill" || env.Env["TC_WORKER_SKILL_BACKEND"] != "ai-cli" {
		t.Fatalf("expected skill ai-cli worker env, got %+v", env.Env)
	}
	if env.Env["TC_WORKER_SKILL_PATHS"] != skillPath || env.Env["TC_WORKER_CAPABILITIES"] != "code.change" {
		t.Fatalf("expected explicit skill path and capability, got %+v", env.Env)
	}
	if env.Env["TC_WORKER_ARTIFACT_DIR"] != filepath.Join(root, ".touch-connect", "workers", "codex", "artifacts") {
		t.Fatalf("unexpected artifact dir: %s", env.Env["TC_WORKER_ARTIFACT_DIR"])
	}
}

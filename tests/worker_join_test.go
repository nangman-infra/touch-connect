package tests

import (
	"path/filepath"
	"slices"
	"strings"
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

func TestWorkerJoinPresetsDoNotWaitForInteractiveApprovals(t *testing.T) {
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

Return a worker artifact.
`)

	cases := []struct {
		name        string
		backend     string
		model       string
		wantModel   string
		wantArgs    []string
		absentArgs  []string
		commandPath string
	}{
		{
			name:        "claude defaults to opus 1m with bypass permissions",
			backend:     tcworker.BackendClaude,
			wantModel:   "opus[1m]",
			wantArgs:    []string{"-p", "--permission-mode", "bypassPermissions", "--model", "opus[1m]"},
			commandPath: "/bin/echo",
		},
		{
			name:        "codex disables approval prompts",
			backend:     tcworker.BackendCodex,
			model:       "gpt-test",
			wantModel:   "gpt-test",
			wantArgs:    []string{"exec", "approval_policy=\"never\"", "-m", "gpt-test", "-"},
			commandPath: "/bin/echo",
		},
		{
			name:        "gemini uses yolo approval mode",
			backend:     tcworker.BackendGemini,
			model:       "gemini-test",
			wantModel:   "gemini-test",
			wantArgs:    []string{"--model", "gemini-test", "-p", "{{prompt}}", "--approval-mode", "yolo"},
			commandPath: "/bin/echo",
		},
		{
			name:        "kiro trusts tools in headless mode",
			backend:     tcworker.BackendKiro,
			model:       "kiro-profile-model",
			wantModel:   "kiro-profile-model",
			wantArgs:    []string{"chat", "--no-interactive", "--trust-all-tools", "{{prompt}}"},
			absentArgs:  []string{"--model"},
			commandPath: "/bin/echo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := tcworker.BuildJoinEnvironment(tcworker.JoinOptions{
				Backend:    tc.backend,
				Model:      tc.model,
				Command:    tc.commandPath,
				SkillPaths: []string{skillPath},
				WorkDir:    root,
			})
			if err != nil {
				t.Fatalf("build join environment: %v", err)
			}
			if env.Model != tc.wantModel {
				t.Fatalf("expected model %q, got %+v", tc.wantModel, env)
			}
			joined := strings.Join(env.Args, "\x00")
			for _, want := range tc.wantArgs {
				if !strings.Contains(joined, want) {
					t.Fatalf("expected arg %q in %+v", want, env.Args)
				}
			}
			for _, absent := range tc.absentArgs {
				if strings.Contains(joined, absent) {
					t.Fatalf("did not expect arg %q in %+v", absent, env.Args)
				}
			}
		})
	}
}

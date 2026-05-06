package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

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

func TestResolveJoinOptionsLoadsConfigAndAppliesOverrides(t *testing.T) {
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

func TestResolveJoinOptionsRequiresConfigOrExplicitFlags(t *testing.T) {
	_, err := resolveJoinOptions(context.Background(), joinRunOptions{
		ConfigPath: filepath.Join(t.TempDir(), "missing.json"),
		Plain:      true,
		Yes:        true,
		Visited:    map[string]bool{},
	})
	if err == nil || !strings.Contains(err.Error(), "worker config not found") {
		t.Fatalf("expected missing config error, got %v", err)
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

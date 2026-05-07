package tcworker

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDetectBackendCandidatesMarksInstalledAndMissing(t *testing.T) {
	lookup := func(command string) (string, error) {
		switch command {
		case "claude":
			return "/mock/bin/claude", nil
		case "codex":
			return "/mock/bin/codex", nil
		default:
			return "", errors.New("missing")
		}
	}
	probe := func(_ context.Context, candidate BackendCandidate) (string, string) {
		if candidate.Backend == BackendClaude {
			return BackendStatusReady, "authenticated"
		}
		return BackendStatusAuthUnknown, "installed"
	}

	candidates := detectBackendCandidates(context.Background(), lookup, probe)
	if len(candidates) != 4 {
		t.Fatalf("expected four backend candidates, got %+v", candidates)
	}
	assertCandidate(t, candidates[0], BackendClaude, BackendStatusReady, "/mock/bin/claude")
	assertCandidate(t, candidates[1], BackendCodex, BackendStatusAuthUnknown, "/mock/bin/codex")
	assertCandidate(t, candidates[2], BackendGemini, BackendStatusMissing, "")
	assertCandidate(t, candidates[3], BackendKiro, BackendStatusMissing, "")
}

func TestRunJoinWizardAutoAcceptsFirstUsableBackend(t *testing.T) {
	lookup := func(command string) (string, error) {
		if command == "claude" {
			return "/mock/bin/claude", nil
		}
		return "", errors.New("missing")
	}
	var out bytes.Buffer

	options, err := RunJoinWizard(context.Background(), JoinWizardOptions{
		Input:      strings.NewReader(""),
		Output:     &out,
		Base:       JoinOptions{SkillsDir: "/tmp/skills"},
		AutoAccept: true,
		LookPath:   lookup,
		AuthProbe: func(_ context.Context, candidate BackendCandidate) (string, string) {
			return BackendStatusReady, "authenticated"
		},
	})
	if err != nil {
		t.Fatalf("run join wizard: %v", err)
	}
	if options.Backend != BackendClaude || options.Model != "opus[1m]" || options.Command != "/mock/bin/claude" {
		t.Fatalf("unexpected wizard options: %+v", options)
	}
	output := out.String()
	for _, want := range []string{"Detected AI CLIs", "Available worker choices", "permission: auto-approve"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected wizard output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestRunJoinWizardCanSelectCodexAndCustomModel(t *testing.T) {
	lookup := func(command string) (string, error) {
		switch command {
		case "claude":
			return "/mock/bin/claude", nil
		case "codex":
			return "/mock/bin/codex", nil
		default:
			return "", errors.New("missing")
		}
	}
	input := strings.NewReader("2\n4\ngpt-custom\n\n")

	options, err := RunJoinWizard(context.Background(), JoinWizardOptions{
		Input:    input,
		Output:   ioDiscard{},
		Base:     JoinOptions{SkillsDir: "/tmp/skills"},
		LookPath: lookup,
		AuthProbe: func(_ context.Context, candidate BackendCandidate) (string, string) {
			return BackendStatusAuthUnknown, "installed"
		},
	})
	if err != nil {
		t.Fatalf("run join wizard: %v", err)
	}
	if options.Backend != BackendCodex || options.Model != "gpt-custom" || options.Command != "/mock/bin/codex" {
		t.Fatalf("unexpected wizard selection: %+v", options)
	}
}

func TestRunJoinWizardReportsNoUsableBackend(t *testing.T) {
	lookup := func(string) (string, error) {
		return "", errors.New("missing")
	}
	var out bytes.Buffer
	_, err := RunJoinWizard(context.Background(), JoinWizardOptions{
		Output:   &out,
		LookPath: lookup,
	})
	if err == nil || !strings.Contains(err.Error(), "no installed AI CLI backend") {
		t.Fatalf("expected no backend error, got %v", err)
	}
	if !strings.Contains(out.String(), "Detected AI CLIs") {
		t.Fatalf("expected candidate report, got %q", out.String())
	}
}

func TestRunJoinWizardCanCancelAndUseFallbackModel(t *testing.T) {
	lookup := func(command string) (string, error) {
		if command == "claude" {
			return "/mock/bin/claude", nil
		}
		return "", errors.New("missing")
	}
	input := strings.NewReader("1\n4\n\nn\n")
	_, err := RunJoinWizard(context.Background(), JoinWizardOptions{
		Input:    input,
		Output:   ioDiscard{},
		Base:     JoinOptions{SkillsDir: "/tmp/skills"},
		LookPath: lookup,
		AuthProbe: func(_ context.Context, candidate BackendCandidate) (string, string) {
			return BackendStatusReady, "authenticated"
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func TestJoinWizardTUIUsesUnifiedOnboardingSurface(t *testing.T) {
	candidates := []BackendCandidate{
		{
			Backend:          BackendClaude,
			DisplayName:      "Claude",
			CommandPath:      "/mock/bin/claude",
			Status:           BackendStatusReady,
			RecommendedModel: "opus[1m]",
		},
		{
			Backend:          BackendCodex,
			DisplayName:      "Codex",
			CommandPath:      "/mock/bin/codex",
			Status:           BackendStatusAuthUnknown,
			RecommendedModel: "gpt-5.4-mini",
		},
	}
	model := newJoinWizardTUIModel(JoinOptions{
		ServerURL:    "http://192.168.10.34:8080",
		Role:         DefaultWorkerRole,
		Capabilities: "code.change,ai.review",
	}, candidates, usableBackendCandidates(candidates))
	model.width = 120
	view := model.View()
	for _, want := range []string{"Connection", "AI Engine", "Worker Contract", "Choose Engine", "http://192.168.10.34:8080"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected unified TUI view to contain %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "STEP 1") || strings.Contains(view, "STEP 2") {
		t.Fatalf("old stepped wizard labels should not be the primary TUI surface:\n%s", view)
	}
}

func assertCandidate(t *testing.T, candidate BackendCandidate, backend string, status string, path string) {
	t.Helper()
	if candidate.Backend != backend || candidate.Status != status || candidate.CommandPath != path {
		t.Fatalf("unexpected candidate: got %+v want backend=%s status=%s path=%s", candidate, backend, status, path)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

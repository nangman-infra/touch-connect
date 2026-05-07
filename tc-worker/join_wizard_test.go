package tcworker

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestJoinWizardTUINavigationAndSelections(t *testing.T) {
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
		Role:         "reviewer",
		Capabilities: "ai.review",
		SkillsDir:    "/tmp/skills",
	}, candidates, usableBackendCandidates(candidates))
	if model.Init() != nil {
		t.Fatalf("join wizard init should not schedule commands")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(joinWizardTUIModel)
	if model.selectedWorker != 1 || model.selectedBackendSummary() != "Codex (auth_unknown)" {
		t.Fatalf("expected down key to select Codex, got worker=%d summary=%q", model.selectedWorker, model.selectedBackendSummary())
	}
	model.moveSelection(1)
	if model.selectedWorker != 0 {
		t.Fatalf("expected worker selection to wrap back to Claude, got %d", model.selectedWorker)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(joinWizardTUIModel)
	if model.screen != joinWizardScreenModel {
		t.Fatalf("expected enter on backend screen to open model chooser, got %v", model.screen)
	}
	model.moveSelection(-1)
	if got := model.models[model.selectedModel].Value; got != "__custom__" {
		t.Fatalf("expected wrapped model selection to land on custom option, got %q", got)
	}
	if !strings.Contains(strings.Join(model.modelChoiceLines(96), "\n"), "custom") {
		t.Fatalf("model choice lines should expose custom model option")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(joinWizardTUIModel)
	if model.screen != joinWizardScreenCustomModel {
		t.Fatalf("expected custom model screen, got %v", model.screen)
	}
	for _, char := range []rune("gpt-custom") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
		model = updated.(joinWizardTUIModel)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(joinWizardTUIModel)
	if model.screen != joinWizardScreenConfirm || model.resolvedModel() != "gpt-custom" {
		t.Fatalf("expected custom model to resolve on confirm screen, got screen=%v model=%q", model.screen, model.resolvedModel())
	}
	confirm := strings.Join(model.confirmLines(96), "\n")
	for _, want := range []string{"gpt-custom", "/tmp/skills", "ai.review", "trusted workspace"} {
		if !strings.Contains(confirm, want) {
			t.Fatalf("expected confirm lines to contain %q:\n%s", want, confirm)
		}
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(joinWizardTUIModel)
	if !model.done || cmd == nil {
		t.Fatalf("expected confirm enter to finish and request quit")
	}
}

func TestJoinWizardTUIBackCancelAndResponsiveLayout(t *testing.T) {
	candidates := []BackendCandidate{
		{
			Backend:          BackendClaude,
			DisplayName:      "Claude",
			CommandPath:      "/mock/bin/claude",
			Status:           BackendStatusReady,
			RecommendedModel: "opus[1m]",
		},
		{
			Backend:     BackendKiro,
			DisplayName: "Kiro",
			Command:     "kiro-cli",
			Status:      BackendStatusMissing,
		},
	}
	model := newJoinWizardTUIModel(JoinOptions{}, candidates, usableBackendCandidates(candidates))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	model = updated.(joinWizardTUIModel)
	if model.width != 72 || !strings.Contains(model.View(), "source  loopback/default") {
		t.Fatalf("expected narrow view to render connection defaults:\n%s", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(joinWizardTUIModel)
	if model.screen != joinWizardScreenModel {
		t.Fatalf("expected model screen after enter, got %v", model.screen)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(joinWizardTUIModel)
	if model.screen != joinWizardScreenBackend {
		t.Fatalf("expected esc to return to backend screen, got %v", model.screen)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model = updated.(joinWizardTUIModel)
	if !model.cancelled || cmd == nil {
		t.Fatalf("expected ctrl+c to cancel and quit")
	}
}

func TestJoinWizardTUIViewCoversAllScreens(t *testing.T) {
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
		{
			Backend:     BackendKiro,
			DisplayName: "Kiro",
			Command:     "kiro-cli",
			Status:      BackendStatusMissing,
		},
	}
	model := newJoinWizardTUIModel(JoinOptions{
		ServerURL:   "http://192.168.10.34:8080",
		EndpointRef: "tc://endpoint/manual_worker",
		DisplayName: "Manual Worker",
		SkillPaths:  []string{"/tmp/skills/reviewer.SKILL.md", "/tmp/skills/coder.SKILL.md"},
	}, candidates, usableBackendCandidates(candidates))
	model.width = 132
	for _, scenario := range []struct {
		screen joinWizardScreen
		want   string
	}{
		{screen: joinWizardScreenBackend, want: "Choose Engine"},
		{screen: joinWizardScreenModel, want: "Choose Model"},
		{screen: joinWizardScreenCustomModel, want: "Custom Model"},
		{screen: joinWizardScreenConfirm, want: "Start Worker"},
	} {
		model.screen = scenario.screen
		view := model.View()
		if !strings.Contains(view, scenario.want) {
			t.Fatalf("expected screen %v view to contain %q:\n%s", scenario.screen, scenario.want, view)
		}
		if model.footer(132) == "" {
			t.Fatalf("expected screen %v to have footer help", scenario.screen)
		}
	}
	if got := model.usableIndex(BackendGemini); got != -1 {
		t.Fatalf("expected missing backend usable index to be -1, got %d", got)
	}
}

func TestJoinWizardTUIUsesBaseModelAndBackendDefaults(t *testing.T) {
	candidates := []BackendCandidate{
		{
			Backend:          BackendClaude,
			DisplayName:      "Claude",
			CommandPath:      "/mock/bin/claude",
			Status:           BackendStatusReady,
			RecommendedModel: "opus[1m]",
		},
		{
			Backend:          BackendGemini,
			DisplayName:      "Gemini",
			CommandPath:      "/mock/bin/gemini",
			Status:           BackendStatusAuthUnknown,
			RecommendedModel: "",
		},
	}
	model := newJoinWizardTUIModel(JoinOptions{Model: "sonnet"}, candidates, usableBackendCandidates(candidates))
	if model.selectedModel != 2 || model.resolvedModel() != "sonnet" {
		t.Fatalf("expected base model to preselect sonnet, selected=%d resolved=%q", model.selectedModel, model.resolvedModel())
	}
	model.selectedWorker = 1
	model.base.Model = "gemini-default"
	model.setModelChoices()
	if model.resolvedModel() != "gemini-default" {
		t.Fatalf("blank backend model choice should fall back to base model, got %q", model.resolvedModel())
	}
	model.screen = joinWizardScreenModel
	model.moveSelection(1)
	if model.models[model.selectedModel].Value != "__custom__" {
		t.Fatalf("expected Gemini custom option after one move, got %+v", model.models[model.selectedModel])
	}
	if model.resolvedModel() != "gemini-default" {
		t.Fatalf("blank custom value should fall back to first model/base default, got %q", model.resolvedModel())
	}
}

func TestJoinWizardTUIHandlesNoUsableBackend(t *testing.T) {
	candidates := []BackendCandidate{
		{
			Backend:     BackendKiro,
			DisplayName: "Kiro",
			Command:     "kiro-cli",
			Status:      BackendStatusMissing,
		},
	}
	model := newJoinWizardTUIModel(JoinOptions{Model: "fallback"}, candidates, nil)
	model.moveSelection(1)
	model.setModelChoices()
	if got := model.selectedBackendSummary(); got != "none" {
		t.Fatalf("expected no selected backend summary, got %q", got)
	}
	if got := model.resolvedModel(); got != "fallback" {
		t.Fatalf("expected fallback model with no choices, got %q", got)
	}
	if lines := strings.Join(model.modelChoiceLines(64), "\n"); !strings.Contains(lines, "No AI CLI backend") {
		t.Fatalf("expected no backend message, got %s", lines)
	}
	if got := wrapIndex(-3, 0); got != 0 {
		t.Fatalf("empty wrap index should be 0, got %d", got)
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

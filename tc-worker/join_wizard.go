package tcworker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	BackendStatusMissing     = "missing"
	BackendStatusReady       = "ready"
	BackendStatusAuthUnknown = "auth_unknown"
)

type BackendCandidate struct {
	Backend          string
	DisplayName      string
	Command          string
	CommandPath      string
	Status           string
	StatusDetail     string
	RecommendedModel string
}

type JoinWizardOptions struct {
	Input        io.Reader
	Output       io.Writer
	Base         JoinOptions
	AutoAccept   bool
	UseTUI       bool
	LookPath     func(string) (string, error)
	AuthProbe    func(context.Context, BackendCandidate) (string, string)
	ConfirmLabel string
}

func RunJoinWizard(ctx context.Context, options JoinWizardOptions) (JoinOptions, error) {
	options = normalizedJoinWizardOptions(options)
	candidates := detectBackendCandidates(ctx, options.LookPath, options.AuthProbe)
	usable := usableBackendCandidates(candidates)
	if len(usable) == 0 {
		printBackendCandidates(options.Output, candidates)
		return JoinOptions{}, errors.New("no installed AI CLI backend found; install Claude Code, Codex, Gemini, or Kiro CLI first")
	}
	if options.UseTUI && !options.AutoAccept {
		return runJoinWizardTUI(ctx, options, candidates, usable)
	}
	return runTextJoinWizard(options, candidates, usable)
}

func normalizedJoinWizardOptions(options JoinWizardOptions) JoinWizardOptions {
	if options.Input == nil {
		options.Input = strings.NewReader("")
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.LookPath == nil {
		options.LookPath = exec.LookPath
	}
	if options.AuthProbe == nil {
		options.AuthProbe = probeBackendAuth
	}
	if strings.TrimSpace(options.ConfirmLabel) == "" {
		options.ConfirmLabel = "Start worker?"
	}
	return options
}

func runTextJoinWizard(options JoinWizardOptions, candidates []BackendCandidate, usable []BackendCandidate) (JoinOptions, error) {
	reader := bufio.NewReader(options.Input)
	output := options.Output
	fmt.Fprintln(output, "touch-connect worker join")
	fmt.Fprintln(output, "")
	printBackendCandidates(output, candidates)
	fmt.Fprintln(output, "")
	printUsableBackendChoices(output, usable)
	selected, err := selectWizardBackend(reader, output, usable, options.AutoAccept)
	if err != nil {
		return JoinOptions{}, err
	}
	model, err := selectWizardModel(reader, output, selected.Backend, options.Base.Model, options.AutoAccept)
	if err != nil {
		return JoinOptions{}, err
	}

	result := wizardResult(options.Base, selected, model)
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Worker summary")
	fmt.Fprintf(output, "  backend:    %s\n", selected.DisplayName)
	fmt.Fprintf(output, "  model:      %s\n", printableModel(model))
	fmt.Fprintf(output, "  command:    %s\n", result.Command)
	fmt.Fprintf(output, "  server:     %s\n", defaultString(result.ServerURL, "http://127.0.0.1:8080"))
	fmt.Fprintf(output, "  permission: %s\n", defaultString(result.Permission, DefaultWorkerPermission))
	if err := confirmJoinWizard(reader, output, options); err != nil {
		return JoinOptions{}, err
	}
	return result, nil
}

func selectWizardBackend(reader *bufio.Reader, output io.Writer, usable []BackendCandidate, autoAccept bool) (BackendCandidate, error) {
	if autoAccept {
		return usable[0], nil
	}
	index, err := promptChoice(reader, output, "Select AI worker", len(usable), 1)
	if err != nil {
		return BackendCandidate{}, err
	}
	return usable[index-1], nil
}

func selectWizardModel(reader *bufio.Reader, output io.Writer, backend string, initial string, autoAccept bool) (string, error) {
	choices := modelChoicesForBackend(backend)
	model := initial
	if model == "" && len(choices) > 0 {
		model = choices[0].Value
	}
	if autoAccept || len(choices) <= 1 {
		return model, nil
	}
	fmt.Fprintln(output, "")
	printModelChoices(output, choices)
	index, err := promptChoice(reader, output, "Select model", len(choices), 1)
	if err != nil {
		return "", err
	}
	selected := choices[index-1].Value
	if selected != "__custom__" {
		return selected, nil
	}
	return promptCustomModel(reader, output, choices[0].Value)
}

func promptCustomModel(reader *bufio.Reader, output io.Writer, fallback string) (string, error) {
	custom, err := promptLine(reader, output, "Custom model")
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(custom)
	if model == "" {
		return fallback, nil
	}
	return model, nil
}

func wizardResult(base JoinOptions, selected BackendCandidate, model string) JoinOptions {
	result := base
	result.Backend = selected.Backend
	result.Model = model
	if result.Command == "" {
		result.Command = selected.CommandPath
	}
	return result
}

func confirmJoinWizard(reader *bufio.Reader, output io.Writer, options JoinWizardOptions) error {
	if options.AutoAccept {
		return nil
	}
	confirmed, err := promptConfirm(reader, output, options.ConfirmLabel, true)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("worker join cancelled")
	}
	return nil
}

func DetectJoinBackends(ctx context.Context) []BackendCandidate {
	return detectBackendCandidates(ctx, exec.LookPath, probeBackendAuth)
}

func detectBackendCandidates(ctx context.Context, lookPath func(string) (string, error), authProbe func(context.Context, BackendCandidate) (string, string)) []BackendCandidate {
	backends := []string{BackendClaude, BackendCodex, BackendGemini, BackendKiro}
	candidates := make([]BackendCandidate, 0, len(backends))
	for _, backend := range backends {
		preset, _ := presetForBackend(backend)
		candidate := BackendCandidate{
			Backend:          backend,
			DisplayName:      preset.DisplayName,
			Command:          preset.Command,
			Status:           BackendStatusMissing,
			RecommendedModel: preset.DefaultModel,
		}
		path, err := lookPath(preset.Command)
		if err != nil {
			candidates = append(candidates, candidate)
			continue
		}
		candidate.CommandPath = path
		candidate.Status = BackendStatusAuthUnknown
		candidate.StatusDetail = "installed"
		if authProbe != nil {
			status, detail := authProbe(ctx, candidate)
			if status != "" {
				candidate.Status = status
			}
			candidate.StatusDetail = detail
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func probeBackendAuth(ctx context.Context, candidate BackendCandidate) (string, string) {
	switch candidate.Backend {
	case BackendClaude:
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		command := exec.CommandContext(probeCtx, candidate.CommandPath, "auth", "status", "--text")
		if err := command.Run(); err != nil {
			return BackendStatusAuthUnknown, "installed, auth not confirmed"
		}
		return BackendStatusReady, "installed and authenticated"
	default:
		return BackendStatusAuthUnknown, "installed, auth not checked"
	}
}

func usableBackendCandidates(candidates []BackendCandidate) []BackendCandidate {
	usable := make([]BackendCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status != BackendStatusMissing {
			usable = append(usable, candidate)
		}
	}
	return usable
}

func printBackendCandidates(w io.Writer, candidates []BackendCandidate) {
	fmt.Fprintln(w, "Detected AI CLIs:")
	for index, candidate := range candidates {
		model := printableModel(candidate.RecommendedModel)
		if candidate.Status == BackendStatusMissing {
			fmt.Fprintf(w, "  %d. %-12s %-12s command=%s\n", index+1, candidate.DisplayName, candidate.Status, candidate.Command)
			continue
		}
		detail := candidate.StatusDetail
		if detail == "" {
			detail = "installed"
		}
		fmt.Fprintf(w, "  %d. %-12s %-12s model=%s path=%s (%s)\n", index+1, candidate.DisplayName, candidate.Status, model, candidate.CommandPath, detail)
	}
}

func printUsableBackendChoices(w io.Writer, candidates []BackendCandidate) {
	fmt.Fprintln(w, "Available worker choices:")
	for index, candidate := range candidates {
		fmt.Fprintf(w, "  %d. %s (%s, model=%s)\n", index+1, candidate.DisplayName, candidate.Status, printableModel(candidate.RecommendedModel))
	}
}

type joinModelChoice struct {
	Label string
	Value string
}

func modelChoicesForBackend(backend string) []joinModelChoice {
	switch backend {
	case BackendClaude:
		return []joinModelChoice{
			{Label: "opus[1m] recommended for Claude Max", Value: "opus[1m]"},
			{Label: "opus", Value: "opus"},
			{Label: "sonnet", Value: "sonnet"},
			{Label: "custom", Value: "__custom__"},
		}
	case BackendCodex:
		return []joinModelChoice{
			{Label: "gpt-5.4-mini recommended", Value: "gpt-5.4-mini"},
			{Label: "gpt-5.4", Value: "gpt-5.4"},
			{Label: "gpt-5.5", Value: "gpt-5.5"},
			{Label: "custom", Value: "__custom__"},
		}
	case BackendGemini:
		return []joinModelChoice{
			{Label: "default from Gemini CLI", Value: ""},
			{Label: "custom", Value: "__custom__"},
		}
	case BackendKiro:
		return []joinModelChoice{
			{Label: "default from Kiro CLI", Value: ""},
		}
	default:
		return nil
	}
}

func printModelChoices(w io.Writer, choices []joinModelChoice) {
	fmt.Fprintln(w, "Model options:")
	for index, choice := range choices {
		fmt.Fprintf(w, "  %d. %s\n", index+1, choice.Label)
	}
}

func promptChoice(reader *bufio.Reader, w io.Writer, label string, max int, fallback int) (int, error) {
	if max <= 0 {
		return 0, errors.New("no choices available")
	}
	for {
		fmt.Fprintf(w, "%s [%d]: ", label, fallback)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		value := strings.TrimSpace(line)
		if value == "" {
			return fallback, nil
		}
		parsed, parseErr := strconv.Atoi(value)
		if parseErr == nil && parsed >= 1 && parsed <= max {
			return parsed, nil
		}
		fmt.Fprintf(w, "Enter a number from 1 to %d.\n", max)
		if errors.Is(err, io.EOF) {
			return fallback, nil
		}
	}
}

func promptLine(reader *bufio.Reader, w io.Writer, label string) (string, error) {
	fmt.Fprintf(w, "%s: ", label)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptConfirm(reader *bufio.Reader, w io.Writer, label string, fallback bool) (bool, error) {
	defaultLabel := "Y/n"
	if !fallback {
		defaultLabel = "y/N"
	}
	for {
		fmt.Fprintf(w, "%s [%s]: ", label, defaultLabel)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		value := strings.ToLower(strings.TrimSpace(line))
		switch value {
		case "":
			return fallback, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(w, "Enter y or n.")
		}
		if errors.Is(err, io.EOF) {
			return fallback, nil
		}
	}
}

func printableModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return "default"
	}
	return model
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

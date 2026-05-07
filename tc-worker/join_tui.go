package tcworker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type joinWizardScreen int

const (
	joinWizardScreenBackend joinWizardScreen = iota
	joinWizardScreenModel
	joinWizardScreenCustomModel
	joinWizardScreenConfirm
)

type joinWizardTUIModel struct {
	base       JoinOptions
	candidates []BackendCandidate
	usable     []BackendCandidate

	screen         joinWizardScreen
	selectedWorker int
	models         []joinModelChoice
	selectedModel  int
	customModel    textinput.Model

	width     int
	height    int
	done      bool
	cancelled bool
}

func runJoinWizardTUI(ctx context.Context, options JoinWizardOptions, candidates []BackendCandidate, usable []BackendCandidate) (JoinOptions, error) {
	model := newJoinWizardTUIModel(options.Base, candidates, usable)
	finalModel, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err != nil {
		return JoinOptions{}, err
	}
	final, ok := finalModel.(joinWizardTUIModel)
	if !ok {
		return JoinOptions{}, errors.New("worker join TUI returned unexpected model")
	}
	if final.cancelled || !final.done {
		return JoinOptions{}, errors.New("worker join cancelled")
	}
	selected := final.usable[final.selectedWorker]
	result := options.Base
	result.Backend = selected.Backend
	result.Model = final.resolvedModel()
	if result.Command == "" {
		result.Command = selected.CommandPath
	}
	if output := options.Output; output != nil {
		fmt.Fprintln(output, "tc-worker join ready")
		fmt.Fprintf(output, "  backend:    %s\n", selected.DisplayName)
		fmt.Fprintf(output, "  model:      %s\n", printableModel(result.Model))
		fmt.Fprintf(output, "  command:    %s\n", result.Command)
		fmt.Fprintf(output, "  server:     %s\n", defaultString(result.ServerURL, DefaultWorkerServerURL))
		fmt.Fprintln(output, "  permission: non-interactive auto-approve")
	}
	return result, nil
}

func newJoinWizardTUIModel(base JoinOptions, candidates []BackendCandidate, usable []BackendCandidate) joinWizardTUIModel {
	input := textinput.New()
	input.Placeholder = "custom model"
	input.Prompt = "> "
	input.CharLimit = 80
	model := joinWizardTUIModel{
		base:        base,
		candidates:  candidates,
		usable:      usable,
		screen:      joinWizardScreenBackend,
		customModel: input,
	}
	model.setModelChoices()
	return model
}

func (m joinWizardTUIModel) Init() tea.Cmd {
	return nil
}

func (m joinWizardTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch item := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = item.Width
		m.height = item.Height
	case tea.KeyMsg:
		switch m.screen {
		case joinWizardScreenCustomModel:
			return m.updateCustomModel(item)
		default:
			return m.updateNavigation(item)
		}
	}
	return m, nil
}

func (m joinWizardTUIModel) updateNavigation(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		switch m.screen {
		case joinWizardScreenModel:
			m.screen = joinWizardScreenBackend
		case joinWizardScreenConfirm:
			m.screen = joinWizardScreenModel
		}
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "enter":
		switch m.screen {
		case joinWizardScreenBackend:
			m.setModelChoices()
			m.screen = joinWizardScreenModel
		case joinWizardScreenModel:
			if len(m.models) == 0 {
				m.screen = joinWizardScreenConfirm
				break
			}
			if m.models[m.selectedModel].Value == "__custom__" {
				m.customModel.Focus()
				m.screen = joinWizardScreenCustomModel
				return m, textinput.Blink
			}
			m.screen = joinWizardScreenConfirm
		case joinWizardScreenConfirm:
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m joinWizardTUIModel) updateCustomModel(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit
	case "esc":
		m.screen = joinWizardScreenModel
		m.customModel.Blur()
		return m, nil
	case "enter":
		m.customModel.Blur()
		m.screen = joinWizardScreenConfirm
		return m, nil
	}
	var cmd tea.Cmd
	m.customModel, cmd = m.customModel.Update(key)
	return m, cmd
}

func (m *joinWizardTUIModel) moveSelection(delta int) {
	switch m.screen {
	case joinWizardScreenBackend:
		if len(m.usable) == 0 {
			return
		}
		m.selectedWorker = wrapIndex(m.selectedWorker+delta, len(m.usable))
	case joinWizardScreenModel:
		if len(m.models) == 0 {
			return
		}
		m.selectedModel = wrapIndex(m.selectedModel+delta, len(m.models))
	}
}

func (m *joinWizardTUIModel) setModelChoices() {
	if len(m.usable) == 0 {
		m.models = nil
		return
	}
	m.models = modelChoicesForBackend(m.usable[m.selectedWorker].Backend)
	m.selectedModel = 0
	if m.base.Model != "" {
		for index, model := range m.models {
			if model.Value == m.base.Model {
				m.selectedModel = index
				break
			}
		}
	}
}

func (m joinWizardTUIModel) resolvedModel() string {
	if len(m.models) == 0 {
		return m.base.Model
	}
	selected := m.models[m.selectedModel]
	if selected.Value == "__custom__" {
		custom := strings.TrimSpace(m.customModel.Value())
		if custom != "" {
			return custom
		}
		if fallback := strings.TrimSpace(m.models[0].Value); fallback != "" {
			return fallback
		}
		return m.base.Model
	}
	if selected.Value == "" {
		return m.base.Model
	}
	return selected.Value
}

func (m joinWizardTUIModel) View() string {
	width := tuiWidth(m.width, 96)
	content := strings.Join([]string{
		m.header(width),
		m.body(width),
		m.footer(width),
	}, "\n")
	return joinBoxStyle(width).Render(content)
}

func (m joinWizardTUIModel) header(width int) string {
	selected := BackendCandidate{DisplayName: "AI worker", Status: "pending"}
	if len(m.usable) > 0 {
		selected = m.usable[m.selectedWorker]
	}
	left := joinTitleStyle.Render("touch-connect worker")
	right := joinMutedStyle.Render("server " + defaultString(m.base.ServerURL, DefaultWorkerServerURL))
	line := left + strings.Repeat(" ", maxInt(1, width-lipgloss.Width(left)-lipgloss.Width(right)-6)) + right
	return line + "\n" + joinMutedStyle.Render("engine "+selected.DisplayName+" / status "+selected.Status+" / mode local AI worker join")
}

func (m joinWizardTUIModel) body(width int) string {
	connection := m.connectionPanel(width)
	engine := m.enginePanel(width)
	contract := m.contractPanel(width)
	active := m.activeOnboardingPanel(width)
	if width < 96 {
		return lipgloss.JoinVertical(lipgloss.Left, connection, engine, contract, active)
	}
	gap := "  "
	panelWidth := maxInt(24, (width-2-lipgloss.Width(gap)*2)/3)
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		m.connectionPanel(panelWidth),
		gap,
		m.enginePanel(panelWidth),
		gap,
		m.contractPanel(panelWidth),
	)
	return lipgloss.JoinVertical(lipgloss.Left, row, active)
}

func (m joinWizardTUIModel) connectionPanel(width int) string {
	serverURL := defaultString(m.base.ServerURL, DefaultWorkerServerURL)
	source := "loopback/default"
	if serverURL != DefaultWorkerServerURL {
		source = "auto-discovered or configured"
	}
	return renderJoinPanel("Connection", width, []string{
		"server  " + compact(serverURL, maxInt(16, width-14)),
		"probe   /healthz",
		"source  " + compact(source, maxInt(16, width-14)),
	})
}

func (m joinWizardTUIModel) enginePanel(width int) string {
	lines := []string{"installed AI CLIs"}
	for _, candidate := range m.candidates {
		marker := " "
		if candidate.Status != BackendStatusMissing && m.usableIndex(candidate.Backend) == m.selectedWorker {
			marker = ">"
		}
		model := printableModel(candidate.RecommendedModel)
		state := candidate.Status
		if candidate.Status == BackendStatusMissing {
			lines = append(lines, fmt.Sprintf("%s %-8s %s", marker, candidate.DisplayName, state))
			continue
		}
		lines = append(lines, compact(fmt.Sprintf("%s %-8s %s %s", marker, candidate.DisplayName, state, model), maxInt(16, width-6)))
	}
	return renderJoinPanel("AI Engine", width, lines)
}

func (m joinWizardTUIModel) contractPanel(width int) string {
	selected := BackendCandidate{Backend: "ai", DisplayName: "AI"}
	if len(m.usable) > 0 {
		selected = m.usable[m.selectedWorker]
	}
	endpoint := defaultString(m.base.EndpointRef, "tc://endpoint/"+safeJoinPart(selected.Backend)+"_worker")
	capabilities := defaultString(m.base.Capabilities, "from SKILL.md")
	role := defaultString(m.base.Role, DefaultWorkerRole)
	return renderJoinPanel("Worker Contract", width, []string{
		"endpoint " + compact(endpoint, maxInt(16, width-12)),
		"role     " + compact(role, maxInt(16, width-12)),
		"caps     " + compact(capabilities, maxInt(16, width-12)),
		"perm     auto-approve",
	})
}

func (m joinWizardTUIModel) activeOnboardingPanel(width int) string {
	switch m.screen {
	case joinWizardScreenBackend:
		return renderJoinPanel("Choose Engine", width, []string{
			"Pick the local AI CLI that will receive touch-connect tasks.",
			"Use the AI Engine panel for installed/auth status.",
			"",
			"Current: " + m.selectedBackendSummary(),
		})
	case joinWizardScreenModel:
		return renderJoinPanel("Choose Model", width, m.modelChoiceLines(width))
	case joinWizardScreenCustomModel:
		return renderJoinPanel("Custom Model", width, []string{
			"Enter the model name passed to the selected AI CLI.",
			"",
			m.customModel.View(),
		})
	case joinWizardScreenConfirm:
		return renderJoinPanel("Start Worker", width, m.confirmLines(width))
	default:
		return ""
	}
}

func (m joinWizardTUIModel) selectedBackendSummary() string {
	if len(m.usable) == 0 {
		return "none"
	}
	selected := m.usable[m.selectedWorker]
	return selected.DisplayName + " (" + selected.Status + ")"
}

func (m joinWizardTUIModel) modelChoiceLines(width int) []string {
	if len(m.usable) == 0 {
		return []string{"No AI CLI backend is available."}
	}
	selected := m.usable[m.selectedWorker]
	lines := []string{"Engine: " + selected.DisplayName}
	if len(m.models) == 0 {
		return append(lines, "Model: default from backend CLI")
	}
	for index, model := range m.models {
		marker := " "
		if index == m.selectedModel {
			marker = ">"
		}
		lines = append(lines, compact(fmt.Sprintf("%s %s", marker, model.Label), maxInt(16, width-6)))
	}
	return lines
}

func (m joinWizardTUIModel) confirmLines(width int) []string {
	selected := m.usable[m.selectedWorker]
	capabilities := defaultString(m.base.Capabilities, "from selected SKILL.md files")
	skills := m.base.SkillsDir
	if len(m.base.SkillPaths) > 0 {
		skills = strings.Join(m.base.SkillPaths, ",")
	}
	if strings.TrimSpace(skills) == "" {
		skills = "examples/skills"
	}
	return []string{
		"Ready to join as " + selected.DisplayName + " worker.",
		"",
		"model       " + compact(printableModel(m.resolvedModel()), maxInt(16, width-14)),
		"skills      " + compact(skills, maxInt(16, width-14)),
		"capability  " + compact(capabilities, maxInt(16, width-14)),
		"",
		"Security: this worker runs without local permission prompts.",
		"Only start it inside a trusted workspace.",
		"",
		"Press enter to start worker.",
	}
}

func (m joinWizardTUIModel) footer(_ int) string {
	switch m.screen {
	case joinWizardScreenBackend:
		return joinMutedStyle.Render("up/down select  enter continue  q quit")
	case joinWizardScreenModel:
		return joinMutedStyle.Render("up/down select  enter continue  esc back  q quit")
	case joinWizardScreenCustomModel:
		return joinMutedStyle.Render("enter continue  esc back  q quit")
	case joinWizardScreenConfirm:
		return joinMutedStyle.Render("enter start  esc back  q cancel")
	default:
		return ""
	}
}

func (m joinWizardTUIModel) usableIndex(backend string) int {
	for index, candidate := range m.usable {
		if candidate.Backend == backend {
			return index
		}
	}
	return -1
}

func wrapIndex(index int, length int) int {
	if length <= 0 {
		return 0
	}
	for index < 0 {
		index += length
	}
	return index % length
}

var (
	joinBorderColor = lipgloss.Color("62")
	joinMutedColor  = lipgloss.Color("245")
	joinTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	joinMutedStyle  = lipgloss.NewStyle().Foreground(joinMutedColor)
	joinPanelStyle  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	joinPanelTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
)

func joinBoxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(joinBorderColor).
		Padding(1, 2).
		Width(maxInt(40, width-2))
}

func renderJoinPanel(title string, width int, lines []string) string {
	body := append([]string{joinPanelTitle.Render(title), ""}, lines...)
	return joinPanelStyle.
		Width(maxInt(24, width-6)).
		Render(strings.Join(body, "\n"))
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func tuiWidth(width int, fallback int) int {
	if width <= 0 {
		return fallback
	}
	if width < 40 {
		return 40
	}
	return width
}

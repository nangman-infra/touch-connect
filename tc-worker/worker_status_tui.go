package tcworker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-worker/internal/client"
)

type workerDoneMsg struct {
	err error
}

type workerSnapshotMsg struct {
	snapshot contracts.SnapshotResponse
	err      error
}

type workerTickMsg time.Time

type workerStatusModel struct {
	runCtx context.Context
	cancel context.CancelFunc
	run    func(context.Context) error
	client *client.HTTPClient
	env    JoinEnvironment

	endpointRef string
	serverURL   string
	artifactDir string

	spinner spinner.Model
	width   int
	height  int

	snapshot      contracts.SnapshotResponse
	snapshotErr   error
	workerErr     error
	workerDone    bool
	stopRequested bool

	seenEndpointState string
	seenAttempts      map[string]string
	seenMessages      map[string]string
	seenCheckpoints   map[string]struct{}
	seenReadbacks     map[string]struct{}
	seenArtifacts     map[string]struct{}
	events            []string
}

func RunWorkerStatusTUI(ctx context.Context, env JoinEnvironment, run func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	restoreLog := redirectWorkerLog(env.Env["TC_WORKER_ARTIFACT_DIR"])
	defer restoreLog()
	model := newWorkerStatusModel(runCtx, cancel, env, run)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	final, ok := finalModel.(workerStatusModel)
	if !ok {
		return nil
	}
	if final.workerErr != nil && !strings.Contains(final.workerErr.Error(), "context canceled") {
		return final.workerErr
	}
	return nil
}

func newWorkerStatusModel(ctx context.Context, cancel context.CancelFunc, env JoinEnvironment, run func(context.Context) error) workerStatusModel {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	serverURL := env.Env["TC_WORKER_SERVER_URL"]
	if serverURL == "" {
		serverURL = "http://127.0.0.1:8080"
	}
	model := workerStatusModel{
		runCtx:          ctx,
		cancel:          cancel,
		run:             run,
		client:          client.NewHTTPClient(serverURL, &http.Client{Timeout: 2 * time.Second}),
		env:             env,
		endpointRef:     env.Env["TC_WORKER_ENDPOINT_REF"],
		serverURL:       serverURL,
		artifactDir:     env.Env["TC_WORKER_ARTIFACT_DIR"],
		spinner:         spin,
		seenAttempts:    map[string]string{},
		seenMessages:    map[string]string{},
		seenCheckpoints: map[string]struct{}{},
		seenReadbacks:   map[string]struct{}{},
		seenArtifacts:   map[string]struct{}{},
	}
	model.addEvent("starting worker backend=" + env.Backend + " model=" + printableModel(env.Model))
	return model
}

func (m workerStatusModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startWorkerCmd(), m.pollSnapshotCmd(), workerTickCmd())
}

func (m workerStatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	switch item := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = item.Width
		m.height = item.Height
	case tea.KeyMsg:
		switch item.String() {
		case "ctrl+c", "q":
			m.addEvent("stopping worker")
			m.stopRequested = true
			m.cancel()
			if m.workerDone {
				return m, tea.Quit
			}
		case "r":
			commands = append(commands, m.pollSnapshotCmd())
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(item)
		commands = append(commands, cmd)
	case workerTickMsg:
		commands = append(commands, m.spinner.Tick, m.pollSnapshotCmd(), workerTickCmd())
	case workerSnapshotMsg:
		if item.err != nil {
			m.snapshotErr = item.err
			m.addEvent("snapshot error: " + item.err.Error())
			break
		}
		m.snapshotErr = nil
		m.snapshot = item.snapshot
		m.observeSnapshot(item.snapshot)
	case workerDoneMsg:
		m.workerDone = true
		m.workerErr = item.err
		if item.err != nil {
			m.addEvent("worker stopped with error: " + item.err.Error())
		} else {
			m.addEvent("worker stopped")
		}
		if m.stopRequested {
			return m, tea.Quit
		}
	}
	return m, tea.Batch(commands...)
}

func (m workerStatusModel) View() string {
	width := tuiWidth(m.width, 100)
	endpoint, endpointOK := m.endpoint()
	activeMessage, activeAttempt, activeOK := m.activeMessage()
	capabilities := m.capabilitySummary(endpoint, endpointOK)
	state := "starting"
	if endpointOK {
		state = endpoint.ConnectionState
	}
	if activeOK {
		state = activeAttempt.State
	}
	if m.workerDone {
		state = "stopped"
	}
	header := workerHeaderStyle.Render("touch-connect worker") + "  " +
		workerMutedStyle.Render("endpoint "+defaultString(m.endpointRef, "unknown")) + "\n" +
		fmt.Sprintf("backend %-10s model %-12s server %s", m.env.Backend, printableModel(m.env.Model), m.serverURL)
	innerWidth := maxInt(40, width-8)
	var body string
	if width < 100 {
		body = strings.Join([]string{
			m.statusPane(innerWidth, state, capabilities, endpoint, endpointOK),
			m.messagePane(innerWidth, activeMessage, activeAttempt, activeOK),
		}, "\n")
	} else {
		leftWidth := 42
		rightWidth := maxInt(40, innerWidth-leftWidth)
		left := m.statusPane(leftWidth, state, capabilities, endpoint, endpointOK)
		right := m.messagePane(rightWidth, activeMessage, activeAttempt, activeOK)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}
	events := m.eventPane(width)
	return workerOuterStyle(width).Render(strings.Join([]string{header, body, events, workerMutedStyle.Render("q stop worker  r refresh  ? help")}, "\n"))
}

func (m workerStatusModel) startWorkerCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.run(m.runCtx)
		return workerDoneMsg{err: err}
	}
}

func (m workerStatusModel) pollSnapshotCmd() tea.Cmd {
	return func() tea.Msg {
		snapshot, err := m.client.Snapshot(m.runCtx)
		return workerSnapshotMsg{snapshot: snapshot, err: err}
	}
}

func workerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return workerTickMsg(t)
	})
}

func (m *workerStatusModel) observeSnapshot(snapshot contracts.SnapshotResponse) {
	if endpoint, ok := findEndpoint(snapshot.Endpoints, m.endpointRef); ok {
		if m.seenEndpointState != endpoint.ConnectionState {
			m.seenEndpointState = endpoint.ConnectionState
			m.addEvent("endpoint " + endpoint.ConnectionState)
		}
	}
	for _, message := range snapshot.Messages {
		if !m.messageBelongsToWorker(snapshot, message) {
			continue
		}
		if previous, ok := m.seenMessages[message.MessageRef]; !ok || previous != message.State {
			m.seenMessages[message.MessageRef] = message.State
			m.addEvent("message " + shortRef(message.MessageRef) + " " + message.State + " cap=" + message.TargetCapability)
		}
	}
	for _, attempt := range snapshot.Attempts {
		if attempt.EndpointRef != m.endpointRef {
			continue
		}
		if previous, ok := m.seenAttempts[attempt.AttemptRef]; !ok || previous != attempt.State {
			m.seenAttempts[attempt.AttemptRef] = attempt.State
			m.addEvent("attempt " + shortRef(attempt.AttemptRef) + " " + attempt.State)
		}
	}
	for _, readback := range snapshot.Readbacks {
		if readback.EndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenReadbacks[readback.ReadbackRef]; ok {
			continue
		}
		m.seenReadbacks[readback.ReadbackRef] = struct{}{}
		m.addEvent("readback recorded " + shortRef(readback.ReadbackRef))
	}
	for _, checkpoint := range snapshot.Checkpoints {
		if checkpoint.EndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenCheckpoints[checkpoint.CheckpointRef]; ok {
			continue
		}
		m.seenCheckpoints[checkpoint.CheckpointRef] = struct{}{}
		m.addEvent("checkpoint " + checkpoint.State + " " + quoteCompact(checkpoint.Summary, 44))
	}
	for _, artifact := range snapshot.Artifacts {
		if artifact.CreatedByEndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenArtifacts[artifact.ArtifactVersionRef]; ok {
			continue
		}
		m.seenArtifacts[artifact.ArtifactVersionRef] = struct{}{}
		m.addEvent("artifact written " + shortRef(artifact.ArtifactVersionRef))
	}
}

func (m workerStatusModel) messageBelongsToWorker(snapshot contracts.SnapshotResponse, message contracts.MessageRecord) bool {
	for _, attempt := range snapshot.Attempts {
		if attempt.EndpointRef == m.endpointRef && attempt.MessageRef == message.MessageRef {
			return true
		}
	}
	return false
}

func (m workerStatusModel) endpoint() (contracts.EndpointRecord, bool) {
	return findEndpoint(m.snapshot.Endpoints, m.endpointRef)
}

func (m workerStatusModel) activeMessage() (contracts.MessageRecord, contracts.AttemptRecord, bool) {
	attempts := make([]contracts.AttemptRecord, 0)
	for _, attempt := range m.snapshot.Attempts {
		if attempt.EndpointRef == m.endpointRef {
			attempts = append(attempts, attempt)
		}
	}
	sort.SliceStable(attempts, func(i, j int) bool {
		return attempts[i].AttemptRef > attempts[j].AttemptRef
	})
	for _, attempt := range attempts {
		if attempt.State == "claimed" || attempt.State == "in_progress" {
			if message, ok := findMessage(m.snapshot.Messages, attempt.MessageRef); ok {
				return message, attempt, true
			}
		}
	}
	if len(attempts) > 0 {
		if message, ok := findMessage(m.snapshot.Messages, attempts[0].MessageRef); ok {
			return message, attempts[0], true
		}
	}
	return contracts.MessageRecord{}, contracts.AttemptRecord{}, false
}

func (m workerStatusModel) capabilitySummary(endpoint contracts.EndpointRecord, ok bool) string {
	if ok && len(endpoint.Capabilities) > 0 {
		names := make([]string, 0, len(endpoint.Capabilities))
		for name := range endpoint.Capabilities {
			names = append(names, name)
		}
		sort.Strings(names)
		return strings.Join(names, ",")
	}
	if value := strings.TrimSpace(m.env.Env["TC_WORKER_CAPABILITIES"]); value != "" {
		return value
	}
	return "from SKILL.md"
}

func (m workerStatusModel) statusPane(width int, state string, capabilities string, endpoint contracts.EndpointRecord, endpointOK bool) string {
	heartbeat := "not registered"
	if endpointOK && endpoint.LastHeartbeatAt != "" {
		heartbeat = endpoint.LastHeartbeatAt
	}
	processed := 0
	failed := 0
	for _, attempt := range m.snapshot.Attempts {
		if attempt.EndpointRef != m.endpointRef {
			continue
		}
		if attempt.State == "completed" {
			processed++
		}
		if attempt.State == "failed" {
			failed++
		}
	}
	artifacts := 0
	for _, artifact := range m.snapshot.Artifacts {
		if artifact.CreatedByEndpointRef == m.endpointRef {
			artifacts++
		}
	}
	lines := []string{
		workerPaneTitle.Render("Live Status"),
		"",
		fmt.Sprintf("state       %s %s", m.spinner.View(), state),
		fmt.Sprintf("heartbeat   %s", heartbeat),
		fmt.Sprintf("capabilities %s", compact(capabilities, 32)),
		fmt.Sprintf("processed   %d", processed),
		fmt.Sprintf("failed      %d", failed),
		fmt.Sprintf("artifacts   %d", artifacts),
	}
	if m.snapshotErr != nil {
		lines = append(lines, "", workerWarnStyle.Render("snapshot error: "+compact(m.snapshotErr.Error(), 42)))
	}
	if m.workerErr != nil {
		lines = append(lines, "", workerErrorStyle.Render("worker error: "+compact(m.workerErr.Error(), 42)))
	}
	return workerPaneStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m workerStatusModel) messagePane(width int, message contracts.MessageRecord, attempt contracts.AttemptRecord, ok bool) string {
	lines := []string{workerPaneTitle.Render("Current Message"), ""}
	if !ok {
		lines = append(lines,
			"no active message",
			"",
			"Waiting for capability:",
			"  "+m.capabilitySummary(contracts.EndpointRecord{}, false),
		)
		return workerPaneStyle.Width(width).Render(strings.Join(lines, "\n"))
	}
	lines = append(lines,
		fmt.Sprintf("message  %s", message.MessageRef),
		fmt.Sprintf("attempt  %s", attempt.AttemptRef),
		fmt.Sprintf("task     %s", defaultString(message.CorrelationRef, "-")),
		fmt.Sprintf("state    %s", message.State),
		fmt.Sprintf("summary  %s", compact(message.Payload.Summary, 38)),
		"",
		"required output",
		"  WORKER_READBACK",
		"  WORKER_ACTION",
		"  WORKER_RESULT_READY",
	)
	if attempt.LeaseExpiresAt != "" {
		lines = append(lines, "", "lease expires "+attempt.LeaseExpiresAt)
	}
	return workerPaneStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m workerStatusModel) eventPane(width int) string {
	events := m.events
	if len(events) > 10 {
		events = events[len(events)-10:]
	}
	lines := []string{workerPaneTitle.Render("Event Log")}
	lines = append(lines, events...)
	if len(events) == 0 {
		lines = append(lines, "waiting for worker events")
	}
	return workerLogStyle.Width(maxInt(40, width-8)).Render(strings.Join(lines, "\n"))
}

func (m *workerStatusModel) addEvent(message string) {
	m.events = append(m.events, time.Now().Format("15:04:05")+"  "+message)
	if len(m.events) > 200 {
		m.events = m.events[len(m.events)-200:]
	}
}

func findEndpoint(items []contracts.EndpointRecord, endpointRef string) (contracts.EndpointRecord, bool) {
	for _, item := range items {
		if item.EndpointRef == endpointRef {
			return item, true
		}
	}
	return contracts.EndpointRecord{}, false
}

func findMessage(items []contracts.MessageRecord, messageRef string) (contracts.MessageRecord, bool) {
	for _, item := range items {
		if item.MessageRef == messageRef {
			return item, true
		}
	}
	return contracts.MessageRecord{}, false
}

func redirectWorkerLog(artifactDir string) func() {
	if strings.TrimSpace(artifactDir) == "" {
		return func() {}
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return func() {}
	}
	file, err := os.OpenFile(filepath.Join(artifactDir, "worker.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return func() {}
	}
	previous := log.Writer()
	log.SetOutput(file)
	return func() {
		log.SetOutput(previous)
		_ = file.Close()
	}
}

func shortRef(ref string) string {
	if index := strings.LastIndex(ref, "/"); index >= 0 && index+1 < len(ref) {
		return ref[index+1:]
	}
	return ref
}

func compact(value string, maxWidth int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxWidth <= 3 || lipgloss.Width(value) <= maxWidth {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxWidth-3 {
		return value
	}
	return string(runes[:maxWidth-3]) + "..."
}

func quoteCompact(value string, maxWidth int) string {
	value = compact(value, maxWidth)
	if value == "" {
		return "\"\""
	}
	return "\"" + value + "\""
}

var (
	workerOuterBorder = lipgloss.Color("62")
	workerPaneBorder  = lipgloss.Color("238")
	workerHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	workerMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	workerPaneTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	workerWarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	workerErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	workerPaneStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(workerPaneBorder).Padding(1, 2)
	workerLogStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(workerPaneBorder).Padding(1, 2)
)

func workerOuterStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(workerOuterBorder).
		Padding(1, 2).
		Width(maxInt(40, width-2))
}

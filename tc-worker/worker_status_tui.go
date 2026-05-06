package tcworker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

type workerFocus int

const (
	workerFocusMessage workerFocus = iota
	workerFocusReadback
	workerFocusResult
	workerFocusArtifacts
	workerFocusEvents
	workerFocusHelp
)

type workerLayoutMode int

const (
	workerLayoutWide workerLayoutMode = iota
	workerLayoutStandard
	workerLayoutCompact
	workerLayoutTiny
)

type workerEventKind string

const (
	workerEventSystem     workerEventKind = "system"
	workerEventEndpoint   workerEventKind = "endpoint"
	workerEventMessage    workerEventKind = "message"
	workerEventAttempt    workerEventKind = "attempt"
	workerEventReadback   workerEventKind = "readback"
	workerEventCheckpoint workerEventKind = "checkpoint"
	workerEventArtifact   workerEventKind = "artifact"
	workerEventError      workerEventKind = "error"
)

const (
	workerTextNotRegistered     = "not registered"
	workerTextWaitingEvents     = "waiting for worker events"
	workerTextNoClaimedMessage  = "No message has been claimed yet."
	workerPermissionDescription = "permission workspace-auto"
)

type workerEvent struct {
	At      time.Time
	Kind    workerEventKind
	Message string
}

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
	events            []workerEvent

	focus             workerFocus
	previousFocus     workerFocus
	bodyOffset        int
	readbackOffset    int
	resultOffset      int
	eventOffset       int
	eventFilter       workerEventKind
	artifactIndex     int
	artifactOffset    int
	artifactOpen      bool
	artifactContent   string
	artifactError     string
	artifactContentOf string
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
		focus:           workerFocusMessage,
	}
	model.addEventKind(workerEventSystem, "starting worker backend="+env.Backend+" model="+printableModel(env.Model))
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
		cmd := m.handleKey(item)
		if cmd != nil {
			commands = append(commands, cmd)
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
			m.addEventKind(workerEventError, "snapshot error: "+item.err.Error())
			break
		}
		m.snapshotErr = nil
		m.snapshot = item.snapshot
		m.observeSnapshot(item.snapshot)
		m.clampSelections()
	case workerDoneMsg:
		m.workerDone = true
		m.workerErr = item.err
		if item.err != nil {
			m.addEventKind(workerEventError, "worker stopped with error: "+item.err.Error())
		} else {
			m.addEventKind(workerEventSystem, "worker stopped")
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
	state := m.displayState(endpoint, endpointOK, activeAttempt, activeOK)
	mode := workerLayoutFor(width, m.height)
	content := m.renderCockpit(workerCockpitView{
		Mode:         mode,
		Width:        width,
		State:        state,
		Capabilities: capabilities,
		Endpoint:     endpoint,
		EndpointOK:   endpointOK,
		Message:      activeMessage,
		Attempt:      activeAttempt,
		ActiveOK:     activeOK,
	})
	if m.focus == workerFocusHelp {
		return content + "\n" + m.helpOverlay(width)
	}
	if m.artifactOpen {
		return content + "\n" + m.artifactViewer(width)
	}
	return content
}

type workerCockpitView struct {
	Mode         workerLayoutMode
	Width        int
	State        string
	Capabilities string
	Endpoint     contracts.EndpointRecord
	EndpointOK   bool
	Message      contracts.MessageRecord
	Attempt      contracts.AttemptRecord
	ActiveOK     bool
}

func (m workerStatusModel) renderCockpit(view workerCockpitView) string {
	contentWidth := maxInt(56, view.Width-4)
	header := m.header(contentWidth, view.State)
	task := m.taskStrip(contentWidth, view.Message, view.Attempt, view.ActiveOK)
	footer := m.footer(view.Mode)
	switch view.Mode {
	case workerLayoutTiny:
		return strings.Join([]string{
			m.tinyHeader(contentWidth, view.State),
			m.tinySummary(contentWidth, view.Message, view.Attempt, view.ActiveOK),
			m.workPane(contentWidth, maxInt(6, m.height-8), view.Message, view.Attempt, view.ActiveOK),
			footer,
		}, "\n")
	case workerLayoutCompact:
		workHeight := maxInt(10, m.height-10)
		return strings.Join([]string{
			header,
			task,
			m.contextSummary(contentWidth, view.State, view.Capabilities, view.Endpoint, view.EndpointOK),
			m.workPane(contentWidth, workHeight, view.Message, view.Attempt, view.ActiveOK),
			m.activitySummary(contentWidth),
			footer,
		}, "\n")
	case workerLayoutStandard:
		workHeight := clampInt(m.height-20, 10, 18)
		bottomHeight := maxInt(8, m.height-workHeight-12)
		gap := 1
		leftWidth := maxInt(40, (contentWidth-gap)*48/100)
		rightWidth := maxInt(40, contentWidth-leftWidth-gap)
		bottom := lipgloss.JoinHorizontal(lipgloss.Top,
			m.contextPane(leftWidth, bottomHeight, view.State, view.Capabilities, view.Endpoint, view.EndpointOK),
			strings.Repeat(" ", gap),
			m.activityPane(rightWidth, bottomHeight),
		)
		return strings.Join([]string{
			header,
			task,
			m.workPane(contentWidth, workHeight, view.Message, view.Attempt, view.ActiveOK),
			bottom,
			footer,
		}, "\n")
	default:
		gap := 1
		contextWidth := clampInt(contentWidth*30/100, 34, 46)
		workWidth := maxInt(56, contentWidth-contextWidth-gap)
		activityHeight := clampInt(m.height/4, 7, 10)
		workHeight := maxInt(12, m.height-activityHeight-12)
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.workPane(workWidth, workHeight, view.Message, view.Attempt, view.ActiveOK),
			strings.Repeat(" ", gap),
			m.contextPane(contextWidth, workHeight, view.State, view.Capabilities, view.Endpoint, view.EndpointOK),
		)
		return strings.Join([]string{
			header,
			task,
			top,
			m.activityPane(contentWidth, activityHeight),
			footer,
		}, "\n")
	}
}

func workerLayoutFor(width int, height int) workerLayoutMode {
	if width < 80 || height > 0 && height < 24 {
		return workerLayoutTiny
	}
	if width < 100 {
		return workerLayoutCompact
	}
	if width < 140 {
		return workerLayoutStandard
	}
	return workerLayoutWide
}

func (m *workerStatusModel) handleKey(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+c", "q":
		return m.stopWorkerKey()
	case "r":
		m.addEventKind(workerEventSystem, "manual refresh")
		return m.pollSnapshotCmd()
	case "?":
		m.toggleHelp()
	case "esc":
		m.closeOverlay()
	case "tab", "right":
		m.moveFocus(1)
	case "shift+tab", "backtab", "left":
		m.moveFocus(-1)
	case "1":
		m.focus = workerFocusMessage
	case "2":
		m.focus = workerFocusReadback
	case "3":
		m.focus = workerFocusResult
	case "4":
		m.focus = workerFocusArtifacts
	case "5":
		m.focus = workerFocusEvents
	case "enter":
		if m.focus == workerFocusArtifacts && !m.artifactOpen {
			m.openSelectedArtifact()
		}
	case "up", "k":
		m.scrollFocused(-1)
	case "down", "j":
		m.scrollFocused(1)
	case "pgup":
		m.scrollFocused(-6)
	case "pgdown":
		m.scrollFocused(6)
	case "home":
		m.scrollHome()
	case "end":
		m.scrollEnd()
	}
	return nil
}

func (m *workerStatusModel) stopWorkerKey() tea.Cmd {
	m.addEventKind(workerEventSystem, "stopping worker")
	m.stopRequested = true
	m.cancel()
	if m.workerDone {
		return tea.Quit
	}
	return nil
}

func (m *workerStatusModel) toggleHelp() {
	if m.focus == workerFocusHelp {
		m.focus = m.previousFocus
		return
	}
	m.previousFocus = m.focus
	m.focus = workerFocusHelp
}

func (m *workerStatusModel) closeOverlay() {
	if m.artifactOpen {
		m.artifactOpen = false
		m.artifactContent = ""
		m.artifactError = ""
		return
	}
	if m.focus == workerFocusHelp {
		m.focus = m.previousFocus
	}
}

func (m *workerStatusModel) moveFocus(direction int) {
	if m.focus == workerFocusHelp || m.artifactOpen {
		return
	}
	if direction > 0 {
		m.focus = nextWorkerFocus(m.focus)
		return
	}
	m.focus = previousWorkerFocus(m.focus)
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
	m.observeEndpoint(snapshot.Endpoints)
	m.observeMessages(snapshot)
	m.observeAttempts(snapshot.Attempts)
	m.observeReadbacks(snapshot.Readbacks)
	m.observeCheckpoints(snapshot.Checkpoints)
	m.observeArtifacts(snapshot.Artifacts)
}

func (m *workerStatusModel) observeEndpoint(endpoints []contracts.EndpointRecord) {
	if endpoint, ok := findEndpoint(endpoints, m.endpointRef); ok {
		if m.seenEndpointState != endpoint.ConnectionState {
			m.seenEndpointState = endpoint.ConnectionState
			m.addEventKind(workerEventEndpoint, "endpoint "+endpoint.ConnectionState)
		}
	}
}

func (m *workerStatusModel) observeMessages(snapshot contracts.SnapshotResponse) {
	for _, message := range snapshot.Messages {
		if !m.messageBelongsToWorker(snapshot, message) {
			continue
		}
		if previous, ok := m.seenMessages[message.MessageRef]; !ok || previous != message.State {
			m.seenMessages[message.MessageRef] = message.State
			m.addEventKind(workerEventMessage, "message "+shortRef(message.MessageRef)+" "+message.State+" cap="+message.TargetCapability)
		}
	}
}

func (m *workerStatusModel) observeAttempts(attempts []contracts.AttemptRecord) {
	for _, attempt := range attempts {
		if attempt.EndpointRef != m.endpointRef {
			continue
		}
		if previous, ok := m.seenAttempts[attempt.AttemptRef]; !ok || previous != attempt.State {
			m.seenAttempts[attempt.AttemptRef] = attempt.State
			m.addEventKind(workerEventAttempt, "attempt "+shortRef(attempt.AttemptRef)+" "+attempt.State)
		}
	}
}

func (m *workerStatusModel) observeReadbacks(readbacks []contracts.ReadbackRecord) {
	for _, readback := range readbacks {
		if readback.EndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenReadbacks[readback.ReadbackRef]; ok {
			continue
		}
		m.seenReadbacks[readback.ReadbackRef] = struct{}{}
		m.addEventKind(workerEventReadback, "readback recorded "+shortRef(readback.ReadbackRef))
	}
}

func (m *workerStatusModel) observeCheckpoints(checkpoints []contracts.CheckpointRecord) {
	for _, checkpoint := range checkpoints {
		if checkpoint.EndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenCheckpoints[checkpoint.CheckpointRef]; ok {
			continue
		}
		m.seenCheckpoints[checkpoint.CheckpointRef] = struct{}{}
		m.addEventKind(workerEventCheckpoint, "checkpoint "+checkpoint.State+" "+quoteCompact(checkpoint.Summary, 44))
	}
}

func (m *workerStatusModel) observeArtifacts(artifacts []contracts.ArtifactRecord) {
	for _, artifact := range artifacts {
		if artifact.CreatedByEndpointRef != m.endpointRef {
			continue
		}
		if _, ok := m.seenArtifacts[artifact.ArtifactVersionRef]; ok {
			continue
		}
		m.seenArtifacts[artifact.ArtifactVersionRef] = struct{}{}
		m.addEventKind(workerEventArtifact, "artifact written "+shortRef(artifact.ArtifactVersionRef))
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

func (m workerStatusModel) displayState(endpoint contracts.EndpointRecord, endpointOK bool, attempt contracts.AttemptRecord, activeOK bool) string {
	state := "starting"
	if endpointOK {
		state = endpoint.ConnectionState
	}
	if activeOK {
		state = attempt.State
	}
	if m.workerDone {
		state = "stopped"
	}
	if m.workerErr != nil {
		state = "error"
	}
	if m.stopRequested && !m.workerDone {
		state = "stopping"
	}
	return state
}

func (m workerStatusModel) header(width int, state string) string {
	title := workerHeaderStyle.Render("touch-connect worker")
	pill := renderWorkerStatePill(state)
	right := workerMutedStyle.Render("server " + m.serverURL)
	gap := strings.Repeat(" ", maxInt(1, width-lipgloss.Width(title)-lipgloss.Width(pill)-lipgloss.Width(right)-2))
	meta := fmt.Sprintf("endpoint %s  backend %s  model %s  %s", defaultString(shortRef(m.endpointRef), "unknown"), m.env.Backend, printableModel(m.env.Model), workerPermissionDescription)
	return title + " " + pill + gap + right + "\n" + workerMutedStyle.Render(compact(meta, maxInt(40, width-8)))
}

func (m workerStatusModel) taskStrip(width int, message contracts.MessageRecord, attempt contracts.AttemptRecord, ok bool) string {
	if !ok {
		return workerMutedStyle.Render(compact("Task idle - waiting for a message matching "+m.capabilitySummary(contracts.EndpointRecord{}, false), width))
	}
	taskRef := defaultString(message.CorrelationRef, "-")
	lineOne := fmt.Sprintf("Task  %s", shortRef(taskRef))
	lineTwo := fmt.Sprintf("Route manager -> %s   capability %s", shortRef(m.endpointRef), message.TargetCapability)
	lineThree := fmt.Sprintf("Refs  %s · %s", shortRef(message.MessageRef), shortRef(attempt.AttemptRef))
	if attempt.LeaseExpiresAt != "" {
		lineThree += "   lease " + compact(attempt.LeaseExpiresAt, 28)
	}
	return strings.Join([]string{
		compact(lineOne, width),
		workerMutedStyle.Render(compact(lineTwo, width)),
		workerMutedStyle.Render(compact(lineThree, width)),
	}, "\n")
}

func (m workerStatusModel) tinyHeader(width int, state string) string {
	title := "tc worker"
	pill := strings.ToUpper(defaultString(state, "unknown"))
	meta := fmt.Sprintf("%s %s", shortRef(m.endpointRef), printableModel(m.env.Model))
	return compact(title+" "+pill+" "+meta, width)
}

func (m workerStatusModel) tinySummary(width int, message contracts.MessageRecord, attempt contracts.AttemptRecord, ok bool) string {
	if !ok {
		return workerMutedStyle.Render("idle · waiting for matching capability")
	}
	return strings.Join([]string{
		compact(shortRef(defaultString(message.CorrelationRef, message.MessageRef)), width),
		workerMutedStyle.Render(compact(shortRef(message.MessageRef)+" · "+shortRef(attempt.AttemptRef)+" · "+message.TargetCapability, width)),
	}, "\n")
}

func (m workerStatusModel) contextSummary(width int, state string, capabilities string, endpoint contracts.EndpointRecord, endpointOK bool) string {
	artifactCount := len(m.workerArtifacts())
	endpointState := workerTextNotRegistered
	if endpointOK {
		endpointState = endpoint.ConnectionState
	}
	line := fmt.Sprintf("Context endpoint %s · capabilities %s · artifacts %d · %s",
		endpointState,
		compact(capabilities, 30),
		artifactCount,
		workerPermissionDescription,
	)
	if m.snapshotErr != nil {
		line += " · snapshot error"
	}
	if m.workerErr != nil {
		line += " · worker error"
	}
	_ = state
	_ = endpoint
	return workerMutedStyle.Render(compact(line, width))
}

func (m workerStatusModel) workPane(width int, height int, message contracts.MessageRecord, attempt contracts.AttemptRecord, ok bool) string {
	focus := m.activeWorkFocus()
	lines := []string{workerPaneTitle.Render("Work"), m.tabBar(width)}
	contentHeight := maxInt(1, height-4)
	var body []string
	switch focus {
	case workerFocusReadback:
		body = m.readbackTabLines(width, contentHeight, attempt, ok)
	case workerFocusResult:
		body = m.resultTabLines(width, contentHeight, attempt, ok)
	case workerFocusArtifacts:
		body = m.artifactTabLines(width, contentHeight)
	case workerFocusEvents:
		body = m.logTabLines(width, contentHeight)
	default:
		body = m.bodyTabLines(width, contentHeight, message, ok)
	}
	lines = append(lines, "")
	lines = append(lines, body...)
	return renderWorkerPane(width, height, lines)
}

func (m workerStatusModel) contextPane(width int, height int, state string, capabilities string, endpoint contracts.EndpointRecord, endpointOK bool) string {
	heartbeat := workerTextNotRegistered
	endpointState := workerTextNotRegistered
	if endpointOK {
		endpointState = endpoint.ConnectionState
		if endpoint.LastHeartbeatAt != "" {
			heartbeat = endpoint.LastHeartbeatAt
		}
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
	lines := []string{
		workerPaneTitle.Render("Context"),
		"",
		"worker",
		"  endpoint   " + endpointState,
		"  heartbeat  " + compact(heartbeat, maxInt(16, width-18)),
		"  backend    " + defaultString(m.env.Backend, "-"),
		"  model      " + printableModel(m.env.Model),
		"",
		"capabilities",
	}
	for _, item := range splitCompactList(capabilities, maxInt(16, width-8), 4) {
		lines = append(lines, "  "+item)
	}
	lines = append(lines,
		"",
		"run",
		fmt.Sprintf("  processed  %d", processed),
		fmt.Sprintf("  failed     %d", failed),
		fmt.Sprintf("  artifacts  %d", len(m.workerArtifacts())),
		"",
		workerWarnStyle.Render(workerPermissionDescription),
		workerMutedStyle.Render("trusted local workspace only"),
	)
	if m.snapshotErr != nil {
		lines = append(lines, "", workerErrorStyle.Render("snapshot "+compact(m.snapshotErr.Error(), maxInt(16, width-12))))
	}
	if m.workerErr != nil {
		lines = append(lines, "", workerErrorStyle.Render("worker "+compact(m.workerErr.Error(), maxInt(16, width-12))))
	}
	_ = state
	return renderWorkerPane(width, height, lines)
}

func (m workerStatusModel) activityPane(width int, height int) string {
	lines := []string{workerPaneTitle.Render("Recent Activity")}
	events := m.filteredEvents()
	if len(events) == 0 {
		lines = append(lines, workerMutedStyle.Render(workerTextWaitingEvents))
		return renderWorkerLogPane(width, height, lines)
	}
	eventLines := make([]string, 0, len(events))
	for _, event := range events {
		eventLines = append(eventLines, m.renderEvent(event, maxInt(20, width-14)))
	}
	visible := paneBodyHeight(height, 2)
	start := maxInt(0, len(eventLines)-visible-m.eventOffset)
	view := windowLines(eventLines, start, visible)
	lines = append(lines, view...)
	lines = append(lines, scrollHint(len(eventLines), start, len(view)))
	return renderWorkerLogPane(width, height, lines)
}

func (m workerStatusModel) activitySummary(width int) string {
	events := m.filteredEvents()
	if len(events) == 0 {
		return workerMutedStyle.Render("Recent " + workerTextWaitingEvents)
	}
	last := events[len(events)-1]
	return workerMutedStyle.Render(compact("Recent "+last.At.Format("15:04:05")+" "+string(last.Kind)+" "+last.Message, width))
}

func (m workerStatusModel) bodyTabLines(width int, height int, message contracts.MessageRecord, ok bool) []string {
	if !ok {
		return []string{
			workerMutedStyle.Render("idle"),
			"",
			"This worker is registered and waiting for a matching capability.",
			"",
			"listening for",
			"  " + compact(m.capabilitySummary(contracts.EndpointRecord{}, false), maxInt(20, width-8)),
		}
	}
	bodyWidth := maxInt(24, width-8)
	bodyLines := wrapText(message.Payload.Body, bodyWidth)
	if len(bodyLines) == 0 {
		bodyLines = []string{workerMutedStyle.Render("No payload body was provided.")}
	}
	view := windowLines(bodyLines, m.bodyOffset, maxInt(1, height-6))
	lines := []string{
		workerPaneSubtitle.Render("body"),
	}
	lines = append(lines, view...)
	lines = append(lines,
		scrollHint(len(bodyLines), m.bodyOffset, len(view)),
		"",
		workerMutedStyle.Render("required output: WORKER_READBACK · WORKER_ACTION · WORKER_RESULT_READY"),
	)
	return lines
}

func (m workerStatusModel) readbackTabLines(width int, height int, attempt contracts.AttemptRecord, ok bool) []string {
	if !ok {
		return []string{workerMutedStyle.Render(workerTextNoClaimedMessage)}
	}
	lines := m.readbackLinesForAttempt(attempt, width)
	view := windowLines(lines, m.readbackOffset, height)
	result := append([]string{}, view...)
	result = append(result, scrollHint(len(lines), m.readbackOffset, len(view)))
	return result
}

func (m workerStatusModel) resultTabLines(width int, height int, attempt contracts.AttemptRecord, ok bool) []string {
	if !ok {
		return []string{workerMutedStyle.Render(workerTextNoClaimedMessage)}
	}
	lines := m.checkpointLinesForAttempt(attempt, width)
	view := windowLines(lines, m.resultOffset, height)
	result := append([]string{}, view...)
	result = append(result, scrollHint(len(lines), m.resultOffset, len(view)))
	return result
}

func (m workerStatusModel) artifactTabLines(width int, height int) []string {
	artifacts := m.workerArtifacts()
	if len(artifacts) == 0 {
		return []string{workerMutedStyle.Render("No artifacts written by this worker yet.")}
	}
	start := clampInt(m.artifactOffset, 0, maxInt(0, len(artifacts)-1))
	end := minInt(len(artifacts), start+maxInt(1, height-2))
	lines := make([]string, 0, end-start+2)
	for index := start; index < end; index++ {
		artifact := artifacts[index]
		marker := " "
		if index == m.artifactIndex {
			marker = ">"
		}
		line := fmt.Sprintf("%s %s  %s  %s", marker, shortRef(artifact.ArtifactVersionRef), artifact.Kind, humanBytes(artifact.SizeBytes))
		lines = append(lines, compact(line, maxInt(20, width-6)))
	}
	lines = append(lines, scrollHint(len(artifacts), start, end-start))
	lines = append(lines, workerMutedStyle.Render("enter open selected artifact"))
	return lines
}

func (m workerStatusModel) logTabLines(width int, height int) []string {
	events := m.filteredEvents()
	if len(events) == 0 {
		return []string{workerMutedStyle.Render(workerTextWaitingEvents)}
	}
	eventLines := make([]string, 0, len(events))
	for _, event := range events {
		eventLines = append(eventLines, m.renderEvent(event, maxInt(20, width-14)))
	}
	view := windowLines(eventLines, m.eventOffset, maxInt(1, height-1))
	lines := append([]string{}, view...)
	lines = append(lines, scrollHint(len(eventLines), m.eventOffset, len(view)))
	return lines
}

func (m workerStatusModel) activeWorkFocus() workerFocus {
	if m.focus == workerFocusHelp {
		if m.previousFocus == workerFocusHelp {
			return workerFocusMessage
		}
		return m.previousFocus
	}
	return m.focus
}

func (m workerStatusModel) tabBar(width int) string {
	active := m.activeWorkFocus()
	items := []struct {
		focus workerFocus
		label string
	}{
		{workerFocusMessage, "1 Body"},
		{workerFocusReadback, "2 Readback"},
		{workerFocusResult, "3 Result"},
		{workerFocusArtifacts, "4 Artifacts"},
		{workerFocusEvents, "5 Log"},
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.focus == active {
			parts = append(parts, workerTabActiveStyle.Render(item.label))
			continue
		}
		parts = append(parts, workerTabStyle.Render(item.label))
	}
	return strings.Join(parts, "  ")
}

func splitCompactList(value string, width int, limit int) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, compact(part, width))
		if len(items) >= limit {
			break
		}
	}
	if len(items) == 0 {
		return []string{"-"}
	}
	if len(parts) > len(items) {
		items = append(items, compact(fmt.Sprintf("+%d more", len(parts)-len(items)), width))
	}
	return items
}

func (m *workerStatusModel) addEventKind(kind workerEventKind, message string) {
	m.events = append(m.events, workerEvent{At: time.Now(), Kind: kind, Message: message})
	if len(m.events) > 200 {
		m.events = m.events[len(m.events)-200:]
	}
}

func (m *workerStatusModel) setEventFilter(kind workerEventKind) {
	m.eventFilter = kind
	m.eventOffset = 0
	m.focus = workerFocusEvents
}

func (m workerStatusModel) eventFilterLabel() string {
	if m.eventFilter == "" {
		return "all"
	}
	return string(m.eventFilter)
}

func (m workerStatusModel) filteredEvents() []workerEvent {
	if m.eventFilter == "" {
		return m.events
	}
	result := make([]workerEvent, 0, len(m.events))
	for _, event := range m.events {
		if event.Kind == m.eventFilter {
			result = append(result, event)
		}
	}
	return result
}

func (m workerStatusModel) renderEvent(event workerEvent, width int) string {
	kind := string(event.Kind)
	style := workerMutedStyle
	switch event.Kind {
	case workerEventError:
		style = workerErrorStyle
	case workerEventArtifact:
		style = workerSuccessStyle
	case workerEventCheckpoint, workerEventReadback:
		style = workerInfoStyle
	}
	prefix := style.Render(fmt.Sprintf("%-10s", kind))
	return event.At.Format("15:04:05") + "  " + prefix + " " + compact(event.Message, width)
}

func (m workerStatusModel) paneTitle(title string, focus workerFocus) string {
	if m.focus == focus {
		return workerPaneTitle.Render("● " + title)
	}
	return workerPaneTitle.Render(title)
}

func (m workerStatusModel) footer(mode workerLayoutMode) string {
	if mode == workerLayoutTiny {
		return workerMutedStyle.Render("1 body  3 result  ? help  q stop")
	}
	return workerMutedStyle.Render("1 body  2 readback  3 result  4 artifacts  5 log   tab switch  j/k scroll  enter open  ? help  q stop")
}

func nextWorkerFocus(focus workerFocus) workerFocus {
	switch focus {
	case workerFocusMessage:
		return workerFocusReadback
	case workerFocusReadback:
		return workerFocusResult
	case workerFocusResult:
		return workerFocusArtifacts
	case workerFocusArtifacts:
		return workerFocusEvents
	default:
		return workerFocusMessage
	}
}

func previousWorkerFocus(focus workerFocus) workerFocus {
	switch focus {
	case workerFocusEvents:
		return workerFocusArtifacts
	case workerFocusArtifacts:
		return workerFocusResult
	case workerFocusResult:
		return workerFocusReadback
	case workerFocusReadback:
		return workerFocusMessage
	default:
		return workerFocusEvents
	}
}

func (m *workerStatusModel) scrollFocused(delta int) {
	if m.artifactOpen {
		m.artifactOffset = clampInt(m.artifactOffset+delta, 0, m.maxArtifactViewerOffset())
		return
	}
	switch m.focus {
	case workerFocusMessage:
		m.bodyOffset = clampInt(m.bodyOffset+delta, 0, m.maxBodyOffset())
	case workerFocusReadback:
		m.readbackOffset = clampInt(m.readbackOffset+delta, 0, m.maxReadbackOffset())
	case workerFocusResult:
		m.resultOffset = clampInt(m.resultOffset+delta, 0, m.maxResultOffset())
	case workerFocusEvents:
		m.eventOffset = clampInt(m.eventOffset+delta, 0, m.maxEventOffset())
	case workerFocusArtifacts:
		artifacts := m.workerArtifacts()
		if len(artifacts) == 0 {
			m.artifactIndex = 0
			m.artifactOffset = 0
			return
		}
		m.artifactIndex = clampInt(m.artifactIndex+delta, 0, len(artifacts)-1)
		if m.artifactIndex < m.artifactOffset {
			m.artifactOffset = m.artifactIndex
		}
		if m.artifactIndex >= m.artifactOffset+6 {
			m.artifactOffset = m.artifactIndex - 5
		}
	}
}

func (m *workerStatusModel) scrollHome() {
	if m.artifactOpen {
		m.artifactOffset = 0
		return
	}
	switch m.focus {
	case workerFocusMessage:
		m.bodyOffset = 0
	case workerFocusReadback:
		m.readbackOffset = 0
	case workerFocusResult:
		m.resultOffset = 0
	case workerFocusEvents:
		m.eventOffset = 0
	case workerFocusArtifacts:
		m.artifactIndex = 0
		m.artifactOffset = 0
	}
}

func (m *workerStatusModel) scrollEnd() {
	if m.artifactOpen {
		m.artifactOffset = m.maxArtifactViewerOffset()
		return
	}
	switch m.focus {
	case workerFocusMessage:
		m.bodyOffset = m.maxBodyOffset()
	case workerFocusReadback:
		m.readbackOffset = m.maxReadbackOffset()
	case workerFocusResult:
		m.resultOffset = m.maxResultOffset()
	case workerFocusEvents:
		m.eventOffset = m.maxEventOffset()
	case workerFocusArtifacts:
		artifacts := m.workerArtifacts()
		if len(artifacts) > 0 {
			m.artifactIndex = len(artifacts) - 1
			m.artifactOffset = maxInt(0, len(artifacts)-6)
		}
	}
}

func (m workerStatusModel) maxBodyOffset() int {
	message, _, ok := m.activeMessage()
	if !ok {
		return 0
	}
	width := maxInt(24, tuiWidth(m.width, 100)-16)
	return maxInt(0, len(wrapText(message.Payload.Body, width))-8)
}

func (m workerStatusModel) maxReadbackOffset() int {
	_, attempt, ok := m.activeMessage()
	if !ok {
		return 0
	}
	return maxInt(0, len(m.readbackLinesForAttempt(attempt, tuiWidth(m.width, 100)))-10)
}

func (m workerStatusModel) maxResultOffset() int {
	_, attempt, ok := m.activeMessage()
	if !ok {
		return 0
	}
	return maxInt(0, len(m.checkpointLinesForAttempt(attempt, tuiWidth(m.width, 100)))-10)
}

func (m workerStatusModel) maxEventOffset() int {
	return maxInt(0, len(m.filteredEvents())-10)
}

func (m workerStatusModel) maxArtifactViewerOffset() int {
	if m.artifactError != "" {
		return 0
	}
	height := maxInt(8, m.height/3)
	return maxInt(0, len(wrapText(m.artifactContent, maxInt(40, tuiWidth(m.width, 100)-12)))-height)
}

func (m *workerStatusModel) clampSelections() {
	artifacts := m.workerArtifacts()
	if len(artifacts) == 0 {
		m.artifactIndex = 0
		m.artifactOffset = 0
		return
	}
	m.artifactIndex = clampInt(m.artifactIndex, 0, len(artifacts)-1)
	m.artifactOffset = clampInt(m.artifactOffset, 0, len(artifacts)-1)
}

func (m workerStatusModel) latestReadback(attemptRef string) (contracts.ReadbackRecord, bool) {
	var latest contracts.ReadbackRecord
	ok := false
	for _, readback := range m.snapshot.Readbacks {
		if readback.AttemptRef != attemptRef || readback.EndpointRef != m.endpointRef {
			continue
		}
		if !ok || readback.Revision > latest.Revision || readback.ReadbackRef > latest.ReadbackRef {
			latest = readback
			ok = true
		}
	}
	return latest, ok
}

func (m workerStatusModel) latestCheckpoint(attemptRef string) (contracts.CheckpointRecord, bool) {
	var latest contracts.CheckpointRecord
	ok := false
	for _, checkpoint := range m.snapshot.Checkpoints {
		if checkpoint.AttemptRef != attemptRef || checkpoint.EndpointRef != m.endpointRef {
			continue
		}
		if !ok || checkpoint.Revision > latest.Revision || checkpoint.CheckpointRef > latest.CheckpointRef {
			latest = checkpoint
			ok = true
		}
	}
	return latest, ok
}

func (m workerStatusModel) readbackLinesForAttempt(attempt contracts.AttemptRecord, width int) []string {
	readback, ok := m.latestReadback(attempt.AttemptRef)
	if !ok {
		return []string{workerMutedStyle.Render("No readback recorded yet.")}
	}
	lines := []string{
		workerPaneSubtitle.Render("readback"),
	}
	understanding := wrapText(readback.Understanding, maxInt(24, width-8))
	if len(understanding) == 0 {
		understanding = []string{workerMutedStyle.Render("No understanding text was recorded.")}
	}
	lines = append(lines, understanding...)
	if len(readback.Questions) > 0 {
		lines = append(lines, "", "questions")
		for _, question := range readback.Questions {
			lines = append(lines, "  - "+compact(question, maxInt(20, width-10)))
		}
	}
	lines = append(lines,
		"",
		workerMutedStyle.Render("ref "+shortRef(readback.ReadbackRef)),
	)
	return lines
}

func (m workerStatusModel) checkpointLinesForAttempt(attempt contracts.AttemptRecord, width int) []string {
	checkpoint, ok := m.latestCheckpoint(attempt.AttemptRef)
	if !ok {
		return []string{workerMutedStyle.Render("No checkpoint result has been recorded yet.")}
	}
	lines := []string{
		workerPaneSubtitle.Render("result"),
		"state   " + checkpoint.State,
	}
	summary := wrapText("summary "+checkpoint.Summary, maxInt(24, width-8))
	lines = append(lines, summary...)
	if len(checkpoint.MissingFields) > 0 {
		lines = append(lines, "missing fields "+strings.Join(checkpoint.MissingFields, ", "))
	}
	if len(checkpoint.MissingReasons) > 0 {
		lines = append(lines, "missing reasons")
		for _, reason := range checkpoint.MissingReasons {
			lines = append(lines, "  - "+compact(reason, maxInt(20, width-10)))
		}
	}
	if len(checkpoint.ArtifactRefs) > 0 {
		lines = append(lines, "", "artifacts")
		for _, ref := range checkpoint.ArtifactRefs {
			lines = append(lines, "  "+shortRef(ref))
		}
	}
	lines = append(lines,
		"",
		workerMutedStyle.Render("checkpoint "+shortRef(checkpoint.CheckpointRef)),
	)
	return lines
}

func (m workerStatusModel) resultLinesForAttempt(attempt contracts.AttemptRecord, width int) []string {
	readback, hasReadback := m.latestReadback(attempt.AttemptRef)
	checkpoint, hasCheckpoint := m.latestCheckpoint(attempt.AttemptRef)
	resultLines := readbackResultLines(readback, hasReadback, width)
	if hasCheckpoint {
		resultLines = append(resultLines, checkpointResultLines(checkpoint, width)...)
	}
	if len(resultLines) == 0 {
		resultLines = append(resultLines, workerMutedStyle.Render("No worker result has been recorded yet."))
	}
	return resultLines
}

func readbackResultLines(readback contracts.ReadbackRecord, ok bool, width int) []string {
	if !ok {
		return []string{workerMutedStyle.Render("No readback recorded yet.")}
	}
	lines := []string{
		workerPaneSubtitle.Render("readback"),
		"understanding " + compact(readback.Understanding, maxInt(20, width-30)),
	}
	return appendQuestionLines(lines, readback.Questions, width)
}

func checkpointResultLines(checkpoint contracts.CheckpointRecord, width int) []string {
	lines := []string{
		"",
		workerPaneSubtitle.Render("checkpoint"),
		"state   " + checkpoint.State,
		"summary " + compact(checkpoint.Summary, maxInt(20, width-28)),
	}
	if len(checkpoint.MissingFields) > 0 {
		lines = append(lines, "missing fields "+strings.Join(checkpoint.MissingFields, ", "))
	}
	lines = appendReasonLines(lines, checkpoint.MissingReasons, width)
	if len(checkpoint.ArtifactRefs) > 0 {
		lines = append(lines, "artifacts "+strings.Join(checkpoint.ArtifactRefs, ", "))
	}
	return lines
}

func appendQuestionLines(lines []string, questions []string, width int) []string {
	if len(questions) == 0 {
		return lines
	}
	lines = append(lines, "questions")
	for _, question := range questions {
		lines = append(lines, "  - "+compact(question, maxInt(20, width-24)))
	}
	return lines
}

func appendReasonLines(lines []string, reasons []string, width int) []string {
	if len(reasons) == 0 {
		return lines
	}
	lines = append(lines, "missing reasons")
	for _, reason := range reasons {
		lines = append(lines, "  - "+compact(reason, maxInt(20, width-24)))
	}
	return lines
}

func (m workerStatusModel) workerArtifacts() []contracts.ArtifactRecord {
	artifacts := make([]contracts.ArtifactRecord, 0)
	for _, artifact := range m.snapshot.Artifacts {
		if artifact.CreatedByEndpointRef == m.endpointRef {
			artifacts = append(artifacts, artifact)
		}
	}
	sort.SliceStable(artifacts, func(i, j int) bool {
		return artifacts[i].ArtifactVersionRef > artifacts[j].ArtifactVersionRef
	})
	return artifacts
}

func (m *workerStatusModel) openSelectedArtifact() {
	artifacts := m.workerArtifacts()
	if len(artifacts) == 0 {
		return
	}
	m.artifactIndex = clampInt(m.artifactIndex, 0, len(artifacts)-1)
	artifact := artifacts[m.artifactIndex]
	m.artifactOpen = true
	m.artifactOffset = 0
	m.artifactContentOf = artifact.ArtifactVersionRef
	content, err := readArtifactPreview(artifact)
	if err != nil {
		m.artifactContent = ""
		m.artifactError = err.Error()
		m.addEventKind(workerEventError, "artifact open failed "+shortRef(artifact.ArtifactVersionRef)+": "+err.Error())
		return
	}
	m.artifactContent = content
	m.artifactError = ""
	m.addEventKind(workerEventArtifact, "artifact opened "+shortRef(artifact.ArtifactVersionRef))
}

func (m workerStatusModel) artifactViewer(width int) string {
	title := "Artifact Viewer"
	lines := []string{workerPaneTitle.Render(title), ""}
	if m.artifactError != "" {
		lines = append(lines, workerErrorStyle.Render(m.artifactError))
	} else {
		lines = append(lines, "artifact "+m.artifactContentOf, "")
		contentLines := wrapText(m.artifactContent, maxInt(40, width-12))
		view := windowLines(contentLines, m.artifactOffset, maxInt(8, m.height/3))
		lines = append(lines, view...)
		lines = append(lines, scrollHint(len(contentLines), m.artifactOffset, len(view)))
	}
	lines = append(lines, "", workerMutedStyle.Render("esc close  j/k scroll  q stop worker"))
	return workerModalStyle(width).Render(strings.Join(lines, "\n"))
}

func (m workerStatusModel) helpOverlay(width int) string {
	lines := []string{
		workerPaneTitle.Render("Worker Console Help"),
		"",
		"navigation",
		"  1..5               switch Body, Readback, Result, Artifacts, Log",
		"  tab / shift+tab    cycle worker tabs",
		"  j/k or arrows      scroll the active tab",
		"  home/end           jump to top or bottom",
		"",
		"actions",
		"  enter              open the selected artifact",
		"  r                  refresh snapshot now",
		"  ? / esc            close this help",
		"  q / ctrl+c         stop worker",
		"",
		"worker contract",
		"  WORKER_READBACK",
		"  WORKER_ACTION",
		"  WORKER_RESULT_READY",
	}
	return workerModalStyle(width).Render(strings.Join(lines, "\n"))
}

func readArtifactPreview(artifact contracts.ArtifactRecord) (string, error) {
	if !strings.HasPrefix(artifact.MediaType, "text/") && artifact.MediaType != "application/json" {
		return strings.Join([]string{
			"binary or non-inline artifact",
			"kind       " + artifact.Kind,
			"media_type " + artifact.MediaType,
			"size       " + humanBytes(artifact.SizeBytes),
			"storage    " + artifact.StorageRef,
		}, "\n"), nil
	}
	path, err := artifactFilePath(artifact.StorageRef)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	const limit = 256 * 1024
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if len(data) > limit {
		return string(data[:limit]) + "\n\n[truncated at 256 KiB]", nil
	}
	return string(data), nil
}

func artifactFilePath(storageRef string) (string, error) {
	if !strings.HasPrefix(storageRef, "file://") {
		return "", fmt.Errorf("artifact storage is not a local file: %s", storageRef)
	}
	value := strings.TrimPrefix(storageRef, "file://")
	if decoded, err := url.PathUnescape(value); err == nil {
		value = decoded
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("artifact storage path is empty")
	}
	return value, nil
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
		return noopLogRestore
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return noopLogRestore
	}
	file, err := os.OpenFile(filepath.Join(artifactDir, "worker.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return noopLogRestore
	}
	previous := log.Writer()
	log.SetOutput(file)
	return func() {
		log.SetOutput(previous)
		_ = file.Close()
	}
}

func noopLogRestore() {
	// No log redirection was installed, so there is nothing to restore.
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

func wrapText(value string, width int) []string {
	width = maxInt(12, width)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	var lines []string
	for _, raw := range strings.Split(value, "\n") {
		if raw == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(raw)
		for len(runes) > width {
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		lines = append(lines, string(runes))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func windowLines(lines []string, offset int, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	maxOffset := len(lines) - height
	offset = clampInt(offset, 0, maxOffset)
	return lines[offset : offset+height]
}

func scrollHint(total int, offset int, visible int) string {
	if total <= 0 {
		return workerMutedStyle.Render("showing 0")
	}
	if visible <= 0 || total <= visible {
		return workerMutedStyle.Render(fmt.Sprintf("showing %d", total))
	}
	maxOffset := maxInt(0, total-visible)
	offset = clampInt(offset, 0, maxOffset)
	return workerMutedStyle.Render(fmt.Sprintf("showing %d-%d of %d", offset+1, minInt(total, offset+visible), total))
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func clampInt(value int, low int, high int) int {
	if high < low {
		return low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func humanBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
}

func quoteCompact(value string, maxWidth int) string {
	value = compact(value, maxWidth)
	if value == "" {
		return "\"\""
	}
	return "\"" + value + "\""
}

var (
	workerOuterBorder    = lipgloss.Color("62")
	workerPaneBorder     = lipgloss.Color("238")
	workerHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	workerMutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	workerPaneTitle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	workerPaneSubtitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	workerTabStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	workerTabActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("24")).
				Bold(true).
				Padding(0, 1)
	workerInfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	workerSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	workerWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	workerErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	workerPaneStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(workerPaneBorder).Padding(1, 2)
	workerLogStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(workerPaneBorder).Padding(1, 2)
)

func renderWorkerStatePill(state string) string {
	label := strings.ToUpper(defaultString(state, "unknown"))
	bg := lipgloss.Color("238")
	fg := lipgloss.Color("250")
	switch state {
	case "online", "available", "completed":
		bg = lipgloss.Color("29")
		fg = lipgloss.Color("15")
	case "claimed", "in_progress", "takeover_candidate":
		bg = lipgloss.Color("25")
		fg = lipgloss.Color("15")
	case "starting", "stopping":
		bg = lipgloss.Color("136")
		fg = lipgloss.Color("15")
	case "failed", "dead_lettered", "error":
		bg = lipgloss.Color("124")
		fg = lipgloss.Color("15")
	case "stopped", "offline", "stale":
		bg = lipgloss.Color("238")
		fg = lipgloss.Color("250")
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func workerOuterStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(workerOuterBorder).
		Padding(1, 2).
		Width(maxInt(40, width-2))
}

func renderWorkerPane(width int, height int, lines []string) string {
	return workerPaneStyle.
		Width(maxInt(24, width-6)).
		Render(strings.Join(fitLines(lines, maxInt(4, height)), "\n"))
}

func renderWorkerLogPane(width int, height int, lines []string) string {
	return workerLogStyle.
		Width(maxInt(32, width-6)).
		Render(strings.Join(fitLines(lines, maxInt(4, height)), "\n"))
}

func paneBodyHeight(paneHeight int, reservedLines int) int {
	return maxInt(1, paneHeight-reservedLines)
}

func fitLines(lines []string, height int) []string {
	result := append([]string(nil), lines...)
	if len(result) > height {
		return result[:height]
	}
	for len(result) < height {
		result = append(result, "")
	}
	return result
}

func workerModalStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2).
		Width(maxInt(40, width-6))
}

package tcworker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestWorkerStatusTUIRendersMessageResultArtifactAndHelp(t *testing.T) {
	artifactDir := t.TempDir()
	artifactPath := filepath.Join(artifactDir, "result.json")
	if err := os.WriteFile(artifactPath, []byte(`{"result":"ready"}`), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newWorkerStatusModel(ctx, cancel, JoinEnvironment{
		Backend: "claude",
		Model:   "opus[1m]",
		Env: map[string]string{
			"TC_WORKER_SERVER_URL":   "http://127.0.0.1:8080",
			"TC_WORKER_ENDPOINT_REF": "tc://endpoint/test_worker",
			"TC_WORKER_ARTIFACT_DIR": artifactDir,
			"TC_WORKER_CAPABILITIES": "code.change",
		},
	}, func(context.Context) error { return nil })
	model.width = 120
	model.height = 40
	model.snapshot = contracts.SnapshotResponse{
		Endpoints: []contracts.EndpointRecord{{
			EndpointRef:     "tc://endpoint/test_worker",
			ConnectionState: "online",
			Capabilities: map[string]contracts.Capability{
				"code.change": {Name: "code.change"},
			},
		}},
		Messages: []contracts.MessageRecord{{
			MessageRef:       "tc://message/msg_test",
			TargetCapability: "code.change",
			State:            "completed",
			AttemptRef:       "tc://attempt/att_test",
			Payload: contracts.Payload{
				Summary: "test handoff",
				Body:    "goal: render the worker message body\nnext_action: verify result panes",
			},
		}},
		Attempts: []contracts.AttemptRecord{{
			AttemptRef:  "tc://attempt/att_test",
			MessageRef:  "tc://message/msg_test",
			EndpointRef: "tc://endpoint/test_worker",
			State:       "completed",
		}},
		Readbacks: []contracts.ReadbackRecord{{
			ReadbackRef:   "tc://readback/rb_test",
			AttemptRef:    "tc://attempt/att_test",
			EndpointRef:   "tc://endpoint/test_worker",
			Understanding: "I understand the manager handoff.",
			Revision:      1,
		}},
		Checkpoints: []contracts.CheckpointRecord{{
			CheckpointRef: "tc://checkpoint/chk_test",
			AttemptRef:    "tc://attempt/att_test",
			EndpointRef:   "tc://endpoint/test_worker",
			State:         "completed",
			Summary:       "Worker result is ready.",
			Revision:      1,
			ArtifactRefs:  []string{"tc://artifact-version/artv_test"},
		}},
		Artifacts: []contracts.ArtifactRecord{{
			ArtifactVersionRef:   "tc://artifact-version/artv_test",
			Kind:                 "log_bundle",
			MediaType:            "application/json",
			SizeBytes:            18,
			StorageRef:           "file://" + artifactPath,
			CreatedByEndpointRef: "tc://endpoint/test_worker",
		}},
	}
	model.observeSnapshot(model.snapshot)

	view := model.View()
	for _, expected := range []string{"Work", "1 Body", "goal: render the worker message body", "Recent Activity"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected worker TUI view to contain %q\n%s", expected, view)
		}
	}
	assertWorkerViewWidth(t, view, 120)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	readbackModel := updated.(workerStatusModel)
	if view := readbackModel.View(); !strings.Contains(view, "I understand the manager handoff.") {
		t.Fatalf("expected readback tab in worker TUI view\n%s", view)
	} else {
		assertWorkerViewWidth(t, view, 120)
	}

	updated, _ = readbackModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	resultModel := updated.(workerStatusModel)
	if view := resultModel.View(); !strings.Contains(view, "Worker result is ready.") {
		t.Fatalf("expected result tab in worker TUI view\n%s", view)
	} else {
		assertWorkerViewWidth(t, view, 120)
	}

	updated, _ = resultModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	artifactListModel := updated.(workerStatusModel)
	if view := artifactListModel.View(); !strings.Contains(view, "artv_test") {
		t.Fatalf("expected artifact tab in worker TUI view\n%s", view)
	} else {
		assertWorkerViewWidth(t, view, 120)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	helpModel := updated.(workerStatusModel)
	if !strings.Contains(helpModel.View(), "Worker Console Help") {
		t.Fatalf("expected help overlay in worker TUI view\n%s", helpModel.View())
	}

	helpModel.focus = workerFocusArtifacts
	updated, _ = helpModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	artifactModel := updated.(workerStatusModel)
	if !artifactModel.artifactOpen || !strings.Contains(artifactModel.View(), `"result":"ready"`) {
		t.Fatalf("expected artifact viewer to render inline artifact\n%s", artifactModel.View())
	}
}

func TestWorkerStatusTUIKeyNavigationFiltersAndUtilityStates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := newWorkerStatusModel(ctx, cancel, JoinEnvironment{
		Backend: "codex",
		Model:   "gpt-5.4-mini",
		Env: map[string]string{
			"TC_WORKER_SERVER_URL":   "http://127.0.0.1:8080",
			"TC_WORKER_ENDPOINT_REF": "tc://endpoint/test_worker",
			"TC_WORKER_CAPABILITIES": "code.change,ai.review,admin.audit",
		},
	}, func(context.Context) error { return nil })
	model.width = 78
	model.height = 20
	if workerLayoutFor(model.width, model.height) != workerLayoutTiny {
		t.Fatalf("expected tiny layout for narrow terminal")
	}
	if view := model.View(); !strings.Contains(view, "idle") || !strings.Contains(view, "waiting for matching capability") {
		t.Fatalf("expected empty worker state in view\n%s", view)
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 150, Height: 44})
	model = updated.(workerStatusModel)
	if workerLayoutFor(model.width, model.height) != workerLayoutWide {
		t.Fatalf("expected wide layout after resize")
	}
	updated, _ = model.Update(workerSnapshotMsg{err: os.ErrNotExist})
	model = updated.(workerStatusModel)
	if model.snapshotErr == nil || len(model.filteredEvents()) == 0 {
		t.Fatalf("expected snapshot error event")
	}
	model.setEventFilter(workerEventError)
	if model.eventFilterLabel() != string(workerEventError) || len(model.filteredEvents()) != 1 {
		t.Fatalf("expected event filter to select error events")
	}
	if !strings.Contains(model.renderEvent(model.filteredEvents()[0], 80), "snapshot error") {
		t.Fatalf("expected rendered error event")
	}

	model.focus = workerFocusMessage
	model.moveFocus(1)
	if model.focus != workerFocusReadback {
		t.Fatalf("expected next focus, got %v", model.focus)
	}
	model.moveFocus(-1)
	if model.focus != workerFocusMessage {
		t.Fatalf("expected previous focus, got %v", model.focus)
	}
	model.toggleHelp()
	model.moveFocus(1)
	if model.focus != workerFocusHelp {
		t.Fatalf("focus should not move while help is open")
	}
	model.closeOverlay()
	if model.focus != workerFocusMessage {
		t.Fatalf("expected help close to restore previous focus")
	}
	model.workerDone = true
	if cmd := model.stopWorkerKey(); cmd == nil {
		t.Fatalf("expected quit command when stopping finished worker")
	}
}

func TestWorkerStatusTUIScrollResultAndArtifactHelpers(t *testing.T) {
	model := newScrollWorkerStatusModel(t)
	_, attempt, ok := model.activeMessage()
	if !ok {
		t.Fatalf("expected active message")
	}
	if lines := model.resultLinesForAttempt(attempt, 120); !containsLineFragment(lines, "manager omitted target") {
		t.Fatalf("expected result lines to include missing reason: %#v", lines)
	}
	if lines := model.logTabLines(80, 8); len(lines) == 0 {
		t.Fatalf("expected log tab lines")
	}
}

func TestWorkerStatusTUIScrollOffsetsAndArtifactOpen(t *testing.T) {
	model := newScrollWorkerStatusModel(t)
	model.focus = workerFocusMessage
	model.scrollEnd()
	if model.bodyOffset == 0 {
		t.Fatalf("expected body scroll end to move offset")
	}
	model.scrollHome()
	if model.bodyOffset != 0 {
		t.Fatalf("expected body scroll home to reset offset")
	}
	model.focus = workerFocusArtifacts
	model.scrollEnd()
	if model.artifactIndex == 0 {
		t.Fatalf("expected artifact scroll end to move selection")
	}
	model.scrollFocused(-1)
	model.openSelectedArtifact()
	if !model.artifactOpen {
		t.Fatalf("expected selected artifact to open")
	}
	model.scrollEnd()
	if model.artifactOffset == 0 {
		t.Fatalf("expected artifact viewer offset to move")
	}
	model.closeOverlay()
	if model.artifactOpen || model.artifactContent != "" || model.artifactError != "" {
		t.Fatalf("expected artifact overlay to close")
	}
}

func TestWorkerStatusTUIArtifactAndFormattingHelpers(t *testing.T) {
	model := newScrollWorkerStatusModel(t)
	binaryPreview, err := readArtifactPreview(model.snapshot.Artifacts[1])
	if err != nil || !strings.Contains(binaryPreview, "binary or non-inline artifact") || !strings.Contains(binaryPreview, "2.0 KiB") {
		t.Fatalf("unexpected binary preview=%q err=%v", binaryPreview, err)
	}
	if _, err := artifactFilePath("s3://bucket/key"); err == nil {
		t.Fatalf("expected non-file artifact path to fail")
	}
	for _, state := range []string{"completed", "claimed", "starting", "failed", "offline", ""} {
		if renderWorkerStatePill(state) == "" {
			t.Fatalf("state pill should render for %q", state)
		}
	}
	if compact("hello world", 8) != "hello..." {
		t.Fatalf("unexpected compact result")
	}
	if quoteCompact("", 10) != "\"\"" {
		t.Fatalf("unexpected empty quote compact")
	}
	if len(windowLines([]string{"a", "b", "c"}, 1, 2)) != 2 {
		t.Fatalf("unexpected window lines")
	}
	if !strings.Contains(scrollHint(10, 2, 3), "3-5 of 10") {
		t.Fatalf("unexpected scroll hint")
	}
	if humanBytes(1024*1024) != "1.0 MiB" {
		t.Fatalf("unexpected human bytes")
	}
}

func newScrollWorkerStatusModel(t *testing.T) workerStatusModel {
	t.Helper()
	artifactDir := t.TempDir()
	artifactPath := filepath.Join(artifactDir, "result.txt")
	if err := os.WriteFile(artifactPath, []byte(strings.Repeat("artifact line\n", 40)), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	model := newWorkerStatusModel(ctx, cancel, JoinEnvironment{
		Backend: "claude",
		Model:   "opus[1m]",
		Env: map[string]string{
			"TC_WORKER_SERVER_URL":   "http://127.0.0.1:8080",
			"TC_WORKER_ENDPOINT_REF": "tc://endpoint/test_worker",
			"TC_WORKER_ARTIFACT_DIR": artifactDir,
		},
	}, func(context.Context) error { return nil })
	model.width = 120
	model.height = 30
	model.snapshot = contracts.SnapshotResponse{
		Messages: []contracts.MessageRecord{{
			MessageRef: "tc://message/msg_scroll",
			State:      "completed",
			AttemptRef: "tc://attempt/att_scroll",
			Payload: contracts.Payload{
				Body: strings.Repeat("body line\n", 30),
			},
		}},
		Attempts: []contracts.AttemptRecord{{
			AttemptRef:  "tc://attempt/att_scroll",
			MessageRef:  "tc://message/msg_scroll",
			EndpointRef: "tc://endpoint/test_worker",
			State:       "completed",
		}},
		Readbacks: []contracts.ReadbackRecord{{
			ReadbackRef:   "tc://readback/rb_scroll",
			AttemptRef:    "tc://attempt/att_scroll",
			EndpointRef:   "tc://endpoint/test_worker",
			Understanding: "understood",
			Questions:     []string{"question one"},
			Revision:      1,
		}},
		Checkpoints: []contracts.CheckpointRecord{{
			CheckpointRef:  "tc://checkpoint/chk_scroll",
			AttemptRef:     "tc://attempt/att_scroll",
			EndpointRef:    "tc://endpoint/test_worker",
			State:          "blocked",
			Summary:        "needs missing field",
			MissingFields:  []string{"target"},
			MissingReasons: []string{"manager omitted target"},
			ArtifactRefs:   []string{"tc://artifact-version/artv_scroll"},
			Revision:       1,
		}},
		Artifacts: []contracts.ArtifactRecord{
			{
				ArtifactVersionRef:   "tc://artifact-version/artv_scroll",
				Kind:                 "log_bundle",
				MediaType:            "text/plain",
				SizeBytes:            512,
				StorageRef:           "file://" + artifactPath,
				CreatedByEndpointRef: "tc://endpoint/test_worker",
			},
			{
				ArtifactVersionRef:   "tc://artifact-version/artv_binary",
				Kind:                 "binary",
				MediaType:            "application/octet-stream",
				SizeBytes:            2048,
				StorageRef:           "s3://bucket/key",
				CreatedByEndpointRef: "tc://endpoint/test_worker",
			},
		},
	}
	model.observeSnapshot(model.snapshot)
	return model
}

func assertWorkerViewWidth(t *testing.T, view string, maxWidth int) {
	t.Helper()
	for lineNo, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > maxWidth {
			t.Fatalf("worker TUI line %d exceeds terminal width: width=%d\n%s", lineNo+1, width, view)
		}
	}
}

func containsLineFragment(lines []string, fragment string) bool {
	for _, line := range lines {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

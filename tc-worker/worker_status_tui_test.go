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

func assertWorkerViewWidth(t *testing.T, view string, maxWidth int) {
	t.Helper()
	for lineNo, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > maxWidth {
			t.Fatalf("worker TUI line %d exceeds terminal width: width=%d\n%s", lineNo+1, width, view)
		}
	}
}

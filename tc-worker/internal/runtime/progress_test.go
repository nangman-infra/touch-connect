package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestProgressHelpersTrackAndClearAttemptState(t *testing.T) {
	worker := NewWithExecutor(&successfulLoopClient{}, Config{
		EndpointRef:   "tc://endpoint/worker_progress_helpers",
		DisplayName:   "progress helper worker",
		ActorID:       "actor.progress-helper",
		WorkspaceID:   "workspace.progress-helper",
		WorkerVersion: "0.1.0-dev",
		Capabilities:  []contracts.Capability{{Name: "code.change"}},
	}, EchoExecutor{})
	if worker.progressInterval() != defaultProgressInterval {
		t.Fatalf("expected default progress interval")
	}
	worker.config.ProgressInterval = 7 * time.Second
	if worker.progressInterval() != 7*time.Second {
		t.Fatalf("expected configured progress interval")
	}

	worker.markProgress("tc://attempt/att_progress", "  still   working  ")
	attemptRef, lastActivityAt, summary := worker.progressSnapshot()
	if attemptRef != "tc://attempt/att_progress" || lastActivityAt.IsZero() || summary != "  still   working  " {
		t.Fatalf("unexpected progress snapshot: attempt=%s last=%s summary=%q", attemptRef, lastActivityAt, summary)
	}
	if worker.readbackAlreadySubmitted("tc://attempt/att_progress") {
		t.Fatalf("readback should not be marked before submission")
	}
	worker.markReadbackSubmitted("tc://attempt/att_progress")
	if !worker.readbackAlreadySubmitted("tc://attempt/att_progress") {
		t.Fatalf("readback marker was not retained")
	}

	worker.clearProgress("tc://attempt/other")
	attemptRef, _, _ = worker.progressSnapshot()
	if attemptRef != "tc://attempt/att_progress" {
		t.Fatalf("non-matching clear should not clear active attempt")
	}
	worker.clearProgress("tc://attempt/att_progress")
	attemptRef, lastActivityAt, summary = worker.progressSnapshot()
	if attemptRef != "" || lastActivityAt.IsZero() || summary != "" {
		t.Fatalf("expected cleared progress snapshot, got attempt=%s last=%s summary=%q", attemptRef, lastActivityAt, summary)
	}
	if worker.readbackAlreadySubmitted("tc://attempt/att_progress") {
		t.Fatalf("cleared attempt should remove readback marker")
	}
}

func TestProgressFormattingHelpers(t *testing.T) {
	if got := formatOptionalWorkerTime(time.Time{}); got != "" {
		t.Fatalf("expected empty zero time, got %q", got)
	}
	now := time.Date(2026, 5, 7, 3, 0, 0, 123, time.UTC)
	if got := formatOptionalWorkerTime(now); got != now.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected formatted time: %q", got)
	}
	if got := compactWorkerProgress(" one \n two \t three "); got != "one two three" {
		t.Fatalf("unexpected compact summary: %q", got)
	}
	long := strings.Repeat("x", 300)
	if got := compactWorkerProgress(long); len(got) != 240 || !strings.HasSuffix(got, "...") {
		t.Fatalf("expected compacted long summary, got len=%d suffix=%q", len(got), got[len(got)-3:])
	}
}

func TestProgressReporterEmitsStillWorkingCheckpoint(t *testing.T) {
	client := &successfulLoopClient{}
	worker := NewWithExecutor(client, Config{
		EndpointRef:      "tc://endpoint/worker_progress_reporter",
		DisplayName:      "progress reporter worker",
		ActorID:          "actor.progress-reporter",
		WorkspaceID:      "workspace.progress-reporter",
		WorkerVersion:    "0.1.0-dev",
		Capabilities:     []contracts.Capability{{Name: "code.change"}},
		ProgressInterval: 2 * time.Millisecond,
	}, EchoExecutor{})
	claim := contracts.ClaimMessageResponse{
		MessageRef: "tc://message/msg_progress_reporter",
		AttemptRef: "tc://attempt/att_progress_reporter",
	}

	reporter := worker.startProgressReporter(context.Background(), claim)
	defer reporter.stop()
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(client.checkpointRequests) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(client.checkpointRequests) == 0 {
		t.Fatalf("expected periodic progress checkpoint")
	}
	got := client.checkpointRequests[0]
	if got.State != "in_progress" || got.Summary != "still working on "+claim.MessageRef {
		t.Fatalf("unexpected progress checkpoint: %+v", got)
	}
}

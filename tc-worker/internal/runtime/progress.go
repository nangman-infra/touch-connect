package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const defaultProgressInterval = 30 * time.Second

type progressReporter struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (r *Runtime) startProgressReporter(ctx context.Context, claim contracts.ClaimMessageResponse) *progressReporter {
	interval := r.progressInterval()
	reporterCtx, cancel := context.WithCancel(ctx)
	reporter := &progressReporter{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(reporter.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-reporterCtx.Done():
				return
			case <-ticker.C:
				summary := "still working on " + claim.MessageRef
				r.markProgress(claim.AttemptRef, summary)
				_ = r.SubmitCheckpoint(reporterCtx, claim.AttemptRef, "in_progress", summary, nil)
			}
		}
	}()
	return reporter
}

func (r *Runtime) progressInterval() time.Duration {
	if r.config.ProgressInterval <= 0 {
		return defaultProgressInterval
	}
	return r.config.ProgressInterval
}

func (r *Runtime) markProgress(attemptRef string, summary string) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	r.currentAttemptRef = attemptRef
	r.lastActivityAt = time.Now().UTC()
	r.progressSummary = summary
}

func (r *Runtime) clearProgress(attemptRef string) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	if r.currentAttemptRef != attemptRef {
		return
	}
	r.currentAttemptRef = ""
	r.progressSummary = ""
	r.lastActivityAt = time.Now().UTC()
	delete(r.readbackSubmittedByAttempt, attemptRef)
}

func (r *Runtime) markReadbackSubmitted(attemptRef string) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	r.readbackSubmittedByAttempt[attemptRef] = true
}

func (r *Runtime) readbackAlreadySubmitted(attemptRef string) bool {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	return r.readbackSubmittedByAttempt[attemptRef]
}

func (r *Runtime) progressSnapshot() (string, time.Time, string) {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	return r.currentAttemptRef, r.lastActivityAt, r.progressSummary
}

func (p *progressReporter) stop() {
	p.cancel()
	<-p.done
}

func formatOptionalWorkerTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func compactWorkerProgress(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const limit = 240
	if len(value) <= limit {
		return value
	}
	return value[:limit-3] + "..."
}

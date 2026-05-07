package runtime

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type LoopOptions struct {
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	MaxMessages       int
}

type ProcessResult struct {
	Empty          bool
	MessageRef     string
	AttemptRef     string
	TaskRef        string
	CorrelationRef string
	Outcome        string
	Completed      bool
	Blocked        bool
	Failed         bool
	Dropped        bool
	DropReason     string
}

func DefaultLoopOptions() LoopOptions {
	return LoopOptions{
		PollInterval:      time.Second,
		HeartbeatInterval: 10 * time.Second,
	}
}

func (r *Runtime) Run(ctx context.Context, options LoopOptions) error {
	accepted, err := options.Validated()
	if err != nil {
		return err
	}
	if err := r.Register(ctx); err != nil {
		return err
	}
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	heartbeatErrors := make(chan error, 1)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		r.heartbeatLoop(heartbeatCtx, accepted.HeartbeatInterval, heartbeatErrors)
	}()
	err = r.runProcessingLoop(ctx, accepted, heartbeatErrors)
	stopHeartbeat()
	<-heartbeatDone
	r.markOffline()
	return err
}

func (r *Runtime) ProcessNext(ctx context.Context) (ProcessResult, error) {
	next, err := r.client.ClaimNextMessage(ctx, contracts.ClaimNextMessageRequest{
		EndpointRef: r.config.EndpointRef,
	})
	if err != nil {
		return ProcessResult{}, err
	}
	if next.Empty || next.Claim == nil {
		return ProcessResult{Empty: true}, nil
	}
	claim := *next.Claim
	if err := r.acknowledgeClaim(ctx, claim); err != nil {
		return ProcessResult{}, err
	}
	attemptRef, outcome, err := r.finishClaimAfterAck(ctx, claim)
	if err != nil {
		if drop, ok := recoverableAttemptDrop(err); ok {
			return ProcessResult{
				MessageRef:     claim.MessageRef,
				AttemptRef:     claim.AttemptRef,
				TaskRef:        taskRefForClaim(claim),
				CorrelationRef: claim.CorrelationRef,
				Outcome:        ExecutionOutcomeDropped,
				Dropped:        true,
				DropReason:     drop.Code,
			}, nil
		}
		return ProcessResult{}, err
	}
	return ProcessResult{
		MessageRef:     claim.MessageRef,
		AttemptRef:     attemptRef,
		TaskRef:        taskRefForClaim(claim),
		CorrelationRef: claim.CorrelationRef,
		Outcome:        outcome,
		Completed:      outcome == ExecutionOutcomeCompleted || outcome == ExecutionOutcomePartialCompleted,
		Blocked:        outcome == ExecutionOutcomeMissingFields,
		Failed:         outcome == ExecutionOutcomeFailed,
	}, nil
}

func (r *Runtime) MarkOffline(ctx context.Context) error {
	return r.sendHeartbeat(ctx, "offline")
}

func (r *Runtime) runProcessingLoop(ctx context.Context, options LoopOptions, heartbeatErrors <-chan error) error {
	processed := 0
	for {
		interrupted, err := processingLoopInterrupted(ctx, heartbeatErrors)
		if interrupted || err != nil {
			return err
		}

		result, err := r.ProcessNext(ctx)
		if err != nil {
			return err
		}

		if result.Empty {
			if err := waitForNextPoll(ctx, options.PollInterval, heartbeatErrors); err != nil {
				return err
			}
			continue
		}

		processed++
		logProcessedResult(result)
		if result.Dropped {
			logDroppedResult(result)
			continue
		}
		if maxMessagesReached(processed, options.MaxMessages) {
			return nil
		}
	}
}

func processingLoopInterrupted(ctx context.Context, heartbeatErrors <-chan error) (bool, error) {
	select {
	case <-ctx.Done():
		return true, nil
	case err := <-heartbeatErrors:
		return false, err
	default:
		return false, nil
	}
}

func logProcessedResult(result ProcessResult) {
	log.Printf("worker processed message_ref=%s attempt_ref=%s task_ref=%s correlation_ref=%s outcome=%s completed=%t blocked=%t failed=%t",
		result.MessageRef,
		result.AttemptRef,
		result.TaskRef,
		result.CorrelationRef,
		result.Outcome,
		result.Completed,
		result.Blocked,
		result.Failed,
	)
}

func logDroppedResult(result ProcessResult) {
	log.Printf("worker dropped attempt_ref=%s message_ref=%s reason=%s", result.AttemptRef, result.MessageRef, result.DropReason)
}

func maxMessagesReached(processed int, maxMessages int) bool {
	return maxMessages > 0 && processed >= maxMessages
}

func (r *Runtime) heartbeatLoop(ctx context.Context, interval time.Duration, errors chan<- error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Heartbeat(ctx); err != nil {
				select {
				case errors <- err:
				default:
				}
				return
			}
		}
	}
}

func (r *Runtime) markOffline() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = r.MarkOffline(ctx)
}

func (o LoopOptions) Validated() (LoopOptions, error) {
	if o.PollInterval == 0 {
		o.PollInterval = time.Second
	}
	if o.HeartbeatInterval == 0 {
		o.HeartbeatInterval = 10 * time.Second
	}
	if o.PollInterval < 0 || o.HeartbeatInterval < 0 || o.MaxMessages < 0 {
		return LoopOptions{}, errors.New("worker loop options must not be negative")
	}
	return o, nil
}

func waitForNextPoll(ctx context.Context, interval time.Duration, heartbeatErrors <-chan error) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil
	case err := <-heartbeatErrors:
		return err
	case <-timer.C:
		return nil
	}
}

package runtime

import (
	"context"
	"sync"
	"time"
)

const minimumLeaseRefreshInterval = time.Millisecond

type leaseKeeper struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu  sync.Mutex
	err error
}

func (r *Runtime) startLeaseKeeper(ctx context.Context, attemptRef string, leaseExpiresAt string, cancelExecution context.CancelFunc) *leaseKeeper {
	keeperCtx, cancel := context.WithCancel(ctx)
	keeper := &leaseKeeper{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() {
		defer close(keeper.done)
		nextLeaseExpiresAt := leaseExpiresAt
		for {
			interval := leaseRefreshInterval(nextLeaseExpiresAt)
			timer := time.NewTimer(interval)
			select {
			case <-keeperCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			lease, err := r.refreshLease(keeperCtx, attemptRef)
			if err != nil {
				keeper.setError(err)
				cancelExecution()
				return
			}
			nextLeaseExpiresAt = lease.LeaseExpiresAt
		}
	}()
	return keeper
}

func (k *leaseKeeper) stop() error {
	k.cancel()
	<-k.done
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.err
}

func (k *leaseKeeper) setError(err error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.err = err
}

func leaseRefreshInterval(leaseExpiresAt string) time.Duration {
	if leaseExpiresAt == "" {
		return time.Second
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, leaseExpiresAt)
	if err != nil {
		return time.Second
	}
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return minimumLeaseRefreshInterval
	}
	interval := remaining / 2
	if interval < minimumLeaseRefreshInterval {
		return minimumLeaseRefreshInterval
	}
	return interval
}

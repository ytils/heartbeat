package heartbeat

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	// DefaultCheckInterval is the default interval between timeout checks.
	DefaultCheckInterval = time.Second
)

// HookFn is the signature of hook functions.
// timeout is the configured timeout of the Heartbeat.
// idle is the time passed since the last Beat() call.
// left is the time left until the Heartbeat context is cancelled if there will be no Beat() call.
type HookFn func(timeout, idle, left time.Duration)

// Options defines optional parameters of Heartbeat.
type Options struct {
	// CheckInterval is the interval between timeout checks.
	CheckInterval time.Duration
	// CheckHook is called on every timeout check.
	CheckHook HookFn
	// CancelHook is called when the context controlled by Heartbeat is cancelled.
	CancelHook HookFn
}

// Heartbeat holds the context Ctx() that is cancelled after the timeout passes since the last Beat() call.
type Heartbeat struct {
	timeout       time.Duration
	checkInterval time.Duration
	checkHook     HookFn
	cancelHook    HookFn

	ctx       context.Context
	cancelCtx context.CancelFunc

	lastBeat atomic.Pointer[time.Time]
}

// New creates a new Heartbeat instance with the copy of the given context.
func New(ctx context.Context, timeout time.Duration, config *Options) *Heartbeat {
	if timeout <= 0 {
		panic("positive timeout is required")
	}

	hctx, cancel := context.WithCancel(ctx)
	h := &Heartbeat{
		ctx:           hctx,
		cancelCtx:     cancel,
		checkInterval: DefaultCheckInterval,
		timeout:       timeout,
	}

	if config != nil {
		if config.CheckInterval > 0 {
			h.checkInterval = config.CheckInterval
		}
		if config.CheckHook != nil {
			h.checkHook = config.CheckHook
		}
		if config.CancelHook != nil {
			h.cancelHook = config.CancelHook
		}
	}

	h.start()

	return h
}

// Ctx returns the child context controlled by the Heartbeat.
func (h *Heartbeat) Ctx() context.Context {
	return h.ctx
}

// Beat tells the Heartbeat that the operation is still making progress
// and resets the timer towards the timeout.
func (h *Heartbeat) Beat() {
	now := time.Now()
	h.lastBeat.Store(&now)
}

// Close cancels the context controlled by the Heartbeat and stops the timeout checks.
// Close must always be called after the operation, whether it timeouted or not, to avoid leaking goroutines.
func (h *Heartbeat) Close() {
	h.cancelCtx()
}

func (h *Heartbeat) start() {
	h.Beat()

	go func() {
		ticker := time.NewTicker(h.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-h.ctx.Done():
				return
			case <-ticker.C:
				last := h.lastBeat.Load()
				idle := time.Since(*last)
				left := h.timeout - idle

				if left <= 0 {
					h.cancelCtx()
					if h.cancelHook != nil {
						h.cancelHook(h.timeout, idle, left)
					}
					return
				}

				if h.checkHook != nil {
					h.checkHook(h.timeout, idle, left)
				}
			}
		}
	}()
}

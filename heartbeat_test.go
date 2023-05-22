package heartbeat_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
	"ytils.dev/heartbeat"
)

func TestNew(t *testing.T) {
	t.Run("zero timeout", func(t *testing.T) {
		assert.Panics(t, func() {
			heartbeat.New(context.Background(), 0, nil)
		})
	})
	t.Run("negative timeout", func(t *testing.T) {
		assert.Panics(t, func() {
			heartbeat.New(context.Background(), -time.Second, nil)
		})
	})
}

func TestHeartbeat(t *testing.T) {
	t.Parallel()

	t.Run("timeout, context cancelled", func(t *testing.T) {
		t.Parallel()

		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 50 * time.Millisecond,
		})
		defer h.Close()

		time.Sleep(1500 * time.Millisecond)

		select {
		case <-h.Ctx().Done():
		default:
			t.Fatal("context is not cancelled")
		}
	})

	t.Run("timeout, cancel hook", func(t *testing.T) {
		t.Parallel()

		testStart := time.Now()
		cancelHookCalled := false

		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 50 * time.Millisecond,
			CancelHook: func(timeout, idle, left time.Duration) {
				cancelHookCalled = true
				assert.Equal(t, time.Second, timeout)
				assert.Greater(t, idle, timeout)
				assert.LessOrEqual(t, left, time.Duration(0))
				assert.WithinDuration(t, testStart, time.Now(), 1100*time.Millisecond)
			},
		})
		defer h.Close()

		time.Sleep(1500 * time.Millisecond)

		require.True(t, cancelHookCalled)
	})

	t.Run("beat before timeout", func(t *testing.T) {
		t.Parallel()

		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 100 * time.Millisecond,
			CancelHook: func(_, _, _ time.Duration) {
				t.Fatal("cancel hook called")
			},
		})
		defer h.Close()

		for i := 0; i < 20; i++ {
			h.Beat()
			time.Sleep(75 * time.Millisecond)
		}
	})

	t.Run("beat before timeout, check hook", func(t *testing.T) {
		t.Parallel()

		var hookCount atomic.Int64

		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 100 * time.Millisecond,
			CheckHook: func(timeout, idle, left time.Duration) {
				hookCount.Add(1)

				assert.Equal(t, time.Second, timeout)
				assert.Greater(t, idle, time.Duration(0))
				assert.Greater(t, left, time.Duration(0))
				assert.Equal(t, idle+left, timeout)
			},
		})
		defer h.Close()

		// The loop runs for 20 * 75ms = 1500ms
		for i := 0; i < 20; i++ {
			h.Beat()
			time.Sleep(75 * time.Millisecond)
		}

		require.GreaterOrEqual(t, hookCount.Load(), int64(14))
	})
}

func TestHeartbeat_Close(t *testing.T) {
	t.Parallel()

	t.Run("context is cancelled on close", func(t *testing.T) {
		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 100 * time.Millisecond,
		})
		h.Close()

		select {
		case <-h.Ctx().Done():
		default:
			t.Fatal("context is not cancelled")
		}
	})

	t.Run("no checks after close", func(t *testing.T) {
		t.Parallel()

		var hookCount atomic.Int64

		h := heartbeat.New(context.Background(), time.Second, &heartbeat.Options{
			CheckInterval: 50 * time.Millisecond,
			CheckHook: func(_, _, _ time.Duration) {
				hookCount.Add(1)
			},
		})
		h.Close()
		hookCount.Store(0)

		time.Sleep(time.Second)

		require.Equal(t, int64(0), hookCount.Load())
	})
}

package actor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// TimerRef is a handle to a scheduled timer. Use Cancel to stop it.
type TimerRef struct {
	id       uint64
	cancel   context.CancelFunc
	interval bool
}

// Cancel stops the timer. Safe to call multiple times.
func (t *TimerRef) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}

// ID returns the timer's unique identifier.
func (t *TimerRef) ID() uint64 { return t.id }

// timerManager tracks active timers per actor for cleanup on stop.
type timerManager struct {
	mu     sync.Mutex
	seq    atomic.Uint64
	timers map[uint64]*TimerRef
}

func newTimerManager() *timerManager {
	return &timerManager{
		timers: make(map[uint64]*TimerRef),
	}
}

func (tm *timerManager) add(ref *TimerRef) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.timers[ref.id] = ref
}

func (tm *timerManager) remove(id uint64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.timers, id)
}

func (tm *timerManager) cancelAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, ref := range tm.timers {
		ref.Cancel()
	}
	tm.timers = make(map[uint64]*TimerRef)
}

// SendAfter schedules a one-shot message delivery after a delay.
// The message is sent to the target PID via SendPID, so it works
// for both local and remote actors.
// Returns a TimerRef that can be used to cancel the timer.
func (r *Runtime) SendAfter(target PID, payload any, delay time.Duration) *TimerRef {
	id := r.timers.seq.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	ref := &TimerRef{id: id, cancel: cancel, interval: false}
	r.timers.add(ref)

	go func() {
		defer r.timers.remove(id)
		select {
		case <-time.After(delay):
			r.SendPID(context.Background(), target, payload)
		case <-ctx.Done():
			// cancelled
		}
	}()

	return ref
}

// SendInterval schedules recurring message delivery at a fixed interval.
// The first message is sent after one interval has elapsed.
// Returns a TimerRef that can be used to cancel the timer.
func (r *Runtime) SendInterval(target PID, payload any, interval time.Duration) *TimerRef {
	id := r.timers.seq.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	ref := &TimerRef{id: id, cancel: cancel, interval: true}
	r.timers.add(ref)

	go func() {
		defer r.timers.remove(id)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.SendPID(context.Background(), target, payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return ref
}

// SendAfterName schedules a one-shot message delivery after a delay,
// resolved by registered name at delivery time. If the name is not
// registered when the timer fires, the message is silently dropped.
func (r *Runtime) SendAfterName(name string, payload any, delay time.Duration) *TimerRef {
	id := r.timers.seq.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	ref := &TimerRef{id: id, cancel: cancel, interval: false}
	r.timers.add(ref)

	go func() {
		defer r.timers.remove(id)
		select {
		case <-time.After(delay):
			r.SendName(context.Background(), name, payload)
		case <-ctx.Done():
		}
	}()

	return ref
}

// SendIntervalName schedules recurring message delivery at a fixed interval,
// resolved by registered name at each delivery. If the name is not
// registered when a tick fires, that tick is silently dropped.
func (r *Runtime) SendIntervalName(name string, payload any, interval time.Duration) *TimerRef {
	id := r.timers.seq.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	ref := &TimerRef{id: id, cancel: cancel, interval: true}
	r.timers.add(ref)

	go func() {
		defer r.timers.remove(id)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.SendName(context.Background(), name, payload)
			case <-ctx.Done():
				return
			}
		}
	}()

	return ref
}

// CancelTimer cancels a timer by its ref. Convenience alias for ref.Cancel().
func (r *Runtime) CancelTimer(ref *TimerRef) {
	if ref != nil {
		ref.Cancel()
	}
}

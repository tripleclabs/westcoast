package actor

import (
	"context"
	"sync"
	"sync/atomic"
)

// DownMessage is delivered to a monitoring actor when the monitored
// actor stops. This is the Go equivalent of Erlang's {'DOWN', ...} message.
type DownMessage struct {
	// Ref is the monitor reference that was returned by Monitor.
	Ref MonitorRef
	// Target is the PID that was being monitored.
	Target PID
	// Reason describes why the actor stopped ("stopped", "crashed", "node_down").
	Reason string
}

// MonitorRef is a handle to an active monitor. Use Demonitor to cancel.
type MonitorRef struct {
	id uint64
}

// ID returns the monitor's unique identifier.
func (r MonitorRef) ID() uint64 { return r.id }

type monitorEntry struct {
	ref     MonitorRef
	watcher PID // who's watching
	target  PID // who's being watched
}

// monitorManager tracks all active monitors.
type monitorManager struct {
	mu  sync.RWMutex
	seq atomic.Uint64
	// byTarget maps target actorID → list of monitors.
	byTarget map[string][]monitorEntry
	// byRef maps monitorID → entry for fast demonitor.
	byRef map[uint64]monitorEntry
}

func newMonitorManager() *monitorManager {
	return &monitorManager{
		byTarget: make(map[string][]monitorEntry),
		byRef:    make(map[uint64]monitorEntry),
	}
}

func (mm *monitorManager) add(watcher, target PID) MonitorRef {
	id := mm.seq.Add(1)
	ref := MonitorRef{id: id}
	entry := monitorEntry{ref: ref, watcher: watcher, target: target}

	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.byTarget[target.ActorID] = append(mm.byTarget[target.ActorID], entry)
	mm.byRef[id] = entry
	return ref
}

func (mm *monitorManager) remove(ref MonitorRef) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entry, ok := mm.byRef[ref.id]
	if !ok {
		return false
	}
	delete(mm.byRef, ref.id)

	// Remove from byTarget list.
	entries := mm.byTarget[entry.target.ActorID]
	for i, e := range entries {
		if e.ref.id == ref.id {
			mm.byTarget[entry.target.ActorID] = append(entries[:i], entries[i+1:]...)
			break
		}
	}
	if len(mm.byTarget[entry.target.ActorID]) == 0 {
		delete(mm.byTarget, entry.target.ActorID)
	}
	return true
}

// drain removes and returns all monitors for a given target actor.
func (mm *monitorManager) drain(targetActorID string) []monitorEntry {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	entries := mm.byTarget[targetActorID]
	delete(mm.byTarget, targetActorID)
	for _, e := range entries {
		delete(mm.byRef, e.ref.id)
	}
	return entries
}

// Monitor registers watcher to receive a DownMessage when target stops.
// Works for both local and remote targets. For remote targets, the
// DownMessage is sent when the target's node fails (detected via membership).
func (r *Runtime) Monitor(watcher, target PID) MonitorRef {
	return r.monitors.add(watcher, target)
}

// Demonitor cancels an active monitor.
func (r *Runtime) Demonitor(ref MonitorRef) {
	r.monitors.remove(ref)
}

// notifyMonitors sends DownMessages to all watchers of a stopped actor.
// Called by the Runtime when an actor stops for any reason.
func (r *Runtime) notifyMonitors(actorID string, reason string) {
	entries := r.monitors.drain(actorID)
	for _, entry := range entries {
		r.SendPID(context.Background(), entry.watcher, DownMessage{
			Ref:    entry.ref,
			Target: entry.target,
			Reason: reason,
		})
	}
}

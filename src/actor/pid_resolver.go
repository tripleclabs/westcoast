package actor

import (
	"sync"
	"time"
)

type PIDResolverEntry struct {
	PID               PID
	RouteState        PIDRouteState
	CurrentGeneration uint64
	UpdatedAt         time.Time
}

type PIDResolver interface {
	Register(pid PID)
	Resolve(pid PID) (PIDResolverEntry, bool)
	SetState(pid PID, state PIDRouteState)
	BumpGeneration(pid PID) (PID, bool)
}

type InMemoryPIDResolver struct {
	mu    sync.RWMutex
	byKey map[string]PIDResolverEntry
	now   func() time.Time
}

func NewInMemoryPIDResolver() *InMemoryPIDResolver {
	return &InMemoryPIDResolver{byKey: map[string]PIDResolverEntry{}, now: time.Now}
}

func (r *InMemoryPIDResolver) Register(pid PID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey[pid.Key()] = PIDResolverEntry{PID: pid, RouteState: PIDRouteReachable, CurrentGeneration: pid.Generation, UpdatedAt: r.now()}
}

func (r *InMemoryPIDResolver) Resolve(pid PID) (PIDResolverEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.byKey[pid.Key()]
	return entry, ok
}

func (r *InMemoryPIDResolver) SetState(pid PID, state PIDRouteState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.byKey[pid.Key()]
	if !ok {
		entry = PIDResolverEntry{PID: pid, CurrentGeneration: pid.Generation}
	}
	entry.RouteState = state
	entry.UpdatedAt = r.now()
	r.byKey[pid.Key()] = entry
}

func (r *InMemoryPIDResolver) BumpGeneration(pid PID) (PID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.byKey[pid.Key()]
	if !ok {
		return PID{}, false
	}
	entry.CurrentGeneration++
	entry.PID.Generation = entry.CurrentGeneration
	entry.RouteState = PIDRouteReachable
	entry.UpdatedAt = r.now()
	r.byKey[pid.Key()] = entry
	return entry.PID, true
}

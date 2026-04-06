package actor

import (
	"sync"
	"time"
)

// PIDResolverEntry holds the routing state for a registered PID.
type PIDResolverEntry struct {
	PID               PID
	RouteState        PIDRouteState
	CurrentGeneration uint64
	UpdatedAt         time.Time
}

// PIDResolver maps PIDs to their current routing state and generation.
type PIDResolver interface {
	Register(pid PID)
	Resolve(pid PID) (PIDResolverEntry, bool)
	SetState(pid PID, state PIDRouteState)
	BumpGeneration(pid PID) (PID, bool)
	Remove(pid PID)
	SetGatewayMode(mode GatewayRouteMode)
	GatewayMode() GatewayRouteMode
	SetGatewayAvailability(available bool)
	GatewayAvailable() bool
}

// InMemoryPIDResolver is a PIDResolver backed by an in-memory map.
type InMemoryPIDResolver struct {
	mu    sync.RWMutex
	byKey map[string]PIDResolverEntry
	now   func() time.Time
	mode  GatewayRouteMode
	gwUp  bool
}

// NewInMemoryPIDResolver creates a new InMemoryPIDResolver with local-direct gateway mode.
func NewInMemoryPIDResolver() *InMemoryPIDResolver {
	return &InMemoryPIDResolver{
		byKey: map[string]PIDResolverEntry{},
		now:   time.Now,
		mode:  GatewayRouteLocalDirect,
		gwUp:  true,
	}
}

// Register adds a PID to the resolver as reachable.
func (r *InMemoryPIDResolver) Register(pid PID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey[pid.Key()] = PIDResolverEntry{PID: pid, RouteState: PIDRouteReachable, CurrentGeneration: pid.Generation, UpdatedAt: r.now()}
}

// Resolve looks up a PID and returns its routing entry.
func (r *InMemoryPIDResolver) Resolve(pid PID) (PIDResolverEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.byKey[pid.Key()]
	return entry, ok
}

// Remove deletes a PID entry from the resolver.
func (r *InMemoryPIDResolver) Remove(pid PID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byKey, pid.Key())
}

// SetState updates the routing state for a PID.
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

// BumpGeneration increments the PID's generation and resets its state to reachable.
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

// SetGatewayMode sets the gateway routing mode for PID resolution.
func (r *InMemoryPIDResolver) SetGatewayMode(mode GatewayRouteMode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mode = mode
}

// GatewayMode returns the current gateway routing mode.
func (r *InMemoryPIDResolver) GatewayMode() GatewayRouteMode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
}

// SetGatewayAvailability sets whether the gateway is available for mediated routing.
func (r *InMemoryPIDResolver) SetGatewayAvailability(available bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gwUp = available
}

// GatewayAvailable returns whether the gateway is available for mediated routing.
func (r *InMemoryPIDResolver) GatewayAvailable() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gwUp
}

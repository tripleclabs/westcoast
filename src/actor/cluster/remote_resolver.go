package cluster

import (
	"sync"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// RemotePIDResolver wraps a local PIDResolver and adds awareness of
// remote PIDs. It implements actor.PIDResolver so it can be injected
// into a Runtime.
//
// Routing logic: if pid.Namespace == localNodeID, delegate to local
// resolver. Otherwise, return a synthetic entry marking the PID as
// remote-reachable (the actual delivery is handled by the transport layer).
type RemotePIDResolver struct {
	local       actor.PIDResolver
	localNodeID NodeID

	mu          sync.RWMutex
	remoteNodes map[NodeID]bool // set of known remote node IDs
	now         func() time.Time
}

func NewRemotePIDResolver(local actor.PIDResolver, localNodeID NodeID) *RemotePIDResolver {
	return &RemotePIDResolver{
		local:       local,
		localNodeID: localNodeID,
		remoteNodes: make(map[NodeID]bool),
		now:         time.Now,
	}
}

// AddRemoteNode marks a node ID as known-reachable.
func (r *RemotePIDResolver) AddRemoteNode(id NodeID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteNodes[id] = true
}

// RemoveRemoteNode marks a node ID as unreachable.
func (r *RemotePIDResolver) RemoveRemoteNode(id NodeID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.remoteNodes, id)
}

// IsRemoteNode returns whether a node ID is known-reachable.
func (r *RemotePIDResolver) IsRemoteNode(id NodeID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.remoteNodes[id]
}

// --- actor.PIDResolver implementation ---

func (r *RemotePIDResolver) Register(pid actor.PID) {
	r.local.Register(pid)
}

func (r *RemotePIDResolver) Resolve(pid actor.PID) (actor.PIDResolverEntry, bool) {
	// Local PIDs: delegate to local resolver.
	if pid.Namespace == string(r.localNodeID) || pid.Namespace == "" {
		return r.local.Resolve(pid)
	}

	// Ask-reply PIDs: delegate to local resolver (they're always local).
	if isAskReplyLocal(pid.Namespace, string(r.localNodeID)) {
		return r.local.Resolve(pid)
	}

	// Remote PIDs: check if the target node is known.
	targetNode := NodeID(pid.Namespace)
	r.mu.RLock()
	known := r.remoteNodes[targetNode]
	r.mu.RUnlock()

	if !known {
		return actor.PIDResolverEntry{}, false
	}

	// Return a synthetic entry — the PID is reachable via remote transport.
	return actor.PIDResolverEntry{
		PID:               pid,
		RouteState:        actor.PIDRouteReachable,
		CurrentGeneration: pid.Generation,
		UpdatedAt:         r.now(),
	}, true
}

func (r *RemotePIDResolver) Remove(pid actor.PID) {
	r.local.Remove(pid)
}

func (r *RemotePIDResolver) SetState(pid actor.PID, state actor.PIDRouteState) {
	r.local.SetState(pid, state)
}

func (r *RemotePIDResolver) BumpGeneration(pid actor.PID) (actor.PID, bool) {
	return r.local.BumpGeneration(pid)
}

func (r *RemotePIDResolver) SetGatewayMode(mode actor.GatewayRouteMode) {
	r.local.SetGatewayMode(mode)
}

func (r *RemotePIDResolver) GatewayMode() actor.GatewayRouteMode {
	return r.local.GatewayMode()
}

func (r *RemotePIDResolver) SetGatewayAvailability(available bool) {
	r.local.SetGatewayAvailability(available)
}

func (r *RemotePIDResolver) GatewayAvailable() bool {
	return r.local.GatewayAvailable()
}

// isAskReplyLocal returns true if the namespace is an ask-reply for the local node.
func isAskReplyLocal(ns, localNodeID string) bool {
	if ns == "__ask_reply" {
		return true
	}
	if localNodeID != "" && ns == "__ask_reply@"+localNodeID {
		return true
	}
	return false
}

package cluster

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
	"github.com/tripleclabs/westcoast/src/crdt"
)

const serviceGroupPrefix = "svcgroup:"

// ClusterRouter provides distributed service routing. Workers on any
// node join a named service group, and any node can route messages to
// the group using configurable strategies.
//
// Worker membership is stored in the CRDT registry and replicates via
// gossip. Routing decisions are made locally using the local view of
// the worker list — no coordinator node.
type ClusterRouter struct {
	runtime  *actor.Runtime
	registry *CRDTRegistry

	mu      sync.RWMutex
	routers map[string]*clusterRouterState
}

type clusterRouterState struct {
	strategy actor.RouterStrategy
	rrNext   atomic.Uint64
}

func NewClusterRouter(runtime *actor.Runtime, registry *CRDTRegistry) *ClusterRouter {
	return &ClusterRouter{
		runtime:  runtime,
		registry: registry,
		routers:  make(map[string]*clusterRouterState),
	}
}

// Configure sets the routing strategy for a service group.
// Must be called on each node that will route to this group.
func (cr *ClusterRouter) Configure(serviceName string, strategy actor.RouterStrategy) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.routers[serviceName] = &clusterRouterState{strategy: strategy}
}

// Join adds a worker PID to a service group. The worker can be on any
// node — the registration replicates via CRDT gossip.
func (cr *ClusterRouter) Join(serviceName string, pid actor.PID) error {
	key := serviceGroupKey(serviceName, pid)
	return cr.registry.set.Add(key, pid)
}

// Leave removes a worker PID from a service group.
func (cr *ClusterRouter) Leave(serviceName string, pid actor.PID) {
	key := serviceGroupKey(serviceName, pid)
	cr.registry.set.Remove(key)
}

// Members returns all worker PIDs in a service group.
func (cr *ClusterRouter) Members(serviceName string) []actor.PID {
	prefix := serviceGroupPrefix + serviceName + ":"
	entries := cr.registry.set.Filter(func(e crdt.Entry) bool {
		return strings.HasPrefix(e.Key, prefix)
	})
	pids := make([]actor.PID, len(entries))
	for i, e := range entries {
		pids[i] = e.Value.(actor.PID)
	}
	// Sort for stable ordering — round-robin depends on consistent member order.
	sort.Slice(pids, func(i, j int) bool {
		return pids[i].Key() < pids[j].Key()
	})
	return pids
}

// Send routes a message to one worker in the service group using the
// configured strategy. Returns PIDRejectedNotFound if no workers exist.
func (cr *ClusterRouter) Send(ctx context.Context, serviceName string, payload any) actor.PIDSendAck {
	members := cr.Members(serviceName)
	if len(members) == 0 {
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound}
	}

	selected := cr.selectWorker(serviceName, members, payload)
	return cr.runtime.SendPID(ctx, selected, payload)
}

// Ask routes a request to one worker and waits for a reply.
func (cr *ClusterRouter) Ask(ctx context.Context, serviceName string, payload any, timeout time.Duration) (actor.AskResult, error) {
	members := cr.Members(serviceName)
	if len(members) == 0 {
		return actor.AskResult{}, fmt.Errorf("no workers in service group %q", serviceName)
	}

	selected := cr.selectWorker(serviceName, members, payload)
	return cr.runtime.AskPID(ctx, selected, payload, timeout)
}

// Broadcast sends a message to ALL workers in the service group.
func (cr *ClusterRouter) Broadcast(ctx context.Context, serviceName string, payload any) []actor.PIDSendAck {
	members := cr.Members(serviceName)
	acks := make([]actor.PIDSendAck, len(members))
	for i, pid := range members {
		acks[i] = cr.runtime.SendPID(ctx, pid, payload)
	}
	return acks
}

func (cr *ClusterRouter) selectWorker(serviceName string, members []actor.PID, payload any) actor.PID {
	cr.mu.RLock()
	state, ok := cr.routers[serviceName]
	cr.mu.RUnlock()

	strategy := actor.RouterStrategyRoundRobin
	if ok {
		strategy = state.strategy
	}

	switch strategy {
	case actor.RouterStrategyRoundRobin:
		if state == nil {
			// Auto-create state for unconfigured groups.
			cr.mu.Lock()
			state = &clusterRouterState{strategy: strategy}
			cr.routers[serviceName] = state
			cr.mu.Unlock()
		}
		idx := state.rrNext.Add(1) - 1
		return members[idx%uint64(len(members))]

	case actor.RouterStrategyRandom:
		return members[rand.Intn(len(members))]

	case actor.RouterStrategyConsistentKey:
		if hm, ok := payload.(actor.HashKeyMessage); ok {
			h := fnv.New32a()
			h.Write([]byte(hm.HashKey()))
			return members[h.Sum32()%uint32(len(members))]
		}
		// Fallback to round-robin if payload doesn't implement HashKey.
		return members[0]

	default:
		return members[0]
	}
}

func serviceGroupKey(serviceName string, pid actor.PID) string {
	return serviceGroupPrefix + serviceName + ":" + pid.Key()
}

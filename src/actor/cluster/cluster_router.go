package cluster

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

const serviceGroupPrefix = "svcgroup:"

// ClusterRouter provides distributed service routing. Workers on any
// node join a named service group, and any node can route messages to
// the group using configurable strategies.
//
// Supports static filtering (Configure-time) and dynamic call-time
// preferences (SendWith/AskWith) for locality-aware routing.
type ClusterRouter struct {
	runtime  *actor.Runtime
	registry *DistributedRegistry // service group membership (separate from name registry)
	cluster  *Cluster             // optional, needed for metadata-based routing

	mu      sync.RWMutex
	routers map[string]*clusterRouterState
}

type clusterRouterState struct {
	strategy   actor.RouterStrategy
	rrNext     atomic.Uint64
	filter     NodeMatcher // static filter set at Configure time
	preference NodeRanker  // static preference set at Configure time
}

// NewClusterRouter creates a cluster router. The cluster parameter is
// optional — only needed for metadata-based filtering and preferences.
func NewClusterRouter(runtime *actor.Runtime, registry *DistributedRegistry, cluster ...*Cluster) *ClusterRouter {
	cr := &ClusterRouter{
		runtime:  runtime,
		registry: registry,
		routers:  make(map[string]*clusterRouterState),
	}
	if len(cluster) > 0 {
		cr.cluster = cluster[0]
	}
	return cr
}

// RouterOption configures a service group's routing behavior.
type RouterOption func(*clusterRouterState)

// WithWorkerFilter restricts routing to workers on nodes matching the predicate.
func WithWorkerFilter(m NodeMatcher) RouterOption {
	return func(s *clusterRouterState) { s.filter = m }
}

// WithWorkerPreference ranks eligible workers by node metadata.
// Higher-ranked workers are preferred by the routing strategy.
func WithWorkerPreference(r NodeRanker) RouterOption {
	return func(s *clusterRouterState) { s.preference = r }
}

// Configure sets the routing strategy and options for a service group.
func (cr *ClusterRouter) Configure(serviceName string, strategy actor.RouterStrategy, opts ...RouterOption) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	state := &clusterRouterState{strategy: strategy}
	for _, opt := range opts {
		opt(state)
	}
	cr.routers[serviceName] = state
}

// Join adds a worker PID to a service group.
func (cr *ClusterRouter) Join(serviceName string, pid actor.PID) error {
	key := serviceGroupKey(serviceName, pid)
	return cr.registry.Register(key, pid)
}

// Leave removes a worker PID from a service group.
func (cr *ClusterRouter) Leave(serviceName string, pid actor.PID) {
	key := serviceGroupKey(serviceName, pid)
	cr.registry.Unregister(key)
}

// Members returns all worker PIDs in a service group, after applying
// the static filter if one is configured.
func (cr *ClusterRouter) Members(serviceName string) []actor.PID {
	prefix := serviceGroupPrefix + serviceName + ":"
	var pids []actor.PID
	cr.registry.Range(func(name string, pid actor.PID) bool {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			pids = append(pids, pid)
		}
		return true
	})

	// Apply static filter if configured.
	cr.mu.RLock()
	state := cr.routers[serviceName]
	cr.mu.RUnlock()

	if state != nil && state.filter != nil && cr.cluster != nil {
		pids = cr.filterByNodeMeta(pids, state.filter)
	}

	sort.Slice(pids, func(i, j int) bool {
		return pids[i].Key() < pids[j].Key()
	})
	return pids
}

// Send routes a message to one worker using the configured strategy.
func (cr *ClusterRouter) Send(ctx context.Context, serviceName string, payload any) actor.PIDSendAck {
	return cr.sendInternal(ctx, serviceName, payload, nil)
}

// SendWith routes a message with call-time routing preferences.
func (cr *ClusterRouter) SendWith(ctx context.Context, serviceName string, payload any, opts ...RoutePreference) actor.PIDSendAck {
	return cr.sendInternal(ctx, serviceName, payload, opts)
}

// Ask routes a request to one worker and waits for a reply.
func (cr *ClusterRouter) Ask(ctx context.Context, serviceName string, payload any, timeout time.Duration) (actor.AskResult, error) {
	return cr.askInternal(ctx, serviceName, payload, timeout, nil)
}

// AskWith routes a request with call-time routing preferences.
func (cr *ClusterRouter) AskWith(ctx context.Context, serviceName string, payload any, timeout time.Duration, opts ...RoutePreference) (actor.AskResult, error) {
	return cr.askInternal(ctx, serviceName, payload, timeout, opts)
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

func (cr *ClusterRouter) sendInternal(ctx context.Context, serviceName string, payload any, prefs []RoutePreference) actor.PIDSendAck {
	members := cr.Members(serviceName)
	if len(members) == 0 {
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound}
	}

	members = cr.applyPreferences(serviceName, members, prefs)
	selected := cr.selectWorker(serviceName, members, payload)
	return cr.runtime.SendPID(ctx, selected, payload)
}

func (cr *ClusterRouter) askInternal(ctx context.Context, serviceName string, payload any, timeout time.Duration, prefs []RoutePreference) (actor.AskResult, error) {
	members := cr.Members(serviceName)
	if len(members) == 0 {
		return actor.AskResult{}, fmt.Errorf("no workers in service group %q", serviceName)
	}

	members = cr.applyPreferences(serviceName, members, prefs)
	selected := cr.selectWorker(serviceName, members, payload)
	return cr.runtime.AskPID(ctx, selected, payload, timeout)
}

// applyPreferences sorts workers by combined preference score (static +
// call-time). Higher scores come first. The strategy then picks from
// the front of the sorted list.
func (cr *ClusterRouter) applyPreferences(serviceName string, pids []actor.PID, callPrefs []RoutePreference) []actor.PID {
	cr.mu.RLock()
	state := cr.routers[serviceName]
	cr.mu.RUnlock()

	hasStaticPref := state != nil && state.preference != nil
	hasCallPrefs := len(callPrefs) > 0

	if (!hasStaticPref && !hasCallPrefs) || cr.cluster == nil {
		return pids
	}

	// Build node metadata lookup.
	nodeMeta := cr.nodeMetaMap()

	type scored struct {
		pid   actor.PID
		score int
	}
	scored_pids := make([]scored, len(pids))
	for i, pid := range pids {
		meta := nodeMeta[NodeID(pid.Namespace)]
		score := 0
		if hasStaticPref {
			score += state.preference(meta)
		}
		for _, pref := range callPrefs {
			score += pref.Rank(meta)
		}
		scored_pids[i] = scored{pid: pid, score: score}
	}

	sort.SliceStable(scored_pids, func(i, j int) bool {
		return scored_pids[i].score > scored_pids[j].score
	})

	out := make([]actor.PID, len(scored_pids))
	for i, s := range scored_pids {
		out[i] = s.pid
	}
	return out
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
		return members[0]

	default:
		return members[0]
	}
}

func (cr *ClusterRouter) filterByNodeMeta(pids []actor.PID, matcher NodeMatcher) []actor.PID {
	nodeMeta := cr.nodeMetaMap()
	var out []actor.PID
	for _, pid := range pids {
		meta, ok := nodeMeta[NodeID(pid.Namespace)]
		if !ok {
			continue // unknown node, skip
		}
		if matcher(meta) {
			out = append(out, pid)
		}
	}
	return out
}

func (cr *ClusterRouter) nodeMetaMap() map[NodeID]NodeMeta {
	m := make(map[NodeID]NodeMeta)
	if cr.cluster != nil {
		self := cr.cluster.Self()
		m[self.ID] = self
		for _, member := range cr.cluster.Members() {
			m[member.ID] = member
		}
	}
	return m
}

func serviceGroupKey(serviceName string, pid actor.PID) string {
	return serviceGroupPrefix + serviceName + ":" + pid.Key()
}

// RoutePreference is a call-time routing preference. It ranks nodes
// to influence worker selection. Higher scores are preferred.
type RoutePreference struct {
	Rank NodeRanker
}

// Nearest returns a RoutePreference that scores workers by how many
// of the given tags match. More matches = higher score. This creates
// hierarchical locality — exact AZ match scores higher than region-only.
//
// Example:
//
//	Nearest(map[string]string{"az": "eu-west-1a", "region": "eu-west-1", "continent": "eu"})
//
// A worker matching all 3 tags scores 3, matching region+continent scores 2, etc.
func Nearest(tags map[string]string) RoutePreference {
	return RoutePreference{
		Rank: func(meta NodeMeta) int {
			score := 0
			for k, v := range tags {
				if meta.Tags[k] == v {
					score++
				}
			}
			return score
		},
	}
}

// PreferTag returns a RoutePreference that ranks workers by a numeric tag.
func PreferTag(key string, direction RankDirection) RoutePreference {
	return RoutePreference{Rank: RankByTag(key, direction)}
}

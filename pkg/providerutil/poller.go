// Package providerutil provides shared infrastructure for building
// ClusterProvider implementations that discover nodes by polling an API.
package providerutil

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// DiscoverFunc polls an external source and returns the current set of
// cluster members. Implementations typically call a cloud API or service
// registry. The returned slice should not include the local node.
type DiscoverFunc func(ctx context.Context) ([]cluster.NodeMeta, error)

// PollerConfig controls the polling behaviour.
type PollerConfig struct {
	// Interval between discovery polls. Default: 10s.
	Interval time.Duration

	// FailureThreshold is the number of consecutive polls in which a
	// previously-seen node must be absent before it is declared failed.
	// Default: 3.
	FailureThreshold int

	// EventBufferSize is the capacity of the MemberEvent channel.
	// Default: 1024.
	EventBufferSize int
}

func (c *PollerConfig) setDefaults() {
	if c.Interval <= 0 {
		c.Interval = 10 * time.Second
	}
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 3
	}
	if c.EventBufferSize <= 0 {
		c.EventBufferSize = 1024
	}
}

// peerState tracks a discovered node across polling cycles.
type peerState struct {
	meta             cluster.NodeMeta
	alive            bool
	consecutiveMisses int
}

// Poller implements the core polling, diffing, and event emission logic
// shared by all poll-based ClusterProvider implementations.
//
// Concrete providers construct a DiscoverFunc and wrap a Poller,
// delegating Start/Stop/Members/Events to it.
type Poller struct {
	cfg      PollerConfig
	discover DiscoverFunc
	self     cluster.NodeMeta
	eventCh  chan cluster.MemberEvent

	mu      sync.RWMutex
	members map[cluster.NodeID]*peerState
	started bool
	cancel  context.CancelFunc

	droppedEvents atomic.Uint64
}

// NewPoller creates a Poller that calls discover on each tick.
func NewPoller(discover DiscoverFunc, cfg PollerConfig) *Poller {
	cfg.setDefaults()
	return &Poller{
		cfg:      cfg,
		discover: discover,
		members:  make(map[cluster.NodeID]*peerState),
		eventCh:  make(chan cluster.MemberEvent, cfg.EventBufferSize),
	}
}

// Start begins the polling loop. Implements cluster.ClusterProvider.Start.
func (p *Poller) Start(self cluster.NodeMeta) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return cluster.ErrProviderAlreadyStarted
	}
	p.self = self
	p.started = true

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.pollLoop(ctx)
	return nil
}

// Stop halts polling and closes the event channel.
// Implements cluster.ClusterProvider.Stop.
func (p *Poller) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return cluster.ErrProviderNotStarted
	}
	p.started = false
	p.cancel()
	close(p.eventCh)
	return nil
}

// Members returns the current set of alive peers.
// Implements cluster.ClusterProvider.Members.
func (p *Poller) Members() []cluster.NodeMeta {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]cluster.NodeMeta, 0, len(p.members))
	for _, ps := range p.members {
		if ps.alive {
			out = append(out, ps.meta)
		}
	}
	return out
}

// Events returns the membership event channel.
// Implements cluster.ClusterProvider.Events.
func (p *Poller) Events() <-chan cluster.MemberEvent {
	return p.eventCh
}

// DroppedEvents returns the number of events dropped because the event
// channel was full.
func (p *Poller) DroppedEvents() uint64 {
	return p.droppedEvents.Load()
}

func (p *Poller) pollLoop(ctx context.Context) {
	// Immediate first poll.
	p.poll(ctx)

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	discovered, err := p.discover(ctx)
	if err != nil {
		// Discovery error — skip this cycle, don't penalize existing members.
		return
	}
	p.diffMembers(discovered)
}

// diffMembers compares the discovered set against the known set and emits
// appropriate membership events.
func (p *Poller) diffMembers(current []cluster.NodeMeta) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Build a set of currently discovered IDs.
	seen := make(map[cluster.NodeID]cluster.NodeMeta, len(current))
	for _, m := range current {
		if m.ID == p.self.ID {
			continue // exclude self
		}
		seen[m.ID] = m
	}

	// Check for new or updated members.
	for id, meta := range seen {
		existing, known := p.members[id]
		if !known {
			// Brand new node.
			p.members[id] = &peerState{meta: meta, alive: true}
			p.emit(cluster.MemberEvent{Type: cluster.MemberJoin, Member: meta})
			continue
		}

		if !existing.alive {
			// Was failed, now back.
			existing.meta = meta
			existing.alive = true
			existing.consecutiveMisses = 0
			p.emit(cluster.MemberEvent{Type: cluster.MemberJoin, Member: meta})
			continue
		}

		// Already alive — reset miss counter, check for tag changes.
		existing.consecutiveMisses = 0
		if !tagsEqual(existing.meta.Tags, meta.Tags) {
			existing.meta = meta
			p.emit(cluster.MemberEvent{Type: cluster.MemberUpdated, Member: meta})
		}
	}

	// Check for missing members (present before, absent now).
	for id, ps := range p.members {
		if _, stillPresent := seen[id]; stillPresent {
			continue
		}
		if !ps.alive {
			continue // already declared failed
		}

		ps.consecutiveMisses++
		if ps.consecutiveMisses >= p.cfg.FailureThreshold {
			ps.alive = false
			p.emit(cluster.MemberEvent{Type: cluster.MemberFailed, Member: ps.meta})
		}
	}
}

func (p *Poller) emit(ev cluster.MemberEvent) {
	select {
	case p.eventCh <- ev:
	default:
		p.droppedEvents.Add(1)
	}
}

func tagsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

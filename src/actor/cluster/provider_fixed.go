package cluster

import (
	"context"
	"sync"
	"time"
)

// FixedProviderConfig configures the static seed-list cluster provider.
type FixedProviderConfig struct {
	// Seeds is the list of known peers. Each entry must have ID and Addr set.
	Seeds []NodeMeta
	// HeartbeatInterval controls how often peers are probed.
	// Defaults to 5s if zero.
	HeartbeatInterval time.Duration
	// FailureThreshold is the number of consecutive missed heartbeats
	// before a node is declared failed. Defaults to 3 if zero.
	FailureThreshold int
}

// FixedProvider discovers cluster members from a static seed list.
// It uses periodic heartbeats to detect node failure.
type FixedProvider struct {
	cfg     FixedProviderConfig
	self    NodeMeta
	eventCh chan MemberEvent

	mu      sync.RWMutex
	members map[NodeID]*fixedPeerState
	started bool
	cancel  context.CancelFunc

	// Probe checks whether a peer at the given address is alive.
	// Return nil for alive, error for unreachable. Injected for testing.
	// If nil, peers are assumed alive on first contact via AddMember.
	Probe func(ctx context.Context, addr string) error
}

type fixedPeerState struct {
	meta             NodeMeta
	alive            bool
	consecutiveFails int
	lastSeen         time.Time
}

func NewFixedProvider(cfg FixedProviderConfig) *FixedProvider {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 5 * time.Second
	}
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 3
	}
	return &FixedProvider{
		cfg:     cfg,
		eventCh: make(chan MemberEvent, 64),
		members: make(map[NodeID]*fixedPeerState),
	}
}

func (p *FixedProvider) Start(self NodeMeta) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return ErrProviderAlreadyStarted
	}
	p.self = self
	p.started = true
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.mu.Unlock()

	go p.heartbeatLoop(ctx)
	return nil
}

func (p *FixedProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return ErrProviderNotStarted
	}
	p.cancel()
	p.started = false
	close(p.eventCh)
	return nil
}

func (p *FixedProvider) Members() []NodeMeta {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []NodeMeta
	for _, ps := range p.members {
		if ps.alive {
			out = append(out, ps.meta)
		}
	}
	return out
}

func (p *FixedProvider) Events() <-chan MemberEvent {
	return p.eventCh
}

func (p *FixedProvider) heartbeatLoop(ctx context.Context) {
	p.probeAll(ctx)

	ticker := time.NewTicker(p.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probeAll(ctx)
		}
	}
}

func (p *FixedProvider) probeAll(ctx context.Context) {
	for _, seed := range p.cfg.Seeds {
		if ctx.Err() != nil {
			return
		}
		if seed.ID == p.self.ID {
			continue
		}
		p.probeSeed(ctx, seed)
	}
}

func (p *FixedProvider) probeSeed(ctx context.Context, seed NodeMeta) {
	if p.Probe == nil {
		// No probe function — assume alive on first contact.
		p.handleProbeSuccess(seed)
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.cfg.HeartbeatInterval/2)
	defer cancel()

	if err := p.Probe(probeCtx, seed.Addr); err != nil {
		p.handleProbeFailure(seed.ID)
		return
	}

	p.handleProbeSuccess(seed)
}

func (p *FixedProvider) handleProbeSuccess(meta NodeMeta) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ps, exists := p.members[meta.ID]
	if !exists {
		p.members[meta.ID] = &fixedPeerState{
			meta:     meta,
			alive:    true,
			lastSeen: time.Now(),
		}
		p.emit(MemberEvent{Type: MemberJoin, Member: meta})
		return
	}

	ps.consecutiveFails = 0
	ps.lastSeen = time.Now()

	if !ps.alive {
		ps.alive = true
		ps.meta = meta
		p.emit(MemberEvent{Type: MemberJoin, Member: meta})
	}
}

func (p *FixedProvider) handleProbeFailure(id NodeID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ps, ok := p.members[id]
	if !ok || !ps.alive {
		return
	}
	ps.consecutiveFails++
	if ps.consecutiveFails >= p.cfg.FailureThreshold {
		ps.alive = false
		p.emit(MemberEvent{Type: MemberFailed, Member: ps.meta})
	}
}

func (p *FixedProvider) emit(ev MemberEvent) {
	select {
	case p.eventCh <- ev:
	default:
	}
}

// AddMember manually registers a node. Useful for testing.
func (p *FixedProvider) AddMember(meta NodeMeta) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.members[meta.ID]; exists {
		return
	}
	p.members[meta.ID] = &fixedPeerState{
		meta:     meta,
		alive:    true,
		lastSeen: time.Now(),
	}
	p.emit(MemberEvent{Type: MemberJoin, Member: meta})
}

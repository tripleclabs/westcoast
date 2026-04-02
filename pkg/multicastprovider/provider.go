// Package multicastprovider implements a ClusterProvider that uses UDP
// multicast for LAN node discovery. Nodes periodically announce themselves
// on a multicast group; peers that hear the announcements maintain a live
// member set and emit membership events.
package multicastprovider

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// Compile-time interface check.
var _ cluster.ClusterProvider = (*MulticastProvider)(nil)

// Config controls the multicast discovery behaviour.
type Config struct {
	// GroupAddr is the multicast group address and port.
	// Default: "239.1.1.1:7946".
	GroupAddr string

	// Interface is the network interface name to bind to.
	// Default: "" (all interfaces).
	Interface string

	// BroadcastInterval is how often this node announces itself.
	// Default: 1s.
	BroadcastInterval time.Duration

	// DeadTimeout is how long a node can be silent before being declared
	// failed. Default: 5s.
	DeadTimeout time.Duration

	// Port is the transport port to advertise. Default: 9000.
	Port int
}

func (c *Config) setDefaults() {
	if c.GroupAddr == "" {
		c.GroupAddr = "239.1.1.1:7946"
	}
	if c.BroadcastInterval <= 0 {
		c.BroadcastInterval = 1 * time.Second
	}
	if c.DeadTimeout <= 0 {
		c.DeadTimeout = 5 * time.Second
	}
	if c.Port <= 0 {
		c.Port = 9000
	}
}

// announcement is the gob-encoded packet sent over multicast.
type announcement struct {
	NodeID  string
	Addr    string
	Tags    map[string]string
	Leaving bool
}

// peerState tracks a discovered node.
type peerState struct {
	meta     cluster.NodeMeta
	lastSeen time.Time
}

// MulticastProvider discovers cluster peers via UDP multicast.
type MulticastProvider struct {
	cfg Config

	mu      sync.RWMutex
	self    cluster.NodeMeta
	members map[cluster.NodeID]*peerState
	eventCh chan cluster.MemberEvent
	started bool
	stopCh  chan struct{}

	conn *net.UDPConn
	pc   *ipv4.PacketConn
}

// New creates a MulticastProvider with default configuration.
func New() *MulticastProvider {
	return NewWithConfig(Config{})
}

// NewWithConfig creates a MulticastProvider with the given configuration.
func NewWithConfig(cfg Config) *MulticastProvider {
	cfg.setDefaults()
	return &MulticastProvider{
		cfg:     cfg,
		members: make(map[cluster.NodeID]*peerState),
		eventCh: make(chan cluster.MemberEvent, 1024),
	}
}

// Start begins multicast discovery. Implements cluster.ClusterProvider.
func (p *MulticastProvider) Start(self cluster.NodeMeta) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return cluster.ErrProviderAlreadyStarted
	}

	p.self = self
	p.stopCh = make(chan struct{})

	groupAddr, err := net.ResolveUDPAddr("udp4", p.cfg.GroupAddr)
	if err != nil {
		return fmt.Errorf("multicastprovider: resolve group addr: %w", err)
	}

	var iface *net.Interface
	if p.cfg.Interface != "" {
		iface, err = net.InterfaceByName(p.cfg.Interface)
		if err != nil {
			return fmt.Errorf("multicastprovider: interface %q: %w", p.cfg.Interface, err)
		}
	}

	// ListenMulticastUDP sets SO_REUSEADDR so multiple processes on
	// the same host can bind the same multicast port.
	conn, err := net.ListenMulticastUDP("udp4", iface, groupAddr)
	if err != nil {
		return fmt.Errorf("multicastprovider: listen multicast udp: %w", err)
	}
	p.conn = conn

	pc := ipv4.NewPacketConn(conn)
	p.pc = pc

	// Allow loopback so we can test on a single machine (self-exclusion
	// filters our own packets at the application level).
	if err := pc.SetMulticastLoopback(true); err != nil {
		conn.Close()
		return fmt.Errorf("multicastprovider: set loopback: %w", err)
	}

	p.started = true

	go p.listenLoop()
	go p.announceLoop(groupAddr)
	go p.sweepLoop()

	return nil
}

// Stop gracefully leaves the cluster and shuts down. Implements cluster.ClusterProvider.
func (p *MulticastProvider) Stop() error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return cluster.ErrProviderNotStarted
	}
	p.started = false
	p.mu.Unlock()

	// Send a leave announcement before shutting down.
	groupAddr, err := net.ResolveUDPAddr("udp4", p.cfg.GroupAddr)
	if err == nil {
		p.sendAnnouncement(groupAddr, true)
	}

	close(p.stopCh)
	p.conn.Close()

	// Close the event channel so consumers can detect shutdown.
	close(p.eventCh)

	return nil
}

// Members returns the current known alive peers (excluding self).
// Implements cluster.ClusterProvider.
func (p *MulticastProvider) Members() []cluster.NodeMeta {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]cluster.NodeMeta, 0, len(p.members))
	for _, ps := range p.members {
		out = append(out, ps.meta)
	}
	return out
}

// Events returns the membership event channel.
// Implements cluster.ClusterProvider.
func (p *MulticastProvider) Events() <-chan cluster.MemberEvent {
	return p.eventCh
}

// UpdateTags updates the tags that will be announced in subsequent
// broadcast packets. This allows peers to detect tag changes and
// emit MemberUpdated events.
func (p *MulticastProvider) UpdateTags(tags map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.self.Tags = tags
}

// --- internal loops ---

func (p *MulticastProvider) announceLoop(groupAddr *net.UDPAddr) {
	// Send an immediate announcement so peers discover us quickly.
	p.sendAnnouncement(groupAddr, false)

	ticker := time.NewTicker(p.cfg.BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.sendAnnouncement(groupAddr, false)
		}
	}
}

func (p *MulticastProvider) sendAnnouncement(groupAddr *net.UDPAddr, leaving bool) {
	p.mu.RLock()
	ann := announcement{
		NodeID:  string(p.self.ID),
		Addr:    p.self.Addr,
		Tags:    p.self.Tags,
		Leaving: leaving,
	}
	p.mu.RUnlock()

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ann); err != nil {
		return
	}

	p.conn.WriteToUDP(buf.Bytes(), groupAddr)
}

func (p *MulticastProvider) listenLoop() {
	buf := make([]byte, 65536)
	for {
		n, _, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			// Check if we're shutting down.
			select {
			case <-p.stopCh:
				return
			default:
			}
			continue
		}

		var ann announcement
		if err := gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(&ann); err != nil {
			continue
		}

		p.handleAnnouncement(ann)
	}
}

func (p *MulticastProvider) handleAnnouncement(ann announcement) {
	nodeID := cluster.NodeID(ann.NodeID)

	// Self-exclusion.
	p.mu.RLock()
	selfID := p.self.ID
	p.mu.RUnlock()

	if nodeID == selfID {
		return
	}

	meta := cluster.NodeMeta{
		ID:   nodeID,
		Addr: ann.Addr,
		Tags: ann.Tags,
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if ann.Leaving {
		if _, known := p.members[nodeID]; known {
			delete(p.members, nodeID)
			p.emit(cluster.MemberEvent{Type: cluster.MemberLeave, Member: meta})
		}
		return
	}

	existing, known := p.members[nodeID]
	if !known {
		meta.JoinedAt = time.Now()
		p.members[nodeID] = &peerState{meta: meta, lastSeen: time.Now()}
		p.emit(cluster.MemberEvent{Type: cluster.MemberJoin, Member: meta})
		return
	}

	// Update last seen.
	existing.lastSeen = time.Now()

	// Check for tag changes.
	if !tagsEqual(existing.meta.Tags, ann.Tags) {
		existing.meta.Tags = ann.Tags
		existing.meta.Addr = ann.Addr
		p.emit(cluster.MemberEvent{Type: cluster.MemberUpdated, Member: existing.meta})
	}
}

func (p *MulticastProvider) sweepLoop() {
	ticker := time.NewTicker(p.cfg.BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.sweep()
		}
	}
}

func (p *MulticastProvider) sweep() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for id, ps := range p.members {
		if now.Sub(ps.lastSeen) > p.cfg.DeadTimeout {
			delete(p.members, id)
			p.emit(cluster.MemberEvent{Type: cluster.MemberFailed, Member: ps.meta})
		}
	}
}

func (p *MulticastProvider) emit(ev cluster.MemberEvent) {
	// Guard against send-on-closed-channel: the listen loop may still
	// be processing a packet when Stop() closes the channel.
	select {
	case <-p.stopCh:
		return
	default:
	}
	select {
	case p.eventCh <- ev:
	case <-p.stopCh:
	default:
		// Drop event if channel is full.
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

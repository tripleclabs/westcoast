package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClusterConfig configures a cluster node.
type ClusterConfig struct {
	Self      NodeMeta
	Provider  ClusterProvider
	Transport Transport
	Auth      ClusterAuth
	Codec     Codec

	// Topology controls which nodes to connect to and how messages are
	// routed. Defaults to FullMeshTopology if nil.
	Topology Topology

	// OnEnvelope is called when an envelope arrives from a remote node.
	// Set by the Runtime integration layer (Phase 2).
	OnEnvelope func(from NodeID, env Envelope)

	// OnMemberEvent is called when cluster membership changes.
	// Set by the Runtime integration layer (Phase 6) to publish
	// membership events on the cluster.membership pubsub topic.
	OnMemberEvent func(event MemberEvent)
}

// Cluster manages the lifecycle of a single node within a cluster.
// It coordinates membership discovery, transport connections, and
// envelope routing between nodes.
type Cluster struct {
	cfg ClusterConfig

	mu      sync.RWMutex
	conns   map[NodeID]Connection // active connections by node
	peers   map[NodeID]NodeMeta   // known live peers
	dialing map[NodeID]bool       // dial attempts in progress

	ctx    context.Context
	cancel context.CancelFunc

	started bool
}

// NewCluster creates a new cluster node with the given configuration.
func NewCluster(cfg ClusterConfig) (*Cluster, error) {
	if cfg.Self.ID == "" {
		return nil, fmt.Errorf("cluster: Self.ID is required")
	}
	if cfg.Self.Addr == "" {
		return nil, fmt.Errorf("cluster: Self.Addr is required")
	}
	if cfg.Provider == nil {
		return nil, fmt.Errorf("cluster: Provider is required")
	}
	if cfg.Transport == nil {
		return nil, fmt.Errorf("cluster: Transport is required")
	}
	if cfg.Auth == nil {
		cfg.Auth = NoopAuth{}
	}
	if cfg.Codec == nil {
		cfg.Codec = NewGobCodec()
	}
	if cfg.Topology == nil {
		cfg.Topology = FullMeshTopology{}
	}
	if cfg.Self.Tags == nil {
		cfg.Self.Tags = make(map[string]string)
	}
	return &Cluster{
		cfg:     cfg,
		conns:   make(map[NodeID]Connection),
		peers:   make(map[NodeID]NodeMeta),
		dialing: make(map[NodeID]bool),
	}, nil
}

// LocalNodeID returns this node's identity.
func (c *Cluster) LocalNodeID() NodeID { return c.cfg.Self.ID }

// Config returns the cluster configuration.
func (c *Cluster) Config() ClusterConfig { return c.cfg }

// Start begins cluster operations: starts listening for inbound connections,
// starts the membership provider, and begins connecting to discovered peers.
func (c *Cluster) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("cluster: already started")
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.started = true
	c.mu.Unlock()

	// Start listening for inbound connections.
	if err := c.cfg.Transport.Listen(c.cfg.Self.Addr, c); err != nil {
		return fmt.Errorf("cluster: transport listen: %w", err)
	}

	// Start the membership provider.
	if err := c.cfg.Provider.Start(c.cfg.Self); err != nil {
		return fmt.Errorf("cluster: provider start: %w", err)
	}

	// Process membership events in the background.
	go c.membershipLoop()

	return nil
}

// Stop gracefully shuts down the cluster node.
func (c *Cluster) Stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = false
	c.cancel()
	c.mu.Unlock()

	// Close all outbound connections.
	c.mu.RLock()
	conns := make([]Connection, 0, len(c.conns))
	for _, conn := range c.conns {
		conns = append(conns, conn)
	}
	c.mu.RUnlock()

	for _, conn := range conns {
		conn.Close()
	}

	c.cfg.Provider.Stop()
	c.cfg.Transport.Close()
	return nil
}

// Members returns all known live peers (excluding self).
func (c *Cluster) Members() []NodeMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]NodeMeta, 0, len(c.peers))
	for _, m := range c.peers {
		out = append(out, m)
	}
	return out
}

// SendRemote sends an envelope to a specific remote node.
// If a direct connection exists, uses it. Otherwise, uses the topology
// to find the next hop and forwards through an intermediate node.
func (c *Cluster) SendRemote(ctx context.Context, target NodeID, env Envelope) error {
	if env.SentAtUnixNano == 0 {
		env.SentAtUnixNano = time.Now().UnixNano()
	}

	// Try direct connection first.
	c.mu.RLock()
	conn, directOK := c.conns[target]
	c.mu.RUnlock()

	if directOK {
		if err := conn.Send(ctx, env); err != nil {
			c.removeConnection(target)
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		return nil
	}

	// No direct connection — use topology to find next hop.
	c.mu.RLock()
	members := c.memberListLocked()
	c.mu.RUnlock()

	nextHop, ok := c.cfg.Topology.Route(c.cfg.Self.ID, target, members)
	if !ok {
		return fmt.Errorf("%w: no route to %s", ErrNodeUnreachable, target)
	}

	// If the next hop IS the target, try to connect on demand.
	if nextHop == target {
		conn, err := c.ensureConnection(ctx, target)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrNodeUnreachable, err)
		}
		if err := conn.Send(ctx, env); err != nil {
			c.removeConnection(target)
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		return nil
	}

	// Forward through intermediate node. The envelope's TargetNode stays
	// the same — the intermediate node will forward it onward.
	c.mu.RLock()
	hopConn, hopOK := c.conns[nextHop]
	c.mu.RUnlock()

	if !hopOK {
		hopConn2, err := c.ensureConnection(ctx, nextHop)
		if err != nil {
			return fmt.Errorf("%w: no connection to next hop %s", ErrNodeUnreachable, nextHop)
		}
		hopConn = hopConn2
	}

	if err := hopConn.Send(ctx, env); err != nil {
		c.removeConnection(nextHop)
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	return nil
}

func (c *Cluster) memberListLocked() []NodeMeta {
	out := make([]NodeMeta, 0, len(c.peers)+1)
	out = append(out, c.cfg.Self)
	for _, m := range c.peers {
		out = append(out, m)
	}
	return out
}

// SetOnEnvelope sets the callback for inbound envelopes.
func (c *Cluster) SetOnEnvelope(fn func(from NodeID, env Envelope)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.OnEnvelope = fn
}

// SetOnMemberEvent sets the callback for membership changes.
func (c *Cluster) SetOnMemberEvent(fn func(event MemberEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg.OnMemberEvent = fn
}

// Self returns this node's current metadata, including dynamic tags.
func (c *Cluster) Self() NodeMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	meta := c.cfg.Self
	meta.Tags = make(map[string]string, len(c.cfg.Self.Tags))
	for k, v := range c.cfg.Self.Tags {
		meta.Tags[k] = v
	}
	return meta
}

// UpdateTags merges the given tags into this node's metadata.
// Changed tags are gossiped to peers automatically.
func (c *Cluster) UpdateTags(tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range tags {
		c.cfg.Self.Tags[k] = v
	}
}

// RemoveTag removes a tag from this node's metadata.
func (c *Cluster) RemoveTag(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cfg.Self.Tags, key)
}

// UpdatePeerMeta updates the stored metadata for a peer node.
// Called by the metadata gossip when a peer's tags change.
// Emits MemberUpdated if the tags actually changed.
func (c *Cluster) UpdatePeerMeta(meta NodeMeta) bool {
	c.mu.Lock()
	existing, ok := c.peers[meta.ID]
	if !ok {
		c.mu.Unlock()
		return false
	}

	if tagsEqual(existing.Tags, meta.Tags) {
		c.mu.Unlock()
		return false
	}

	c.peers[meta.ID] = meta
	fn := c.cfg.OnMemberEvent
	c.mu.Unlock()

	if fn != nil {
		fn(MemberEvent{Type: MemberUpdated, Member: meta})
	}
	return true
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

// IsConnected returns whether a direct connection to the target exists.
func (c *Cluster) IsConnected(target NodeID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.conns[target]
	return ok
}

// --- InboundHandler implementation ---

// OnEnvelope implements InboundHandler by forwarding envelopes to the configured callback.
func (c *Cluster) OnEnvelope(from NodeID, env Envelope) {
	if c.cfg.OnEnvelope != nil {
		c.cfg.OnEnvelope(from, env)
	}
}

// OnConnectionEstablished implements InboundHandler. Inbound connections are not stored
// since the transport delivers envelopes directly via OnEnvelope.
func (c *Cluster) OnConnectionEstablished(remote NodeID, conn Connection) {
	// Inbound connections are used for receiving only.
	// Outbound connections (from dialPeer) are used for sending.
	// We don't store inbound connections — the transport's stream handler
	// delivers envelopes directly via OnEnvelope.
}

// OnConnectionLost implements InboundHandler. Connection loss is handled lazily
// when SendRemote fails, so this callback does not proactively remove connections.
func (c *Cluster) OnConnectionLost(remote NodeID, err error) {
	// Connection loss is handled lazily: when SendRemote fails,
	// it removes the dead connection and the next send retries.
	// We don't proactively remove here because the server-side stream
	// handler also calls this, and we don't want to remove our
	// outbound connections when an inbound stream drops.
}

// --- internal ---

func (c *Cluster) membershipLoop() {
	events := c.cfg.Provider.Events()
	for {
		select {
		case <-c.ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			c.handleMemberEvent(ev)
		}
	}
}

func (c *Cluster) handleMemberEvent(ev MemberEvent) {
	c.mu.Lock()
	switch ev.Type {
	case MemberJoin, MemberUpdated:
		c.peers[ev.Member.ID] = ev.Member

	case MemberLeave, MemberFailed:
		delete(c.peers, ev.Member.ID)
		if conn, ok := c.conns[ev.Member.ID]; ok {
			conn.Close()
			delete(c.conns, ev.Member.ID)
		}
	}

	c.reconcileConnectionsLocked()
	fn := c.cfg.OnMemberEvent
	c.mu.Unlock()

	// Invoke callback outside the lock to avoid deadlocks.
	if fn != nil {
		fn(ev)
	}
}

// reconcileConnectionsLocked adjusts connections to match the topology's
// ShouldConnect output. Must be called with c.mu held.
func (c *Cluster) reconcileConnectionsLocked() {
	members := c.memberListLocked()
	desired := c.cfg.Topology.ShouldConnect(c.cfg.Self.ID, members)

	desiredSet := make(map[NodeID]bool, len(desired))
	for _, id := range desired {
		desiredSet[id] = true
	}

	// Dial nodes we should be connected to but aren't.
	for _, id := range desired {
		if _, connected := c.conns[id]; !connected && !c.dialing[id] {
			if meta, known := c.peers[id]; known {
				c.dialing[id] = true
				go c.dialPeer(meta)
			}
		}
	}

	// Close connections to nodes we no longer need.
	for id, conn := range c.conns {
		if !desiredSet[id] {
			conn.Close()
			delete(c.conns, id)
		}
	}
}

func (c *Cluster) dialPeer(meta NodeMeta) {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	conn, err := c.cfg.Transport.Dial(ctx, meta.Addr, c.cfg.Auth)

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.dialing, meta.ID)

	if err != nil {
		return
	}

	if _, ok := c.conns[meta.ID]; ok {
		conn.Close()
		return
	}
	c.conns[meta.ID] = conn
}

func (c *Cluster) ensureConnection(ctx context.Context, target NodeID) (Connection, error) {
	c.mu.RLock()
	meta, known := c.peers[target]
	c.mu.RUnlock()

	if !known {
		return nil, fmt.Errorf("unknown node %s", target)
	}

	conn, err := c.cfg.Transport.Dial(ctx, meta.Addr, c.cfg.Auth)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	// Double-check: another goroutine may have connected.
	if existing, ok := c.conns[target]; ok {
		c.mu.Unlock()
		conn.Close()
		return existing, nil
	}
	c.conns[target] = conn
	c.mu.Unlock()

	return conn, nil
}

func (c *Cluster) removeConnection(nodeID NodeID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if conn, ok := c.conns[nodeID]; ok {
		conn.Close()
		delete(c.conns, nodeID)
	}
}

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

	// OnEnvelope is called when an envelope arrives from a remote node.
	// Set by the Runtime integration layer (Phase 2).
	OnEnvelope func(from NodeID, env Envelope)
}

// Cluster manages the lifecycle of a single node within a cluster.
// It coordinates membership discovery, transport connections, and
// envelope routing between nodes.
type Cluster struct {
	cfg ClusterConfig

	mu    sync.RWMutex
	conns map[NodeID]Connection // active connections by node
	peers map[NodeID]NodeMeta   // known live peers

	ctx    context.Context
	cancel context.CancelFunc

	started bool
}

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
	return &Cluster{
		cfg:   cfg,
		conns: make(map[NodeID]Connection),
		peers: make(map[NodeID]NodeMeta),
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
func (c *Cluster) SendRemote(ctx context.Context, target NodeID, env Envelope) error {
	c.mu.RLock()
	conn, ok := c.conns[target]
	c.mu.RUnlock()

	if !ok {
		// Try to establish a connection on demand.
		var err error
		conn, err = c.ensureConnection(ctx, target)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrNodeUnreachable, err)
		}
	}

	if env.SentAtUnixNano == 0 {
		env.SentAtUnixNano = time.Now().UnixNano()
	}

	if err := conn.Send(ctx, env); err != nil {
		// Connection may be dead; remove it so next send retries.
		c.removeConnection(target)
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}
	return nil
}

// IsConnected returns whether a direct connection to the target exists.
func (c *Cluster) IsConnected(target NodeID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.conns[target]
	return ok
}

// --- InboundHandler implementation ---

func (c *Cluster) OnEnvelope(from NodeID, env Envelope) {
	if c.cfg.OnEnvelope != nil {
		c.cfg.OnEnvelope(from, env)
	}
}

func (c *Cluster) OnConnectionEstablished(remote NodeID, conn Connection) {
	// Inbound connections are used for receiving only.
	// Outbound connections (from dialPeer) are used for sending.
	// We don't store inbound connections — the transport's stream handler
	// delivers envelopes directly via OnEnvelope.
}

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
	defer c.mu.Unlock()

	switch ev.Type {
	case MemberJoin, MemberUpdated:
		c.peers[ev.Member.ID] = ev.Member
		// Proactively connect to new peers.
		if _, connected := c.conns[ev.Member.ID]; !connected {
			meta := ev.Member
			go c.dialPeer(meta)
		}

	case MemberLeave, MemberFailed:
		delete(c.peers, ev.Member.ID)
		if conn, ok := c.conns[ev.Member.ID]; ok {
			conn.Close()
			delete(c.conns, ev.Member.ID)
		}
	}
}

func (c *Cluster) dialPeer(meta NodeMeta) {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	conn, err := c.cfg.Transport.Dial(ctx, meta.Addr, c.cfg.Auth)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
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

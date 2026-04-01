package cluster

import (
	"context"
	"encoding/gob"
	"math/rand"
	"sync"
	"time"
)

func init() {
	gob.Register(GossipEnvelope{})
}

// GossipEnvelope wraps a gossip payload for transport. It's sent as an
// Envelope with a well-known TypeName so the inbound dispatcher can
// route it to the gossip subsystem rather than an actor.
type GossipEnvelope struct {
	Protocol string // identifies the gossip protocol (e.g. "crdt_registry")
	Payload  []byte // protocol-specific data, encoded by the caller
}

// GossipProtocol implements periodic state exchange between cluster members.
// It's a push-based protocol: each round, a node selects `fanout` random
// peers and sends them a state delta. Peers merge the delta and respond
// with their own delta if needed.
type GossipProtocol struct {
	cluster  *Cluster
	codec    Codec
	protocol string // protocol identifier for routing

	interval time.Duration
	fanout   int

	mu        sync.RWMutex
	onProduce func() []byte                  // called to produce delta for a gossip round
	onReceive func(from NodeID, data []byte) // called when a delta arrives

	cancel context.CancelFunc
}

// GossipConfig configures a gossip protocol instance.
type GossipConfig struct {
	// Protocol is the name used to route gossip envelopes.
	Protocol string
	// Interval between gossip rounds. Defaults to 1s.
	Interval time.Duration
	// Fanout is the number of peers to gossip with per round. Defaults to 3.
	Fanout int
	// OnProduce is called each round to generate a delta to send to peers.
	// Return nil to skip the round.
	OnProduce func() []byte
	// OnReceive is called when a gossip delta arrives from a peer.
	OnReceive func(from NodeID, data []byte)
}

// NewGossipProtocol creates a new gossip protocol instance with the given configuration.
func NewGossipProtocol(cluster *Cluster, codec Codec, cfg GossipConfig) *GossipProtocol {
	if cfg.Interval == 0 {
		cfg.Interval = 1 * time.Second
	}
	if cfg.Fanout == 0 {
		cfg.Fanout = 3
	}
	return &GossipProtocol{
		cluster:   cluster,
		codec:     codec,
		protocol:  cfg.Protocol,
		interval:  cfg.Interval,
		fanout:    cfg.Fanout,
		onProduce: cfg.OnProduce,
		onReceive: cfg.OnReceive,
	}
}

// Start begins the periodic gossip loop.
func (g *GossipProtocol) Start(ctx context.Context) {
	ctx, g.cancel = context.WithCancel(ctx)
	go g.gossipLoop(ctx)
}

// Stop terminates the gossip loop.
func (g *GossipProtocol) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
}

// HandleInbound processes a gossip envelope received from a remote node.
// Called by the cluster's inbound dispatcher when it receives a gossip message.
func (g *GossipProtocol) HandleInbound(from NodeID, env GossipEnvelope) {
	if env.Protocol != g.protocol {
		return
	}
	g.mu.RLock()
	handler := g.onReceive
	g.mu.RUnlock()

	if handler != nil {
		handler(from, env.Payload)
	}
}

func (g *GossipProtocol) gossipLoop(ctx context.Context) {
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.doRound(ctx)
		}
	}
}

func (g *GossipProtocol) doRound(ctx context.Context) {
	g.mu.RLock()
	producer := g.onProduce
	g.mu.RUnlock()

	if producer == nil {
		return
	}

	delta := producer()
	if delta == nil {
		return
	}

	// Select random peers.
	members := g.cluster.Members()
	peers := selectRandomPeers(members, g.fanout)
	if len(peers) == 0 {
		return
	}

	// Encode the gossip envelope.
	gossipEnv := GossipEnvelope{
		Protocol: g.protocol,
		Payload:  delta,
	}
	encoded, err := g.codec.Encode(gossipEnv)
	if err != nil {
		return
	}

	// Send to selected peers via the cluster transport.
	for _, peer := range peers {
		env := Envelope{
			SenderNode:     g.cluster.LocalNodeID(),
			TargetNode:     peer.ID,
			TypeName:       "__gossip",
			Payload:        encoded,
			SentAtUnixNano: time.Now().UnixNano(),
		}
		g.cluster.SendRemote(ctx, peer.ID, env)
	}
}

// GossipRouter multiplexes inbound gossip envelopes to the correct
// GossipProtocol by the Protocol field. Register it once with the
// InboundDispatcher, then add protocols to it.
type GossipRouter struct {
	codec     Codec
	mu        sync.RWMutex
	protocols map[string]*GossipProtocol
}

// NewGossipRouter creates a router and registers it with the dispatcher
// for the __gossip envelope type.
func NewGossipRouter(d *InboundDispatcher, codec Codec) *GossipRouter {
	gr := &GossipRouter{
		codec:     codec,
		protocols: make(map[string]*GossipProtocol),
	}
	d.RegisterHandler("__gossip", gr.handle)
	return gr
}

// Add registers a gossip protocol with the router.
func (gr *GossipRouter) Add(g *GossipProtocol) {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	gr.protocols[g.protocol] = g
}

func (gr *GossipRouter) handle(from NodeID, env Envelope) {
	var decoded any
	if err := gr.codec.Decode(env.Payload, &decoded); err != nil {
		return
	}
	ge, ok := decoded.(GossipEnvelope)
	if !ok {
		return
	}

	gr.mu.RLock()
	g, ok := gr.protocols[ge.Protocol]
	gr.mu.RUnlock()

	if ok {
		g.HandleInbound(from, ge)
	}
}

// RegisterGossipHandler is a convenience for registering a single gossip
// protocol with a dispatcher. For multiple protocols, use GossipRouter.
func RegisterGossipHandler(d *InboundDispatcher, codec Codec, g *GossipProtocol) {
	d.RegisterHandler("__gossip", func(from NodeID, env Envelope) {
		var decoded any
		if err := codec.Decode(env.Payload, &decoded); err != nil {
			return
		}
		ge, ok := decoded.(GossipEnvelope)
		if !ok {
			return
		}
		g.HandleInbound(from, ge)
	})
}

func selectRandomPeers(members []NodeMeta, n int) []NodeMeta {
	if len(members) <= n {
		return members
	}
	// Fisher-Yates shuffle on a copy, take first n.
	shuffled := make([]NodeMeta, len(members))
	copy(shuffled, members)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled[:n]
}

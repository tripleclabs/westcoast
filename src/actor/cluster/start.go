package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
	"github.com/tripleclabs/westcoast/src/internal/metrics"
)

// Config is the high-level cluster configuration. Only Addr and Provider
// are required — everything else has sensible defaults.
type Config struct {
	// Addr is the listen address for the cluster transport (e.g. ":9000").
	Addr string

	// Provider discovers and monitors cluster peers.
	Provider ClusterProvider

	// Transport handles inter-node communication. Defaults to TCPTransport.
	Transport Transport

	// Auth authenticates cluster connections. Defaults to NoopAuth.
	Auth ClusterAuth

	// Codec serializes messages for the wire. Defaults to GobCodec.
	Codec Codec

	// Topology controls connection decisions and message routing.
	// Defaults to FullMeshTopology.
	Topology Topology

	// Metrics hooks for observability. Defaults to no-op.
	Metrics metrics.Hooks
}

// Start creates a fully-wired cluster node and starts it. The cluster
// owns the dispatcher, election, registry, singleton manager, and
// daemon manager internally — no manual wiring required.
//
// Usage:
//
//	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
//	c, err := cluster.Start(ctx, rt, cluster.Config{
//	    Addr:     ":9000",
//	    Provider: cluster.NewFixedProvider(cluster.FixedProviderConfig{...}),
//	})
//	defer c.Stop()
//
//	c.RegisterSingleton(cluster.SingletonSpec{Name: "scheduler", Handler: h})
//	c.RegisterDaemon(cluster.DaemonSpec{Name: "metrics", Handler: h})
func Start(ctx context.Context, rt *actor.Runtime, cfg Config) (*Cluster, error) {
	nodeID := NodeID(rt.NodeID())
	if nodeID == "" {
		return nil, fmt.Errorf("cluster: runtime must have a node ID (use actor.WithNodeID)")
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("cluster: Addr is required")
	}
	if cfg.Provider == nil {
		return nil, fmt.Errorf("cluster: Provider is required")
	}

	// If the provider implements BootstrappingProvider, let it configure
	// Transport and Auth (e.g. via join handshake + cert generation).
	if bp, ok := cfg.Provider.(BootstrappingProvider); ok {
		if cfg.Transport == nil && cfg.Auth == nil {
			transport, auth, err := bp.Bootstrap(ctx, NodeMeta{ID: nodeID, Addr: cfg.Addr})
			if err != nil {
				return nil, fmt.Errorf("cluster bootstrap: %w", err)
			}
			cfg.Transport = transport
			cfg.Auth = auth
		}
	}

	// Apply defaults.
	if cfg.Codec == nil {
		cfg.Codec = NewGobCodec()
	}
	if cfg.Transport == nil {
		if defaultTransportFactory != nil {
			cfg.Transport = defaultTransportFactory(nodeID, nil)
		} else {
			cfg.Transport = NewTCPTransport(nodeID)
		}
	}
	if cfg.Auth == nil {
		cfg.Auth = NoopAuth{}
	}
	if cfg.Topology == nil {
		cfg.Topology = FullMeshTopology{}
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NopHooks{}
	}

	// 1. Create the core cluster.
	c, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: nodeID, Addr: cfg.Addr},
		Provider:  cfg.Provider,
		Transport: cfg.Transport,
		Auth:      cfg.Auth,
		Codec:     cfg.Codec,
		Topology:  cfg.Topology,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster: %w", err)
	}

	// 2. Remote sender — enables the runtime to send to remote actors.
	remoteSender := NewRemoteSender(c, cfg.Codec, cfg.Metrics)
	rt.SetRemoteSend(remoteSender.Send)
	rt.SetRemoteAskSend(remoteSender.SendAsk)

	// 3. Remote PID resolver — tracks which nodes are reachable.
	remoteResolver := NewRemotePIDResolver(actor.NewInMemoryPIDResolver(), nodeID)

	// 4. Inbound dispatcher — routes incoming envelopes to actors and
	//    system handlers (gossip, handoff, CRDT replication, etc.).
	dispatcher := NewInboundDispatcher(rt, cfg.Codec)
	dispatcher.SetCluster(c)
	c.SetDispatcher(dispatcher)
	c.SetOnEnvelope(func(from NodeID, env Envelope) {
		dispatcher.Dispatch(ctx, from, env)
	})

	// 5. Election — deterministic leader election from membership.
	election := NewRingElection(nodeID)

	// 6. Distributed registry — CRDT-replicated name registry.
	registry := NewDistributedRegistry(nodeID, WithClusterReplication(c))
	registry.WireInbound(dispatcher)

	// 7. Singleton manager — at-most-one actors with handoff protocol.
	singletons := NewSingletonManager(rt, election, registry, c, cfg.Codec)

	// 8. Daemon manager — run-everywhere actors with optional CRDT state.
	daemons := NewDaemonSetManager(rt, c, cfg.Codec)

	// 9. Wire membership events to all subsystems.
	c.SetOnMemberEvent(func(ev MemberEvent) {
		// Update election membership.
		election.OnMembershipChange(ev)

		// Track remote node reachability.
		switch ev.Type {
		case MemberJoin, MemberUpdated:
			remoteResolver.AddRemoteNode(ev.Member.ID)
		case MemberLeave, MemberFailed:
			remoteResolver.RemoveRemoteNode(ev.Member.ID)
		}

		// Notify subsystems.
		singletons.OnMemberEvent(ev)
		registry.OnMembershipChange(ev)

		// Publish on the cluster.membership pubsub topic.
		rt.PublishMembershipEvent(ctx, actor.ClusterMembershipEvent{
			Type: ev.Type.String(),
			Member: actor.ClusterMemberInfo{
				ID:   string(ev.Member.ID),
				Addr: ev.Member.Addr,
				Tags: ev.Member.Tags,
			},
		})
	})

	// 10. Wire cluster registry into the runtime for name lookups.
	rt.SetClusterRegistry(registry.Register, registry.Lookup, registry.Unregister)
	rt.SetClusterMembers(func() []actor.ClusterMemberInfo {
		members := c.Members()
		out := make([]actor.ClusterMemberInfo, len(members))
		for i, m := range members {
			out[i] = actor.ClusterMemberInfo{
				ID:   string(m.ID),
				Addr: m.Addr,
				Tags: m.Tags,
			}
		}
		return out
	})

	// Store owned subsystems on the cluster.
	c.runtime = rt
	c.election = election
	c.registry = registry
	c.singletonMgr = singletons
	c.daemonMgr = daemons

	// 11. Start the cluster transport and provider.
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("cluster start: %w", err)
	}

	// 12. If the provider has seeds, wait for peer connectivity and
	// registry convergence before starting singletons. This prevents
	// duplicate singletons when two nodes discover each other — the
	// joining node needs to see what the existing cluster is running.
	if sp, ok := cfg.Provider.(SeedProvider); ok && sp.HasSeeds() {
		waitCtx, waitCancel := context.WithTimeout(ctx, 30*time.Second)
		defer waitCancel()
		// Wait for at least one peer connection (not just membership —
		// we need actual transport connectivity for CRDT replication).
		for {
			members := c.Members()
			connected := false
			for _, m := range members {
				if c.IsConnected(m.ID) {
					connected = true
					break
				}
			}
			if connected {
				break
			}
			select {
			case <-waitCtx.Done():
				return nil, fmt.Errorf("cluster: timeout waiting for peer connection")
			case <-time.After(50 * time.Millisecond):
			}
		}
	}

	// 13. Start subsystem managers.
	singletons.Start(ctx)
	daemons.Start(ctx)

	_ = remoteResolver

	return c, nil
}

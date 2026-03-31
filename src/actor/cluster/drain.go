package cluster

import (
	"context"
	"time"
)

// DrainConfig configures graceful drain behavior.
type DrainConfig struct {
	// Timeout is the maximum time to wait for in-flight work to complete.
	// After this, remaining actors are force-stopped. Defaults to 30s.
	Timeout time.Duration
}

// Drain performs a graceful shutdown of the cluster node:
//
//  1. Emits a MemberLeave event so peers know this is planned, not a failure.
//  2. Stops the singleton manager (if provided), allowing singletons to migrate.
//  3. Deregisters all names from the cluster registry (if provided).
//  4. Waits for the drain timeout to let in-flight messages complete.
//  5. Calls Cluster.Stop() to close transport and provider.
//
// This should be called instead of Cluster.Stop() for planned shutdowns
// (deploys, scaling down). Cluster.Stop() is still used for the final cleanup.
func Drain(ctx context.Context, c *Cluster, cfg DrainConfig, opts ...DrainOption) error {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	o := drainOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// 1. Notify peers of planned departure.
	c.mu.RLock()
	fn := c.cfg.OnMemberEvent
	c.mu.RUnlock()
	if fn != nil {
		fn(MemberEvent{
			Type:   MemberLeave,
			Member: c.cfg.Self,
		})
	}

	// 2. Stop singletons — they'll migrate to other nodes.
	if o.singletonManager != nil {
		o.singletonManager.Stop()
	}

	// 3. Deregister all names from the cluster registry.
	if o.registry != nil {
		o.registry.UnregisterByNode(c.cfg.Self.ID)
	}

	// 4. Wait for drain period — gives in-flight messages time to complete.
	// The caller's actors should be draining their mailboxes during this time.
	select {
	case <-ctx.Done():
	case <-time.After(cfg.Timeout / 2):
		// Half the timeout for drain, other half for cleanup.
	}

	// 5. Stop the cluster (transport + provider).
	return c.Stop()
}

// DrainOption configures optional drain behavior.
type DrainOption func(*drainOptions)

type drainOptions struct {
	singletonManager *SingletonManager
	registry         *CRDTRegistry
}

// WithSingletonManager stops singletons during drain so they migrate.
func WithSingletonManager(sm *SingletonManager) DrainOption {
	return func(o *drainOptions) { o.singletonManager = sm }
}

// WithRegistry deregisters all local names during drain.
func WithRegistry(r *CRDTRegistry) DrainOption {
	return func(o *drainOptions) { o.registry = r }
}

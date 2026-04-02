package providerutil

import (
	"context"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// ProbeFunc checks whether a node at the given address is reachable.
// Return nil if alive, non-nil error if unreachable.
type ProbeFunc func(ctx context.Context, addr string) error

// WithProbe wraps a DiscoverFunc so that each discovered member is
// probed before being included in the result. Members that fail the
// probe are silently dropped from the returned list.
//
// This is useful when a cloud API reports an instance as "running" but
// the process hasn't started listening on its transport port yet.
func WithProbe(discover DiscoverFunc, probe ProbeFunc) DiscoverFunc {
	return func(ctx context.Context) ([]cluster.NodeMeta, error) {
		members, err := discover(ctx)
		if err != nil {
			return nil, err
		}

		alive := make([]cluster.NodeMeta, 0, len(members))
		for _, m := range members {
			if err := probe(ctx, m.Addr); err == nil {
				alive = append(alive, m)
			}
		}
		return alive, nil
	}
}

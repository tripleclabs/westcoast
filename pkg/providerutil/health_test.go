package providerutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

func TestWithProbe_FiltersUnreachable(t *testing.T) {
	base := staticDiscover([]cluster.NodeMeta{
		{ID: "node-1", Addr: "10.0.0.1:9000"},
		{ID: "node-2", Addr: "10.0.0.2:9000"},
		{ID: "node-3", Addr: "10.0.0.3:9000"},
	})

	// Only node-1 and node-3 are reachable.
	probe := func(ctx context.Context, addr string) error {
		if addr == "10.0.0.2:9000" {
			return fmt.Errorf("unreachable")
		}
		return nil
	}

	discover := WithProbe(base, probe)
	members, err := discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	ids := map[cluster.NodeID]bool{}
	for _, m := range members {
		ids[m.ID] = true
	}
	if !ids["node-1"] || !ids["node-3"] {
		t.Errorf("expected node-1 and node-3, got %v", ids)
	}
	if ids["node-2"] {
		t.Error("node-2 should have been filtered out")
	}
}

func TestWithProbe_AllReachable(t *testing.T) {
	base := staticDiscover([]cluster.NodeMeta{
		{ID: "node-1", Addr: "10.0.0.1:9000"},
	})

	probe := func(ctx context.Context, addr string) error { return nil }

	members, err := WithProbe(base, probe)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func TestWithProbe_PropagatesDiscoverError(t *testing.T) {
	base := func(ctx context.Context) ([]cluster.NodeMeta, error) {
		return nil, fmt.Errorf("api error")
	}

	probe := func(ctx context.Context, addr string) error { return nil }

	_, err := WithProbe(base, probe)(context.Background())
	if err == nil {
		t.Fatal("expected error from discover to propagate")
	}
}

package cluster

import (
	"context"
	"sync"
	"time"

	crdt "github.com/tripleclabs/crdt-go"
	"github.com/tripleclabs/westcoast/src/actor"
)

// DaemonSpec defines an actor that should run on every matching node.
// If Placement is nil, the daemon runs on ALL nodes.
type DaemonSpec struct {
	// Name is the actor ID — identical on every node that runs it.
	Name         string
	InitialState any
	Handler      actor.Handler
	Options      []actor.ActorOption
	// Placement restricts which nodes run this daemon. If nil, all nodes match.
	Placement NodeMatcher
}

// DaemonSetManager ensures registered daemon actors run on every matching
// node. On this node, daemons are created when the local metadata matches
// the placement predicate. When metadata changes (via UpdateTags + gossip),
// daemons are started or stopped reactively.
type DaemonSetManager struct {
	runtime *actor.Runtime
	cluster *Cluster
	codec   Codec

	mu      sync.Mutex
	daemons map[string]*daemonState
	started bool
	cancel  context.CancelFunc
}

type daemonState struct {
	spec       DaemonSpec
	running    bool
	replicated *replicatedConfig // non-nil for replicated daemons
	closer     func()           // cleanup for CRDT state on stop
}

// NewDaemonSetManager creates a manager for daemon actors. The cluster
// and codec may be nil for single-node operation (SendTo/AskTo local only).
func NewDaemonSetManager(runtime *actor.Runtime, cluster *Cluster, codec Codec) *DaemonSetManager {
	return &DaemonSetManager{
		runtime: runtime,
		cluster: cluster,
		codec:   codec,
		daemons: make(map[string]*daemonState),
	}
}

// Register adds a daemon spec. If the manager is already started and
// this node matches the placement predicate, the daemon is created.
func (dm *DaemonSetManager) Register(spec DaemonSpec) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.daemons[spec.Name] = &daemonState{spec: spec}
	if dm.started {
		dm.reconcileDaemon(spec.Name)
	}
}

// Start creates daemon actors that match this node's metadata.
func (dm *DaemonSetManager) Start(ctx context.Context) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	ctx, dm.cancel = context.WithCancel(ctx)
	dm.started = true

	for name := range dm.daemons {
		dm.reconcileDaemon(name)
	}
}

// Stop terminates all daemon actors on this node.
func (dm *DaemonSetManager) Stop() {
	if dm.cancel != nil {
		dm.cancel()
	}
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.started = false
	for name, d := range dm.daemons {
		if d.running {
			dm.runtime.Stop(name)
			if d.closer != nil {
				d.closer()
				d.closer = nil
			}
			d.running = false
		}
	}
}

// Reconcile re-evaluates placement for all daemons against current
// local metadata. Call this when local tags change to reactively
// start/stop daemons based on updated predicates.
func (dm *DaemonSetManager) Reconcile() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if !dm.started {
		return
	}
	for name := range dm.daemons {
		dm.reconcileDaemon(name)
	}
}

// Running returns the names of daemons currently running on this node.
func (dm *DaemonSetManager) Running() []string {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	var out []string
	for name, d := range dm.daemons {
		if d.running {
			out = append(out, name)
		}
	}
	return out
}

// SendTo sends a fire-and-forget message to a daemon on a specific node.
// If nodeID matches the local node, delivers directly without transport.
func (dm *DaemonSetManager) SendTo(ctx context.Context, name string, nodeID NodeID, payload any) actor.PIDSendAck {
	if string(nodeID) == dm.runtime.NodeID() || (dm.cluster == nil && nodeID == "") {
		ack := dm.runtime.Send(ctx, name, payload)
		return actor.PIDSendAck{
			Outcome:   pidOutcomeFromSubmit(ack.Result),
			MessageID: ack.MessageID,
		}
	}

	if dm.cluster == nil || dm.codec == nil {
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound}
	}

	encoded, err := dm.codec.Encode(payload)
	if err != nil {
		return actor.PIDSendAck{Outcome: actor.PIDRejectedNotFound}
	}

	env := Envelope{
		SenderNode:     dm.cluster.LocalNodeID(),
		TargetNode:     nodeID,
		TargetActorID:  name,
		TypeName:       typeNameOf(payload),
		Payload:        encoded,
		SentAtUnixNano: time.Now().UnixNano(),
	}

	if err := dm.cluster.SendRemote(ctx, nodeID, env); err != nil {
		return actor.PIDSendAck{Outcome: actor.PIDUnresolved}
	}
	return actor.PIDSendAck{Outcome: actor.PIDDelivered}
}

// AskTo sends a request to a daemon on a specific node and waits for a reply.
func (dm *DaemonSetManager) AskTo(ctx context.Context, name string, nodeID NodeID, payload any, timeout time.Duration) (actor.AskResult, error) {
	if string(nodeID) == dm.runtime.NodeID() || (dm.cluster == nil && nodeID == "") {
		return dm.runtime.Ask(ctx, name, payload, timeout)
	}

	remotePID := actor.PID{
		Namespace:  string(nodeID),
		ActorID:    name,
		Generation: 1,
	}
	return dm.runtime.AskPID(ctx, remotePID, payload, timeout)
}

// Broadcast sends a message to the named daemon on ALL matching nodes.
func (dm *DaemonSetManager) Broadcast(ctx context.Context, name string, payload any) []actor.PIDSendAck {
	nodes := dm.matchingNodes(name)

	acks := make([]actor.PIDSendAck, len(nodes))
	for i, nodeID := range nodes {
		acks[i] = dm.SendTo(ctx, name, nodeID, payload)
	}
	return acks
}

// matchingNodes returns the node IDs where a daemon should be running,
// based on its placement predicate.
func (dm *DaemonSetManager) matchingNodes(name string) []NodeID {
	dm.mu.Lock()
	d, ok := dm.daemons[name]
	dm.mu.Unlock()
	if !ok {
		return nil
	}

	selfID := NodeID(dm.runtime.NodeID())
	var nodes []NodeID

	// Check self.
	if dm.shouldRunLocally(d.spec) {
		nodes = append(nodes, selfID)
	}

	// Check peers.
	if dm.cluster != nil {
		for _, m := range dm.cluster.Members() {
			if d.spec.Placement == nil || d.spec.Placement(m) {
				nodes = append(nodes, m.ID)
			}
		}
	}

	return nodes
}

// reconcileDaemon starts or stops a single daemon based on whether this
// node matches the placement predicate. Must be called with dm.mu held.
func (dm *DaemonSetManager) reconcileDaemon(name string) {
	d, ok := dm.daemons[name]
	if !ok {
		return
	}

	shouldRun := dm.shouldRunLocally(d.spec)

	if shouldRun && !d.running {
		if d.replicated != nil {
			dm.startReplicatedDaemon(d)
		} else {
			_, err := dm.runtime.CreateActor(d.spec.Name, d.spec.InitialState, d.spec.Handler, d.spec.Options...)
			if err == nil {
				d.running = true
			}
		}
	}

	if !shouldRun && d.running {
		dm.runtime.Stop(name)
		if d.closer != nil {
			d.closer()
			d.closer = nil
		}
		d.running = false
	}
}

// shouldRunLocally checks if this node matches a daemon's placement predicate.
func (dm *DaemonSetManager) shouldRunLocally(spec DaemonSpec) bool {
	if spec.Placement == nil {
		return true // no predicate = run everywhere
	}
	if dm.cluster == nil {
		// Single-node mode: build a minimal NodeMeta for matching.
		return spec.Placement(NodeMeta{ID: NodeID(dm.runtime.NodeID())})
	}
	return spec.Placement(dm.cluster.Self())
}

// startReplicatedDaemon creates a CRDT-backed daemon actor with transport
// and topology wired to the cluster. The topology is scoped to nodes that
// match the daemon's placement predicate — only those nodes run the daemon
// and participate in replication.
func (dm *DaemonSetManager) startReplicatedDaemon(d *daemonState) {
	var transport crdt.Transport
	var topology crdt.TopologyProvider

	if dm.cluster != nil {
		transport = NewClusterCRDTTransport(dm.cluster)
		topology = &daemonTopology{
			cluster:   dm.cluster,
			placement: d.spec.Placement,
		}
	}
	// Without a cluster, transport and topology stay nil — the CRDT
	// works as a local-only data structure (no replication).

	replicaID := nodeIDToReplicaID(NodeID(dm.runtime.NodeID()))
	state, handler, closer := d.replicated.create(replicaID, transport, topology)

	_, err := dm.runtime.CreateActor(d.spec.Name, state, handler, d.spec.Options...)
	if err == nil {
		d.running = true
		d.closer = closer
	} else if closer != nil {
		closer()
	}
}

// daemonTopology implements crdt.TopologyProvider scoped to nodes that
// match a daemon's placement predicate. This ensures the CRDT only
// replicates to nodes that actually run the daemon.
type daemonTopology struct {
	cluster   *Cluster
	placement NodeMatcher
}

func (t *daemonTopology) Peers() []crdt.ReplicaID {
	members := t.cluster.Members()
	var peers []crdt.ReplicaID
	for _, m := range members {
		if t.placement == nil || t.placement(m) {
			peers = append(peers, nodeIDToReplicaID(m.ID))
		}
	}
	return peers
}

func pidOutcomeFromSubmit(result actor.SubmitResult) actor.PIDDeliveryOutcome {
	switch result {
	case actor.SubmitAccepted:
		return actor.PIDDelivered
	default:
		return actor.PIDRejectedNotFound
	}
}

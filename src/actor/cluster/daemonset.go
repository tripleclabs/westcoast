package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// DaemonSpec defines an actor that should run on every node in the cluster.
// The same spec is registered on every node's DaemonSetManager.
type DaemonSpec struct {
	// Name is the actor ID — identical on every node.
	Name         string
	InitialState any
	Handler      actor.Handler
	Options      []actor.ActorOption
}

// DaemonSetManager ensures registered daemon actors run on every node.
// On this node, daemons are created locally. For cross-node communication,
// SendTo/AskTo/Broadcast route messages via the cluster transport using
// actor-ID-based delivery (no PID or generation required).
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
	spec    DaemonSpec
	running bool
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

// Register adds a daemon spec. If the manager is already started,
// the daemon is created immediately.
func (dm *DaemonSetManager) Register(spec DaemonSpec) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.daemons[spec.Name] = &daemonState{spec: spec}
	if dm.started {
		dm.startDaemon(spec)
	}
}

// Start creates all registered daemon actors on this node.
func (dm *DaemonSetManager) Start(ctx context.Context) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	ctx, dm.cancel = context.WithCancel(ctx)
	dm.started = true

	for _, d := range dm.daemons {
		dm.startDaemon(d.spec)
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
			d.running = false
		}
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
	// Local delivery — no transport, no codec, no PID.
	if string(nodeID) == dm.runtime.NodeID() || (dm.cluster == nil && nodeID == "") {
		ack := dm.runtime.Send(ctx, name, payload)
		return actor.PIDSendAck{
			Outcome:   pidOutcomeFromSubmit(ack.Result),
			MessageID: ack.MessageID,
		}
	}

	// Remote delivery via transport.
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
// If nodeID matches the local node, uses the local Ask path.
func (dm *DaemonSetManager) AskTo(ctx context.Context, name string, nodeID NodeID, payload any, timeout time.Duration) (actor.AskResult, error) {
	// Local ask — direct, no transport.
	if string(nodeID) == dm.runtime.NodeID() || (dm.cluster == nil && nodeID == "") {
		return dm.runtime.Ask(ctx, name, payload, timeout)
	}

	// Remote ask via AskPID. The PID uses the remote node's namespace
	// so it routes through the transport. The remote side delivers by
	// actor ID via InboundDispatcher.
	remotePID := actor.PID{
		Namespace:  string(nodeID),
		ActorID:    name,
		Generation: 1,
	}
	return dm.runtime.AskPID(ctx, remotePID, payload, timeout)
}

// Broadcast sends a message to the named daemon on ALL nodes, including self.
func (dm *DaemonSetManager) Broadcast(ctx context.Context, name string, payload any) []actor.PIDSendAck {
	// Collect all node IDs: self + cluster members.
	selfID := NodeID(dm.runtime.NodeID())
	var nodes []NodeID
	nodes = append(nodes, selfID)

	if dm.cluster != nil {
		for _, m := range dm.cluster.Members() {
			nodes = append(nodes, m.ID)
		}
	}

	acks := make([]actor.PIDSendAck, len(nodes))
	for i, nodeID := range nodes {
		acks[i] = dm.SendTo(ctx, name, nodeID, payload)
	}
	return acks
}

func (dm *DaemonSetManager) startDaemon(spec DaemonSpec) {
	_, err := dm.runtime.CreateActor(spec.Name, spec.InitialState, spec.Handler, spec.Options...)
	if err != nil {
		return // may already exist
	}
	dm.daemons[spec.Name].running = true
}

func pidOutcomeFromSubmit(result actor.SubmitResult) actor.PIDDeliveryOutcome {
	switch result {
	case actor.SubmitAccepted:
		return actor.PIDDelivered
	default:
		return actor.PIDRejectedNotFound
	}
}
